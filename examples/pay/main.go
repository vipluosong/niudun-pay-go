// 牛盾聚合支付下单示例 — Go
// 运行：go run examples/pay/main.go（需先启动后端 niudun-start，端口 9999）
package main

import (
	"context"
	"fmt"
	"time"

	"gitee.com/qyyapp/niudun-pay-go/niudun"
)

func main() {
	// 商户私钥 + 平台公钥（PEM 文本，生产环境从环境变量/配置中心读取，切勿硬编码）
	const privateKey = `-----BEGIN PRIVATE KEY-----
（替换为你的商户私钥）
-----END PRIVATE KEY-----`
	const publicKey = `-----BEGIN PUBLIC KEY-----
（替换为平台公钥）
-----END PUBLIC KEY-----`

	config := niudun.Config{
		ServiceUrl: "http://127.0.0.1:9999",
		MchNo:      "M200000001",
		AppId:      "APP001",
		PrivateKey: privateKey,
		PublicKey:  publicKey,
		Timeout:    30 * time.Second,
	}
	client := niudun.NewClient(config)

	result, err := client.Pay(context.Background(), &niudun.PayParam{
		BizOrderNo: fmt.Sprintf("PAY_%d", time.Now().UnixMilli()),
		Title:      "测试商品",
		Amount:     100, // 分
		Method:     "wechat_qr",
		NotifyUrl:  "https://example.com/notify",
	})
	if err != nil {
		fmt.Println("支付失败:", err)
		return
	}
	fmt.Println("订单号:", result.OrderNo)
	fmt.Println("交易号:", result.TradeNo)
	fmt.Println("状态:", result.Status)
	fmt.Println("支付参数体:", result.PayBody)
	fmt.Println("支付参数体类型:", result.PayBodyType)
}
