package output

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
	"kctl/config"
)

// TablePrinter 表格打印器
type TablePrinter struct {
	writer  io.Writer
	style   config.TableStyle
	printer Printer
}

// NewTablePrinter 创建表格打印器
func NewTablePrinter() *TablePrinter {
	return &TablePrinter{
		writer: os.Stdout,
		style:  config.DefaultTableStyle,
	}
}

// NewTablePrinterWithPrinter 创建带 Printer 的表格打印器
func NewTablePrinterWithPrinter(p Printer) *TablePrinter {
	return &TablePrinter{
		writer:  os.Stdout,
		style:   config.DefaultTableStyle,
		printer: p,
	}
}

// WithWriter 设置输出
func (t *TablePrinter) WithWriter(w io.Writer) *TablePrinter {
	t.writer = w
	return t
}

// WithStyle 设置样式
func (t *TablePrinter) WithStyle(style config.TableStyle) *TablePrinter {
	t.style = style
	return t
}

// createTable 创建基础表格
func (t *TablePrinter) createTable(header []string) *tablewriter.Table {
	table := tablewriter.NewWriter(t.writer)
	table.SetHeader(header)
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetBorder(true)
	table.SetRowLine(true)
	table.SetHeaderLine(true)
	table.SetTablePadding(" ")

	// 设置表头颜色
	headerColors := make([]tablewriter.Colors, len(header))
	for i := range headerColors {
		headerColors[i] = tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor}
	}
	table.SetHeaderColor(headerColors...)

	return table
}

// Print 打印表格
func (t *TablePrinter) Print(header []string, rows [][]string, caption string) {
	table := tablewriter.NewWriter(t.writer)
	table.SetHeader(header)

	// 应用样式
	if t.style.AutoMerge {
		table.SetAutoMergeCells(true)
	}
	if t.style.RowLine {
		table.SetRowLine(true)
	}

	// 设置对齐
	switch t.style.Alignment {
	case "left":
		table.SetAlignment(tablewriter.ALIGN_LEFT)
	case "right":
		table.SetAlignment(tablewriter.ALIGN_RIGHT)
	default:
		table.SetAlignment(tablewriter.ALIGN_CENTER)
	}

	// 设置表头颜色
	headerColors := make([]tablewriter.Colors, len(header))
	for i := range headerColors {
		headerColors[i] = tablewriter.Colors{tablewriter.Bold, tablewriter.FgGreenColor}
	}
	table.SetHeaderColor(headerColors...)

	if caption != "" {
		table.SetCaption(true, caption)
	}

	table.AppendBulk(rows)
	table.Render()
}

// PrintSimple 打印简洁表格（带边框）
func (t *TablePrinter) PrintSimple(header []string, rows [][]string) {
	table := t.createTable(header)
	table.AppendBulk(rows)
	table.Render()
}

// PrintWithTitle 打印带标题的表格
func (t *TablePrinter) PrintWithTitle(title string, header []string, rows [][]string) {
	// 打印标题
	fmt.Fprintf(t.writer, "\n%s\n", title)
	fmt.Fprintf(t.writer, "%s\n", strings.Repeat("─", len(title)+4))

	// 打印表格
	t.PrintSimple(header, rows)
}

// PrintSummary 打印摘要表格（两列：项目-值）
func (t *TablePrinter) PrintSummary(title string, items []SummaryItem) {
	// 打印标题
	fmt.Fprintf(t.writer, "\n%s\n", title)
	fmt.Fprintf(t.writer, "%s\n\n", strings.Repeat("─", len(title)+4))

	// 找到最长的标签
	maxLen := 0
	for _, item := range items {
		if len(item.Label) > maxLen {
			maxLen = len(item.Label)
		}
	}

	// 打印每一行
	for _, item := range items {
		fmt.Fprintf(t.writer, "  %-*s  %s\n", maxLen, item.Label, item.Value)
	}
}

// SummaryItem 摘要项
type SummaryItem struct {
	Label string
	Value string
}

// PrintPods 打印 Pod 表格
func (t *TablePrinter) PrintPods(pods []PodRow) {
	header := []string{"NAME", "NAMESPACE", "SERVICE ACCOUNT", "POD IP", "NODE", "FLAGS"}
	t.PrintSimple(header, t.podRowsToStrings(pods))
}

func (t *TablePrinter) podRowsToStrings(pods []PodRow) [][]string {
	var rows [][]string
	for _, pod := range pods {
		rows = append(rows, []string{
			pod.Name,
			pod.Namespace,
			pod.ServiceAccount,
			pod.PodIP,
			pod.NodeName,
			pod.Flags,
		})
	}
	return rows
}

// PrintPermissions 打印权限表格
func (t *TablePrinter) PrintPermissions(perms []PermissionRow) {
	header := []string{"RISK", "RESOURCE", "VERB"}
	t.PrintSimple(header, t.permRowsToStrings(perms))
}

func (t *TablePrinter) permRowsToStrings(perms []PermissionRow) [][]string {
	var rows [][]string
	for _, perm := range perms {
		if !perm.Allowed {
			continue
		}
		rows = append(rows, []string{
			perm.RiskLevel,
			perm.Resource,
			perm.Verb,
		})
	}
	return rows
}

// PrintScanResults 打印扫描结果表格
func (t *TablePrinter) PrintScanResults(results []ScanResultRow, showPerms bool, showToken bool) {
	// 如果显示 Token，使用详细格式而不是表格
	if showToken {
		t.printScanResultsDetailed(results, showPerms)
		return
	}

	header := []string{"RISK", "NAMESPACE", "POD", "SERVICE ACCOUNT", "TOKEN STATUS", "FLAGS"}
	if showPerms {
		header = append(header, "PERMISSIONS")
	}
	t.PrintSimple(header, t.scanRowsToStrings(results, showPerms, false))
}

// printScanResultsDetailed 详细格式打印扫描结果（用于显示 Token）
func (t *TablePrinter) printScanResultsDetailed(results []ScanResultRow, showPerms bool) {
	for i, r := range results {
		fmt.Fprintf(t.writer, "\n[%d] %s  %s/%s\n", i+1, r.Risk, r.Namespace, r.Pod)
		fmt.Fprintf(t.writer, "    ServiceAccount: %s\n", r.ServiceAccount)
		fmt.Fprintf(t.writer, "    Token Status:   %s\n", r.TokenStatus)
		fmt.Fprintf(t.writer, "    Flags:          %s\n", r.Flags)
		if showPerms && r.Permissions != "" && r.Permissions != "-" {
			fmt.Fprintf(t.writer, "    Permissions:\n")
			for _, line := range strings.Split(r.Permissions, "\n") {
				fmt.Fprintf(t.writer, "      %s\n", line)
			}
		}
		fmt.Fprintf(t.writer, "    Token:\n")
		// Token 每 80 字符换行
		token := r.Token
		for len(token) > 80 {
			fmt.Fprintf(t.writer, "      %s\n", token[:80])
			token = token[80:]
		}
		if len(token) > 0 {
			fmt.Fprintf(t.writer, "      %s\n", token)
		}
	}
}

func (t *TablePrinter) scanRowsToStrings(results []ScanResultRow, showPerms bool, showToken bool) [][]string {
	var rows [][]string
	for _, r := range results {
		row := []string{
			r.Risk,
			r.Namespace,
			r.Pod,
			r.ServiceAccount,
			r.TokenStatus,
			r.Flags,
		}
		if showPerms {
			row = append(row, r.Permissions)
		}
		if showToken {
			row = append(row, r.Token)
		}
		rows = append(rows, row)
	}
	return rows
}

// PrintServiceAccounts 打印 SA 表格
func (t *TablePrinter) PrintServiceAccounts(sas []SARow, showPerms bool, showToken bool) {
	// 如果显示 Token，使用详细格式而不是表格
	if showToken {
		t.printSADetailed(sas, showPerms)
		return
	}

	header := []string{"RISK", "NAMESPACE", "NAME", "TOKEN STATUS", "FLAGS"}
	if showPerms {
		header = append(header, "PERMISSIONS")
	}
	t.PrintSimple(header, t.saRowsToStrings(sas, showPerms, false))
}

// printSADetailed 详细格式打印 SA（用于显示 Token）
func (t *TablePrinter) printSADetailed(sas []SARow, showPerms bool) {
	for i, sa := range sas {
		fmt.Fprintf(t.writer, "\n[%d] %s  %s/%s\n", i+1, sa.Risk, sa.Namespace, sa.Name)
		fmt.Fprintf(t.writer, "    Token Status: %s\n", sa.TokenStatus)
		fmt.Fprintf(t.writer, "    Flags:        %s\n", sa.Flags)
		if showPerms && sa.Permissions != "" && sa.Permissions != "-" {
			fmt.Fprintf(t.writer, "    Permissions:\n")
			for _, line := range strings.Split(sa.Permissions, "\n") {
				fmt.Fprintf(t.writer, "      %s\n", line)
			}
		}
		fmt.Fprintf(t.writer, "    Token:\n")
		// Token 每 80 字符换行
		token := sa.Token
		for len(token) > 80 {
			fmt.Fprintf(t.writer, "      %s\n", token[:80])
			token = token[80:]
		}
		if len(token) > 0 {
			fmt.Fprintf(t.writer, "      %s\n", token)
		}
	}
}

func (t *TablePrinter) saRowsToStrings(sas []SARow, showPerms bool, showToken bool) [][]string {
	var rows [][]string
	for _, sa := range sas {
		row := []string{
			sa.Risk,
			sa.Namespace,
			sa.Name,
			sa.TokenStatus,
			sa.Flags,
		}
		if showPerms {
			row = append(row, sa.Permissions)
		}
		if showToken {
			row = append(row, sa.Token)
		}
		rows = append(rows, row)
	}
	return rows
}

// PrintMounts 打印挂载汇总表格
func (t *TablePrinter) PrintMounts(mounts []MountRow) {
	header := []string{"TYPE", "NAME/PATH", "POD COUNT", "PODS"}
	t.PrintSimple(header, t.mountRowsToStrings(mounts))
}

func (t *TablePrinter) mountRowsToStrings(mounts []MountRow) [][]string {
	var rows [][]string
	for _, m := range mounts {
		rows = append(rows, []string{
			m.Type,
			m.Name,
			m.PodCount,
			m.Pods,
		})
	}
	return rows
}

// ==================== 行数据类型 ====================

// PodRow Pod 行数据
type PodRow struct {
	Name           string
	Namespace      string
	ServiceAccount string
	PodIP          string
	NodeName       string
	Flags          string
}

// PermissionRow 权限行数据
type PermissionRow struct {
	Resource  string
	Verb      string
	Allowed   bool
	RiskLevel string
}

// ScanResultRow 扫描结果行数据
type ScanResultRow struct {
	Risk           string
	Namespace      string
	Pod            string
	ServiceAccount string
	TokenStatus    string
	Flags          string
	Permissions    string
	Token          string
}

// SARow ServiceAccount 行数据
type SARow struct {
	Risk        string
	Namespace   string
	Name        string
	TokenStatus string
	Flags       string
	Permissions string
	Token       string
}

// MountRow 挂载行数据
type MountRow struct {
	Type     string
	Name     string
	PodCount string
	Pods     string
}

// ==================== 兼容旧 API ====================

// Table 表格结构（兼容旧 API）
type Table struct {
	Header []string
	Body   [][]string
}

// Print 打印表格（兼容旧 API）
func (t *Table) Print(caption string) {
	printer := NewTablePrinter()
	printer.PrintSimple(t.Header, t.Body)
}
