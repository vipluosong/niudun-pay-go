package niudun

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
)

// 加载私钥（PKCS#8 优先，回退 PKCS#1）— 对照后端 RsaSignUtil#loadPrivateKeyFromPem
// 文本先经 [pemToDER] 容错归一，兼容换行丢失 / 混入不可见字符的粘贴形态
func parsePrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	der, err := pemToDER(pemStr)
	if err != nil {
		return nil, fmt.Errorf("私钥解析失败：%w", err)
	}
	if key, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("私钥非 RSA 类型")
		}
		return rsaKey, nil
	}
	if rsaKey, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return rsaKey, nil
	}
	return nil, errors.New("私钥解析失败：需为 PKCS#8（-----BEGIN PRIVATE KEY-----）或 PKCS#1（-----BEGIN RSA PRIVATE KEY-----）格式的 RSA 私钥")
}

// 加载公钥（X.509 优先，回退 PKCS#1）— 对照后端 RsaSignUtil#loadPublicKeyFromPem
// 文本先经 [pemToDER] 容错归一，兼容换行丢失 / 混入不可见字符的粘贴形态
func parsePublicKey(pemStr string) (*rsa.PublicKey, error) {
	der, err := pemToDER(pemStr)
	if err != nil {
		return nil, fmt.Errorf("公钥解析失败：%w", err)
	}
	if key, err := x509.ParsePKIXPublicKey(der); err == nil {
		rsaKey, ok := key.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("公钥非 RSA 类型")
		}
		return rsaKey, nil
	}
	if rsaKey, err := x509.ParsePKCS1PublicKey(der); err == nil {
		return rsaKey, nil
	}
	return nil, errors.New("公钥解析失败：需为 X.509（-----BEGIN PUBLIC KEY-----）或 PKCS#1（-----BEGIN RSA PUBLIC KEY-----）格式的 RSA 公钥")
}

// ValidatePrivateKeyPEM 校验商户私钥 PEM 是否可解析（PKCS#8 优先，回退 PKCS#1）
// 联调页面保存配置时做即时校验用；解析失败返回带原因的 error
func ValidatePrivateKeyPEM(pemStr string) error {
	_, err := parsePrivateKey(pemStr)
	return err
}

// ValidatePublicKeyPEM 校验平台公钥 PEM 是否可解析（X.509 优先，回退 PKCS#1）
func ValidatePublicKeyPEM(pemStr string) error {
	_, err := parsePublicKey(pemStr)
	return err
}

// RsaSign 用商户私钥对 data 进行 SHA256withRSA 签名，返回 Base64
// 对照后端 RsaSignUtil#sign（UTF-8 字节，PKCS1v15 确定性签名）
func RsaSign(data string, privateKeyPem string) (string, error) {
	key, err := parsePrivateKey(privateKeyPem)
	if err != nil {
		return "", fmt.Errorf("签名失败: %w", err)
	}
	hashed := sha256.Sum256([]byte(data)) // UTF-8 字节
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hashed[:])
	if err != nil {
		return "", fmt.Errorf("签名失败: %w", err)
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// RsaVerify 用平台公钥验签（SHA256withRSA）— 对照后端 RsaSignUtil#verify
func RsaVerify(data string, signB64 string, publicKeyPem string) (bool, error) {
	key, err := parsePublicKey(publicKeyPem)
	if err != nil {
		return false, fmt.Errorf("验签失败: %w", err)
	}
	sig, err := base64.StdEncoding.DecodeString(signB64)
	if err != nil {
		return false, fmt.Errorf("签名 Base64 解码失败: %w", err)
	}
	hashed := sha256.Sum256([]byte(data))
	return rsa.VerifyPKCS1v15(key, crypto.SHA256, hashed[:], sig) == nil, nil
}
