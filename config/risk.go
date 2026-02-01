package config

// ==================== 风险等级定义 ====================

// RiskLevel 风险等级
type RiskLevel string

const (
	RiskAdmin    RiskLevel = "ADMIN"    // 集群管理员
	RiskCritical RiskLevel = "CRITICAL" // 高危
	RiskHigh     RiskLevel = "HIGH"     // 危险
	RiskMedium   RiskLevel = "MEDIUM"   // 中危
	RiskLow      RiskLevel = "LOW"      // 低危
	RiskNone     RiskLevel = "NONE"     // 无风险
)

// RiskLevelOrder 风险等级排序（用于排序，数字越小优先级越高）
var RiskLevelOrder = map[RiskLevel]int{
	RiskAdmin:    0,
	RiskCritical: 1,
	RiskHigh:     2,
	RiskMedium:   3,
	RiskLow:      4,
	RiskNone:     5,
}

// ==================== 权限敏感级别 ====================

// PermissionLevel 权限敏感级别
type PermissionLevel int

const (
	PermLevelNormal    PermissionLevel = iota // 普通权限
	PermLevelSensitive                        // 敏感权限
	PermLevelDangerous                        // 危险权限
	PermLevelAdmin                            // 管理员权限
)

// PermissionLevelNames 权限级别名称
var PermissionLevelNames = map[PermissionLevel]string{
	PermLevelAdmin:     "管理员",
	PermLevelDangerous: "危险",
	PermLevelSensitive: "敏感",
	PermLevelNormal:    "普通",
}

// ==================== 权限风险规则 ====================

// PermissionRiskRule 权限风险规则
type PermissionRiskRule struct {
	Resource    string          // 资源，"*" 表示任意
	Verb        string          // 操作，"*" 表示任意
	Group       string          // API Group，"*" 表示任意
	Subresource string          // 子资源，"*" 表示任意
	Level       PermissionLevel // 敏感级别
	Description string          // 描述
}

// PermissionRiskRules 权限风险规则列表
// 按优先级从高到低排序，匹配到第一个规则即返回
var PermissionRiskRules = []PermissionRiskRule{
	// ==================== ADMIN 级别 ====================
	// 通配符权限 - 集群管理员
	{"*", "*", "*", "", PermLevelAdmin, "集群管理员权限 (cluster-admin)"},
	{"*", "*", "", "", PermLevelAdmin, "全资源管理权限"},

	// RBAC 权限提升 - 可以给自己或他人授权
	{"clusterroles", "create", "rbac.authorization.k8s.io", "", PermLevelAdmin, "可创建集群角色"},
	{"clusterroles", "update", "rbac.authorization.k8s.io", "", PermLevelAdmin, "可修改集群角色"},
	{"clusterroles", "patch", "rbac.authorization.k8s.io", "", PermLevelAdmin, "可修补集群角色"},
	{"clusterroles", "bind", "rbac.authorization.k8s.io", "", PermLevelAdmin, "可绑定集群角色"},
	{"clusterroles", "escalate", "rbac.authorization.k8s.io", "", PermLevelAdmin, "可提升集群角色权限"},
	{"clusterrolebindings", "create", "rbac.authorization.k8s.io", "", PermLevelAdmin, "可创建集群角色绑定"},
	{"clusterrolebindings", "update", "rbac.authorization.k8s.io", "", PermLevelAdmin, "可修改集群角色绑定"},
	{"roles", "create", "rbac.authorization.k8s.io", "", PermLevelAdmin, "可创建角色"},
	{"roles", "update", "rbac.authorization.k8s.io", "", PermLevelAdmin, "可修改角色"},
	{"roles", "bind", "rbac.authorization.k8s.io", "", PermLevelAdmin, "可绑定角色"},
	{"roles", "escalate", "rbac.authorization.k8s.io", "", PermLevelAdmin, "可提升角色权限"},
	{"rolebindings", "create", "rbac.authorization.k8s.io", "", PermLevelAdmin, "可创建角色绑定"},
	{"rolebindings", "update", "rbac.authorization.k8s.io", "", PermLevelAdmin, "可修改角色绑定"},

	// ==================== DANGEROUS 级别 ====================
	// Pod 执行权限 - 可以在容器内执行命令
	{"pods", "create", "", "exec", PermLevelDangerous, "可在 Pod 内执行命令"},
	{"pods", "get", "", "exec", PermLevelDangerous, "可在 Pod 内执行命令"},
	{"pods", "*", "", "exec", PermLevelDangerous, "可在 Pod 内执行命令"},

	// Pod attach 权限 - 可以连接到容器
	{"pods", "create", "", "attach", PermLevelDangerous, "可连接到 Pod 容器"},
	{"pods", "get", "", "attach", PermLevelDangerous, "可连接到 Pod 容器"},
	{"pods", "*", "", "attach", PermLevelDangerous, "可连接到 Pod 容器"},

	// Pod portforward 权限
	{"pods", "create", "", "portforward", PermLevelDangerous, "可转发 Pod 端口"},
	{"pods", "get", "", "portforward", PermLevelDangerous, "可转发 Pod 端口"},

	// Node proxy 权限 - 可以访问 Kubelet API
	{"nodes", "get", "", "proxy", PermLevelDangerous, "可访问节点 Kubelet API"},
	{"nodes", "create", "", "proxy", PermLevelDangerous, "可访问节点 Kubelet API"},
	{"nodes", "*", "", "proxy", PermLevelDangerous, "可访问节点 Kubelet API"},

	// ServiceAccount Token 创建 - 可以伪造身份
	{"serviceaccounts", "create", "", "token", PermLevelDangerous, "可创建 ServiceAccount Token"},
	{"serviceaccounts", "*", "", "token", PermLevelDangerous, "可创建 ServiceAccount Token"},

	// CSR 权限 - 可以签发证书
	{"certificatesigningrequests", "create", "certificates.k8s.io", "", PermLevelDangerous, "可创建证书签名请求"},
	{"certificatesigningrequests", "update", "certificates.k8s.io", "approval", PermLevelDangerous, "可批准证书签名请求"},

	// Webhook 配置 - 可以拦截 API 请求
	{"mutatingwebhookconfigurations", "create", "admissionregistration.k8s.io", "", PermLevelDangerous, "可创建变更 Webhook"},
	{"mutatingwebhookconfigurations", "update", "admissionregistration.k8s.io", "", PermLevelDangerous, "可修改变更 Webhook"},
	{"validatingwebhookconfigurations", "create", "admissionregistration.k8s.io", "", PermLevelDangerous, "可创建验证 Webhook"},
	{"validatingwebhookconfigurations", "update", "admissionregistration.k8s.io", "", PermLevelDangerous, "可修改验证 Webhook"},

	// ==================== SENSITIVE 级别 ====================
	// Secrets - 可能包含凭据、密钥等
	{"secrets", "get", "", "", PermLevelSensitive, "可获取 Secret 内容"},
	{"secrets", "list", "", "", PermLevelSensitive, "可列出 Secrets"},
	{"secrets", "watch", "", "", PermLevelSensitive, "可监听 Secrets 变化"},
	{"secrets", "create", "", "", PermLevelSensitive, "可创建 Secrets"},
	{"secrets", "update", "", "", PermLevelSensitive, "可更新 Secrets"},
	{"secrets", "delete", "", "", PermLevelSensitive, "可删除 Secrets"},
	{"secrets", "*", "", "", PermLevelSensitive, "Secret 完全访问权限"},

	// Pod 日志 - 可能包含敏感信息
	{"pods", "get", "", "log", PermLevelSensitive, "可查看 Pod 日志"},
	{"pods", "*", "", "log", PermLevelSensitive, "可查看 Pod 日志"},

	// Pod 创建/删除 - 可以部署恶意工作负载
	{"pods", "create", "", "", PermLevelSensitive, "可创建 Pod"},
	{"pods", "delete", "", "", PermLevelSensitive, "可删除 Pod"},
	{"pods", "update", "", "", PermLevelSensitive, "可更新 Pod"},
	{"pods", "patch", "", "", PermLevelSensitive, "可修补 Pod"},

	// Deployments/DaemonSets/StatefulSets 创建 - 可以部署工作负载
	{"deployments", "create", "apps", "", PermLevelSensitive, "可创建 Deployment"},
	{"deployments", "update", "apps", "", PermLevelSensitive, "可更新 Deployment"},
	{"deployments", "delete", "apps", "", PermLevelSensitive, "可删除 Deployment"},
	{"daemonsets", "create", "apps", "", PermLevelSensitive, "可创建 DaemonSet"},
	{"daemonsets", "update", "apps", "", PermLevelSensitive, "可更新 DaemonSet"},
	{"daemonsets", "delete", "apps", "", PermLevelSensitive, "可删除 DaemonSet"},
	{"statefulsets", "create", "apps", "", PermLevelSensitive, "可创建 StatefulSet"},
	{"statefulsets", "update", "apps", "", PermLevelSensitive, "可更新 StatefulSet"},
	{"replicasets", "create", "apps", "", PermLevelSensitive, "可创建 ReplicaSet"},
	{"jobs", "create", "batch", "", PermLevelSensitive, "可创建 Job"},
	{"cronjobs", "create", "batch", "", PermLevelSensitive, "可创建 CronJob"},

	// ServiceAccount 创建/修改
	{"serviceaccounts", "create", "", "", PermLevelSensitive, "可创建 ServiceAccount"},
	{"serviceaccounts", "update", "", "", PermLevelSensitive, "可更新 ServiceAccount"},

	// PV/PVC - 可能访问持久化数据
	{"persistentvolumes", "create", "", "", PermLevelSensitive, "可创建 PersistentVolume"},
	{"persistentvolumes", "update", "", "", PermLevelSensitive, "可更新 PersistentVolume"},
	{"persistentvolumeclaims", "create", "", "", PermLevelSensitive, "可创建 PersistentVolumeClaim"},

	// RBAC 读取权限
	{"clusterroles", "list", "rbac.authorization.k8s.io", "", PermLevelSensitive, "可列出集群角色"},
	{"clusterroles", "get", "rbac.authorization.k8s.io", "", PermLevelSensitive, "可获取集群角色"},
	{"clusterrolebindings", "list", "rbac.authorization.k8s.io", "", PermLevelSensitive, "可列出集群角色绑定"},
	{"roles", "list", "rbac.authorization.k8s.io", "", PermLevelSensitive, "可列出角色"},
	{"rolebindings", "list", "rbac.authorization.k8s.io", "", PermLevelSensitive, "可列出角色绑定"},

	// Endpoints/Services - 服务发现信息
	{"endpoints", "list", "", "", PermLevelSensitive, "可列出服务端点"},
	{"endpointslices", "list", "discovery.k8s.io", "", PermLevelSensitive, "可列出服务端点切片"},
}

// ==================== 高危权限快速查找表 ====================
// 用于 scan 命令快速判断风险等级

// CriticalPermissions 高危权限定义
var CriticalPermissions = map[string][]string{
	"*":                   {"*"},                                                  // 所有资源所有操作
	"secrets":             {"get", "list", "watch", "create", "*"},                // secrets 读写
	"pods":                {"create", "*"},                                        // 创建 Pod
	"pods/exec":           {"create", "*"},                                        // Pod exec
	"clusterroles":        {"create", "update", "patch", "bind", "escalate", "*"}, // RBAC 修改
	"clusterrolebindings": {"create", "update", "patch", "*"},
	"roles":               {"create", "update", "patch", "bind", "escalate", "*"},
	"rolebindings":        {"create", "update", "patch", "*"},
	"serviceaccounts":     {"create", "impersonate", "*"},
	"nodes":               {"proxy", "*"},
	"nodes/proxy":         {"create", "get", "*"},
}

// HighPermissions 高危权限定义
var HighPermissions = map[string][]string{
	"configmaps":             {"get", "list", "create", "update", "*"},
	"deployments":            {"create", "update", "patch", "*"},
	"daemonsets":             {"create", "update", "patch", "*"},
	"cronjobs":               {"create", "update", "*"},
	"jobs":                   {"create", "*"},
	"pods/log":               {"get", "*"},
	"persistentvolumeclaims": {"create", "*"},
	"persistentvolumes":      {"create", "*"},
	"serviceaccounts/token":  {"create", "*"},
}

// MediumPermissions 中危权限定义
var MediumPermissions = map[string][]string{
	"services":        {"create", "update", "*"},
	"endpoints":       {"create", "update", "*"},
	"ingresses":       {"create", "update", "*"},
	"networkpolicies": {"create", "update", "delete", "*"},
}

// PrivilegeEquivalentPermissions 等同于特权的权限
// 这些权限虽然不是容器特权，但可以实现类似特权的效果
var PrivilegeEquivalentPermissions = map[string][]string{
	"nodes/proxy":           {"get", "create", "*"}, // 可通过 Kubelet API 执行任意命令
	"pods/exec":             {"create", "*"},        // 可在 Pod 内执行命令
	"pods/attach":           {"create", "*"},        // 可连接到 Pod 容器
	"*":                     {"*"},                  // 集群管理员
	"serviceaccounts/token": {"create", "*"},        // 可创建任意 SA 的 Token
	"clusterroles":          {"bind", "escalate"},   // 可提升权限
	"roles":                 {"bind", "escalate"},   // 可提升权限
	"clusterrolebindings":   {"create", "update"},   // 可绑定任意角色
	"rolebindings":          {"create", "update"},   // 可绑定任意角色
}

// IsPrivilegeEquivalent 检查权限是否等同于特权
func IsPrivilegeEquivalent(resource, verb string) bool {
	if verbs, ok := PrivilegeEquivalentPermissions[resource]; ok {
		for _, v := range verbs {
			if v == "*" || v == verb {
				return true
			}
		}
	}
	return false
}

// IsCriticalPermission 检查是否是高危权限
func IsCriticalPermission(resource, verb string) bool {
	if verbs, ok := CriticalPermissions[resource]; ok {
		for _, v := range verbs {
			if v == "*" || v == verb {
				return true
			}
		}
	}
	return false
}

// IsHighPermission 检查是否是高危权限
func IsHighPermission(resource, verb string) bool {
	if verbs, ok := HighPermissions[resource]; ok {
		for _, v := range verbs {
			if v == "*" || v == verb {
				return true
			}
		}
	}
	return false
}
