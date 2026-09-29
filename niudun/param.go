package niudun

// 本文件登记全部 15 个业务接口 + 签名自检探针的请求参数结构体 — 对照 sdk-contract.md 第二节 + 6.1–6.14
// 金额单位一律为「分」（int64）；公共字段（mchNo/appId/reqId/reqTime/nonceStr）由 Client.Execute 自动注入

// TerminalInfo 终端信息（线下 POS/收银台场景）— 对照契约 6.6
type TerminalInfo struct {
	TerminalNo string  `json:"terminalNo,omitempty"` // 终端设备号
	StoreNo    string  `json:"storeNo,omitempty"`    // 门店编号
	OperatorId string  `json:"operatorId,omitempty"` // 操作员号
	DeviceName string  `json:"deviceName,omitempty"` // 设备名称
	DeviceIp   string  `json:"deviceIp,omitempty"`   // 设备 IP 地址
	Longitude  float64 `json:"longitude,omitempty"`  // 经度
	Latitude   float64 `json:"latitude,omitempty"`   // 纬度
}

// GoodsDetail 订单商品明细（单个）— 对照契约 6.6
// 挂在 PayParam 与 GatewayPrePayParam 上，用于单品营销/电子发票
type GoodsDetail struct {
	GoodsId     string `json:"goodsId"`               // 商户侧商品编码（必填）
	GoodsName   string `json:"goodsName"`             // 商品名称（必填）
	Quantity    int    `json:"quantity"`              // 商品数量（必填，≥1）
	UnitPrice   int64  `json:"unitPrice"`             // 商品单价，分（必填）
	Category    string `json:"category,omitempty"`    // 商品分类（支付宝独有）
	Description string `json:"description,omitempty"` // 商品描述
	ShowUrl     string `json:"showUrl,omitempty"`     // 商品展示链接
}

// PayParam 支付下单请求参数 — 对照契约 6.1
type PayParam struct {
	BizOrderNo   string        `json:"bizOrderNo"`            // 商户订单号（必填，≤100）
	Title        string        `json:"title"`                 // 支付标题（必填，≤100）
	Description  string        `json:"description,omitempty"` // 支付描述（≤50）
	Amount       int64         `json:"amount"`                // 支付金额，分（必填，1 ≤ x ≤ 9999999999）
	Currency     string        `json:"currency,omitempty"`    // 币种 ISO 4217，缺省 cny
	Product      string        `json:"product,omitempty"`     // 支付产品编码（空则通道路由自动选择）
	Method       string        `json:"method,omitempty"`      // 支付方式编码（见 PayMethodEnum）
	Capability   string        `json:"capability,omitempty"`  // 支付能力编码
	OpenId       string        `json:"openId,omitempty"`      // 用户 OpenId（微信 jsapi/mini 场景必填）
	ChannelAppId string        `json:"channelAppId,omitempty"`
	AuthCode     string        `json:"authCode,omitempty"`    // 付款码（被扫支付必填）
	LimitPay     []string      `json:"limitPay,omitempty"`    // 限制支付类型（如 ["no_credit"]）
	ExtraParam   string        `json:"extraParam,omitempty"`  // 支付扩展参数（JSON 字符串，通道长尾参数）
	NotifyUrl    string        `json:"notifyUrl,omitempty"`   // 异步通知地址（≤200）
	ReturnUrl    string        `json:"returnUrl,omitempty"`   // 同步跳转地址（≤200）
	Attach       string        `json:"attach,omitempty"`      // 商户扩展参数，回调原样返回（≤500）
	ExpiredTime  string        `json:"expiredTime,omitempty"` // 过期时间（GMT+8 yyyy-MM-dd HH:mm:ss，空默认 30 分钟）
	GoodsDetail  []GoodsDetail `json:"goodsDetail,omitempty"` // 订单商品明细
	Terminal     *TerminalInfo `json:"terminal,omitempty"`    // 终端信息（线下 POS/收银台场景）
	Source       string        `json:"source,omitempty"`      // 订单来源标识
	Allocation   *bool         `json:"allocation,omitempty"`  // 是否分账订单（分账链路前置条件）
}

// CloseParam 关闭/撤销订单请求参数 — 对照契约 6.2（orderNo 与 bizOrderNo 至少传一个，优先 orderNo）
type CloseParam struct {
	OrderNo    string `json:"orderNo,omitempty"`    // 平台支付订单号(tradeNo)或网关订单号（优先）
	BizOrderNo string `json:"bizOrderNo,omitempty"` // 商户订单号
	UseCancel  *bool  `json:"useCancel,omitempty"`  // 是否使用撤销方式（部分通道支持，不支持则忽略）
}

// PayQueryParam 查询支付订单请求参数 — 对照契约 6.4（orderNo 与 bizOrderNo 至少传一个，优先 orderNo）
type PayQueryParam struct {
	OrderNo    string `json:"orderNo,omitempty"`    // 平台业务单号（优先）
	BizOrderNo string `json:"bizOrderNo,omitempty"` // 商户订单号
}

// PaySyncParam 支付订单同步参数 — 对照契约 6.11（orderNo / bizOrderNo / outOrderNo 至少传一个）
type PaySyncParam struct {
	OrderNo    string `json:"orderNo,omitempty"`    // 平台业务单号
	BizOrderNo string `json:"bizOrderNo,omitempty"` // 商户订单号
	OutOrderNo string `json:"outOrderNo,omitempty"` // 通道系统交易号
}

// RefundParam 退款请求参数 — 对照契约 6.3（tradeNo 与 bizOrderNo 至少传一个，优先 tradeNo）
type RefundParam struct {
	TradeNo     string `json:"tradeNo,omitempty"`     // 原支付资金交易号（优先）
	BizOrderNo  string `json:"bizOrderNo,omitempty"`  // 原支付商户业务订单号
	Amount      int64  `json:"amount"`                // 退款金额，分（必填，>0，支持部分退款）
	Reason      string `json:"reason,omitempty"`      // 退款原因（≤50）
	BizRefundNo string `json:"bizRefundNo,omitempty"` // 商户退款号（不传则系统生成）
}

// RefundQueryParam 查询退款订单请求参数 — 对照契约 6.5（refundNo 与 bizRefundNo 至少传一个，优先 refundNo）
type RefundQueryParam struct {
	RefundNo    string `json:"refundNo,omitempty"`    // 平台退款号（优先）
	BizRefundNo string `json:"bizRefundNo,omitempty"` // 商户退款号
}

// RefundSyncParam 退款订单同步参数 — 对照契约 6.11（refundNo 与 bizRefundNo 至少传一个，优先 refundNo）
type RefundSyncParam struct {
	RefundNo    string `json:"refundNo,omitempty"`    // 平台退款号（优先）
	BizRefundNo string `json:"bizRefundNo,omitempty"` // 商户退款号
}

// ReportInfo 转账场景报备信息（单个）— 对照契约 6.7
// 微信转账场景报备，字段含义由通道场景定义决定（如 1000 现金营销场景需报备活动名称）
type ReportInfo struct {
	InfoType    string `json:"infoType,omitempty"`    // 报备信息类型
	InfoContent string `json:"infoContent,omitempty"` // 报备信息内容
}

// TransferParam 转账请求参数 — 对照契约 6.7
// 幂等维度为 通道 + 商户转账号 + 商户号：同组合重复发起会拦截，失败单可复用原单号重试
type TransferParam struct {
	Channel       string       `json:"channel"`                 // 转账通道（wechat/alipay/douyin，必填）
	ChannelMchNo  string       `json:"channelMchNo"`            // 通道商户号（必填）
	BizTransferNo string       `json:"bizTransferNo"`           // 商户转账号（幂等键，必填）
	Amount        int64        `json:"amount"`                  // 转账金额，分（必填，>0）
	Title         string       `json:"title,omitempty"`         // 转账标题
	Reason        string       `json:"reason,omitempty"`        // 转账原因/备注（≤200）
	PayeeType     string       `json:"payeeType"`               // 收款人账号类型（必填）
	PayeeAccount  string       `json:"payeeAccount"`            // 收款人账号（必填）
	PayeeName     string       `json:"payeeName,omitempty"`     // 收款人姓名（微信：<0.3 元禁填，≥2000 元必填）
	Attach        string       `json:"attach,omitempty"`        // 商户扩展参数，回调原样返回
	NotifyUrl     string       `json:"notifyUrl,omitempty"`     // 回调通知地址
	ReportInfos   []ReportInfo `json:"reportInfos,omitempty"`   // 转账场景报备信息（微信转账场景必填，留空由通道兜底）
	TransferScene string       `json:"transferScene,omitempty"` // 转账场景标识（支付宝=场景配置 ID；抖音=枚举码；微信不传）
}

// TransferQueryParam 转账订单查询参数 — 对照契约 6.10
// transferNo 单独可查；bizTransferNo 须配 channel（与发起幂等维度保持一致）
type TransferQueryParam struct {
	TransferNo    string `json:"transferNo,omitempty"`    // 平台转账单号（优先）
	Channel       string `json:"channel,omitempty"`       // 转账通道（与商户转账号配对使用）
	BizTransferNo string `json:"bizTransferNo,omitempty"` // 商户转账号（与转账通道配对使用）
}

// TransferSyncParam 转账订单同步参数 — 对照契约 6.11
type TransferSyncParam struct {
	TransferNo    string `json:"transferNo,omitempty"`    // 平台转账单号（优先）
	Channel       string `json:"channel,omitempty"`       // 转账通道（与商户转账号配对使用）
	BizTransferNo string `json:"bizTransferNo,omitempty"` // 商户转账号（与转账通道配对使用）
}

// AllocReceiver 分账接收方（单个）— 对照契约 6.8（Java 侧为 AllocParam.Receiver）
// 接收方类型取值见平台 AllocReceiverTypeEnum：MERCHANT_ID / PERSONAL_OPENID /
// PERSONAL_SUB_OPENID / USER_ID / LOGIN_NAME
type AllocReceiver struct {
	ReceiverType    string `json:"receiverType"`           // 接收方类型（必填）
	ReceiverAccount string `json:"receiverAccount"`        // 接收方账号（必填，≤128）
	ReceiverName    string `json:"receiverName,omitempty"` // 接收方姓名（部分通道/类型必填）
	Amount          int64  `json:"amount"`                 // 分账金额，分（必填，≥1）
}

// AllocParam 分账请求参数 — 对照契约 6.8
// 前置条件：原支付订单须在下单时声明 allocation=true，否则通道拒绝分账
type AllocParam struct {
	BizAllocNo  string          `json:"bizAllocNo"`            // 商户分账单号（幂等键，必填，≤100）
	TradeNo     string          `json:"tradeNo,omitempty"`     // 原支付资金交易号（与 bizOrderNo 二选一，优先本字段）
	BizOrderNo  string          `json:"bizOrderNo,omitempty"`  // 原支付商户业务订单号
	Title       string          `json:"title,omitempty"`       // 分账标题
	Description string          `json:"description,omitempty"` // 分账描述（≤500）
	Receivers   []AllocReceiver `json:"receivers"`             // 接收方列表（至少一个，必填）
	Attach      string          `json:"attach,omitempty"`      // 商户扩展参数，回调时原样返回
	NotifyUrl   string          `json:"notifyUrl,omitempty"`   // 异步通知地址
}

// AllocQueryParam 查询分账订单请求参数 — 对照契约 6.9（allocNo 与 bizAllocNo 至少传一个，优先 allocNo）
type AllocQueryParam struct {
	AllocNo    string `json:"allocNo,omitempty"`    // 平台分账单号（优先）
	BizAllocNo string `json:"bizAllocNo,omitempty"` // 商户分账单号
}

// AllocSyncParam 分账订单同步参数 — 对照契约 6.11
type AllocSyncParam struct {
	AllocNo    string `json:"allocNo,omitempty"`    // 平台分账单号（优先）
	BizAllocNo string `json:"bizAllocNo,omitempty"` // 商户分账单号
}

// GatewayPrePayParam 网关预下单参数 — 对照契约 6.12
// 产品语义「网关支付」：由平台收银台承接支付项选择与调起，商户侧只需拿到跳转地址（h5Url / miniUrl）
type GatewayPrePayParam struct {
	BizOrderNo     string        `json:"bizOrderNo"`               // 商户订单号（必填，≤100）
	Title          string        `json:"title"`                    // 支付标题（必填，≤100）
	Description    string        `json:"description,omitempty"`    // 支付描述
	Amount         int64         `json:"amount"`                   // 支付金额，分（必填）
	Currency       string        `json:"currency,omitempty"`       // 币种 ISO 4217，缺省 cny
	GatewayPayType string        `json:"gatewayPayType,omitempty"` // 网关支付类型：cashier / aggregate
	NotifyUrl      string        `json:"notifyUrl,omitempty"`      // 异步通知地址
	ReturnUrl      string        `json:"returnUrl,omitempty"`      // 同步跳转地址
	Attach         string        `json:"attach,omitempty"`         // 商户扩展参数
	ExtraParam     string        `json:"extraParam,omitempty"`     // 支付扩展参数（JSON 字符串）
	ExpiredTime    string        `json:"expiredTime,omitempty"`    // 过期时间（GMT+8 字面量）
	StoreNo        string        `json:"storeNo,omitempty"`        // 门店编号
	GoodsDetail    []GoodsDetail `json:"goodsDetail,omitempty"`    // 订单商品明细
	Allocation     *bool         `json:"allocation,omitempty"`     // 是否为分账订单
}

// GatewayOrderQueryParam 网关订单查询参数 — 对照契约 6.13
// 注意基类差异：本参数继承平台公共参数 PaymentCommonParam（非商户公共参数），
// mchNo / appId 是参数自身的可选字段，用于网关侧按应用定位订单（留空由 Client 注入配置值）
type GatewayOrderQueryParam struct {
	OrderNo    string `json:"orderNo,omitempty"`    // 平台网关单号（二选一）
	BizOrderNo string `json:"bizOrderNo,omitempty"` // 商户业务单号（二选一）
	AppId      string `json:"appId,omitempty"`      // 应用号
	MchNo      string `json:"mchNo,omitempty"`      // 商户号
}

// PingParam 签名自检探针请求参数 — 对照契约 6.14
// 仅公共参数（无业务字段），公共字段由 Client.Execute 注入
type PingParam struct{}
