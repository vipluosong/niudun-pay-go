# 牛盾支付 Open SDK for Go

牛盾支付 开放支付平台 Go SDK，封装支付/退款/转账/分账/查询/同步/网关共 15 个开放接口与回调验签。

> **适配 牛盾支付 Open ≥ 1.0** · **Go 1.21+** · Apache-2.0 · 零第三方依赖（标准库实现）

## 功能

- RSA 双向签名（SHA256withRSA），自动签名请求 / 验签响应与回调
- 走 JSON 签名路径，与开源版后端 `reqTime`（北京时间字面量）契约对齐
- 开放接口全覆盖（15 个）：
  - **支付**：`Pay` / `Close` / `QueryPayOrder` / `SyncPayOrder`
  - **退款**：`Refund` / `QueryRefundOrder` / `SyncRefundOrder`
  - **转账**：`Transfer` / `QueryTransferOrder` / `SyncTransferOrder`
  - **分账**：`Alloc` / `QueryAllocOrder` / `SyncAllocOrder`
  - **网关**：`GatewayPrePay` / `GatewayQuery`
- 异步回调验签 `VerifyNotice(rawBody)`；部署自检探针 `Ping(ctx)`
- 联调回显钩子 `Client.OnRequest` / `Client.OnResponse`（不设置则零开销）

## 安装（源码引入）

```bash
go get gitee.com/qyyapp/niudun-pay-go@main
```

```go
import "gitee.com/qyyapp/niudun-pay-go/niudun"
```

## 快速开始

```go
config := niudun.Config{
    ServiceUrl: "https://sandbox.niudunpay.cn",
    MchNo:      "M200000001",
    AppId:      "APP001",
    PrivateKey: merchantPrivateKeyPem,   // PEM 文本
    PublicKey:  platformPublicKeyPem,    // PEM 文本
    Timeout:    30 * time.Second,
}
client := niudun.NewClient(config)

// 支付下单
ctx := context.Background()
result, err := client.Pay(ctx, &niudun.PayParam{
    BizOrderNo: "PAY20250805001",
    Title:      "测试商品",
    Amount:     100,            // 分
    Method:     "wechat_qr",
    NotifyUrl:  "https://example.com/notify",
})

// 联调回显（可选）：拿到签名后的请求体与平台原始响应体，不设置则零开销
// client.OnRequest = func(signedJson string) { fmt.Println("请求:", signedJson) }
// client.OnResponse = func(rawBody string) { fmt.Println("响应:", rawBody) }

// 回调验签
// ok := client.VerifyNotice(rawBody)
```

> 完整可运行示例见 [`examples/pay/main.go`](examples/pay/main.go)。

## 联调 Demo（推荐入门方式）

仓内自带一个**单命令启动的联调 Demo**：内嵌调试页 + 全接口表单 + 回调接收，
所有交易调用都经 `niudun.Client` 真实签名发出，同时验证 SDK 与平台接口两侧。

### 1. 启动（无需任何配置文件）

```bash
go run ./cmd/demo              # 默认端口 9791
go run ./cmd/demo --port=9791  # 显式指定端口
```

浏览器打开 <http://127.0.0.1:9791>，页面为**三栏布局**：左侧接口导航（5 个业务域 / 15 个接口）、
中间表单与结果、右侧**随表单实时生成的 Go 调用代码**（可直接复制到项目里用）。

点击右上角 **「连接配置」** 打开弹窗填写参数，保存即生效：

| 配置项 | 说明 |
|--------|------|
| 平台服务地址 | 如 `http://127.0.0.1:9999` |
| 商户号 / 应用号 | 应用号可空（回落平台默认应用） |
| 商户私钥 | PKCS#8 PEM，粘贴后即时校验格式 |
| 平台公钥 | X.509 PEM，用于响应与回调验签 |

弹窗内 **「测试连接」** 按钮会经服务端中转调用平台自检探针 `GET /unipay/callback/ping`
（浏览器直连平台地址会跨域），可快速区分「地址写错」「网关未放行 /unipay/callback 前缀」「后端未启动」。
注意探针需平台版本包含该端点，旧版本会返回 401。

配置保存在**浏览器 localStorage**（仅本机、服务端只存内存不落盘），服务重启后打开页面自动恢复。

> **调试页由 `go:embed` 在编译期打包**，改动 `cmd/demo/index.html` 后必须重新 `go build` / `go run` 才生效
> （不像 Java 版从 classpath 读资源那样改完即生效）。

### 2. 使用

- 左侧导航切换 **15 个开放接口**（支付 / 退款 / 转账 / 分账 / 网关五族），
  每个接口的表单都带必填校验与类型校验，嵌套结构（商品明细 / 终端信息 / 转账报备 / 分账接收方）
  用可增删的行编辑器填写
- 结果区回显「SDK 签名后的完整请求体 + 平台原始响应 + 响应验签结果 + 耗时」，
  支付类接口单独透出 `payBody` / `h5Url` / `confirmUrl` 等跳转地址
- **回调通知记录** 区实时轮询展示平台异步通知（自动验签）；
  表单 `notifyUrl` 填 Demo 提示的回调地址（默认已填好 `http://127.0.0.1:9791/callback/pay`）
  即可完成「支付 → 回调 → 验签」全链路闭环

## 接口文档

- [接入准备](https://doc.open.niudun.cn/api/getting-started) · [签名规则](https://doc.open.niudun.cn/api/signature)
- 黄金测试向量：见 [`niudun/golden_test.go`](niudun/golden_test.go)（与后端签名契约同源断言）

## License

Apache-2.0，可自由用于商业项目与闭源集成，协议全文见 [LICENSE](LICENSE)。主仓库 [牛盾支付 Open](https://gitee.com/qyyapp) 核心为 LGPL-3.0-or-later，本 SDK 作为独立仓按 Apache-2.0 单独发布。

## 开源许可与来源

本项目基于 [DaxPay](https://github.com/opendaxpay) 开放的支付 SDK 改造，
按牛盾支付的命名规则重命名并适配，核心签名验签协议保持一致。

原项目采用 **Apache License 2.0**，本项目遵循同一许可：
`LICENSE` 文件中保留了原始版权与许可声明，任何修改均在源码中可见。
