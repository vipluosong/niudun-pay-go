package niudun

import "time"

// Config SDK 配置 — 对照 sdk-contract.md 第十节
type Config struct {
	// ServiceUrl 网关地址（自动去尾斜杠），如 https://ndpay-api.qyyapp.com
	ServiceUrl string
	// MchNo 商户号
	MchNo string
	// AppId 应用号（可选，空回落默认应用）
	AppId string
	// PrivateKey 商户私钥 PEM（PKCS#8，-----BEGIN PRIVATE KEY-----）
	PrivateKey string
	// PublicKey 平台公钥 PEM（X.509，-----BEGIN PUBLIC KEY-----）
	PublicKey string
	// Timeout 请求超时，默认 30s
	Timeout time.Duration
}
