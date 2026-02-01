package commands

import (
	"context"
	"fmt"

	"kctl/config"
	"kctl/internal/session"
)

// ConnectCmd connect 命令
type ConnectCmd struct{}

func init() {
	Register(&ConnectCmd{})
}

func (c *ConnectCmd) Name() string {
	return "connect"
}

func (c *ConnectCmd) Aliases() []string {
	return []string{"conn"}
}

func (c *ConnectCmd) Description() string {
	return "连接到 Kubelet（可选，命令会自动连接）"
}

func (c *ConnectCmd) Usage() string {
	return `connect [ip]

显式连接到 Kubelet 并验证

注意：此命令是可选的，其他命令（如 pods, sa scan）会自动连接。
使用此命令可以提前验证连接是否正常。

参数：
  ip    可选，Kubelet IP 地址（会自动设置 target）

示例：
  connect                 使用当前配置连接
  connect 10.0.0.1        连接到指定 IP
  set target 10.0.0.1 && connect`
}

func (c *ConnectCmd) Execute(sess *session.Session, args []string) error {
	p := sess.Printer
	ctx := context.Background()

	// 如果提供了 IP 参数，自动设置 target
	if len(args) > 0 {
		sess.Config.KubeletIP = args[0]
		p.Printf("%s Target set to %s\n",
			p.Colored(config.ColorBlue, "[*]"),
			args[0])
	}

	// 检查配置
	if sess.Config.KubeletIP == "" {
		return fmt.Errorf("未设置 Kubelet IP，请使用 'set target <ip>' 设置或 'connect <ip>'")
	}

	if sess.Config.Token == "" {
		return fmt.Errorf("未设置 Token，请使用 'set token <token>' 或 'set token-file <path>' 设置")
	}

	p.Printf("%s Connecting to Kubelet %s:%d...\n",
		p.Colored(config.ColorBlue, "[*]"),
		sess.Config.KubeletIP,
		sess.Config.KubeletPort)

	// 使用懒加载的 GetKubeletClient（会自动连接）
	kubelet, err := sess.GetKubeletClient()
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}

	// 验证连接
	result, err := kubelet.ValidatePort(ctx)
	if err != nil {
		p.Warning("连接成功，但无法验证 Kubelet 端口")
	} else if result.IsKubelet {
		p.Success("Connected successfully")
	} else {
		p.Warning("连接成功，但目标可能不是 Kubelet")
	}

	return nil
}
