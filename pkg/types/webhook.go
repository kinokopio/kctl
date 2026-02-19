package types

// ==================== Webhook 攻击相关类型 ====================

// WebhookAttackType 攻击类型
type WebhookAttackType string

const (
	WebhookAttackSecretExfil  WebhookAttackType = "secret-exfil"  // Secret 窃取
	WebhookAttackPodBackdoor  WebhookAttackType = "pod-backdoor"  // Pod 后门注入
	WebhookAttackImageReplace WebhookAttackType = "image-replace" // 镜像替换 (后续实现)
	WebhookAttackEnvInject    WebhookAttackType = "env-inject"    // 环境变量注入 (后续实现)
	WebhookAttackTokenExfil   WebhookAttackType = "token-exfil"   // SA Token 窃取 (后续实现)
)

// String 返回攻击类型的字符串表示
func (w WebhookAttackType) String() string {
	return string(w)
}

// WebhookInjectOptions 注入选项
type WebhookInjectOptions struct {
	AttackType        WebhookAttackType // 攻击类型
	Namespace         string            // Webhook 服务部署命名空间
	WebhookName       string            // Webhook 配置名称
	ServiceName       string            // Webhook 服务名称
	ExfilURL          string            // 外泄目标 URL (secret-exfil, token-exfil)
	BackdoorImage     string            // 后门容器镜像 (pod-backdoor)
	BackdoorCommand   []string          // 后门容器命令 (pod-backdoor)
	BackdoorName      string            // 后门容器名称 (pod-backdoor)
	TargetNamespaces  []string          // 目标命名空间 (空=所有命名空间)
	ExcludeNamespaces []string          // 排除的命名空间
	WebhookImage      string            // Webhook 服务镜像 (可选，默认 ghcr.io/kctl/webhook)
	ExternalURL       string            // 外部 Webhook URL (不部署服务，直接使用外部 URL)
	CertDir           string            // 证书保存目录 (外部模式下保存证书文件)
	DryRun            bool              // 仅预览，不执行
	ServerURL         string            // API Server URL (可选覆盖)
	Token             string            // 认证 Token (可选覆盖)
}

// WebhookInfo Webhook 配置信息
type WebhookInfo struct {
	Name            string            // Webhook 名称
	Type            string            // Mutating/Validating
	ServiceNS       string            // 服务命名空间
	ServiceName     string            // 服务名称
	ServicePort     int32             // 服务端口
	Path            string            // Webhook 路径
	FailurePolicy   string            // 失败策略
	Rules           []WebhookRule     // 规则列表
	Labels          map[string]string // 标签
	IsManagedByKctl bool              // 是否由 kctl 管理
}

// WebhookRule Webhook 规则
type WebhookRule struct {
	Operations []string // 操作类型 (CREATE, UPDATE, DELETE)
	APIGroups  []string // API 组
	Resources  []string // 资源类型
}

// WebhookCert Webhook TLS 证书
type WebhookCert struct {
	CACert     []byte // CA 证书 (用于 WebhookConfiguration)
	ServerCert []byte // 服务端证书
	ServerKey  []byte // 服务端私钥
}

// WebhookDeployment Webhook 部署信息
type WebhookDeployment struct {
	Namespace   string       // 命名空间
	ServiceName string       // 服务名称
	SecretName  string       // 证书 Secret 名称
	Deployment  string       // Deployment 名称
	WebhookName string       // WebhookConfiguration 名称
	ExternalURL string       // 外部 URL (如果使用外部模式)
	Cert        *WebhookCert // 生成的证书 (外部模式下供用户配置服务器使用)
}

// WebhookAttackTemplate 攻击模板
type WebhookAttackTemplate struct {
	Type        WebhookAttackType // 攻击类型
	Name        string            // 显示名称
	Description string            // 描述
	Operations  []string          // 拦截的操作
	Resources   []string          // 拦截的资源
	APIGroups   []string          // API 组
}

// WebhookAttackTemplates 攻击模板定义
var WebhookAttackTemplates = []WebhookAttackTemplate{
	{
		Type:        WebhookAttackSecretExfil,
		Name:        "Secret 窃取",
		Description: "拦截 Secret 创建/更新，将内容外泄到指定服务器",
		Operations:  []string{"CREATE", "UPDATE"},
		Resources:   []string{"secrets"},
		APIGroups:   []string{""},
	},
	{
		Type:        WebhookAttackPodBackdoor,
		Name:        "Pod 后门注入",
		Description: "在 Pod 创建时自动注入恶意 sidecar 容器",
		Operations:  []string{"CREATE"},
		Resources:   []string{"pods"},
		APIGroups:   []string{""},
	},
}

// GetWebhookAttackTemplate 获取攻击模板
func GetWebhookAttackTemplate(attackType WebhookAttackType) *WebhookAttackTemplate {
	for _, t := range WebhookAttackTemplates {
		if t.Type == attackType {
			return &t
		}
	}
	return nil
}
