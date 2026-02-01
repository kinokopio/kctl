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
	return "连接到 Kubelet"
}

func (c *ConnectCmd) Usage() string {
	return `connect

使用当前配置连接到 Kubelet

在连接前，请确保已设置：
  - target (Kubelet IP)
  - token 或 token-file

示例：
  # 使用当前配置连接
  connect

  # 先设置目标再连接
  set target 10.0.0.1
  connect`
}

func (c *ConnectCmd) Execute(sess *session.Session, args []string) error {
	p := sess.Printer
	ctx := context.Background()

	// 检查配置
	if sess.Config.KubeletIP == "" {
		return fmt.Errorf("未设置 Kubelet IP，请使用 'set target <ip>' 设置")
	}

	if sess.Config.Token == "" {
		return fmt.Errorf("未设置 Token，请使用 'set token <token>' 或 'set token-file <path>' 设置")
	}

	p.Printf("%s Connecting to Kubelet %s:%d...\n",
		p.Colored(config.ColorBlue, "[*]"),
		sess.Config.KubeletIP,
		sess.Config.KubeletPort)

	// 连接
	if err := sess.Connect(); err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}

	// 验证连接
	kubelet, err := sess.GetKubeletClient()
	if err != nil {
		return err
	}

	// 尝试获取节点信息
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
