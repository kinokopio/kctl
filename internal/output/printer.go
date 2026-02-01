package output

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"
	"kctl/config"
)

// Printer 打印器接口
type Printer interface {
	// 基础输出
	Print(a ...interface{})
	Println(a ...interface{})
	Printf(format string, a ...interface{})

	// 带颜色输出
	Colored(colorName config.ColorName, text string) string
	PrintColored(colorName config.ColorName, a ...interface{})
	PrintColoredln(colorName config.ColorName, a ...interface{})

	// 语义输出
	Success(msg string)
	Warning(msg string)
	Error(msg string)
	Info(msg string)
	Tip(msg string)

	// 结构化输出
	Title(title string)
	TitleWide(title string)
	Section(title string)
	SubSection(title string)
	Separator()
	SeparatorWide()

	// 键值对
	KeyValue(key, value string)
	KeyValueNote(key, value, note string)
	KeyValueStatus(key, value string, ok bool)

	// 获取格式化器
	Formatter() *Formatter

	// 获取输出宽度
	Width() int
	SetWidth(width int)
}

// printer 打印器实现
type printer struct {
	out       io.Writer
	errOut    io.Writer
	colors    map[config.ColorName]*color.Color
	formatter *Formatter
	width     int
}

// NewPrinter 创建打印器
func NewPrinter() Printer {
	return NewPrinterWithWriter(os.Stdout, os.Stderr)
}

// NewPrinterWithWriter 创建带自定义输出的打印器
func NewPrinterWithWriter(out, errOut io.Writer) Printer {
	p := &printer{
		out:    out,
		errOut: errOut,
		colors: initColors(),
		width:  config.Layout.DefaultWidth,
	}
	p.formatter = NewFormatter(p)
	return p
}

// initColors 初始化颜色映射
func initColors() map[config.ColorName]*color.Color {
	return map[config.ColorName]*color.Color{
		config.ColorRed:     color.New(color.FgRed),
		config.ColorGreen:   color.New(color.FgGreen),
		config.ColorYellow:  color.New(color.FgYellow),
		config.ColorBlue:    color.New(color.FgBlue),
		config.ColorMagenta: color.New(color.FgMagenta),
		config.ColorCyan:    color.New(color.FgCyan),
		config.ColorWhite:   color.New(color.FgWhite),
		config.ColorGray:    color.New(color.FgHiBlack),
	}
}

// getColor 获取颜色
func (p *printer) getColor(name config.ColorName) *color.Color {
	if c, ok := p.colors[name]; ok {
		return c
	}
	return p.colors[config.ColorWhite]
}

// getThemeColor 获取主题颜色
func (p *printer) getThemeColor(key string) *color.Color {
	if colorName, ok := config.ThemeColors[key]; ok {
		return p.getColor(colorName)
	}
	return p.colors[config.ColorWhite]
}

// Width 获取输出宽度
func (p *printer) Width() int {
	return p.width
}

// SetWidth 设置输出宽度
func (p *printer) SetWidth(width int) {
	p.width = width
}

// Print 基础打印
func (p *printer) Print(a ...interface{}) {
	fmt.Fprint(p.out, a...)
}

func (p *printer) Println(a ...interface{}) {
	fmt.Fprintln(p.out, a...)
}

func (p *printer) Printf(format string, a ...interface{}) {
	fmt.Fprintf(p.out, format, a...)
}

// Colored 返回带颜色的字符串
func (p *printer) Colored(colorName config.ColorName, text string) string {
	return p.getColor(colorName).Sprint(text)
}

// PrintColored 带颜色打印
func (p *printer) PrintColored(colorName config.ColorName, a ...interface{}) {
	p.getColor(colorName).Fprint(p.out, a...)
}

func (p *printer) PrintColoredln(colorName config.ColorName, a ...interface{}) {
	p.getColor(colorName).Fprintln(p.out, a...)
}

// Success 成功消息
func (p *printer) Success(msg string) {
	symbol := config.Symbols["success"]
	p.getThemeColor("success").Fprintf(p.out, "%s %s\n", symbol, msg)
}

// Warning 警告消息
func (p *printer) Warning(msg string) {
	symbol := config.Symbols["warning"]
	p.getThemeColor("warning").Fprintf(p.out, "%s %s\n", symbol, msg)
}

// Error 错误消息
func (p *printer) Error(msg string) {
	symbol := config.Symbols["error"]
	p.getThemeColor("error").Fprintf(p.errOut, "%s %s\n", symbol, msg)
}

// Info 信息消息
func (p *printer) Info(msg string) {
	symbol := config.Symbols["info"]
	p.getThemeColor("highlight").Fprintf(p.out, "%s %s\n", symbol, msg)
}

// Tip 提示消息
func (p *printer) Tip(msg string) {
	symbol := config.Symbols["tip"]
	p.getThemeColor("highlight").Fprintf(p.out, "%s %s\n", symbol, msg)
}

// Title 打印标题
func (p *printer) Title(title string) {
	line := strings.Repeat(config.Symbols["border_bold"], p.width)
	titleColor := p.getThemeColor("title")

	p.Println()
	titleColor.Fprintln(p.out, line)

	// 居中标题
	padding := (p.width - len(title)) / 2
	if padding > 0 {
		p.Printf("%s", strings.Repeat(" ", padding))
	}
	titleColor.Fprintln(p.out, title)
	titleColor.Fprintln(p.out, line)
}

// TitleWide 打印宽标题
func (p *printer) TitleWide(title string) {
	width := config.Layout.WideWidth
	line := strings.Repeat(config.Symbols["border_bold"], width)
	titleColor := p.getThemeColor("title")
	subtitleColor := p.getThemeColor("subtitle")

	p.Println()
	titleColor.Fprintln(p.out, line)

	// 居中标题
	padding := (width - len(title)) / 2
	if padding > 0 {
		p.Printf("%s", strings.Repeat(" ", padding))
	}
	subtitleColor.Fprintln(p.out, title)
	titleColor.Fprintln(p.out, line)
	p.Println()
}

// Section 打印章节
func (p *printer) Section(title string) {
	p.Println()
	p.getThemeColor("subtitle").Fprintf(p.out, "━━━ %s ━━━\n", title)
	p.Println()
}

// SubSection 打印子章节
func (p *printer) SubSection(title string) {
	p.Println()
	p.getThemeColor("muted").Fprintf(p.out, "  ─── %s ───\n", title)
	p.Println()
}

// Separator 打印分隔线
func (p *printer) Separator() {
	line := strings.Repeat(config.Symbols["border_single"], p.width)
	p.Println(line)
}

// SeparatorWide 打印宽分隔线
func (p *printer) SeparatorWide() {
	line := strings.Repeat(config.Symbols["border_single"], config.Layout.WideWidth)
	p.Println(line)
}

// KeyValue 打印键值对
func (p *printer) KeyValue(key, value string) {
	labelWidth := config.Layout.LabelWidth
	p.getThemeColor("label").Fprintf(p.out, "  %-*s: ", labelWidth, key)
	p.Println(value)
}

// KeyValueNote 打印键值对带注释
func (p *printer) KeyValueNote(key, value, note string) {
	labelWidth := config.Layout.LabelWidth
	p.getThemeColor("label").Fprintf(p.out, "  %-*s: ", labelWidth, key)
	p.Printf("%s ", value)
	p.getThemeColor("muted").Fprintln(p.out, note)
}

// KeyValueStatus 打印键值对带状态
func (p *printer) KeyValueStatus(key, value string, ok bool) {
	labelWidth := config.Layout.LabelWidth
	p.getThemeColor("label").Fprintf(p.out, "  %-*s: ", labelWidth, key)
	p.Printf("%s ", value)
	if ok {
		p.getThemeColor("success").Fprintln(p.out, config.Symbols["success"])
	} else {
		p.getThemeColor("error").Fprintln(p.out, config.Symbols["error"])
	}
}

// Formatter 获取格式化器
func (p *printer) Formatter() *Formatter {
	return p.formatter
}

// ==================== 全局默认打印器 ====================

var defaultPrinter Printer

func init() {
	defaultPrinter = NewPrinter()
}

// Default 获取默认打印器
func Default() Printer {
	return defaultPrinter
}

// SetDefault 设置默认打印器
func SetDefault(p Printer) {
	defaultPrinter = p
}
