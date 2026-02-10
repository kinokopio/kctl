package golden

import (
	"fmt"
	"strings"

	"kctl/internal/session"
)

// SubCommand golden 子命令接口
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

// Execute 执行 golden 子命令
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

// Usage 返回 golden 命令的用法
func Usage() string {
	return `golden <subcommand> [options]

Kubernetes Golden Ticket - 证书和 Token 伪造

子命令：
  user-cert     伪造用户证书 (获取管理员权限)
  node-cert     伪造节点证书 (模拟集群节点)
  sa-token      伪造 ServiceAccount Token
  update-uid    从 API Server 更新 SA UID 缓存到内存
  uid-list      列出 UID 缓存 (内存或文件)
  test          测试密钥文件是否完整有效

示例：
  golden user-cert -c ./ca.crt -k ./ca.key
  golden node-cert -c ./ca.crt -k ./ca.key -n phantom
  golden update-uid -c ./ca.crt -k ./ca.key           # 保存到内存
  golden uid-list                                      # 列出内存中的 UID
  golden sa-token -s ./sa.key --namespace kube-system --name controller
  golden test -c ./ca.crt -k ./ca.key -s ./sa.key

使用 'golden <subcommand> --help' 查看子命令详细用法`
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
