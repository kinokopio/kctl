package commands

import (
	"fmt"
	"strconv"

	"kctl/config"
	"kctl/internal/session"
	"kctl/pkg/token"
)

// SetCmd set 命令
type SetCmd struct{}

func init() {
	Register(&SetCmd{})
}

func (c *SetCmd) Name() string {
	return "set"
}

func (c *SetCmd) Aliases() []string {
	return nil
}

func (c *SetCmd) Description() string {
	return "设置配置项"
}

func (c *SetCmd) Usage() string {
	return `set <key> <value>

设置配置项

可用配置项：
  target, kubelet-ip    Kubelet IP 地址
  port, kubelet-port    Kubelet 端口 (默认: 10250)
  token                 Token 字符串
  token-file            Token 文件路径
  api-server            API Server 地址
  api-port              API Server 端口 (默认: 443)
  proxy                 SOCKS5 代理地址
  concurrency           扫描并发数 (默认: 3)

示例：
  set target 10.0.0.1
  set port 10250
  set token eyJhbGciOiJSUzI1NiIs...
  set token-file /path/to/token
  set proxy socks5://127.0.0.1:1080`
}

func (c *SetCmd) Execute(sess *session.Session, args []string) error {
	p := sess.Printer

	if len(args) < 2 {
		return fmt.Errorf("用法: set <key> <value>")
	}

	key := args[0]
	value := args[1]

	switch key {
	case "target", "kubelet-ip":
		sess.Config.KubeletIP = value
		p.Success(fmt.Sprintf("Kubelet IP set to: %s", value))

	case "port", "kubelet-port":
		port, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("无效的端口号: %s", value)
		}
		sess.Config.KubeletPort = port
		p.Success(fmt.Sprintf("Kubelet Port set to: %d", port))

	case "token":
		sess.Config.Token = value
		// 截断显示
		display := value
		if len(display) > 20 {
			display = display[:20] + "..."
		}
		p.Success(fmt.Sprintf("Token set to: %s", display))

	case "token-file":
		tokenStr, err := token.Read(value)
		if err != nil {
			return fmt.Errorf("读取 Token 文件失败: %w", err)
		}
		sess.Config.Token = tokenStr
		sess.Config.TokenFile = value
		p.Success(fmt.Sprintf("Token loaded from: %s", value))

	case "api-server":
		sess.Config.APIServer = value
		p.Success(fmt.Sprintf("API Server set to: %s", value))

	case "api-port":
		port, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("无效的端口号: %s", value)
		}
		sess.Config.APIServerPort = port
		p.Success(fmt.Sprintf("API Server Port set to: %d", port))

	case "proxy":
		sess.Config.ProxyURL = value
		if value == "" || value == "none" {
			sess.Config.ProxyURL = ""
			p.Success("Proxy disabled")
		} else {
			p.Success(fmt.Sprintf("Proxy set to: %s", value))
		}
		// 断开现有连接，下次连接时使用新代理
		sess.Disconnect()

	case "concurrency":
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 {
			return fmt.Errorf("无效的并发数: %s (必须 >= 1)", value)
		}
		sess.Config.Concurrency = n
		p.Success(fmt.Sprintf("Concurrency set to: %d", n))

	default:
		p.Println()
		p.Printf("  %s\n\n", p.Colored(config.ColorYellow, "可用配置项:"))
		p.Printf("    %-16s %s\n", "target", "Kubelet IP 地址")
		p.Printf("    %-16s %s\n", "port", "Kubelet 端口")
		p.Printf("    %-16s %s\n", "token", "Token 字符串")
		p.Printf("    %-16s %s\n", "token-file", "Token 文件路径")
		p.Printf("    %-16s %s\n", "api-server", "API Server 地址")
		p.Printf("    %-16s %s\n", "api-port", "API Server 端口")
		p.Printf("    %-16s %s\n", "proxy", "SOCKS5 代理地址")
		p.Printf("    %-16s %s\n", "concurrency", "扫描并发数")
		p.Println()
		return fmt.Errorf("未知配置项: %s", key)
	}

	return nil
}
