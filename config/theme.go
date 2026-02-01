package config

// ==================== 颜色主题 ====================

// ColorName 颜色名称
type ColorName string

const (
	ColorRed     ColorName = "red"
	ColorGreen   ColorName = "green"
	ColorYellow  ColorName = "yellow"
	ColorBlue    ColorName = "blue"
	ColorMagenta ColorName = "magenta"
	ColorCyan    ColorName = "cyan"
	ColorWhite   ColorName = "white"
	ColorGray    ColorName = "gray"
)

// ThemeColors 主题颜色配置
var ThemeColors = map[string]ColorName{
	// 语义颜色
	"title":     ColorCyan,
	"subtitle":  ColorYellow,
	"label":     ColorWhite,
	"value":     ColorWhite,
	"highlight": ColorCyan,
	"muted":     ColorGray,

	// 状态颜色
	"success": ColorGreen,
	"warning": ColorYellow,
	"error":   ColorRed,
	"danger":  ColorRed,
	"admin":   ColorRed,

	// 风险等级颜色
	"risk_admin":    ColorRed,
	"risk_critical": ColorRed,
	"risk_high":     ColorYellow,
	"risk_medium":   ColorYellow,
	"risk_low":      ColorGray,
	"risk_none":     ColorGray,
}

// ==================== 符号配置 ====================

// Symbols 输出符号配置
var Symbols = map[string]string{
	// 状态符号
	"success": "✓",
	"error":   "✗",
	"warning": "⚠",
	"info":    "ℹ",
	"tip":     "💡",

	// 列表符号
	"bullet":       "●",
	"bullet_empty": "○",
	"arrow":        "→",
	"arrow_right":  "▶",

	// 风险等级符号
	"risk_admin":    "⚠",
	"risk_critical": "★",
	"risk_high":     "★",
	"risk_medium":   "★",
	"risk_low":      "○",
	"risk_none":     "○",
	"danger":        "🔴",
	"sensitive":     "🟡",
	"star":          "★",
	"diamond":       "◆",

	// 安全标识符号
	"flag_privileged": "★",
	"flag_hostpath":   "★",
	"flag_secret":     "★",
	"flag_pe":         "★",

	// 边框符号
	"border_double": "═",
	"border_single": "─",
	"border_bold":   "━",

	// 框角符号
	"box_top_left":     "┌",
	"box_top_right":    "┐",
	"box_bottom_left":  "└",
	"box_bottom_right": "┘",
	"box_vertical":     "│",
	"box_horizontal":   "─",
}

// ==================== 布局配置 ====================

// Layout 布局配置
var Layout = struct {
	DefaultWidth  int // 默认输出宽度
	WideWidth     int // 宽输出宽度
	LabelWidth    int // 标签宽度
	IndentSize    int // 缩进大小
	TableMinWidth int // 表格最小宽度
	BoxPadding    int // 信息框内边距
}{
	DefaultWidth:  80,
	WideWidth:     110,
	LabelWidth:    16,
	IndentSize:    2,
	TableMinWidth: 60,
	BoxPadding:    2,
}

// ==================== 风险等级显示配置 ====================

// RiskLevelDisplay 风险等级显示配置
type RiskLevelDisplay struct {
	Symbol      string    // 显示符号
	Color       ColorName // 颜色
	Label       string    // 显示标签
	Description string    // 描述
}

// RiskLevelDisplayConfig 风险等级显示配置映射
var RiskLevelDisplayConfig = map[RiskLevel]RiskLevelDisplay{
	RiskAdmin: {
		Symbol:      "⚠",
		Color:       ColorRed,
		Label:       "ADMIN",
		Description: "集群管理员权限，可完全控制集群",
	},
	RiskCritical: {
		Symbol:      "★",
		Color:       ColorRed,
		Label:       "CRITICAL",
		Description: "高危权限，接近管理员级别",
	},
	RiskHigh: {
		Symbol:      "★",
		Color:       ColorYellow,
		Label:       "HIGH",
		Description: "可权限提升或泄露敏感信息",
	},
	RiskMedium: {
		Symbol:      "★",
		Color:       ColorYellow,
		Label:       "MEDIUM",
		Description: "可能被滥用的权限",
	},
	RiskLow: {
		Symbol:      "○",
		Color:       ColorGray,
		Label:       "LOW",
		Description: "低危权限",
	},
	RiskNone: {
		Symbol:      "○",
		Color:       ColorGray,
		Label:       "NONE",
		Description: "无风险",
	},
}

// ==================== Pod 状态显示配置 ====================

// PodStatusDisplay Pod 状态显示配置
type PodStatusDisplay struct {
	Symbol string
	Color  ColorName
}

// PodStatusDisplayConfig Pod 状态显示配置映射
var PodStatusDisplayConfig = map[string]PodStatusDisplay{
	"Running":   {Symbol: "●", Color: ColorGreen},
	"Pending":   {Symbol: "○", Color: ColorYellow},
	"Succeeded": {Symbol: "✓", Color: ColorGreen},
	"Failed":    {Symbol: "✗", Color: ColorRed},
	"Unknown":   {Symbol: "?", Color: ColorGray},
}

// ==================== 安全标识显示配置 ====================

// SecurityFlagDisplay 安全标识显示配置
type SecurityFlagDisplay struct {
	Abbrev      string    // 简写
	Symbol      string    // 符号
	Color       ColorName // 颜色
	Description string    // 描述
}

// SecurityFlagDisplayConfig 安全标识显示配置映射
var SecurityFlagDisplayConfig = map[string]SecurityFlagDisplay{
	"Privileged": {
		Abbrev:      "PRIV",
		Symbol:      "★",
		Color:       ColorRed,
		Description: "特权容器",
	},
	"AllowPrivilegeEscalation": {
		Abbrev:      "PE",
		Symbol:      "★",
		Color:       ColorYellow,
		Description: "允许权限提升",
	},
	"HostPath": {
		Abbrev:      "HP",
		Symbol:      "★",
		Color:       ColorRed,
		Description: "HostPath 挂载",
	},
	"SecretMount": {
		Abbrev:      "SEC",
		Symbol:      "★",
		Color:       ColorMagenta,
		Description: "Secret 挂载",
	},
	"RunAsRoot": {
		Abbrev:      "ROOT",
		Symbol:      "★",
		Color:       ColorRed,
		Description: "以 root 运行",
	},
	"HostNetwork": {
		Abbrev:      "HNET",
		Symbol:      "★",
		Color:       ColorYellow,
		Description: "主机网络",
	},
	"HostPID": {
		Abbrev:      "HPID",
		Symbol:      "★",
		Color:       ColorYellow,
		Description: "主机 PID",
	},
}

// ==================== 表格样式配置 ====================

// TableStyle 表格样式
type TableStyle struct {
	HeaderColor ColorName
	HeaderBold  bool
	RowLine     bool
	AutoMerge   bool
	Alignment   string // "left", "center", "right"
}

// DefaultTableStyle 默认表格样式
var DefaultTableStyle = TableStyle{
	HeaderColor: ColorGreen,
	HeaderBold:  true,
	RowLine:     true,
	AutoMerge:   true,
	Alignment:   "center",
}

// ==================== 信息框样式配置 ====================

// BoxStyleConfig 信息框样式配置
type BoxStyleConfig struct {
	Color       ColorName
	TopLeft     string
	TopRight    string
	BottomLeft  string
	BottomRight string
	Horizontal  string
	Vertical    string
}

// BoxStyles 信息框样式映射
var BoxStyles = map[string]BoxStyleConfig{
	"normal": {
		Color:       ColorCyan,
		TopLeft:     "┌",
		TopRight:    "┐",
		BottomLeft:  "└",
		BottomRight: "┘",
		Horizontal:  "─",
		Vertical:    "│",
	},
	"warning": {
		Color:       ColorYellow,
		TopLeft:     "┌",
		TopRight:    "┐",
		BottomLeft:  "└",
		BottomRight: "┘",
		Horizontal:  "─",
		Vertical:    "│",
	},
	"danger": {
		Color:       ColorRed,
		TopLeft:     "┌",
		TopRight:    "┐",
		BottomLeft:  "└",
		BottomRight: "┘",
		Horizontal:  "─",
		Vertical:    "│",
	},
	"admin": {
		Color:       ColorRed,
		TopLeft:     "╔",
		TopRight:    "╗",
		BottomLeft:  "╚",
		BottomRight: "╝",
		Horizontal:  "═",
		Vertical:    "║",
	},
}
