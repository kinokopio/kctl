package runtime

import (
	"os"

	"kctl/config"
)

// IsInPod 检测是否在 Kubernetes Pod 内运行
func IsInPod() bool {
	// 检查 SA Token 文件
	if _, err := os.Stat(config.DefaultTokenPath); err == nil {
		return true
	}
	// 检查 KUBERNETES_SERVICE_HOST 环境变量
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return true
	}
	return false
}

// GetKubernetesServiceHost 获取 Kubernetes API Server 地址
func GetKubernetesServiceHost() string {
	return os.Getenv("KUBERNETES_SERVICE_HOST")
}

// GetKubernetesServicePort 获取 Kubernetes API Server 端口
func GetKubernetesServicePort() string {
	port := os.Getenv("KUBERNETES_SERVICE_PORT")
	if port == "" {
		return "443"
	}
	return port
}

// GetPodNamespace 获取当前 Pod 所在的命名空间
func GetPodNamespace() string {
	// 尝试从文件读取
	data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err == nil {
		return string(data)
	}
	return "default"
}
