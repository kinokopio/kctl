package config

import "time"

// ==================== Kubelet 配置 ====================

const (
	// DefaultKubeletPort Kubelet 默认端口
	DefaultKubeletPort = 10250

	// DefaultTokenPath ServiceAccount Token 默认路径
	DefaultTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"

	// DefaultK8sAPIServer K8s API Server 默认地址
	DefaultK8sAPIServer = "https://kubernetes.default.svc"
)

// ==================== 超时配置 ====================

const (
	// DefaultHTTPTimeout HTTP 请求默认超时
	DefaultHTTPTimeout = 30 * time.Second

	// DefaultProbeTimeout 端口探测默认超时
	DefaultProbeTimeout = 5 * time.Second

	// DefaultConnectTimeout 连接默认超时
	DefaultConnectTimeout = 10 * time.Second

	// DefaultWebSocketTimeout WebSocket 握手超时
	DefaultWebSocketTimeout = 30 * time.Second
)

// ==================== 数据库配置 ====================

const (
	// DefaultDBPath 默认数据库路径
	DefaultDBPath = "kubelet_pods.db"
)

// ==================== 扫描配置 ====================

const (
	// DefaultScanConcurrency 默认扫描并发数
	DefaultScanConcurrency = 3

	// DefaultMaxRetries 默认最大重试次数
	DefaultMaxRetries = 3
)

// ==================== 路由表配置 ====================

const (
	// ProcNetRoute Linux 路由表文件路径
	ProcNetRoute = "/proc/net/route"
)
