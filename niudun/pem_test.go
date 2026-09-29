package niudun

import (
	"encoding/base64"
	"strings"
	"testing"
)

// splitPEM 拆出 PEM 的头行 / 正文 / 尾行
func splitPEM(t *testing.T, pemStr string) (head, body, foot string) {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(pemStr), "\n")
	if len(lines) < 3 {
		t.Fatalf("测试密钥格式异常：仅 %d 行", len(lines))
	}
	return lines[0], strings.Join(lines[1:len(lines)-1], "\n"), lines[len(lines)-1]
}

// rewrap 把正文压平后按 width 重新分行，行间以 sep 连接（width <= 0 表示整段一行）
func rewrap(body, sep string, width int) string {
	flat := strings.ReplaceAll(body, "\n", "")
	if width <= 0 {
		return flat
	}
	var parts []string
	for i := 0; i < len(flat); i += width {
		end := i + width
		if end > len(flat) {
			end = len(flat)
		}
		parts = append(parts, flat[i:end])
	}
	return strings.Join(parts, sep)
}

// 公钥：各种「复制粘贴变形」都应解析出同一把公钥
// 标准库 pem.Decode 只剔除正文空格与制表符，且要求 END 前必须是换行，这些形态它一律判无效
func TestParsePublicKeyToleratesPasteArtifacts(t *testing.T) {
	head, body, foot := splitPEM(t, testPublicKey)
	flat := rewrap(body, "", 0)
	want, err := parsePublicKey(testPublicKey)
	if err != nil {
		t.Fatalf("标准 PEM 应能解析: %v", err)
	}

	cases := []struct {
		name string
		text string
	}{
		{"标准 PEM", testPublicKey},
		{"正文单行不换行", head + "\n" + flat + "\n" + foot},
		{"正文换行→空格", head + "\n" + rewrap(body, " ", 64) + "\n" + foot},
		{"全文压成一行", head + " " + rewrap(body, " ", 64) + " " + foot},
		{"BEGIN 行尾换行→空格", head + " " + rewrap(body, "\n", 64) + "\n" + foot},
		{"END 前换行→空格", head + "\n" + rewrap(body, "\n", 64) + " " + foot},
		{"正文含 NBSP", head + "\n" + flat[:64] + "\u00a0" + flat[64:] + "\n" + foot},
		{"NBSP 当换行分隔符", head + "\n" + rewrap(body, "\u00a0", 64) + "\n" + foot},
		{"正文含零宽空格", head + "\n" + flat[:64] + "\u200b" + flat[64:] + "\n" + foot},
		{"正文含 BOM", head + "\n" + flat[:64] + "\ufeff" + flat[64:] + "\n" + foot},
		{"裸 Base64（无头尾标记）", flat},
		{"CRLF 换行", strings.ReplaceAll(testPublicKey, "\n", "\r\n")},
		{"CR 换行", strings.ReplaceAll(strings.TrimSpace(testPublicKey), "\n", "\r")},
		{"前后带说明文字", "这是平台公钥：\n" + testPublicKey + "\n请妥善保管"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parsePublicKey(c.text)
			if err != nil {
				t.Fatalf("应能解析却失败: %v", err)
			}
			if got.N.Cmp(want.N) != 0 || got.E != want.E {
				t.Fatal("解析结果与标准 PEM 不一致")
			}
		})
	}
}

// 私钥：同样覆盖变形形态（PKCS#8）
func TestParsePrivateKeyToleratesPasteArtifacts(t *testing.T) {
	head, body, foot := splitPEM(t, testPrivateKey)
	flat := rewrap(body, "", 0)
	want, err := parsePrivateKey(testPrivateKey)
	if err != nil {
		t.Fatalf("标准 PEM 应能解析: %v", err)
	}

	cases := []struct {
		name string
		text string
	}{
		{"标准 PEM", testPrivateKey},
		{"正文单行不换行", head + "\n" + flat + "\n" + foot},
		{"正文换行→空格", head + "\n" + rewrap(body, " ", 64) + "\n" + foot},
		{"全文压成一行", head + " " + rewrap(body, " ", 64) + " " + foot},
		{"正文含 NBSP", head + "\n" + flat[:64] + "\u00a0" + flat[64:] + "\n" + foot},
		{"裸 Base64（无头尾标记）", flat},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parsePrivateKey(c.text)
			if err != nil {
				t.Fatalf("应能解析却失败: %v", err)
			}
			if got.N.Cmp(want.N) != 0 || got.D.Cmp(want.D) != 0 {
				t.Fatal("解析结果与标准 PEM 不一致")
			}
		})
	}
}

// Base64 末尾填充等号被去掉时也应能解出
func TestPemToDERToleratesUnpaddedBase64(t *testing.T) {
	der := []byte("niudun-sdk")
	padded := base64.StdEncoding.EncodeToString(der)
	unpadded := strings.TrimRight(padded, "=")
	if padded == unpadded {
		t.Fatalf("测试数据未产生填充等号：%s", padded)
	}
	got, err := pemToDER(unpadded)
	if err != nil {
		t.Fatalf("无填充 Base64 应能解析: %v", err)
	}
	if string(got) != string(der) {
		t.Fatalf("解出内容不一致：%q", got)
	}
}

// 非法输入应给出可定位的报错
func TestParseKeyErrorMessages(t *testing.T) {
	cases := []struct {
		name    string
		text    string
		wantSub string
	}{
		{"空内容", "   ", "密钥内容为空"},
		{"非密钥文本", "这不是密钥", "不是合法的 Base64"},
		{"只有 BEGIN 无 END", "-----BEGIN PUBLIC KEY-----", "未找到密钥内容"},
		{"Base64 合法但非密钥", "YWJjZGVmZ2g=", "需为 X.509"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := parsePublicKey(c.text)
			if err == nil {
				t.Fatal("应报错却解析成功")
			}
			if !strings.Contains(err.Error(), c.wantSub) {
				t.Fatalf("错误信息应含 %q，实际为: %v", c.wantSub, err)
			}
		})
	}
}

// 联调页保存配置走的两个导出校验入口（变形文本同样放行）
func TestValidateKeyPEM(t *testing.T) {
	head, body, foot := splitPEM(t, testPublicKey)
	flat := head + " " + rewrap(body, " ", 64) + " " + foot
	if err := ValidatePublicKeyPEM(flat); err != nil {
		t.Fatalf("变形公钥应通过校验: %v", err)
	}
	if err := ValidatePrivateKeyPEM(testPrivateKey); err != nil {
		t.Fatalf("标准私钥应通过校验: %v", err)
	}
}
