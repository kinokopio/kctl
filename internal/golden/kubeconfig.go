package golden

import (
	"encoding/base64"
	"fmt"
	"os"

	"kctl/pkg/types"
)

// kubeconfigCertTemplate 证书认证的 kubeconfig 模板 (带 CA 证书)
// nolint:unused // 保留用于未来支持安全模式
const kubeconfigCertTemplate = `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: %s
    server: %s
  name: kubernetes
contexts:
- context:
    cluster: kubernetes
    user: %s
  name: %s@kubernetes
current-context: %s@kubernetes
kind: Config
preferences: {}
users:
- name: %s
  user:
    client-certificate-data: %s
    client-key-data: %s
`

// kubeconfigCertInsecureTemplate 证书认证的 kubeconfig 模板 (跳过 TLS 验证)
const kubeconfigCertInsecureTemplate = `apiVersion: v1
clusters:
- cluster:
    insecure-skip-tls-verify: true
    server: %s
  name: kubernetes
contexts:
- context:
    cluster: kubernetes
    user: %s
  name: %s@kubernetes
current-context: %s@kubernetes
kind: Config
preferences: {}
users:
- name: %s
  user:
    client-certificate-data: %s
    client-key-data: %s
`

// kubeconfigTokenTemplate Token 认证的 kubeconfig 模板
const kubeconfigTokenTemplate = `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: %s
    server: %s
  name: kubernetes
contexts:
- context:
    cluster: kubernetes
    user: %s
  name: %s@kubernetes
current-context: %s@kubernetes
kind: Config
preferences: {}
users:
- name: %s
  user:
    token: %s
`

// kubeconfigTokenInsecureTemplate Token 认证的 kubeconfig 模板 (无 CA 证书)
const kubeconfigTokenInsecureTemplate = `apiVersion: v1
clusters:
- cluster:
    insecure-skip-tls-verify: true
    server: %s
  name: kubernetes
contexts:
- context:
    cluster: kubernetes
    user: %s
  name: %s@kubernetes
current-context: %s@kubernetes
kind: Config
preferences: {}
users:
- name: %s
  user:
    token: %s
`

// GenerateKubeconfigFromCert 从证书生成 kubeconfig
// 注意：默认使用 insecure 模式，因为签发客户端证书的 CA 可能与 API Server 的 TLS CA 不同
func GenerateKubeconfigFromCert(result *types.ForgeResult, caCertPath, serverURL, outputPath string, force bool) error {
	if serverURL == "" {
		return fmt.Errorf("未指定 API Server URL")
	}

	// 检查文件是否已存在
	if err := CheckFileOverwrite(outputPath, force); err != nil {
		return err
	}

	// 读取客户端证书
	certData, err := os.ReadFile(result.CertPath)
	if err != nil {
		return fmt.Errorf("读取客户端证书失败: %w", err)
	}
	certB64 := base64.StdEncoding.EncodeToString(certData)

	// 读取客户端私钥
	keyData, err := os.ReadFile(result.KeyPath)
	if err != nil {
		return fmt.Errorf("读取客户端私钥失败: %w", err)
	}
	keyB64 := base64.StdEncoding.EncodeToString(keyData)

	username := result.Identity

	// 默认使用 insecure 模式，因为 client CA 和 server CA 可能不同
	// 如果用户需要安全模式，可以手动编辑 kubeconfig 添加正确的 server CA
	content := fmt.Sprintf(kubeconfigCertInsecureTemplate,
		serverURL,
		username,
		username,
		username,
		username,
		certB64,
		keyB64,
	)

	// 写入文件
	if err := os.WriteFile(outputPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("写入 kubeconfig 失败: %w", err)
	}

	result.KubeconfigPath = outputPath
	return nil
}

// GenerateKubeconfigFromToken 从 Token 生成 kubeconfig
func GenerateKubeconfigFromToken(result *types.ForgeResult, caCertPath, serverURL, outputPath string, force bool) error {
	if serverURL == "" {
		return fmt.Errorf("未指定 API Server URL")
	}

	// 检查文件是否已存在
	if err := CheckFileOverwrite(outputPath, force); err != nil {
		return err
	}

	var content string
	username := result.Identity

	// 如果提供了 CA 证书，使用安全模式
	if caCertPath != "" && FileExists(caCertPath) {
		caCertData, err := os.ReadFile(caCertPath)
		if err != nil {
			return fmt.Errorf("读取 CA 证书失败: %w", err)
		}
		caCertB64 := base64.StdEncoding.EncodeToString(caCertData)

		content = fmt.Sprintf(kubeconfigTokenTemplate,
			caCertB64,
			serverURL,
			username,
			username,
			username,
			username,
			result.Token,
		)
	} else {
		// 无 CA 证书，使用不安全模式
		content = fmt.Sprintf(kubeconfigTokenInsecureTemplate,
			serverURL,
			username,
			username,
			username,
			username,
			result.Token,
		)
	}

	// 写入文件
	if err := os.WriteFile(outputPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("写入 kubeconfig 失败: %w", err)
	}

	result.KubeconfigPath = outputPath
	return nil
}
