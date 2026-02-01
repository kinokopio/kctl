package types

import "kctl/config"

// ==================== 权限检查相关类型 ====================

// PermissionCheck 表示权限检查结果
type PermissionCheck struct {
	Resource    string
	Verb        string
	Allowed     bool
	Group       string // API Group (e.g., "", "apps", "rbac.authorization.k8s.io")
	Subresource string // 子资源 (e.g., "proxy", "exec", "log")
}

// PermissionCheckResult 权限检查结果（带风险信息）
type PermissionCheckResult struct {
	PermissionCheck
	Level       config.PermissionLevel
	Description string
}

// ==================== 风险评估相关类型 ====================

// RiskAssessment 风险评估结果
type RiskAssessment struct {
	Level          config.RiskLevel
	IsClusterAdmin bool
	AdminPerms     []PermissionCheckResult
	DangerousPerms []PermissionCheckResult
	SensitivePerms []PermissionCheckResult
	NormalPerms    []PermissionCheckResult
}

// ==================== 扫描结果类型 ====================

// SATokenScanResult SA Token 扫描结果
type SATokenScanResult struct {
	Namespace      string
	PodName        string
	Container      string
	ServiceAccount string
	Token          string
	TokenInfo      *TokenInfo
	Permissions    []PermissionCheck
	SecurityFlags  SecurityFlags
	RiskLevel      config.RiskLevel
	IsClusterAdmin bool
	Error          string
}
