package console

import (
	"fmt"
	"strings"

	"kctl/internal/console/commands"
	"kctl/internal/session"
)

// Executor 命令执行器
type Executor struct {
	session *session.Session
}

// NewExecutor 创建命令执行器
func NewExecutor(sess *session.Session) *Executor {
	return &Executor{session: sess}
}

// Execute 执行命令
func (e *Executor) Execute(input string) {
	input = strings.TrimSpace(input)
	if input == "" {
		return
	}

	// 解析命令和参数
	args := parseArgs(input)
	if len(args) == 0 {
		return
	}

	cmdName := args[0]
	cmdArgs := args[1:]
	mode := e.session.GetMode()

	// 查找命令（检查模式）
	cmd, ok := commands.GetForMode(cmdName, mode)
	if !ok {
		// 检查命令是否存在但不在当前模式
		if existCmd, exists := commands.Get(cmdName); exists {
			modeStr := e.getModeForCommand(existCmd)
			e.session.Printer.Error(fmt.Sprintf("命令 '%s' 在当前模式 (%s) 下不可用，请切换到 %s 模式",
				cmdName, mode, modeStr))
			e.session.Printer.Printf("  使用: mode %s\n", modeStr)
		} else {
			e.session.Printer.Error("未知命令: " + cmdName + "，输入 'help' 查看可用命令")
		}
		return
	}

	// 执行命令
	if err := cmd.Execute(e.session, cmdArgs); err != nil {
		e.session.Printer.Error(err.Error())
	}
}

// getModeForCommand 获取命令所属模式的字符串
func (e *Executor) getModeForCommand(cmd commands.Command) string {
	switch cmd.Mode() {
	case commands.ModeKubeletOnly:
		return "kubelet"
	case commands.ModeKubernetesOnly:
		return "kubernetes"
	default:
		return "any"
	}
}

// parseArgs 解析命令行参数（支持引号）
func parseArgs(input string) []string {
	var args []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, r := range input {
		switch {
		case r == '"' || r == '\'':
			if inQuote && r == quoteChar {
				// 结束引号
				inQuote = false
				quoteChar = 0
			} else if !inQuote {
				// 开始引号
				inQuote = true
				quoteChar = r
			} else {
				// 引号内的另一种引号
				current.WriteRune(r)
			}
		case r == ' ' || r == '\t':
			if inQuote {
				current.WriteRune(r)
			} else if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args
}
