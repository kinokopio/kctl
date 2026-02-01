package console

import (
	"strings"

	"github.com/c-bata/go-prompt"

	"kctl/internal/console/commands"
	"kctl/internal/session"
)

// Completer 自动补全器
type Completer struct {
	session *session.Session
}

// NewCompleter 创建补全器
func NewCompleter(sess *session.Session) *Completer {
	return &Completer{session: sess}
}

// Complete 执行补全
func (c *Completer) Complete(d prompt.Document) []prompt.Suggest {
	text := d.TextBeforeCursor()
	if text == "" {
		return nil
	}

	args := strings.Fields(text)
	if len(args) == 0 {
		return c.getCommandSuggestions("")
	}

	cmd := args[0]
	word := d.GetWordBeforeCursor()

	// 如果只有一个词且没有空格，补全命令
	if len(args) == 1 && !strings.HasSuffix(text, " ") {
		return c.getCommandSuggestions(word)
	}

	// 根据命令补全参数
	switch cmd {
	case "set":
		return c.getSetSuggestions(args, word)
	case "show":
		return c.getShowSuggestions(args, word)
	case "export":
		return c.getExportSuggestions(args, word)
	case "help", "?", "h":
		return c.getCommandSuggestions(word)
	case "sa":
		return c.getSASuggestions(args, word)
	case "pods", "po":
		return c.getPodsSuggestions(args, word)
	}

	return nil
}

// getCommandSuggestions 获取命令建议
func (c *Completer) getCommandSuggestions(prefix string) []prompt.Suggest {
	suggestions := []prompt.Suggest{
		{Text: "help", Description: "显示帮助信息"},
		{Text: "connect", Description: "连接到 Kubelet"},
		{Text: "scan", Description: "扫描 SA 权限"},
		{Text: "sa", Description: "列出 ServiceAccount"},
		{Text: "pods", Description: "列出 Pod"},
		{Text: "use", Description: "选择 ServiceAccount"},
		{Text: "info", Description: "显示当前 SA 详情"},
		{Text: "exec", Description: "执行命令"},
		{Text: "set", Description: "设置配置"},
		{Text: "show", Description: "显示信息"},
		{Text: "export", Description: "导出结果"},
		{Text: "clear", Description: "清除缓存"},
		{Text: "exit", Description: "退出控制台"},
	}

	return prompt.FilterHasPrefix(suggestions, prefix, true)
}

// getSetSuggestions 获取 set 命令建议
func (c *Completer) getSetSuggestions(args []string, word string) []prompt.Suggest {
	if len(args) == 1 || (len(args) == 2 && word != "") {
		suggestions := []prompt.Suggest{
			{Text: "target", Description: "Kubelet IP 地址"},
			{Text: "port", Description: "Kubelet 端口"},
			{Text: "token", Description: "Token 字符串"},
			{Text: "token-file", Description: "Token 文件路径"},
			{Text: "api-server", Description: "API Server 地址"},
			{Text: "api-port", Description: "API Server 端口"},
			{Text: "proxy", Description: "SOCKS5 代理地址"},
			{Text: "concurrency", Description: "扫描并发数"},
		}
		return prompt.FilterHasPrefix(suggestions, word, true)
	}
	return nil
}

// getShowSuggestions 获取 show 命令建议
func (c *Completer) getShowSuggestions(args []string, word string) []prompt.Suggest {
	if len(args) == 1 || (len(args) == 2 && word != "") {
		suggestions := []prompt.Suggest{
			{Text: "options", Description: "显示当前配置"},
			{Text: "status", Description: "显示会话状态"},
			{Text: "env", Description: "显示环境信息"},
		}
		return prompt.FilterHasPrefix(suggestions, word, true)
	}
	return nil
}

// getExportSuggestions 获取 export 命令建议
func (c *Completer) getExportSuggestions(args []string, word string) []prompt.Suggest {
	if len(args) == 1 || (len(args) == 2 && word != "") {
		suggestions := []prompt.Suggest{
			{Text: "json", Description: "JSON 格式"},
			{Text: "csv", Description: "CSV 格式"},
		}
		return prompt.FilterHasPrefix(suggestions, word, true)
	}
	return nil
}

// getSASuggestions 获取 sa 命令建议
func (c *Completer) getSASuggestions(args []string, word string) []prompt.Suggest {
	suggestions := []prompt.Suggest{
		{Text: "--admin", Description: "只显示 cluster-admin"},
		{Text: "--risky", Description: "只显示有风险的 SA"},
		{Text: "-n", Description: "按命名空间过滤"},
		{Text: "--perms", Description: "显示权限"},
		{Text: "--token", Description: "显示 Token"},
	}
	return prompt.FilterHasPrefix(suggestions, word, true)
}

// getPodsSuggestions 获取 pods 命令建议
func (c *Completer) getPodsSuggestions(args []string, word string) []prompt.Suggest {
	suggestions := []prompt.Suggest{
		{Text: "--privileged", Description: "只显示特权 Pod"},
		{Text: "--running", Description: "只显示 Running 状态"},
		{Text: "-n", Description: "按命名空间过滤"},
		{Text: "--refresh", Description: "强制刷新"},
	}
	return prompt.FilterHasPrefix(suggestions, word, true)
}

// 确保 commands 包被导入
var _ = commands.All
