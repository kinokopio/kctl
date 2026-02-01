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

func (c *HelpCmd) Usage() string {
	return `help [command]

显示可用命令列表或指定命令的详细帮助

示例：
  help          显示所有命令
  help scan     显示 scan 命令的详细帮助`
}

func (c *HelpCmd) Execute(sess *session.Session, args []string) error {
	p := sess.Printer

	if len(args) > 0 {
		// 显示指定命令的帮助
		cmdName := args[0]
		cmd, ok := Get(cmdName)
		if !ok {
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

	// 显示所有命令
	p.Println()
	p.Printf("  %s\n\n", p.Colored(config.ColorCyan, "可用命令"))

	// 按类别分组
	categories := map[string][]Command{
		"连接": {},
		"扫描": {},
		"查询": {},
		"操作": {},
		"配置": {},
		"其他": {},
	}

	categoryOrder := []string{"连接", "扫描", "查询", "操作", "配置", "其他"}

	// 分类命令
	for _, cmd := range All() {
		switch cmd.Name() {
		case "connect":
			categories["连接"] = append(categories["连接"], cmd)
		case "scan":
			categories["扫描"] = append(categories["扫描"], cmd)
		case "sa", "pods", "info":
			categories["查询"] = append(categories["查询"], cmd)
		case "use", "exec", "export":
			categories["操作"] = append(categories["操作"], cmd)
		case "set", "show", "clear":
			categories["配置"] = append(categories["配置"], cmd)
		default:
			categories["其他"] = append(categories["其他"], cmd)
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

	p.Printf("  输入 '%s' 查看命令详细帮助\n\n",
		p.Colored(config.ColorCyan, "help <command>"))

	return nil
}
