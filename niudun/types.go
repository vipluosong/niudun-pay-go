package niudun

import (
	"encoding/json"
	"fmt"
)

// NiuDunResult 统一响应结构 — 对照 sdk-contract.md 第五节
type NiuDunResult[T any] struct {
	Code    int    `json:"code"`              // 业务状态码，0 成功
	Msg     string `json:"msg"`               // 提示信息（msg 非 message）
	Data    T      `json:"data"`              // 业务数据
	Sign    string `json:"sign,omitempty"`    // 平台 RSA 响应签名
	ResTime string `json:"resTime,omitempty"` // 响应时间（北京时间 yyyy-MM-dd HH:mm:ss）
	ReqId   string `json:"reqId,omitempty"`   // 请求 ID 回显
}

// CommonParam 公共请求参数（所有业务请求继承）— 对照契约第四节
type CommonParam struct {
	MchNo    string `json:"mchNo,omitempty"`
	AppId    string `json:"appId,omitempty"`
	ReqId    string `json:"reqId,omitempty"`
	ReqTime  string `json:"reqTime,omitempty"` // GMT+8 yyyy-MM-dd HH:mm:ss
	NonceStr string `json:"nonceStr,omitempty"`
	ClientIp string `json:"clientIp,omitempty"`
	Sign     string `json:"sign,omitempty"`
}

// BizError 业务异常（Code != 0 时返回）
// 文案格式对齐其它语言 SDK：[code] 消息（如 [20023] 未找到指定的商户配置）
type BizError struct {
	Code int    // 平台业务码
	Msg  string // 平台消息（msg 为空时回退读 message）
}

func (e *BizError) Error() string {
	return fmt.Sprintf("[%d] %s", e.Code, e.Msg)
}

// 为方便 Execute 返回未类型化 data 的结果，使用 RawResult 别名
type RawResult = NiuDunResult[json.RawMessage]
