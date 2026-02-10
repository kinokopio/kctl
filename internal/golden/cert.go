package golden

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kctl/pkg/types"
)

const (
	// RSAKeySize RSA 密钥长度
	RSAKeySize = 2048

	// DefaultCertDays 默认证书有效天数
	DefaultCertDays = 365

	// DefaultRole 默认集群角色
	DefaultRole = "cluster-admins"

	// DefaultUsername 默认用户名
	DefaultUsername = "kubernetes-admin"
)

// ForgeUserCert 伪造用户证书
func ForgeUserCert(opts *types.UserCertOptions) (*types.ForgeResult, error) {
	// 验证必需参数
	if err := ValidateCACertFile(opts.CACertPath); err != nil {
		return nil, err
	}
	if err := ValidateCAKeyFile(opts.CAKeyPath); err != nil {
		return nil, err
	}

	// 设置默认值
	if opts.Role == "" {
		opts.Role = DefaultRole
	}
	if opts.Username == "" {
		opts.Username = DefaultUsername
	}
	if opts.Days <= 0 {
		opts.Days = DefaultCertDays
	}

	// 确保输出目录存在
	if err := EnsureOutputDir(opts.OutputDir); err != nil {
		return nil, fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 计算输出路径
	certPath, keyPath := getUserCertOutputPaths(opts.OutputDir, opts.Role, opts.Username)

	// 检查文件是否已存在
	if err := CheckFileOverwrite(certPath, opts.Force); err != nil {
		return nil, err
	}
	if err := CheckFileOverwrite(keyPath, opts.Force); err != nil {
		return nil, err
	}

	// 加载 CA 证书和私钥
	caCert, _, err := LoadCACert(opts.CACertPath)
	if err != nil {
		return nil, err
	}
	caKey, err := LoadRSAPrivateKey(opts.CAKeyPath)
	if err != nil {
		return nil, err
	}

	// 生成新的 RSA 密钥对
	privateKey, err := rsa.GenerateKey(rand.Reader, RSAKeySize)
	if err != nil {
		return nil, fmt.Errorf("生成 RSA 密钥失败: %w", err)
	}

	// 构建证书模板
	template := buildUserCertTemplate(opts.Role, opts.Username, opts.Days, caCert)

	// 使用 CA 签名证书
	certDER, err := x509.CreateCertificate(rand.Reader, template, caCert, &privateKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("签名证书失败: %w", err)
	}

	// 保存证书和私钥
	if err := saveCertAndRSAKey(certPath, keyPath, certDER, privateKey); err != nil {
		return nil, err
	}

	// 构建结果
	result := &types.ForgeResult{
		Type:      types.ForgeTypeUserCert,
		CertPath:  certPath,
		KeyPath:   keyPath,
		Identity:  opts.Username,
		ExpiresAt: template.NotAfter,
	}

	return result, nil
}

// ForgeNodeCert 伪造节点证书
func ForgeNodeCert(opts *types.NodeCertOptions) (*types.ForgeResult, error) {
	// 验证必需参数
	if err := ValidateCACertFile(opts.CACertPath); err != nil {
		return nil, err
	}
	if err := ValidateCAKeyFile(opts.CAKeyPath); err != nil {
		return nil, err
	}
	if opts.NodeName == "" {
		return nil, fmt.Errorf("未指定节点名称")
	}

	// 设置默认值
	if opts.Days <= 0 {
		opts.Days = DefaultCertDays
	}

	// 确保输出目录存在
	if err := EnsureOutputDir(opts.OutputDir); err != nil {
		return nil, fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 计算输出路径
	certPath, keyPath := getNodeCertOutputPaths(opts.OutputDir, opts.NodeName)

	// 检查文件是否已存在
	if err := CheckFileOverwrite(certPath, opts.Force); err != nil {
		return nil, err
	}
	if err := CheckFileOverwrite(keyPath, opts.Force); err != nil {
		return nil, err
	}

	// 加载 CA 证书和私钥
	caCert, _, err := LoadCACert(opts.CACertPath)
	if err != nil {
		return nil, err
	}
	caKey, err := LoadRSAPrivateKey(opts.CAKeyPath)
	if err != nil {
		return nil, err
	}

	// 生成新的 ECC 密钥对 (prime256v1)
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("生成 ECC 密钥失败: %w", err)
	}

	// 构建证书模板
	template := buildNodeCertTemplate(opts.NodeName, opts.Days, caCert)

	// 使用 CA 签名证书
	certDER, err := x509.CreateCertificate(rand.Reader, template, caCert, &privateKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("签名证书失败: %w", err)
	}

	// 保存证书和私钥
	if err := saveCertAndECKey(certPath, keyPath, certDER, privateKey); err != nil {
		return nil, err
	}

	// 构建结果
	result := &types.ForgeResult{
		Type:      types.ForgeTypeNodeCert,
		CertPath:  certPath,
		KeyPath:   keyPath,
		Identity:  fmt.Sprintf("system:node:%s", opts.NodeName),
		ExpiresAt: template.NotAfter,
	}

	return result, nil
}

// buildUserCertTemplate 构建用户证书模板
func buildUserCertTemplate(role, username string, days int, caCert *x509.Certificate) *x509.Certificate {
	// 生成随机序列号
	serialNumber, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 64))

	notBefore := time.Now()
	notAfter := notBefore.AddDate(0, 0, days)

	// 构建 Organization 字段
	// system: 开头的组直接使用（如 system:masters）
	// 其他组添加 kubeadm: 前缀（如 kubeadm:cluster-admins）
	org := role
	if !strings.HasPrefix(role, "system:") {
		org = "kubeadm:" + role
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   username,
			Organization: []string{org},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
	}

	// 设置 AuthorityKeyId
	if caCert != nil {
		template.AuthorityKeyId = caCert.SubjectKeyId
	}

	return template
}

// buildNodeCertTemplate 构建节点证书模板
func buildNodeCertTemplate(nodeName string, days int, caCert *x509.Certificate) *x509.Certificate {
	// 生成随机序列号 (节点证书使用更长的序列号)
	serialNumber, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))

	notBefore := time.Now()
	notAfter := notBefore.AddDate(0, 0, days)

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   fmt.Sprintf("system:node:%s", nodeName),
			Organization: []string{"system:nodes"},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
	}

	// 设置 AuthorityKeyId
	if caCert != nil {
		template.AuthorityKeyId = caCert.SubjectKeyId
	}

	return template
}

// saveCertAndRSAKey 保存证书和 RSA 私钥
func saveCertAndRSAKey(certPath, keyPath string, certDER []byte, key *rsa.PrivateKey) error {
	// 保存证书
	certFile, err := os.Create(certPath)
	if err != nil {
		return fmt.Errorf("创建证书文件失败: %w", err)
	}
	defer func() { _ = certFile.Close() }()

	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return fmt.Errorf("写入证书失败: %w", err)
	}

	// 保存私钥
	keyFile, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("创建私钥文件失败: %w", err)
	}
	defer func() { _ = keyFile.Close() }()

	keyDER := x509.MarshalPKCS1PrivateKey(key)
	if err := pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyDER}); err != nil {
		return fmt.Errorf("写入私钥失败: %w", err)
	}

	return nil
}

// saveCertAndECKey 保存证书和 EC 私钥
func saveCertAndECKey(certPath, keyPath string, certDER []byte, key *ecdsa.PrivateKey) error {
	// 保存证书
	certFile, err := os.Create(certPath)
	if err != nil {
		return fmt.Errorf("创建证书文件失败: %w", err)
	}
	defer func() { _ = certFile.Close() }()

	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return fmt.Errorf("写入证书失败: %w", err)
	}

	// 保存私钥
	keyFile, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("创建私钥文件失败: %w", err)
	}
	defer func() { _ = keyFile.Close() }()

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return fmt.Errorf("序列化 EC 私钥失败: %w", err)
	}
	if err := pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}); err != nil {
		return fmt.Errorf("写入私钥失败: %w", err)
	}

	return nil
}

// getUserCertOutputPaths 获取用户证书输出路径
func getUserCertOutputPaths(outputDir, role, username string) (certPath, keyPath string) {
	// 替换 role 中的冒号为短横线
	safeRole := strings.ReplaceAll(role, ":", "-")
	baseName := fmt.Sprintf("%s_%s", safeRole, username)

	if outputDir != "" {
		certPath = filepath.Join(outputDir, baseName+".crt")
		keyPath = filepath.Join(outputDir, baseName+".key")
	} else {
		certPath = baseName + ".crt"
		keyPath = baseName + ".key"
	}
	return
}

// getNodeCertOutputPaths 获取节点证书输出路径
func getNodeCertOutputPaths(outputDir, nodeName string) (certPath, keyPath string) {
	baseName := fmt.Sprintf("node_%s", nodeName)

	if outputDir != "" {
		certPath = filepath.Join(outputDir, baseName+".crt")
		keyPath = filepath.Join(outputDir, baseName+".key")
	} else {
		certPath = baseName + ".crt"
		keyPath = baseName + ".key"
	}
	return
}

// GetUserCertKubeconfigPath 获取用户证书 kubeconfig 输出路径
func GetUserCertKubeconfigPath(outputDir, role, username string) string {
	safeRole := strings.ReplaceAll(role, ":", "-")
	filename := fmt.Sprintf("kubeconfig_%s_%s", safeRole, username)
	if outputDir != "" {
		return filepath.Join(outputDir, filename)
	}
	return filename
}

// GetNodeCertKubeconfigPath 获取节点证书 kubeconfig 输出路径
func GetNodeCertKubeconfigPath(outputDir, nodeName string) string {
	filename := fmt.Sprintf("kubeconfig_node_%s", nodeName)
	if outputDir != "" {
		return filepath.Join(outputDir, filename)
	}
	return filename
}
