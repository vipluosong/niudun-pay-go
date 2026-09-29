package niudun

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultTimeout = 30 * time.Second

// Client 牛盾支付 SDK 客户端 — 对照 sdk-contract.md 第十节
type Client struct {
	config     Config
	httpClient *http.Client

	// OnRequest 联调回显钩子：请求已签名、即将发出时回调，参数为含 sign 的完整请求 JSON 字符串
	// 不设置则零开销；联调场景建议每次调用创建独立 Client 实例，避免多线程共享状态
	OnRequest func(signedJson string)
	// OnResponse 联调回显钩子：收到平台原始响应体时回调（在响应验签**之前**，验签失败也能拿到原文）
	// 不设置则零开销
	OnResponse func(rawBody string)
}

// NewClient 创建客户端
func NewClient(config Config) *Client {
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &Client{
		config:     config,
		httpClient: &http.Client{Timeout: timeout},
	}
}

// Execute 通用执行入口：自动填充公共参数 → JSON 签名 → POST → 验签 → 返回 RawResult
// 走 JSON 签名路径（reqTime 已序列化为 GMT+8 字面量），与后端验签一致
// param 可传入 struct（如 *PayParam）或 map，内部统一转 map 注入公共字段
// 非 0 业务码返回 *BizError；需要「非 0 码不报错」的调用方（签名自检探针）走 throwOnBizErr=false 的变体
func (c *Client) Execute(ctx context.Context, path string, param interface{}) (*RawResult, error) {
	return c.execute(ctx, path, param, true)
}

// execute 完整执行形态：throwOnBizErr=false 时非 0 业务码不返回 error 而是原样返回 RawResult
// （签名自检探针的职责是报告检查结果，失败码/失败消息本身就是有效答案）；
// 响应验签失败仍返回 error（那是平台公钥配置问题，属于硬错误而非探针答案）
func (c *Client) execute(ctx context.Context, path string, param interface{}, throwOnBizErr bool) (*RawResult, error) {
	// 统一转 map（struct 或 map 均可；金额单位为分，float64 中间表示对分金额无精度损失）
	paramBytes, err := json.Marshal(param)
	if err != nil {
		return nil, fmt.Errorf("请求序列化失败: %w", err)
	}
	var paramMap map[string]interface{}
	if err := json.Unmarshal(paramBytes, &paramMap); err != nil {
		return nil, fmt.Errorf("请求解析失败: %w", err)
	}
	// 注入公共字段
	if _, ok := paramMap["mchNo"]; !ok {
		paramMap["mchNo"] = c.config.MchNo
	}
	if _, ok := paramMap["appId"]; !ok && c.config.AppId != "" {
		paramMap["appId"] = c.config.AppId
	}
	if _, ok := paramMap["reqId"]; !ok {
		paramMap["reqId"] = newUUID()
	}
	if _, ok := paramMap["reqTime"]; !ok {
		paramMap["reqTime"] = nowGmt8()
	}
	if _, ok := paramMap["nonceStr"]; !ok {
		paramMap["nonceStr"] = newNonce()
	}

	// 走 JSON 签名路径
	jsonForSign, err := json.Marshal(paramMap)
	if err != nil {
		return nil, fmt.Errorf("请求序列化失败: %w", err)
	}
	signStr, err := BuildSignStr(string(jsonForSign))
	if err != nil {
		return nil, fmt.Errorf("签名串构造失败: %w", err)
	}
	sign, err := RsaSign(signStr, c.config.PrivateKey)
	if err != nil {
		return nil, err
	}
	paramMap["sign"] = sign
	body, err := json.Marshal(paramMap)
	if err != nil {
		return nil, fmt.Errorf("请求序列化失败: %w", err)
	}
	// 回显钩子：签名后、发送前（供联调平台逐字对照请求报文）
	if c.OnRequest != nil {
		c.OnRequest(string(body))
	}

	url := strings.TrimRight(c.config.ServiceUrl, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// 回显钩子：收到响应体后、验签之前（HTTP 非 200 也能拿到原文，便于排障）
	if c.OnResponse != nil {
		c.OnResponse(string(rawBody))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(rawBody))
	}

	var result RawResult
	if err := json.Unmarshal(rawBody, &result); err != nil {
		return nil, fmt.Errorf("响应解析失败: %w", err)
	}
	// 消息读取兼容：msg 为空时回退读 message
	// （平台业务异常经全局异常处理器返回管理 API 的 Result 形状，消息字段名是 message 而非 msg）
	if result.Msg == "" {
		var compat struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(rawBody, &compat) == nil {
			result.Msg = compat.Message
		}
	}

	// 验签策略（对照契约第五节 / 施工规范 §4.2）：
	// ① 带 sign 的响应强制验签；② 无 sign 的成功响应不可信（防伪造成功响应）；
	// ③ 无 sign 的业务失败响应直接透出业务码与消息，不视为验签失败
	if result.Sign != "" {
		verifyStr, err := BuildSignStr(string(rawBody))
		if err != nil {
			return nil, fmt.Errorf("验签串构造失败: %w", err)
		}
		ok, err := RsaVerify(verifyStr, result.Sign, c.config.PublicKey)
		if err != nil || !ok {
			return nil, errors.New("响应验签失败")
		}
	} else if result.Code == 0 {
		return nil, errors.New("响应缺少签名，无法验证来源")
	}
	if result.Code != 0 && throwOnBizErr {
		return nil, &BizError{Code: result.Code, Msg: result.Msg}
	}
	// 非 0 码且 throwOnBizErr=false：按原样返回，由探针类调用方按 Code 分类诊断
	return &result, nil
}

// execData 通用调用：Execute → 反序列化 NiuDunResult.data 为具体结果结构体
// data 为 null/缺省（如 /unipay/close）时返回零值结构，不报错
func execData[T any](c *Client, ctx context.Context, path string, param interface{}) (*T, error) {
	raw, err := c.Execute(ctx, path, param)
	if err != nil {
		return nil, err
	}
	var data T
	if len(raw.Data) > 0 && string(raw.Data) != "null" {
		if err := json.Unmarshal(raw.Data, &data); err != nil {
			return nil, fmt.Errorf("响应 data 解析失败: %w", err)
		}
	}
	return &data, nil
}

// Pay 支付下单 — POST /unipay/pay
func (c *Client) Pay(ctx context.Context, param *PayParam) (*PayResult, error) {
	return execData[PayResult](c, ctx, "/unipay/pay", param)
}

// Close 关闭/撤销订单 — POST /unipay/close（响应 data 为 null，凭 code == 0 判断成功）
func (c *Client) Close(ctx context.Context, param *CloseParam) error {
	_, err := c.Execute(ctx, "/unipay/close", param)
	return err
}

// QueryPayOrder 查询支付订单 — POST /unipay/query/pay-order（仅查本地单，不调通道）
func (c *Client) QueryPayOrder(ctx context.Context, param *PayQueryParam) (*PayOrderResult, error) {
	return execData[PayOrderResult](c, ctx, "/unipay/query/pay-order", param)
}

// SyncPayOrder 支付订单同步 — POST /unipay/sync/order/pay（主动拉通道最新状态并回写本地，回调丢失的兜底补偿）
func (c *Client) SyncPayOrder(ctx context.Context, param *PaySyncParam) (*PaySyncResult, error) {
	return execData[PaySyncResult](c, ctx, "/unipay/sync/order/pay", param)
}

// Refund 退款 — POST /unipay/refund
func (c *Client) Refund(ctx context.Context, param *RefundParam) (*RefundResult, error) {
	return execData[RefundResult](c, ctx, "/unipay/refund", param)
}

// QueryRefundOrder 查询退款订单 — POST /unipay/query/refund-order（仅查本地单，不调通道）
func (c *Client) QueryRefundOrder(ctx context.Context, param *RefundQueryParam) (*RefundOrderResult, error) {
	return execData[RefundOrderResult](c, ctx, "/unipay/query/refund-order", param)
}

// SyncRefundOrder 退款订单同步 — POST /unipay/sync/order/refund
func (c *Client) SyncRefundOrder(ctx context.Context, param *RefundSyncParam) (*RefundSyncResult, error) {
	return execData[RefundSyncResult](c, ctx, "/unipay/sync/order/refund", param)
}

// Transfer 转账 — POST /unipay/transfer（通道直连转账，幂等维度：通道+商户转账号+商户号；金额单位：分）
func (c *Client) Transfer(ctx context.Context, param *TransferParam) (*TransferResult, error) {
	return execData[TransferResult](c, ctx, "/unipay/transfer", param)
}

// QueryTransferOrder 查询转账订单 — POST /unipay/query/transfer-order（仅查本地单，不调通道）
func (c *Client) QueryTransferOrder(ctx context.Context, param *TransferQueryParam) (*TransferOrderResult, error) {
	return execData[TransferOrderResult](c, ctx, "/unipay/query/transfer-order", param)
}

// SyncTransferOrder 转账订单同步 — POST /unipay/sync/order/transfer
func (c *Client) SyncTransferOrder(ctx context.Context, param *TransferSyncParam) (*TransferSyncResult, error) {
	return execData[TransferSyncResult](c, ctx, "/unipay/sync/order/transfer", param)
}

// Alloc 分账 — POST /unipay/alloc（原支付单须在下单时声明 allocation=true；金额单位：分）
func (c *Client) Alloc(ctx context.Context, param *AllocParam) (*AllocResult, error) {
	return execData[AllocResult](c, ctx, "/unipay/alloc", param)
}

// QueryAllocOrder 查询分账订单 — POST /unipay/query/alloc-order（仅查本地单，不调通道）
func (c *Client) QueryAllocOrder(ctx context.Context, param *AllocQueryParam) (*AllocOrderResult, error) {
	return execData[AllocOrderResult](c, ctx, "/unipay/query/alloc-order", param)
}

// SyncAllocOrder 分账订单同步 — POST /unipay/sync/order/alloc
func (c *Client) SyncAllocOrder(ctx context.Context, param *AllocSyncParam) (*AllocSyncResult, error) {
	return execData[AllocSyncResult](c, ctx, "/unipay/sync/order/alloc", param)
}

// GatewayPrePay 网关预下单 — POST /unipay/gateway/pre-pay（返回收银台跳转地址 h5Url/miniUrl）
func (c *Client) GatewayPrePay(ctx context.Context, param *GatewayPrePayParam) (*GatewayPrePayResult, error) {
	return execData[GatewayPrePayResult](c, ctx, "/unipay/gateway/pre-pay", param)
}

// GatewayQuery 网关订单查询 — POST /unipay/gateway/query
func (c *Client) GatewayQuery(ctx context.Context, param *GatewayOrderQueryParam) (*GatewayOrderResult, error) {
	return execData[GatewayOrderResult](c, ctx, "/unipay/gateway/query", param)
}

// SignedPing 签名自检探针 — POST /unipay/ping（走完整验签链路，一键判定商户号/应用/商户私钥/签名串是否可用）
//
// 与免签名的 [Client.Ping] 互补：本方法由持商户私钥方发起，非 0 业务码不返回 error 而是原样返回 NiuDunResult，
// 供调用方按 Code 分类诊断（20052=验签失败且 Msg 含服务端待签串；10408-10411=nonce/时钟；
// 其余=商户号/应用类错误）；响应验签失败仍返回 error（平台公钥配置问题）
func (c *Client) SignedPing(ctx context.Context, param *PingParam) (*NiuDunResult[PingResult], error) {
	raw, err := c.execute(ctx, "/unipay/ping", param, false)
	if err != nil {
		return nil, err
	}
	result := NiuDunResult[PingResult]{
		Code:    raw.Code,
		Msg:     raw.Msg,
		Sign:    raw.Sign,
		ResTime: raw.ResTime,
		ReqId:   raw.ReqId,
	}
	// 非 0 码时 data 必为空，直接返回组装结果（探针诊断路径）
	if len(raw.Data) > 0 && string(raw.Data) != "null" {
		if err := json.Unmarshal(raw.Data, &result.Data); err != nil {
			return nil, fmt.Errorf("响应 data 解析失败: %w", err)
		}
	}
	return &result, nil
}

// Ping 回调链路自检探针 — GET /unipay/callback/ping（免签名免登录，返回固定标识文本）
// 用于部署自检：探针可达即代表「通道回调」接口组已放行、后端地址配置正确
func (c *Client) Ping(ctx context.Context) (string, error) {
	url := strings.TrimRight(c.config.ServiceUrl, "/") + "/unipay/callback/ping"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("探针请求失败: HTTP %d: %s", resp.StatusCode, string(rawBody))
	}
	return string(rawBody), nil
}

// VerifyNotice 回调验签（原始 HTTP body 字符串）— 对照契约第八节
func (c *Client) VerifyNotice(rawBody string) bool {
	var probe struct {
		Sign string `json:"sign"`
	}
	if json.Unmarshal([]byte(rawBody), &probe) != nil {
		return false
	}
	if probe.Sign == "" {
		return false
	}
	verifyStr, err := BuildSignStr(rawBody)
	if err != nil {
		return false
	}
	ok, _ := RsaVerify(verifyStr, probe.Sign, c.config.PublicKey)
	return ok
}

// nowGmt8 当前时间的 GMT+8 字面量（yyyy-MM-dd HH:mm:ss）— 对照后端 @JsonFormat(GMT+8)
func nowGmt8() string {
	return time.Now().UTC().Add(8 * time.Hour).Format("2006-01-02 15:04:05")
}

// newUUID 生成 UUID v4
func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// newNonce 生成 32 位随机 hex
func newNonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
