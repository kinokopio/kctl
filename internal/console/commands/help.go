package commands

import (
	"fmt"
	"sort"
	"strings"

	"kctl/config"
	"kctl/internal/session"
)

// HelpCmd help 命令
type HelpCmd struct{}

func init() {
	Register(&HelpCmd{})
}

func (c *HelpCmd) Name() string {
	return "help"
}

func (c *HelpCmd) Aliases() []string {
	return []string{"?", "h"}
}

func (c *HelpCmd) Description() string {
	return "显示帮助信息"
}

func (c *HelpCmd) Mode() CommandMode {
	return ModeAll
}

func (c *HelpCmd) Usage() string {
	return `help [command]

显示可用命令列表或指定命令的详细帮助

示例：
  help          显示所有命令
  help scan     显示 scan 命令的详细帮助`
}

func (c *HelpCmd) Execute(sess *session.Session, args []string) error {
	p := sess.Printer
	mode := sess.GetMode()

	if len(args) > 0 {
		// 显示指定命令的帮助
		cmdName := args[0]
		cmd, ok := GetForMode(cmdName, mode)
		if !ok {
			// 检查命令是否存在但不在当前模式
			if existCmd, exists := Get(cmdName); exists {
				return fmt.Errorf("命令 '%s' 在当前模式 (%s) 下不可用，请切换到 %s 模式",
					cmdName, mode, c.getModeForCommand(existCmd))
			}
			return fmt.Errorf("未知命令: %s", cmdName)
		}

		p.Println()
		p.Printf("  %s - %s\n\n",
			p.Colored(config.ColorCyan, cmd.Name()),
			cmd.Description())

		if len(cmd.Aliases()) > 0 {
			p.Printf("  %s: %s\n\n",
				p.Colored(config.ColorGray, "别名"),
				strings.Join(cmd.Aliases(), ", "))
		}

		p.Printf("  %s:\n", p.Colored(config.ColorGray, "用法"))
		for _, line := range strings.Split(cmd.Usage(), "\n") {
			p.Printf("    %s\n", line)
		}
		p.Println()
		return nil
	}

	// 显示当前模式下的所有命令
	c.showCommandsForMode(sess, mode)

	return nil
}

func (c *HelpCmd) getModeForCommand(cmd Command) string {
	switch cmd.Mode() {
	case ModeKubeletOnly:
		return "kubelet"
	case ModeKubernetesOnly:
		return "kubernetes"
	default:
		return "any"
	}
}

func (c *HelpCmd) showCommandsForMode(sess *session.Session, mode session.Mode) {
	p := sess.Printer

	p.Println()
	modeStr := string(mode)
	p.Printf("  %s [%s]\n\n",
		p.Colored(config.ColorCyan, "可用命令"),
		p.Colored(config.ColorYellow, modeStr))

	// 按类别分组
	var categories map[string][]Command
	var categoryOrder []string

	if mode == session.ModeKubelet {
		categories = map[string][]Command{
			"连接":   {},
			"信息收集": {},
			"执行":   {},
			"配置":   {},
			"其他":   {},
		}
		categoryOrder = []string{"连接", "信息收集", "执行", "配置", "其他"}
	} else {
		categories = map[string][]Command{
			"Golden Ticket": {},
			"配置":            {},
			"其他":            {},
		}
		categoryOrder = []string{"Golden Ticket", "配置", "其他"}
	}

	// 分类命令
	for _, cmd := range AllForMode(mode) {
		if mode == session.ModeKubelet {
			switch cmd.Name() {
			case "connect", "discover":
				categories["连接"] = append(categories["连接"], cmd)
			case "pods", "sa":
				categories["信息收集"] = append(categories["信息收集"], cmd)
			case "exec", "run", "portforward", "pid2pod":
				categories["执行"] = append(categories["执行"], cmd)
			case "set", "show", "clear", "mode", "export":
				categories["配置"] = append(categories["配置"], cmd)
			default:
				categories["其他"] = append(categories["其他"], cmd)
			}
		} else {
			switch cmd.Name() {
			case "golden":
				categories["Golden Ticket"] = append(categories["Golden Ticket"], cmd)
			case "set", "show", "clear", "mode", "export":
				categories["配置"] = append(categories["配置"], cmd)
			default:
				categories["其他"] = append(categories["其他"], cmd)
			}
		}
	}

	// 打印分类命令
	for _, cat := range categoryOrder {
		cmds := categories[cat]
		if len(cmds) == 0 {
			continue
		}

		// 按名称排序
		sort.Slice(cmds, func(i, j int) bool {
			return cmds[i].Name() < cmds[j].Name()
		})

		p.Printf("  %s:\n", p.Colored(config.ColorYellow, cat))
		for _, cmd := range cmds {
			aliases := ""
			if len(cmd.Aliases()) > 0 {
				aliases = fmt.Sprintf(" (%s)", strings.Join(cmd.Aliases(), ", "))
			}
			p.Printf("    %-12s %s%s\n",
				p.Colored(config.ColorGreen, cmd.Name()),
				cmd.Description(),
				p.Colored(config.ColorGray, aliases))
		}
		p.Println()
	}

	p.Printf("  输入 '%s' 查看命令详细帮助\n",
		p.Colored(config.ColorCyan, "help <command>"))
	p.Printf("  输入 '%s' 切换模式\n\n",
		p.Colored(config.ColorCyan, "mode <kubelet|kubernetes>"))
}
