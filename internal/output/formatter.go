package output

import (
	"fmt"
	"strings"

	"kctl/config"
	"kctl/pkg/types"
)

// Formatter 数据格式化器
type Formatter struct {
	printer Printer
}

// NewFormatter 创建格式化器
func NewFormatter(p Printer) *Formatter {
	return &Formatter{printer: p}
}

// FormatRiskLevel 格式化风险等级
func (f *Formatter) FormatRiskLevel(level config.RiskLevel) string {
	display := config.RiskLevelDisplayConfig[level]
	return fmt.Sprintf("%s %s", display.Symbol, display.Label)
}

// FormatRiskLevelColored 格式化风险等级（带颜色）
func (f *Formatter) FormatRiskLevelColored(level config.RiskLevel) string {
	display := config.RiskLevelDisplayConfig[level]
	text := fmt.Sprintf("%s %s", display.Symbol, display.Label)
	return f.printer.Colored(display.Color, text)
}

// FormatSecurityFlags 格式化安全标识
func (f *Formatter) FormatSecurityFlags(flags types.SecurityFlags) string {
	var parts []string

	if flags.Privileged {
		parts = append(parts, f.formatSecurityFlag("Privileged"))
	}
	if flags.AllowPrivilegeEscalation {
		parts = append(parts, f.formatSecurityFlag("AllowPrivilegeEscalation"))
	}
	if flags.HasHostPath {
		parts = append(parts, f.formatSecurityFlag("HostPath"))
	}
	if flags.HasSecretMount {
		parts = append(parts, f.formatSecurityFlag("SecretMount"))
	}

	return strings.Join(parts, " ")
}

// formatSecurityFlag 格式化单个安全标识
func (f *Formatter) formatSecurityFlag(name string) string {
	display, ok := config.SecurityFlagDisplayConfig[name]
	if !ok {
		return name
	}
	text := fmt.Sprintf("%s%s", display.Symbol, display.Abbrev)
	return f.printer.Colored(display.Color, text)
}

// FormatPodStatus 格式化 Pod 状态
func (f *Formatter) FormatPodStatus(status string) string {
	display, ok := config.PodStatusDisplayConfig[status]
	if !ok {
		display = config.PodStatusDisplayConfig["Unknown"]
	}
	text := fmt.Sprintf("%s %s", display.Symbol, status)
	return f.printer.Colored(display.Color, text)
}

// FormatPermission 格式化权限
func (f *Formatter) FormatPermission(resource, verb, group, subresource string, allowed bool) string {
	var permStr string
	if group != "" {
		permStr = fmt.Sprintf("%s.%s", resource, group)
	} else {
		permStr = resource
	}

	if subresource != "" {
		permStr = fmt.Sprintf("%s/%s", permStr, subresource)
	}

	permStr = fmt.Sprintf("%s [%s]", permStr, verb)

	if allowed {
		return f.printer.Colored(config.ColorGreen, config.Symbols["success"]+permStr)
	}
	return f.printer.Colored(config.ColorGray, config.Symbols["error"]+permStr)
}

// FormatBytes 格式化字节数
func (f *Formatter) FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatDuration 格式化时间
func (f *Formatter) FormatDuration(seconds int64) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%dm%ds", seconds/60, seconds%60)
	}
	return fmt.Sprintf("%dh%dm", seconds/3600, (seconds%3600)/60)
}

// FormatRiskFlags 格式化风险标记列表
func (f *Formatter) FormatRiskFlags(record *types.PodRecord) []string {
	var flags []string

	containers := record.Containers
	volumes := record.Volumes

	if strings.Contains(containers, `"privileged":true`) {
		flags = append(flags, f.printer.Colored(config.ColorRed, "PRIV"))
	}
	if strings.Contains(containers, `"allowPrivilegeEscalation":true`) {
		flags = append(flags, f.printer.Colored(config.ColorYellow, "PE"))
	}
	if strings.Contains(volumes, `"type":"hostPath"`) {
		flags = append(flags, f.printer.Colored(config.ColorRed, "HP"))
	}
	if strings.Contains(volumes, `"type":"secret"`) {
		flags = append(flags, f.printer.Colored(config.ColorYellow, "SEC"))
	}

	return flags
}
