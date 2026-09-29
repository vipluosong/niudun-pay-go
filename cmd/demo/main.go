// 牛盾支付 Go SDK 联调 Demo 服务
//
// 单命令启动的本地联调工具，所有交易调用都经 niudun.Client 走 SDK 真实调用链（签名/请求/验签），
// 同时验证 SDK 与平台 unipay 接口两侧：
//
//   - GET /                     内嵌调试页（go:embed index.html）
//   - GET|POST /demo/config     连接配置：页面弹窗内直接填写服务地址/商户号/私钥/公钥，
//     配置保存在**浏览器 localStorage**（服务端只存内存、不落盘）
//   - POST /demo/ping           连通性自检：服务端代调 GET /unipay/callback/ping 探针
//     （浏览器直连平台会跨域，故由本服务中转）
//   - POST /demo/signed-ping    签名链路自检：服务端代调 POST /unipay/ping 签名探针（「测试连接」第二段，
//     判定当前配置的商户号/应用/商户私钥是否正确、能否发起真实调用）
//   - POST /demo/{action}       调 SDK 发起真实请求，回显「签名后请求体 + 平台原始响应 + 响应验签结果 + 耗时」，
//     支持全部 15 个开放接口，action 取值见 actions 表
//   - POST /callback/{pay|refund|alloc|transfer}（及通用 /callback）
//     接收平台异步通知，用平台公钥验签后暂存，返回固定文本 SUCCESS
//   - GET /demo/callbacks       回调记录（页面轮询）；POST /demo/callbacks/clear 清空
//
// 启动（仓根执行）：
//
//	go run ./cmd/demo             # 默认端口 9791
//	go run ./cmd/demo --port=9791 # 显式指定端口
//
// 注意：调试页由 go:embed 在**编译期**打包，改动 index.html 后必须重新 go build / go run 才生效。
//
// HTTP 层只用标准库 net/http，不给 SDK 引入任何 Web 框架依赖；
// 每次交易调用创建带独立 OnRequest/OnResponse 回显钩子的 client 实例，无线程共享状态。
package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/niudunpay/niudun-pay-sdk-go/niudun"
)

// maxCallbacks 回调记录保留上限（超出丢弃最老记录）
const maxCallbacks = 200

// indexHTML 内嵌调试页（编译期打包，改页面后需重新构建）
//
//go:embed index.html
var indexHTML []byte

// actionFunc 单个 action 的执行体：把页面提交的 JSON 请求体绑到 param 结构体后调 SDK 方法
type actionFunc func(client *niudun.Client, body []byte) error

// actions action → SDK 调用映射表（15 个业务接口 + 签名自检探针），新增接口只需在此登记一行
//
// 用方法表达式 (*niudun.Client).Xxx 取得「接收者作为首参」的函数值（SDK 方法均为指针接收者），
// 由 bind / bindErr 泛型从方法签名自动推导 param 与 result 类型，无需在表里重复类型名
var actions = map[string]actionFunc{
	// 支付族
	"pay":             bind((*niudun.Client).Pay),
	"close":           bindErr((*niudun.Client).Close),
	"query-pay-order": bind((*niudun.Client).QueryPayOrder),
	"sync-pay-order":  bind((*niudun.Client).SyncPayOrder),
	// 退款族
	"refund":             bind((*niudun.Client).Refund),
	"query-refund-order": bind((*niudun.Client).QueryRefundOrder),
	"sync-refund-order":  bind((*niudun.Client).SyncRefundOrder),
	// 转账族
	"transfer":             bind((*niudun.Client).Transfer),
	"query-transfer-order": bind((*niudun.Client).QueryTransferOrder),
	"sync-transfer-order":  bind((*niudun.Client).SyncTransferOrder),
	// 分账族
	"alloc":             bind((*niudun.Client).Alloc),
	"query-alloc-order": bind((*niudun.Client).QueryAllocOrder),
	"sync-alloc-order":  bind((*niudun.Client).SyncAllocOrder),
	// 网关族
	"gateway-pre-pay": bind((*niudun.Client).GatewayPrePay),
	"gateway-query":   bind((*niudun.Client).GatewayQuery),
	// 自检族：探针非 0 码在此转成错误路径，与其它接口的失败回显行为一致（完整诊断走 /demo/signed-ping）
	"signed-ping": func(client *niudun.Client, body []byte) error {
		param, err := decodeParam[niudun.PingParam](body)
		if err != nil {
			return err
		}
		res, err := client.SignedPing(context.Background(), param)
		if err != nil {
			return err
		}
		// 非 0 业务码转成 BizError，错误文案与其它接口一致（[code] msg）
		if res.Code != 0 {
			return &niudun.BizError{Code: res.Code, Msg: res.Msg}
		}
		return nil
	},
}

// bind 把「JSON 体 → param 结构体 → SDK 方法」压成一行；P 为 param 类型，R 为结果类型
func bind[P any, R any](fn func(*niudun.Client, context.Context, *P) (*R, error)) actionFunc {
	return func(client *niudun.Client, body []byte) error {
		param, err := decodeParam[P](body)
		if err != nil {
			return err
		}
		_, err = fn(client, context.Background(), param)
		return err
	}
}

// bindErr 同上，用于无 data 的接口（close）
func bindErr[P any](fn func(*niudun.Client, context.Context, *P) error) actionFunc {
	return func(client *niudun.Client, body []byte) error {
		param, err := decodeParam[P](body)
		if err != nil {
			return err
		}
		return fn(client, context.Background(), param)
	}
}

// decodeParam 反序列化页面提交的 JSON 请求体（空体按空对象处理）
func decodeParam[P any](body []byte) (*P, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		body = []byte("{}")
	}
	var param P
	if err := json.Unmarshal(body, &param); err != nil {
		return nil, fmt.Errorf("请求参数解析失败: %w", err)
	}
	return &param, nil
}

// callbackRecord 回调记录（内存暂存，重启即清）
type callbackRecord struct {
	Time         string `json:"time"`         // GMT+8 字面量 yyyy-MM-dd HH:mm:ss
	Type         string `json:"type"`         // pay/refund/transfer/alloc（通用端点为 common）
	SignVerified bool   `json:"signVerified"` // 平台公钥验签结果
	Code         *int   `json:"code"`         // 报文业务码（未解析出为 null）
	Msg          string `json:"msg"`          // 报文提示信息
	Body         string `json:"body"`         // 回调原始报文
}

// configInfo 配置脱敏状态（只报是否已配置，不返回密钥内容）
type configInfo struct {
	ServiceURL    string `json:"serviceUrl"`
	MchNo         string `json:"mchNo"`
	AppID         any    `json:"appId"` // 未配置时为 null
	CallbackBase  string `json:"callbackBase"`
	PrivateKeySet bool   `json:"privateKeySet"`
	PublicKeySet  bool   `json:"publicKeySet"`
}

// pingResult 探针自检结果（成功透出标记文本，失败透出可操作提示）
type pingResult struct {
	ServiceURL string `json:"serviceUrl"`
	Success    bool   `json:"success"`
	Marker     string `json:"marker,omitempty"`
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"durationMs"`
}

// signedPingResult 签名自检探针回显（字段名与 Java 版 DemoServer 一致：
// success/code/msg/data/hint/error/requestBody/responseBody/durationMs）
type signedPingResult struct {
	ServiceURL   string      `json:"serviceUrl"`
	Success      bool        `json:"success"`
	Code         *int        `json:"code,omitempty"`         // 业务码（探针请求发出且拿到响应时才有）
	Msg          string      `json:"msg,omitempty"`          // 平台消息（20052 时含服务端待签串）
	Data         interface{} `json:"data,omitempty"`         // 探针成功时的 *niudun.PingResult（失败码时无 data）
	Hint         string      `json:"hint,omitempty"`         // 排查提示（按错误码分类诊断 / 硬错误提示）
	Error        string      `json:"error,omitempty"`        // 硬错误消息（网络不通 / HTTP 非 200 / 响应验签失败）
	RequestBody  *string     `json:"requestBody,omitempty"`  // SDK 实际发出的、含 sign 的完整请求 JSON
	ResponseBody *string     `json:"responseBody,omitempty"` // 平台返回的原始响应体文本
	DurationMs   int64       `json:"durationMs"`
}

// tradeResult 交易调试回显（业务失败也是 200 + success:false，页面据此渲染）
type tradeResult struct {
	Success      bool        `json:"success"`
	RequestBody  *string     `json:"requestBody"`  // SDK 实际发出的、含 sign 的完整请求 JSON
	ResponseBody *string     `json:"responseBody"` // 平台返回的原始响应体文本
	DurationMs   int64       `json:"durationMs"`
	SignVerified *bool       `json:"signVerified"` // true/false/null（null = 响应不带签名）
	Result       interface{} `json:"result"`       // 响应体解析出的对象（解析失败给 {"parseError": 原文}）
	Error        *string     `json:"error"`        // SDK 抛出的错误消息
}

// server 联调 Demo 服务端状态（配置与回调记录只存内存）
type server struct {
	mu           sync.Mutex    // 保护 cfg 与 callbacks（页面保存配置与并发请求）
	cfg          niudun.Config // 当前生效配置（页面保存时整体替换）
	callbackBase string        // 对外可达的回调基址（生成默认 notifyUrl 用）
	callbacks    []callbackRecord
}

func main() {
	port := flag.Int("port", 9791, "监听端口（也可用 --port=9791）")
	flag.Parse()

	srv := &server{
		// 默认指向本地平台后端，启动后可在页面「连接配置」中修改
		cfg:          niudun.Config{ServiceUrl: "http://127.0.0.1:9999"},
		callbackBase: fmt.Sprintf("http://127.0.0.1:%d", *port),
	}
	cfg := srv.snapshot()

	fmt.Println("牛盾支付 Go SDK 联调 Demo 已启动")
	fmt.Printf("  调试页面 : http://127.0.0.1:%d\n", *port)
	fmt.Printf("  平台地址 : %s  (商户 %s)\n", cfg.ServiceUrl, dashIfEmpty(cfg.MchNo))
	fmt.Printf("  密钥状态 : 商户私钥 %s / 平台公钥 %s  (可在页面「连接配置」中随时修改)\n", keyState(cfg.PrivateKey), keyState(cfg.PublicKey))
	fmt.Printf("  回调基址 : %s  (支付通知可填 %s/callback/pay)\n", srv.callbackBase, srv.callbackBase)
	fmt.Println("  提示     : 未加载配置文件，请打开页面在「连接配置」中填写参数（保存在浏览器本地，不落盘）")
	fmt.Println("  提示     : index.html 由 go:embed 编译期打包，改页面后需重新 go build / go run")

	mux := http.NewServeMux()
	mux.HandleFunc("/", srv.dispatch)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", *port), mux); err != nil {
		log.Fatalf("Demo 服务启动失败: %v", err)
	}
}

// ==================================================================
// HTTP 分发
// ==================================================================

// dispatch 单入口分发
//
// 路由匹配顺序（对齐 Java 版踩过的坑）：/demo/callbacks、/demo/callbacks/clear、/demo/ping、
// /demo/signed-ping 必须排在 /demo/* 通配交易路由之前，否则会被当成 action 进交易处理、解析空 body 报错。
func (s *server) dispatch(w http.ResponseWriter, r *http.Request) {
	// panic 兜底：回 500 JSON，避免连接被直接断开（与其它语言 demo 行为一致）
	defer func() {
		if rec := recover(); rec != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprint(rec)})
		}
	}()

	path := r.URL.Path
	switch {
	// 调试页
	case r.Method == http.MethodGet && (path == "/" || path == "/index.html"):
		s.servePage(w)
	// 连接配置：GET 读取脱敏状态 / POST 页面保存（配置存浏览器，服务端仅内存）
	case path == "/demo/config" && (r.Method == http.MethodGet || r.Method == http.MethodPost):
		s.handleConfig(w, r)
	// 回调记录（须在 /demo/* 交易路由之前匹配）
	case r.Method == http.MethodGet && path == "/demo/callbacks":
		s.handleCallbacks(w)
	case r.Method == http.MethodPost && path == "/demo/callbacks/clear":
		s.handleCallbacksClear(w)
	// 连通性自检（服务端代调平台探针）
	case r.Method == http.MethodPost && path == "/demo/ping":
		s.handlePing(w, r)
	// 签名链路自检：服务端代调签名自检探针 POST /unipay/ping（「测试连接」第二段）
	case r.Method == http.MethodPost && path == "/demo/signed-ping":
		s.handleSignedPing(w, r)
	// 交易调试（经 SDK 真实调用链）
	case r.Method == http.MethodPost && strings.HasPrefix(path, "/demo/"):
		s.handleTrade(w, r, strings.TrimPrefix(path, "/demo/"))
	// 平台异步通知接收端点
	case r.Method == http.MethodPost && (path == "/callback" || strings.HasPrefix(path, "/callback/")):
		s.handleCallback(w, r, callbackType(path))
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found: " + path})
	}
}

// servePage 返回内嵌调试页
func (s *server) servePage(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(indexHTML)
}

// ==================================================================
// /demo/config 连接配置（页面内配置，服务端只存内存不落盘）
// ==================================================================

func (s *server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, s.info())
		return
	}
	body, _ := io.ReadAll(r.Body)
	// 密钥字段用指针区分「字段缺省」与「空串」
	var req struct {
		ServiceURL string  `json:"serviceUrl"`
		MchNo      string  `json:"mchNo"`
		AppID      string  `json:"appId"`
		PrivateKey *string `json:"privateKey"`
		PublicKey  *string `json:"publicKey"`
	}
	// 空 body 容错：按空对象处理，由下面的必填校验给出提示
	_ = json.Unmarshal(body, &req)

	serviceURL := strings.TrimSpace(req.ServiceURL)
	mchNo := strings.TrimSpace(req.MchNo)
	if serviceURL == "" || mchNo == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "服务地址与商户号不能为空"})
		return
	}
	info, errMsg := s.applyConfig(serviceURL, mchNo, req.AppID, req.PrivateKey, req.PublicKey)
	if errMsg != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": errMsg})
		return
	}
	log.Printf("[配置] 页面更新连接配置: %s 商户 %s", serviceURL, mchNo)
	writeJSON(w, http.StatusOK, info)
}

// applyConfig 按三态密钥语义整体替换配置；返回脱敏状态或错误文案
func (s *server) applyConfig(serviceURL, mchNo, appID string, privateKey, publicKey *string) (configInfo, string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := niudun.Config{
		ServiceUrl: serviceURL,
		MchNo:      mchNo,
		AppId:      strings.TrimSpace(appID),
	}
	// 密钥字段语义：缺省=保持原值；空串=清空；非空=替换（先做 PEM 解析校验，即时反馈格式错误）
	next.PrivateKey = s.cfg.PrivateKey
	if privateKey != nil {
		value := strings.TrimSpace(*privateKey)
		if value == "" {
			next.PrivateKey = ""
		} else if err := niudun.ValidatePrivateKeyPEM(value); err != nil {
			return configInfo{}, "商户私钥无效: " + err.Error()
		} else {
			next.PrivateKey = value
		}
	}
	next.PublicKey = s.cfg.PublicKey
	if publicKey != nil {
		value := strings.TrimSpace(*publicKey)
		if value == "" {
			next.PublicKey = ""
		} else if err := niudun.ValidatePublicKeyPEM(value); err != nil {
			return configInfo{}, "平台公钥无效: " + err.Error()
		} else {
			next.PublicKey = value
		}
	}

	s.cfg = next
	return s.infoLocked(), ""
}

// info 当前配置的脱敏状态
func (s *server) info() configInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.infoLocked()
}

// infoLocked 脱敏状态（须持锁调用）
func (s *server) infoLocked() configInfo {
	// appId 空串视为未配置（页面显示「默认应用」）
	var appID any
	if s.cfg.AppId != "" {
		appID = s.cfg.AppId
	}
	return configInfo{
		ServiceURL:    s.cfg.ServiceUrl,
		MchNo:         s.cfg.MchNo,
		AppID:         appID,
		CallbackBase:  s.callbackBase,
		PrivateKeySet: strings.TrimSpace(s.cfg.PrivateKey) != "",
		PublicKeySet:  strings.TrimSpace(s.cfg.PublicKey) != "",
	}
}

// snapshot 取当前配置快照（页面保存时整体替换引用，读方每次取最新值）
func (s *server) snapshot() niudun.Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg
}

// ==================================================================
// /demo/ping 连通性自检（服务端中转，规避浏览器跨域）
// ==================================================================

// handlePing 代调平台自检探针 GET /unipay/callback/ping，供页面「测试连接」按钮使用
func (s *server) handlePing(w http.ResponseWriter, r *http.Request) {
	cfg := s.snapshot()
	result := pingResult{ServiceURL: cfg.ServiceUrl}
	begin := time.Now()
	marker, err := niudun.NewClient(cfg).Ping(r.Context())
	result.DurationMs = time.Since(begin).Milliseconds()
	if err != nil {
		result.Error = err.Error()
		// 401/404 是探针链路上最常见的两种情况，直接给出可操作的排查方向
		if strings.Contains(result.Error, "HTTP 401") || strings.Contains(result.Error, "HTTP 404") {
			result.Error += "（需平台版本包含部署自检探针 /unipay/callback/ping，且网关放行该前缀）"
		}
	} else {
		result.Success = true
		result.Marker = marker
	}
	writeJSON(w, http.StatusOK, result)
}

// ==================================================================
// /demo/signed-ping 签名链路自检（服务端中转，规避浏览器跨域）
// ==================================================================

// handleSignedPing 代调签名自检探针 POST /unipay/ping，供页面「测试连接」第二段使用：
// 判定当前配置的商户号/应用/商户私钥/签名串构造是否正确、能否发起真实调用
func (s *server) handleSignedPing(w http.ResponseWriter, r *http.Request) {
	cfg := s.snapshot()
	result := signedPingResult{ServiceURL: cfg.ServiceUrl}
	begin := time.Now()
	if strings.TrimSpace(cfg.PrivateKey) == "" || strings.TrimSpace(cfg.PublicKey) == "" {
		result.Success = false
		// 探针走完整验签链路，两把钥匙缺一不可，各自给可操作文案
		if strings.TrimSpace(cfg.PrivateKey) == "" {
			result.Hint = "尚未配置商户私钥，请先在「连接配置」中填写"
		} else {
			result.Hint = "尚未配置平台公钥（响应无法验签），请先在「连接配置」中填写"
		}
		result.DurationMs = time.Since(begin).Milliseconds()
		writeJSON(w, http.StatusOK, result)
		return
	}
	// 回显钩子捕获发出报文与原始响应，供页面比对签名串（发出 JSON vs 服务端待签串）
	var captured [2]string
	client := niudun.NewClient(cfg)
	client.OnRequest = func(signedJSON string) { captured[0] = signedJSON }
	client.OnResponse = func(rawBody string) { captured[1] = rawBody }

	res, err := client.SignedPing(r.Context(), &niudun.PingParam{})
	if err != nil {
		// 走到 err 只会是硬错误：网络不通 / HTTP 非 200 / 响应验签失败（平台公钥问题）
		msg := err.Error()
		result.Success = false
		result.Error = msg
		if strings.Contains(msg, "响应验签失败") {
			result.Hint = "平台响应验签失败：请核对「连接配置」中的平台公钥"
		} else if strings.Contains(msg, "HTTP 404") {
			result.Hint = "网关未放行「商户开放 API」(/unipay) 接口组，需在部署面板开启"
		}
	} else {
		code := res.Code
		result.Success = code == 0
		result.Code = &code
		result.Msg = res.Msg
		if code == 0 {
			result.Data = &res.Data
		} else {
			result.Hint = classifyProbeError(code)
		}
	}
	if captured[0] != "" {
		result.RequestBody = &captured[0]
	}
	if captured[1] != "" {
		result.ResponseBody = &captured[1]
	}
	result.DurationMs = time.Since(begin).Milliseconds()
	writeJSON(w, http.StatusOK, result)
}

// classifyProbeError 探针错误码分类提示（对照契约 6.14 诊断表）
func classifyProbeError(code int) string {
	if code == 20052 {
		return "验签失败：商户私钥与平台上配置的公钥不配对，或签名串构造不一致——比对「发出报文」与响应 msg 中的服务端待签串"
	}
	if code == 10408 || code == 10409 {
		return "Nonce 防重放拦截：请勿复用请求（每次点击都会生成新 nonce）"
	}
	if code == 10410 || code == 10411 {
		return "请求时间超窗：本机时钟偏差过大，或 reqTime 未按 GMT+8 yyyy-MM-dd HH:mm:ss 字面量"
	}
	return fmt.Sprintf("商户号/应用类错误（code %d）：核对 mchNo 与 appId 是否存在且启用", code)
}

// ==================================================================
// /demo/* 交易调试
// ==================================================================

// handleTrade 经 SDK 真实调用链发起交易，回显请求/响应/验签/耗时
func (s *server) handleTrade(w http.ResponseWriter, r *http.Request, action string) {
	run, ok := actions[action]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown action: " + action})
		return
	}
	body, _ := io.ReadAll(r.Body)

	// 配置快照（一次读取，保证单次调用内一致）
	cfg := s.snapshot()
	if strings.TrimSpace(cfg.PrivateKey) == "" {
		msg := "尚未配置商户私钥，请点击右上角「连接配置」填写后重试"
		writeJSON(w, http.StatusOK, tradeResult{Error: &msg})
		return
	}

	// 回显钩子捕获本次调用的请求体/响应体（每次调用独立 client 实例，无线程共享状态）
	var captured [2]string
	client := niudun.NewClient(cfg)
	client.OnRequest = func(signedJSON string) { captured[0] = signedJSON }
	client.OnResponse = func(rawBody string) { captured[1] = rawBody }

	begin := time.Now()
	// SDK 抛出（业务失败/验签失败/网络异常）也属联调有效结果，回显给页面
	runErr := run(client, body)
	result := tradeResult{
		Success:      runErr == nil,
		DurationMs:   time.Since(begin).Milliseconds(),
		SignVerified: verifyResponse(captured[1], cfg),
		Result:       parseResult(captured[1]),
	}
	if captured[0] != "" {
		result.RequestBody = &captured[0]
	}
	if captured[1] != "" {
		result.ResponseBody = &captured[1]
	}
	if runErr != nil {
		msg := runErr.Error()
		result.Error = &msg
	}
	writeJSON(w, http.StatusOK, result)
}

// verifyResponse 响应验签（demo 层独立复核，便于对照 SDK 内部验签行为）
//
// 返回 nil 表示响应不带签名——平台业务异常经全局异常处理器返回 Result 形状（无 sign），
// 页面据此显示「未签名（平台异常响应）」而非「验签失败」。
func verifyResponse(responseBody string, cfg niudun.Config) *bool {
	if responseBody == "" {
		return nil
	}
	var probe struct {
		Sign string `json:"sign"`
	}
	if err := json.Unmarshal([]byte(responseBody), &probe); err != nil {
		verified := false
		return &verified
	}
	if probe.Sign == "" {
		return nil
	}
	verified := niudun.NewClient(cfg).VerifyNotice(responseBody)
	return &verified
}

// parseResult 把响应体解析成展示对象（业务失败不报错，原样透出 code/msg）
// 用 UseNumber 保持数字原始字面量（避免金额等大整数走 float64 变科学计数法）
func parseResult(responseBody string) interface{} {
	if responseBody == "" {
		return nil
	}
	dec := json.NewDecoder(strings.NewReader(responseBody))
	dec.UseNumber()
	var obj interface{}
	if err := dec.Decode(&obj); err != nil {
		return map[string]string{"parseError": responseBody}
	}
	return obj
}

// ==================================================================
// /callback/* 异步通知接收
// ==================================================================

// handleCallback 接收平台异步通知，用平台公钥验签后记录，返回固定文本 SUCCESS
// （平台要求 HTTP 2xx 且 body 为 SUCCESS，否则重试）
func (s *server) handleCallback(w http.ResponseWriter, r *http.Request, typ string) {
	body, _ := io.ReadAll(r.Body)
	cfg := s.snapshot()
	record := callbackRecord{
		Time: time.Now().UTC().Add(8 * time.Hour).Format("2006-01-02 15:04:05"),
		Type: typ,
		Body: string(body),
	}
	if strings.TrimSpace(cfg.PublicKey) == "" {
		record.SignVerified = false
		record.Msg = "平台公钥未配置，无法验签（请在页面「连接配置」中补充）"
	} else {
		record.SignVerified = niudun.NewClient(cfg).VerifyNotice(string(body))
		var parsed struct {
			Code *int   `json:"code"`
			Msg  string `json:"msg"`
		}
		if err := json.Unmarshal(body, &parsed); err != nil {
			record.SignVerified = false
			record.Msg = "解析失败: " + err.Error()
		} else {
			record.Code = parsed.Code
			record.Msg = parsed.Msg
		}
	}
	s.addCallback(record)
	log.Printf("[回调] %s %s 验签=%v", record.Time, record.Type, record.SignVerified)
	writeText(w, http.StatusOK, "SUCCESS")
}

// callbackType 从 /callback/{type} 取业务类型；通用端点 /callback 记为 common
func callbackType(path string) string {
	typ := strings.Trim(strings.TrimPrefix(path, "/callback"), "/")
	if typ == "" {
		return "common"
	}
	return typ
}

// addCallback 记入队首（新的在前），超出上限丢弃最老记录
func (s *server) addCallback(record callbackRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.callbacks = append([]callbackRecord{record}, s.callbacks...)
	if len(s.callbacks) > maxCallbacks {
		s.callbacks = s.callbacks[:maxCallbacks]
	}
}

// handleCallbacks 回调记录列表（页面轮询）
func (s *server) handleCallbacks(w http.ResponseWriter) {
	s.mu.Lock()
	records := make([]callbackRecord, len(s.callbacks))
	copy(records, s.callbacks)
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"count":   len(records),
		"records": records,
	})
}

// handleCallbacksClear 清空回调记录
func (s *server) handleCallbacksClear(w http.ResponseWriter) {
	s.mu.Lock()
	s.callbacks = nil
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ==================================================================
// HTTP 基础设施
// ==================================================================

// writeJSON 输出紧凑 JSON 响应（UTF-8）
func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	data, err := json.Marshal(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

// writeText 输出纯文本响应
func writeText(w http.ResponseWriter, status int, text string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(text))
}

// dashIfEmpty 空值显示为 -（启动信息用）
func dashIfEmpty(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

// keyState 密钥状态文案（只报是否已配置，不打印内容）
func keyState(key string) string {
	if strings.TrimSpace(key) == "" {
		return "未配置"
	}
	return "已配置"
}
