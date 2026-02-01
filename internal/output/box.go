package output

import (
	"strings"

	"kctl/config"
)

// BoxPrinter 信息框打印器
type BoxPrinter struct {
	printer Printer
	width   int
}

// NewBoxPrinter 创建信息框打印器
func NewBoxPrinter(p Printer) *BoxPrinter {
	return &BoxPrinter{
		printer: p,
		width:   56, // 默认框宽度
	}
}

// WithWidth 设置宽度
func (b *BoxPrinter) WithWidth(width int) *BoxPrinter {
	b.width = width
	return b
}

// Print 打印信息框
func (b *BoxPrinter) Print(title string, lines []string, styleName string) {
	style, ok := config.BoxStyles[styleName]
	if !ok {
		style = config.BoxStyles["normal"]
	}

	// 构建边框
	topBorder := style.TopLeft + strings.Repeat(style.Horizontal, b.width) + style.TopRight
	bottomBorder := style.BottomLeft + strings.Repeat(style.Horizontal, b.width) + style.BottomRight
	divider := style.Vertical + strings.Repeat(style.Horizontal, b.width) + style.Vertical

	// 打印顶部边框
	b.printer.Print("  ")
	b.printer.PrintColoredln(style.Color, topBorder)

	// 打印标题
	b.printer.Print("  ")
	b.printer.PrintColored(style.Color, style.Vertical)
	b.printer.Printf("  %-*s", b.width-2, title)
	b.printer.PrintColoredln(style.Color, style.Vertical)

	// 打印分隔线
	b.printer.Print("  ")
	b.printer.PrintColoredln(style.Color, divider)

	// 打印内容
	for _, line := range lines {
		b.printer.Print("  ")
		b.printer.PrintColored(style.Color, style.Vertical)
		b.printer.Printf("  %-*s", b.width-2, line)
		b.printer.PrintColoredln(style.Color, style.Vertical)
	}

	// 打印底部边框
	b.printer.Print("  ")
	b.printer.PrintColoredln(style.Color, bottomBorder)
	b.printer.Println()
}

// PrintAdmin 打印管理员级别信息框
func (b *BoxPrinter) PrintAdmin(title string, lines []string) {
	b.Print(title, lines, "admin")
}

// PrintDanger 打印危险信息框
func (b *BoxPrinter) PrintDanger(title string, lines []string) {
	b.Print(title, lines, "danger")
}

// PrintWarning 打印警告信息框
func (b *BoxPrinter) PrintWarning(title string, lines []string) {
	b.Print(title, lines, "warning")
}

// PrintNormal 打印普通信息框
func (b *BoxPrinter) PrintNormal(title string, lines []string) {
	b.Print(title, lines, "normal")
}
