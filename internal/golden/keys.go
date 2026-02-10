package golden

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

// LoadCACert 加载 CA 证书
// 返回解析后的证书和原始 PEM 数据
func LoadCACert(path string) (*x509.Certificate, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("读取 CA 证书失败: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, nil, fmt.Errorf("解析 CA 证书 PEM 失败")
	}

	if block.Type != "CERTIFICATE" {
		return nil, nil, fmt.Errorf("无效的证书类型: %s", block.Type)
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("解析 CA 证书失败: %w", err)
	}

	return cert, data, nil
}

// LoadRSAPrivateKey 加载 RSA 私钥
func LoadRSAPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取私钥文件失败: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("解析私钥 PEM 失败")
	}

	// 尝试解析 PKCS#1 格式
	if block.Type == "RSA PRIVATE KEY" {
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("解析 PKCS#1 RSA 私钥失败: %w", err)
		}
		return key, nil
	}

	// 尝试解析 PKCS#8 格式
	if block.Type == "PRIVATE KEY" {
		keyInterface, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("解析 PKCS#8 私钥失败: %w", err)
		}
		rsaKey, ok := keyInterface.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("私钥不是 RSA 类型")
		}
		return rsaKey, nil
	}

	return nil, fmt.Errorf("不支持的私钥类型: %s", block.Type)
}

// LoadECPrivateKey 加载 EC 私钥
func LoadECPrivateKey(path string) (*ecdsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取私钥文件失败: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("解析私钥 PEM 失败")
	}

	// 尝试解析 EC 私钥格式
	if block.Type == "EC PRIVATE KEY" {
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("解析 EC 私钥失败: %w", err)
		}
		return key, nil
	}

	// 尝试解析 PKCS#8 格式
	if block.Type == "PRIVATE KEY" {
		keyInterface, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("解析 PKCS#8 私钥失败: %w", err)
		}
		ecKey, ok := keyInterface.(*ecdsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("私钥不是 EC 类型")
		}
		return ecKey, nil
	}

	return nil, fmt.Errorf("不支持的私钥类型: %s", block.Type)
}

// ValidateRSAKeyPair 验证 RSA 证书和私钥是否匹配
func ValidateRSAKeyPair(cert *x509.Certificate, key *rsa.PrivateKey) error {
	certPubKey, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("证书公钥不是 RSA 类型")
	}

	if certPubKey.N.Cmp(key.N) != 0 || certPubKey.E != key.E {
		return fmt.Errorf("证书和私钥不匹配")
	}

	return nil
}

// FileExists 检查文件是否存在
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// CheckFileOverwrite 检查文件是否存在，如果存在且 force=false 则返回错误
func CheckFileOverwrite(path string, force bool) error {
	if FileExists(path) && !force {
		return fmt.Errorf("文件已存在: %s，使用 --force 覆盖", path)
	}
	return nil
}

// ValidateCACertFile 验证 CA 证书文件
func ValidateCACertFile(path string) error {
	if path == "" {
		return fmt.Errorf("未指定 CA 证书路径")
	}
	if !FileExists(path) {
		return fmt.Errorf("CA 证书文件不存在: %s", path)
	}
	_, _, err := LoadCACert(path)
	return err
}

// ValidateCAKeyFile 验证 CA 私钥文件
func ValidateCAKeyFile(path string) error {
	if path == "" {
		return fmt.Errorf("未指定 CA 私钥路径")
	}
	if !FileExists(path) {
		return fmt.Errorf("CA 私钥文件不存在: %s", path)
	}
	_, err := LoadRSAPrivateKey(path)
	return err
}

// ValidateSAKeyFile 验证 SA 私钥文件
func ValidateSAKeyFile(path string) error {
	if path == "" {
		return fmt.Errorf("未指定 SA 私钥路径")
	}
	if !FileExists(path) {
		return fmt.Errorf("SA 私钥文件不存在: %s", path)
	}
	_, err := LoadRSAPrivateKey(path)
	return err
}

// EnsureOutputDir 确保输出目录存在
func EnsureOutputDir(dir string) error {
	if dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0755)
}
