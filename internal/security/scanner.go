package security

import (
	"encoding/json"
	"strings"

	"kctl/pkg/types"
)

// PodSecuritySummary 计算 Pod 安全摘要
func CalculatePodSecuritySummary(records []*types.PodRecord) *types.PodSecuritySummary {
	summary := &types.PodSecuritySummary{
		TotalPods:       len(records),
		Namespaces:      make(map[string]int),
		ServiceAccounts: make(map[string]int),
	}

	for _, r := range records {
		// 统计命名空间
		summary.Namespaces[r.Namespace]++

		// 统计 ServiceAccount
		if r.ServiceAccount != "" {
			summary.ServiceAccounts[r.ServiceAccount]++
		}

		// 检查安全风险
		if CheckPrivileged(r.Containers) || CheckAllowPrivilegeEscalation(r.Containers) {
			summary.PrivilegedCount++
		}
		if CheckSecretMount(r.Volumes) {
			summary.SecretsCount++
		}
		if CheckHostPath(r.Volumes) {
			summary.HostPathCount++
		}
		if IsPodRisky(r) {
			summary.RiskyPodCount++
		}
	}

	summary.NamespaceCount = len(summary.Namespaces)
	summary.SACount = len(summary.ServiceAccounts)

	return summary
}

// AggregateSecrets 聚合所有 Pod 的 Secret 挂载信息
// 返回 map[secretName][]podFullName
func AggregateSecrets(records []*types.PodRecord) map[string][]string {
	secretMap := make(map[string][]string)

	for _, r := range records {
		podFullName := r.Namespace + "/" + r.Name
		volumes := GetSensitiveVolumes(r.Volumes)

		for _, v := range volumes {
			if v.Type == "secret" || v.Type == "projected-secret" {
				if v.SecretName != "" {
					secretMap[v.SecretName] = append(secretMap[v.SecretName], podFullName)
				}
			}
		}
	}
	return secretMap
}

// AggregateHostPaths 聚合所有 Pod 的 HostPath 挂载信息
// 返回 map[hostPath][]podFullName
func AggregateHostPaths(records []*types.PodRecord) map[string][]string {
	hostPathMap := make(map[string][]string)

	for _, r := range records {
		podFullName := r.Namespace + "/" + r.Name
		volumes := GetSensitiveVolumes(r.Volumes)

		for _, v := range volumes {
			if v.Type == "hostPath" && v.HostPath != "" {
				hostPathMap[v.HostPath] = append(hostPathMap[v.HostPath], podFullName)
			}
		}
	}
	return hostPathMap
}

// GetSensitiveVolumes 解析敏感卷信息
func GetSensitiveVolumes(volumesJSON string) []types.SensitiveVolume {
	if volumesJSON == "" {
		return nil
	}
	var volumes []types.SensitiveVolume
	if err := json.Unmarshal([]byte(volumesJSON), &volumes); err != nil {
		return nil
	}
	return volumes
}

// ClassifyVolumes 对卷进行分类
func ClassifyVolumes(volumesJSON string) *types.VolumeClassification {
	volumes := GetSensitiveVolumes(volumesJSON)
	if volumes == nil {
		return nil
	}

	classification := &types.VolumeClassification{}
	for _, v := range volumes {
		switch v.Type {
		case "secret", "projected-secret":
			classification.Secrets = append(classification.Secrets, v)
		case "hostPath":
			classification.HostPaths = append(classification.HostPaths, v)
		case "configMap":
			classification.ConfigMaps = append(classification.ConfigMaps, v)
		case "projected-sa-token":
			classification.SATokens = append(classification.SATokens, v)
		case "emptyDir":
			classification.EmptyDirs = append(classification.EmptyDirs, v)
		default:
			classification.Others = append(classification.Others, v)
		}
	}
	return classification
}

// GetContainerSecurityInfo 解析容器安全上下文
func GetContainerSecurityInfo(containersJSON string) []types.ContainerSecurityInfo {
	var containers []types.ContainerInfo
	if err := json.Unmarshal([]byte(containersJSON), &containers); err != nil {
		return nil
	}

	var result []types.ContainerSecurityInfo
	for _, c := range containers {
		info := types.ContainerSecurityInfo{
			Name:                     c.Name,
			Image:                    c.Image,
			Privileged:               c.Privileged,
			AllowPrivilegeEscalation: c.AllowPrivilegeEscalation,
			ReadOnlyRootFilesystem:   c.ReadOnlyRootFilesystem,
			VolumeMounts:             c.VolumeMounts,
		}
		if c.RunAsUser != nil {
			info.RunAsUser = c.RunAsUser
			info.RunAsRoot = *c.RunAsUser == 0
		}
		if c.RunAsGroup != nil {
			info.RunAsGroup = c.RunAsGroup
		}

		// 检查敏感挂载路径
		for _, mp := range c.VolumeMounts {
			if IsSensitivePath(mp) {
				info.SensitiveMounts = append(info.SensitiveMounts, mp)
			}
		}

		result = append(result, info)
	}
	return result
}

// CheckSensitivePathsInRecord 检查记录中是否有敏感路径
func CheckSensitivePathsInRecord(record *types.PodRecord) bool {
	var containers []types.ContainerInfo
	if err := json.Unmarshal([]byte(record.Containers), &containers); err != nil {
		return false
	}
	for _, c := range containers {
		for _, mp := range c.VolumeMounts {
			if IsSensitivePath(mp) {
				return true
			}
		}
	}
	return false
}

// ParseContainers 解析容器 JSON 字符串
func ParseContainers(containersJSON string) ([]types.ContainerInfo, error) {
	if containersJSON == "" {
		return nil, nil
	}
	var containers []types.ContainerInfo
	err := json.Unmarshal([]byte(containersJSON), &containers)
	return containers, err
}

// ParseVolumes 解析卷 JSON 字符串
func ParseVolumes(volumesJSON string) ([]types.SensitiveVolume, error) {
	if volumesJSON == "" {
		return nil, nil
	}
	var volumes []types.SensitiveVolume
	err := json.Unmarshal([]byte(volumesJSON), &volumes)
	return volumes, err
}

// FormatRiskFlagsColored 格式化风险标记（带颜色标识）
func FormatRiskFlagsColored(record *types.PodRecord) string {
	flags := GetRiskFlags(record)
	if len(flags) == 0 {
		return ""
	}
	return strings.Join(flags, ",")
}
