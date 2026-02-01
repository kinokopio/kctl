package output

import (
	"fmt"
	"strings"

	"kctl/config"
	"kctl/pkg/types"
)

// ExecInfo 执行命令信息
type ExecInfo struct {
	Target   string // namespace/pod/container
	Command  string
	Endpoint string // ip:port
}

// PrintExecInfo 打印执行信息
func PrintExecInfo(p Printer, info ExecInfo) {
	tablePrinter := NewTablePrinter()

	p.Println()
	p.Section("执行信息")

	items := []SummaryItem{
		{Label: "目标", Value: info.Target},
		{Label: "命令", Value: p.Colored(config.ColorYellow, info.Command)},
		{Label: "Kubelet", Value: info.Endpoint},
	}

	tablePrinter.PrintSummary("", items)
	p.Println()
}

// PrintUsageExample 打印使用示例
func PrintUsageExample(p Printer, title string, examples []string) {
	p.Println()
	p.PrintColoredln(config.ColorCyan, fmt.Sprintf("%s:", title))
	for _, ex := range examples {
		p.Printf("  %s\n", ex)
	}
}

// PrintPrompt 打印输入提示
func PrintPrompt(p Printer, prompt string) {
	p.PrintColored(config.ColorCyan, prompt)
}

// PrintInteractiveHint 打印交互式操作提示
func PrintInteractiveHint(p Printer, hint string) {
	p.Println()
	p.PrintColoredln(config.ColorYellow, hint)
	p.Println()
}

// PrintPodSummary 打印 Pod 安全摘要
func PrintPodSummary(p Printer, summary *types.PodSecuritySummary) {
	tablePrinter := NewTablePrinter()

	items := []SummaryItem{
		{Label: "总计 Pod", Value: fmt.Sprintf("%d", summary.TotalPods)},
		{Label: "命名空间", Value: fmt.Sprintf("%d", summary.NamespaceCount)},
		{Label: "ServiceAccount", Value: fmt.Sprintf("%d", summary.SACount)},
	}

	if summary.PrivilegedCount > 0 {
		items = append(items, SummaryItem{
			Label: p.Colored(config.ColorRed, "★ 特权/可提权容器"),
			Value: p.Colored(config.ColorRed, fmt.Sprintf("%d", summary.PrivilegedCount)),
		})
	}
	if summary.SecretsCount > 0 {
		items = append(items, SummaryItem{
			Label: p.Colored(config.ColorYellow, "◆ 挂载 Secret"),
			Value: p.Colored(config.ColorYellow, fmt.Sprintf("%d", summary.SecretsCount)),
		})
	}
	if summary.HostPathCount > 0 {
		items = append(items, SummaryItem{
			Label: p.Colored(config.ColorRed, "◆ 挂载 HostPath"),
			Value: p.Colored(config.ColorRed, fmt.Sprintf("%d", summary.HostPathCount)),
		})
	}

	tablePrinter.PrintSummary("Pod 收集摘要", items)
}

// PrintScanResult 打印扫描结果
func PrintScanResult(p Printer, result *types.SATokenScanResult) {
	// 风险等级标签
	var riskLabel string
	if result.IsClusterAdmin {
		riskLabel = p.Colored(config.ColorRed, "ADMIN")
	} else {
		display := config.RiskLevelDisplayConfig[result.RiskLevel]
		riskLabel = p.Colored(display.Color, display.Label)
	}

	// Token 状态
	tokenStatus := p.Colored(config.ColorGreen, "有效")
	if result.TokenInfo != nil && result.TokenInfo.IsExpired {
		tokenStatus = p.Colored(config.ColorRed, "已过期")
	}

	// 关键权限
	keyPerms := getKeyPerms(p, result.Permissions)
	if result.IsClusterAdmin {
		keyPerms = p.Colored(config.ColorRed, "*/*")
	}

	// 权限数量
	permCount := fmt.Sprintf("%d", countAllowed(result.Permissions))
	if result.IsClusterAdmin {
		permCount = "*"
	}

	p.Printf("  %-8s %-15s %-25s %-20s %-6s %-8s %s\n",
		riskLabel,
		truncateStr(result.Namespace, 15),
		truncateStr(result.PodName, 25),
		truncateStr(result.ServiceAccount, 20),
		permCount,
		tokenStatus,
		keyPerms,
	)
}

// truncateStr 截断字符串
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-2] + ".."
}

// getKeyPerms 获取关键权限
func getKeyPerms(p Printer, permissions []types.PermissionCheck) string {
	var critical, high []string

	for _, perm := range permissions {
		if !perm.Allowed {
			continue
		}

		resource := perm.Resource
		if perm.Subresource != "" {
			resource = perm.Resource + "/" + perm.Subresource
		}

		if _, ok := config.CriticalPermissions[resource]; ok {
			critical = append(critical, resource)
		} else if _, ok := config.HighPermissions[resource]; ok {
			high = append(high, resource)
		}
	}

	if len(critical) > 0 {
		if len(critical) > 2 {
			return strings.Join(critical[:2], ",") + "..."
		}
		return strings.Join(critical, ",")
	}
	if len(high) > 0 {
		if len(high) > 2 {
			return strings.Join(high[:2], ",") + "..."
		}
		return strings.Join(high, ",")
	}
	return "-"
}

// countAllowed 统计允许的权限数量
func countAllowed(permissions []types.PermissionCheck) int {
	count := 0
	for _, p := range permissions {
		if p.Allowed {
			count++
		}
	}
	return count
}
