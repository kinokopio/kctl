package output

import (
	"fmt"
	"strings"

	"kctl/config"
	"kctl/pkg/types"
)

// ListPrinter 列表打印器
type ListPrinter struct {
	printer   Printer
	formatter *Formatter
}

// NewListPrinter 创建列表打印器
func NewListPrinter(p Printer) *ListPrinter {
	return &ListPrinter{
		printer:   p,
		formatter: p.Formatter(),
	}
}

// ListItem 列表项
type ListItem struct {
	Index    int
	Status   string
	Title    string
	Subtitle string
	Details  map[string]string
	Flags    types.SecurityFlags
}

// Print 打印列表项
func (l *ListPrinter) Print(item ListItem) {
	// 获取状态显示配置
	statusDisplay, ok := config.PodStatusDisplayConfig[item.Status]
	if !ok {
		statusDisplay = config.PodStatusDisplayConfig["Unknown"]
	}

	// 打印主行
	l.printer.PrintColored(statusDisplay.Color, statusDisplay.Symbol)
	l.printer.Printf(" [%d] ", item.Index)
	l.printer.PrintColored(statusDisplay.Color, item.Status)
	l.printer.Printf("  %s/%s\n", item.Subtitle, item.Title)

	// 打印详情
	for key, value := range item.Details {
		l.printer.Printf("     %s: %s\n", key, value)
	}

	// 打印安全标识
	flags := l.formatter.FormatSecurityFlags(item.Flags)
	if flags != "" {
		l.printer.Printf("     Security: %s\n", flags)
	}

	l.printer.Println()
}

// PrintAll 打印所有列表项
func (l *ListPrinter) PrintAll(items []ListItem) {
	for _, item := range items {
		l.Print(item)
	}
}

// PrintPods 打印 Pod 列表
func (l *ListPrinter) PrintPods(pods []types.PodContainerInfo) {
	for i, pod := range pods {
		// 获取容器名称列表
		var containerNames []string
		for _, c := range pod.Containers {
			containerNames = append(containerNames, c.Name)
		}
		item := ListItem{
			Index:    i + 1,
			Status:   pod.Status,
			Title:    pod.PodName,
			Subtitle: pod.Namespace,
			Details: map[string]string{
				"Containers": strings.Join(containerNames, ", "),
			},
			Flags: pod.SecurityFlags,
		}
		if pod.PodIP != "" {
			item.Details["PodIP"] = pod.PodIP
		}
		l.Print(item)
	}
}

// PrintLegend 打印安全标识图例说明
func (l *ListPrinter) PrintLegend() {
	l.printer.Println()
	l.printer.Section("安全标识说明")

	for name, display := range config.SecurityFlagDisplayConfig {
		l.printer.Print("  ")
		l.printer.PrintColored(display.Color, fmt.Sprintf("%s%s", display.Symbol, display.Abbrev))
		l.printer.Printf(" - %s\n", display.Description)
		_ = name // 避免未使用警告
	}
}

// PrintRiskLegend 打印风险等级说明
func (l *ListPrinter) PrintRiskLegend() {
	l.printer.Println()
	l.printer.Section("风险等级说明")

	levels := []config.RiskLevel{
		config.RiskAdmin,
		config.RiskCritical,
		config.RiskHigh,
		config.RiskMedium,
		config.RiskLow,
	}

	for _, level := range levels {
		display := config.RiskLevelDisplayConfig[level]
		l.printer.Print("  ")
		l.printer.PrintColored(display.Color, fmt.Sprintf("%s %s", display.Symbol, display.Label))
		l.printer.Printf(" - %s\n", display.Description)
	}
}

// PrintStats 打印统计信息
func (l *ListPrinter) PrintStats(items []StatItem) {
	var parts []string
	for _, item := range items {
		if item.Color != "" {
			parts = append(parts, l.printer.Colored(item.Color, fmt.Sprintf("%s: %d", item.Label, item.Value)))
		} else {
			parts = append(parts, fmt.Sprintf("%s: %d", item.Label, item.Value))
		}
	}
	l.printer.Printf("  %s\n", strings.Join(parts, "  "))
}

// StatItem 统计项
type StatItem struct {
	Label string
	Value int
	Color config.ColorName
}

// PrintTotal 打印总计
func (l *ListPrinter) PrintTotal(label string, count int) {
	l.printer.Separator()
	l.printer.Printf("%s: %d\n", label, count)
}

// PrintTotalWide 打印总计（宽）
func (l *ListPrinter) PrintTotalWide(label string, count int) {
	l.printer.SeparatorWide()
	l.printer.Printf("%s: %d\n", label, count)
}
