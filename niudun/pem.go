package niudun

import (
	"encoding/base64"
	"encoding/pem"
	"errors"
	"strings"
	"unicode"
)

// 密钥文本容错归一。
//
// 从网页、聊天窗口、PDF、IDE 复制 PEM 时，正文换行常被替换成空格或不换行空格（NBSP），
// 或整段压成一行、混入零宽字符，甚至只剩裸 Base64。
// Go 标准库 pem.Decode 只剔除正文里的空格与制表符，且硬性要求 "-----END" 前必须是换行，
// 遇到上述形态会直接判定为无效 PEM（对照其它语言 SDK：Python/Java 均可解析，Node 与 Go 同样严格）。
// 这里在标准解析失败后做一次归一：剥掉头尾标记、剔除全部空白与不可见字符，
// 再按 Base64（含无填充变体）解出 DER，交由上层按 X.509 / PKCS#1 解析。

// pemToDER 提取密钥 DER 字节：标准 PEM 优先，失败回退到容错归一
func pemToDER(text string) ([]byte, error) {
	if strings.TrimSpace(text) == "" {
		return nil, errors.New("密钥内容为空")
	}
	if block, _ := pem.Decode([]byte(text)); block != nil {
		return block.Bytes, nil
	}
	b64 := stripInvisible(stripPEMArmor(text))
	if b64 == "" {
		return nil, errors.New("未找到密钥内容，请确认已完整复制 PEM（含 BEGIN / END 两行）")
	}
	if der, err := base64.StdEncoding.DecodeString(b64); err == nil {
		return der, nil
	}
	// 部分来源会去掉 Base64 末尾的填充等号
	if der, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(b64, "=")); err == nil {
		return der, nil
	}
	return nil, errors.New("密钥内容不是合法的 Base64，请确认复制完整且未混入其它字符")
}

// stripPEMArmor 去掉 -----BEGIN xxx----- / -----END xxx----- 标记，返回中间内容
func stripPEMArmor(text string) string {
	body := text
	if i := strings.Index(body, "-----BEGIN"); i >= 0 {
		body = body[i+len("-----BEGIN"):]
		// 跳过 " xxx-----" 到起始标记结束
		if j := strings.Index(body, "-----"); j >= 0 {
			body = body[j+len("-----"):]
		}
	}
	if i := strings.Index(body, "-----END"); i >= 0 {
		body = body[:i]
	}
	return body
}

// stripInvisible 剔除全部空白与常见不可见字符
// unicode.IsSpace 覆盖 \t \n \v \f \r、空格、U+0085、U+00A0（NBSP）；
// 零宽空格 / 零宽连接符 / 词连接符 / BOM / 软连字符不属于空白，需单独列出
func stripInvisible(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '\u200b', '\u200c', '\u200d', '\u2060', '\ufeff', '\u00ad':
			return -1
		}
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}
