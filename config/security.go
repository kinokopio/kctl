package config

// ==================== 敏感路径规则 ====================

// SensitivePaths 敏感路径关键词列表
// 用于检测容器挂载路径是否包含敏感信息
var SensitivePaths = []string{
	// Token 和凭据相关
	"secret",
	"token",
	"serviceaccount",
	"credential",
	"password",
	"key",

	// 系统敏感目录
	"/etc/",
	"/var/run/",
	"/root",
	"/home",

	// 主机相关
	"/host",
	"/hostfs",

	// 内核相关
	"/proc",
	"/sys",

	// Docker/容器运行时
	"/var/lib/docker",
	"/var/lib/containerd",
	"/var/run/docker.sock",
	"/run/containerd",
}

// DangerousHostPaths 高危主机路径
// 挂载这些路径可能导致容器逃逸或敏感信息泄露
var DangerousHostPaths = []string{
	"/",                    // 根目录
	"/etc",                 // 系统配置
	"/var/run/docker.sock", // Docker socket
	"/var/lib/kubelet",     // Kubelet 数据
	"/var/lib/docker",      // Docker 数据
	"/proc",                // 进程信息
	"/sys",                 // 系统信息
	"/dev",                 // 设备
}

// ==================== 安全上下文检测规则 ====================

// SecurityContextRule 安全上下文检测规则
type SecurityContextRule struct {
	Name        string // 规则名称
	Field       string // 检测字段
	DangerValue any    // 危险值
	Level       string // 风险等级: CRITICAL, HIGH, MEDIUM, LOW
	Description string // 描述
}

// SecurityContextRules 安全上下文检测规则列表
var SecurityContextRules = []SecurityContextRule{
	{
		Name:        "Privileged",
		Field:       "privileged",
		DangerValue: true,
		Level:       "CRITICAL",
		Description: "特权容器，可完全访问主机",
	},
	{
		Name:        "AllowPrivilegeEscalation",
		Field:       "allowPrivilegeEscalation",
		DangerValue: true,
		Level:       "HIGH",
		Description: "允许权限提升",
	},
	{
		Name:        "RunAsRoot",
		Field:       "runAsUser",
		DangerValue: int64(0),
		Level:       "MEDIUM",
		Description: "以 root 用户运行",
	},
	{
		Name:        "HostNetwork",
		Field:       "hostNetwork",
		DangerValue: true,
		Level:       "HIGH",
		Description: "使用主机网络",
	},
	{
		Name:        "HostPID",
		Field:       "hostPID",
		DangerValue: true,
		Level:       "HIGH",
		Description: "使用主机 PID 命名空间",
	},
	{
		Name:        "HostIPC",
		Field:       "hostIPC",
		DangerValue: true,
		Level:       "MEDIUM",
		Description: "使用主机 IPC 命名空间",
	},
}

// ==================== 安全标识配置 ====================

// SecurityFlagConfig 安全标识配置
type SecurityFlagConfig struct {
	Abbrev      string // 简写
	Description string // 描述
	Level       string // 风险等级
}

// SecurityFlagConfigs 安全标识配置映射
var SecurityFlagConfigs = map[string]SecurityFlagConfig{
	"Privileged": {
		Abbrev:      "PRIV",
		Description: "特权容器",
		Level:       "CRITICAL",
	},
	"AllowPrivilegeEscalation": {
		Abbrev:      "PE",
		Description: "允许权限提升",
		Level:       "HIGH",
	},
	"HostPath": {
		Abbrev:      "HP",
		Description: "HostPath 挂载",
		Level:       "HIGH",
	},
	"SecretMount": {
		Abbrev:      "SEC",
		Description: "Secret 挂载",
		Level:       "MEDIUM",
	},
	"RunAsRoot": {
		Abbrev:      "ROOT",
		Description: "以 root 运行",
		Level:       "MEDIUM",
	},
	"HostNetwork": {
		Abbrev:      "HNET",
		Description: "主机网络",
		Level:       "HIGH",
	},
	"HostPID": {
		Abbrev:      "HPID",
		Description: "主机 PID",
		Level:       "HIGH",
	},
	"SATokenMount": {
		Abbrev:      "SA",
		Description: "SA Token 挂载",
		Level:       "LOW",
	},
}
