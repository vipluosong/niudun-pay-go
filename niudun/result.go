package niudun

// 本文件登记全部 15 个业务接口 + 签名自检探针的响应结果结构体 — 对照 sdk-contract.md 6.1–6.14
// 均为只读结构（不参与请求签名），金额单位一律为「分」（int64）；
// 时间字段为北京时间字面量 yyyy-MM-dd HH:mm:ss，按契约以 string 原样承载

// PayResult 支付下单响应结果 — 对照契约 6.1 NormalPayResult
type PayResult struct {
	BizOrderNo  string `json:"bizOrderNo"`  // 商户订单号
	OrderNo     string `json:"orderNo"`     // 平台业务单号
	TradeNo     string `json:"tradeNo"`     // 资金交易号（与 orderNo 独立）
	Status      string `json:"status"`      // 资金态: init/processing/success/fail/close/cancel
	PayBody     string `json:"payBody"`     // 二维码链接/调起参数/跳转 URL
	PayBodyType string `json:"payBodyType"` // code_url/pay_info/redirect_url
}

// PayOrderResult 支付订单查询结果 — 对照契约 6.4 NormalPayOrderResult
type PayOrderResult struct {
	BizOrderNo        string `json:"bizOrderNo"`        // 商户订单号
	OrderNo           string `json:"orderNo"`           // 平台业务单号
	TradeNo           string `json:"tradeNo"`           // 资金交易号
	OutOrderNo        string `json:"outOrderNo"`        // 通道系统交易号
	Title             string `json:"title"`             // 支付标题
	Description       string `json:"description"`       // 支付描述
	Channel           string `json:"channel"`           // 支付通道
	Method            string `json:"method"`            // 支付方式
	LimitPay          string `json:"limitPay"`          // 限制用户支付类型
	Amount            int64  `json:"amount"`            // 金额，分
	Currency          string `json:"currency"`          // 币种 ISO 4217（如 cny/usd，缺省 cny）
	RealAmount        int64  `json:"realAmount"`        // 实收金额，分
	RefundableBalance int64  `json:"refundableBalance"` // 可退款余额，分
	Status            string `json:"status"`            // 支付状态
	RefundStatus      string `json:"refundStatus"`      // 退款状态
	Provider          string `json:"provider"`          // 支付渠道（微信/支付宝/银联）
	PayTime           string `json:"payTime"`           // 支付时间
	CloseTime         string `json:"closeTime"`         // 关闭时间
	ExpiredTime       string `json:"expiredTime"`       // 过期时间
	TerminalNo        string `json:"terminalNo"`        // 终端设备编码
	StoreNo           string `json:"storeNo"`           // 门店号
	BuyerId           string `json:"buyerId"`           // 付款用户 ID
	Attach            string `json:"attach"`            // 商户扩展参数（原样返回）
	ErrorMsg          string `json:"errorMsg"`          // 错误信息
}

// PaySyncResult 支付订单同步结果 — 对照契约 6.11
type PaySyncResult struct {
	OrderStatus string `json:"orderStatus"` // 同步后的支付订单状态
	Adjust      bool   `json:"adjust"`      // 本次同步是否订正了本地状态（true=本地状态被通道结果修正）
}

// RefundResult 退款响应结果 — 对照契约 6.3
type RefundResult struct {
	RefundNo    string `json:"refundNo"`    // 平台退款号
	BizRefundNo string `json:"bizRefundNo"` // 商户退款号
	Status      string `json:"status"`      // 退款状态
	ErrorMsg    string `json:"errorMsg"`    // 错误信息（失败时返回）
}

// RefundOrderResult 退款订单查询结果 — 对照契约 6.5
type RefundOrderResult struct {
	RefundNo    string `json:"refundNo"`    // 平台退款号
	BizRefundNo string `json:"bizRefundNo"` // 商户退款号
	TradeNo     string `json:"tradeNo"`     // 原支付资金交易号
	BizOrderNo  string `json:"bizOrderNo"`  // 原支付商户业务订单号
	OutRefundNo string `json:"outRefundNo"` // 通道退款流水号
	Amount      int64  `json:"amount"`      // 退款金额，分
	OrderAmount int64  `json:"orderAmount"` // 订单总金额，分
	Status      string `json:"status"`      // 退款状态
	Reason      string `json:"reason"`      // 退款原因
	FinishTime  string `json:"finishTime"`  // 退款完成时间
	ErrorMsg    string `json:"errorMsg"`    // 错误信息
}

// RefundSyncResult 退款订单同步结果 — 对照契约 6.11
type RefundSyncResult struct {
	OrderStatus string `json:"orderStatus"` // 同步后的退款订单状态
	Adjust      bool   `json:"adjust"`      // 本次同步是否订正了本地状态
}

// TransferResult 转账响应结果 — 对照契约 6.7 TransferCreateResult（金额单位：分）
type TransferResult struct {
	TransferNo    string `json:"transferNo"`    // 平台转账单号
	BizTransferNo string `json:"bizTransferNo"` // 商户转账号
	Status        string `json:"status"`        // 转账状态
	ConfirmUrl    string `json:"confirmUrl"`    // 确认收款跳转地址（部分通道需收款人确认收款）
}

// TransferOrderResult 转账订单查询结果 — 对照契约 6.10
type TransferOrderResult struct {
	TransferNo    string `json:"transferNo"`    // 平台转账单号
	BizTransferNo string `json:"bizTransferNo"` // 商户转账号
	OutTransferNo string `json:"outTransferNo"` // 通道转账单号
	RelationNo    string `json:"relationNo"`    // 关联单号
	Amount        int64  `json:"amount"`        // 转账金额，分
	Currency      string `json:"currency"`      // 币种 ISO 4217
	Channel       string `json:"channel"`       // 转账通道
	Provider      string `json:"provider"`      // 支付渠道（微信/支付宝/抖音）
	Status        string `json:"status"`        // 转账状态
	Title         string `json:"title"`         // 转账标题
	FinishTime    string `json:"finishTime"`    // 转账完成时间
	ErrorMsg      string `json:"errorMsg"`      // 错误信息
}

// TransferSyncResult 转账订单同步结果 — 对照契约 6.11
type TransferSyncResult struct {
	OrderStatus string `json:"orderStatus"` // 同步后的转账订单状态
	Adjust      bool   `json:"adjust"`      // 本次同步是否订正了本地状态
}

// AllocResult 分账响应结果 — 对照契约 6.8
type AllocResult struct {
	AllocNo    string `json:"allocNo"`    // 平台分账单号
	BizAllocNo string `json:"bizAllocNo"` // 商户分账单号
	Status     string `json:"status"`     // 分账状态
	ErrorMsg   string `json:"errorMsg"`   // 错误信息（失败时返回）
}

// AllocDetail 分账接收方明细（单个）— 对照契约 6.9
type AllocDetail struct {
	ReceiverType    string `json:"receiverType"`    // 接收方类型
	ReceiverAccount string `json:"receiverAccount"` // 接收方账号
	ReceiverName    string `json:"receiverName"`    // 接收方姓名
	Amount          int64  `json:"amount"`          // 分账金额，分
	Result          string `json:"result"`          // 该接收方分账结果
	ErrorMsg        string `json:"errorMsg"`        // 错误信息
	FinishTime      string `json:"finishTime"`      // 完成时间
}

// AllocOrderResult 分账订单查询结果 — 对照契约 6.9
type AllocOrderResult struct {
	AllocNo    string        `json:"allocNo"`    // 平台分账单号
	BizAllocNo string        `json:"bizAllocNo"` // 商户分账单号
	TradeNo    string        `json:"tradeNo"`    // 原支付资金交易号
	BizOrderNo string        `json:"bizOrderNo"` // 商户业务订单号
	OutAllocNo string        `json:"outAllocNo"` // 通道分账单号
	Amount     int64         `json:"amount"`     // 分账总金额，分
	Status     string        `json:"status"`     // 分账状态
	FinishTime string        `json:"finishTime"` // 分账完成时间
	Channel    string        `json:"channel"`    // 支付通道
	Attach     string        `json:"attach"`     // 商户扩展参数（原样返回）
	ErrorMsg   string        `json:"errorMsg"`   // 错误信息
	Details    []AllocDetail `json:"details"`    // 分账接收方明细列表
}

// AllocSyncResult 分账订单同步结果 — 对照契约 6.11
type AllocSyncResult struct {
	OrderStatus string `json:"orderStatus"` // 同步后的分账订单状态
	Adjust      bool   `json:"adjust"`      // 本次同步是否订正了本地状态
}

// GatewayPrePayResult 网关预下单结果 — 对照契约 6.12
type GatewayPrePayResult struct {
	OrderNo     string `json:"orderNo"`     // 平台网关单号
	BizOrderNo  string `json:"bizOrderNo"`  // 商户订单号
	Status      string `json:"status"`      // 订单状态
	GatewayType string `json:"gatewayType"` // 网关支付类型（cashier/aggregate）
	H5Url       string `json:"h5Url"`       // H5 收银台跳转地址
	MiniUrl     string `json:"miniUrl"`     // 小程序收银台跳转地址
	ExpiredTime string `json:"expiredTime"` // 过期时间
}

// GatewayOrderResult 网关订单查询结果 — 对照契约 6.13
type GatewayOrderResult struct {
	OrderNo     string `json:"orderNo"`     // 平台网关单号
	BizOrderNo  string `json:"bizOrderNo"`  // 商户业务单号
	GatewayType string `json:"gatewayType"` // 网关支付类型
	Title       string `json:"title"`       // 支付标题
	Description string `json:"description"` // 支付描述
	Amount      int64  `json:"amount"`      // 金额，分
	Currency    string `json:"currency"`    // 币种
	Status      string `json:"status"`      // 订单状态
	ExpiredTime string `json:"expiredTime"` // 过期时间
	PayTime     string `json:"payTime"`     // 支付时间
	Channel     string `json:"channel"`     // 支付通道
	Method      string `json:"method"`      // 支付方式
	Product     string `json:"product"`     // 支付产品编码
	TradeNo     string `json:"tradeNo"`     // 资金交易号
	OutOrderNo  string `json:"outOrderNo"`  // 通道系统交易号
	FundStatus  string `json:"fundStatus"`  // 资金状态
	Attach      string `json:"attach"`      // 商户扩展参数
	ReturnUrl   string `json:"returnUrl"`   // 同步跳转地址
}

// PingResult 签名自检探针结果 — 对照契约 6.14
// 回显平台侧解析结果，供对接方核对商户身份与签名串构造
type PingResult struct {
	MchNo          string `json:"mchNo"`          // 平台侧解析出的商户号
	AppId          string `json:"appId"`          // 平台侧解析出的应用号
	AppFromDefault bool   `json:"appFromDefault"` // 应用号是否回落平台默认应用（请求未携带 appId 时）
	ServerSignStr  string `json:"serverSignStr"`  // 服务端待签串（与本地签名串比对可定位差异）
}
