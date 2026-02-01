package types

import "time"

// ==================== ServiceAccount 相关类型 ====================

// ServiceAccountRecord 表示存储在数据库中的 ServiceAccount 记录
type ServiceAccountRecord struct {
	ID              int64     `json:"id"`
	Name            string    `json:"name"`            // SA 名称
	Namespace       string    `json:"namespace"`       // 命名空间
	Token           string    `json:"token"`           // Token 内容
	TokenExpiration string    `json:"tokenExpiration"` // Token 过期时间
	IsExpired       bool      `json:"isExpired"`       // 是否已过期
	RiskLevel       string    `json:"riskLevel"`       // 风险等级: CRITICAL, HIGH, MEDIUM, LOW, NONE, ADMIN
	Permissions     string    `json:"permissions"`     // JSON 格式的权限列表
	IsClusterAdmin  bool      `json:"isClusterAdmin"`  // 是否是集群管理员
	SecurityFlags   string    `json:"securityFlags"`   // JSON 格式的安全标识
	Pods            string    `json:"pods"`            // JSON 格式的关联 Pod 列表
	CollectedAt     time.Time `json:"collectedAt"`     // 收集时间
	KubeletIP       string    `json:"kubeletIP"`       // 收集来源 Kubelet IP
}

// SAPermission 存储单个权限信息
type SAPermission struct {
	Resource    string `json:"resource"`
	Verb        string `json:"verb"`
	Group       string `json:"group,omitempty"`
	Subresource string `json:"subresource,omitempty"`
	Allowed     bool   `json:"allowed"`
}

// SASecurityFlags 存储安全标识
type SASecurityFlags struct {
	Privileged               bool `json:"privileged"`
	AllowPrivilegeEscalation bool `json:"allowPrivilegeEscalation"`
	HasHostPath              bool `json:"hasHostPath"`
	HasSecretMount           bool `json:"hasSecretMount"`
	HasSATokenMount          bool `json:"hasSATokenMount"`
}

// SAPodInfo 存储关联的 Pod 信息
type SAPodInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Container string `json:"container"`
}

// ==================== Token 相关类型 ====================

// TokenInfo 表示解析后的 Token 信息
type TokenInfo struct {
	ServiceAccount string
	Namespace      string
	Issuer         string
	Expiration     time.Time
	IsExpired      bool
}
