package security

import (
	"encoding/json"
	"strings"

	"kctl/config"
	"kctl/pkg/types"
)

// IsSensitivePath 检查路径是否敏感
func IsSensitivePath(path string) bool {
	pathLower := strings.ToLower(path)
	for _, sensitive := range config.SensitivePaths {
		if strings.Contains(pathLower, strings.ToLower(sensitive)) {
			return true
		}
	}
	return false
}

// IsDangerousHostPath 检查是否是危险的主机路径
func IsDangerousHostPath(path string) bool {
	for _, dangerous := range config.DangerousHostPaths {
		if path == dangerous || strings.HasPrefix(path, dangerous+"/") {
			return true
		}
	}
	return false
}

// CheckPrivileged 检查是否有特权容器
func CheckPrivileged(containersJSON string) bool {
	return strings.Contains(containersJSON, `"privileged":true`)
}

// CheckAllowPrivilegeEscalation 检查是否允许权限提升
func CheckAllowPrivilegeEscalation(containersJSON string) bool {
	return strings.Contains(containersJSON, `"allowPrivilegeEscalation":true`)
}

// CheckHostPath 检查是否挂载 HostPath
func CheckHostPath(volumesJSON string) bool {
	return strings.Contains(volumesJSON, `"type":"hostPath"`)
}

// CheckSecretMount 检查是否挂载 Secret
func CheckSecretMount(volumesJSON string) bool {
	return strings.Contains(volumesJSON, `"type":"secret"`) ||
		strings.Contains(volumesJSON, `"type":"projected-secret"`)
}

// CheckRunAsRoot 检查容器是否以 root 用户运行
func CheckRunAsRoot(containersJSON string) bool {
	var containers []types.ContainerInfo
	if err := json.Unmarshal([]byte(containersJSON), &containers); err != nil {
		return false
	}
	for _, c := range containers {
		if c.RunAsUser != nil && *c.RunAsUser == 0 {
			return true
		}
	}
	return false
}

// GetSecurityFlags 获取 Pod 的安全风险标记
func GetSecurityFlags(record *types.PodRecord) types.SecurityFlags {
	return types.SecurityFlags{
		Privileged:               CheckPrivileged(record.Containers),
		AllowPrivilegeEscalation: CheckAllowPrivilegeEscalation(record.Containers),
		HasHostPath:              CheckHostPath(record.Volumes),
		HasSecretMount:           CheckSecretMount(record.Volumes),
	}
}

// GetRiskFlags 获取风险标记字符串列表
func GetRiskFlags(record *types.PodRecord) []string {
	var flags []string

	if CheckPrivileged(record.Containers) {
		flags = append(flags, "PRIV")
	}
	if CheckAllowPrivilegeEscalation(record.Containers) {
		flags = append(flags, "PE")
	}
	if CheckHostPath(record.Volumes) {
		flags = append(flags, "HP")
	}
	if CheckSecretMount(record.Volumes) {
		flags = append(flags, "SEC")
	}
	if CheckRunAsRoot(record.Containers) {
		flags = append(flags, "ROOT")
	}

	return flags
}

// IsPodRisky 检查 Pod 是否有风险
func IsPodRisky(record *types.PodRecord) bool {
	return CheckPrivileged(record.Containers) ||
		CheckAllowPrivilegeEscalation(record.Containers) ||
		CheckHostPath(record.Volumes) ||
		CheckSecretMount(record.Volumes) ||
		CheckRunAsRoot(record.Containers)
}
