package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"kctl/config"
	"kctl/internal/session"
	"kctl/pkg/types"
)

// ExecCmd exec 命令
type ExecCmd struct{}

// 常用 shell 列表
var defaultShells = []string{
	"/bin/bash",
	"/bin/sh",
	"/bin/ash",
	"/bin/zsh",
}

func init() {
	Register(&ExecCmd{})
}

func (c *ExecCmd) Name() string {
	return "exec"
}

func (c *ExecCmd) Aliases() []string {
	return nil
}

func (c *ExecCmd) Description() string {
	return "执行命令"
}

func (c *ExecCmd) Usage() string {
	return `exec [options] [pod] -- <command>
exec -it [pod]                    进入交互式 shell

在 Pod 中执行命令

选项：
  -n <namespace>    指定命名空间
  -c <container>    指定容器
  -it               交互式 shell（自动探测可用 shell）
  --shell <shell>   指定 shell 路径（默认自动探测）

示例：
  exec -- whoami                    执行单条命令
  exec nginx -- cat /etc/passwd     在指定 Pod 中执行
  exec -it                          进入当前 SA Pod 的交互式 shell
  exec -it nginx                    进入指定 Pod 的交互式 shell
  exec -it --shell /bin/bash nginx  使用指定 shell`
}

func (c *ExecCmd) Execute(sess *session.Session, args []string) error {
	p := sess.Printer
	ctx := context.Background()

	// 检查连接
	kubelet, err := sess.GetKubeletClient()
	if err != nil {
		return err
	}

	// 解析参数
	namespace := ""
	container := ""
	podName := ""
	interactive := false
	shellPath := ""
	var command []string

	// 查找 -- 分隔符
	cmdStart := -1
	for i, arg := range args {
		if arg == "--" {
			cmdStart = i + 1
			break
		}
	}

	// 解析选项
	for i := 0; i < len(args); i++ {
		if cmdStart != -1 && i >= cmdStart {
			break
		}
		switch args[i] {
		case "-n":
			if i+1 < len(args) {
				namespace = args[i+1]
				i++
			}
		case "-c":
			if i+1 < len(args) {
				container = args[i+1]
				i++
			}
		case "-it", "-ti", "--interactive":
			interactive = true
		case "--shell":
			if i+1 < len(args) {
				shellPath = args[i+1]
				i++
			}
		case "--":
			// 跳过
		default:
			if !strings.HasPrefix(args[i], "-") && podName == "" {
				podName = args[i]
			}
		}
	}

	// 获取命令
	if cmdStart != -1 && cmdStart < len(args) {
		command = args[cmdStart:]
	}

	// 如果是交互模式但没有指定命令，需要探测 shell
	if interactive && len(command) == 0 {
		// 稍后探测 shell
	} else if !interactive && len(command) == 0 {
		return fmt.Errorf("用法: exec [pod] -- <command> 或 exec -it [pod]")
	}

	// 如果没有指定 Pod，尝试使用当前 SA 的 Pod
	if podName == "" {
		sa := sess.GetCurrentSA()
		if sa != nil && sa.Pods != "" && sa.Pods != "[]" {
			var pods []types.SAPodInfo
			if err := json.Unmarshal([]byte(sa.Pods), &pods); err == nil && len(pods) > 0 {
				podName = pods[0].Name
				if namespace == "" {
					namespace = pods[0].Namespace
				}
				if container == "" && pods[0].Container != "" {
					container = pods[0].Container
				}
				p.Printf("%s Using pod: %s/%s (from current SA)\n",
					p.Colored(config.ColorBlue, "[*]"),
					namespace, podName)
			}
		}
	}

	if podName == "" {
		return fmt.Errorf("请指定 Pod 名称或先使用 'use' 选择一个 SA")
	}

	// 如果没有指定命名空间，尝试从缓存中查找
	if namespace == "" {
		pods := sess.GetCachedPods()
		for _, pod := range pods {
			if pod.PodName == podName {
				namespace = pod.Namespace
				if container == "" && len(pod.Containers) > 0 {
					container = pod.Containers[0].Name
				}
				break
			}
		}
	}

	if namespace == "" {
		namespace = "default"
	}

	// 如果没有指定容器，获取第一个容器
	if container == "" {
		pods := sess.GetCachedPods()
		for _, pod := range pods {
			if pod.PodName == podName && pod.Namespace == namespace {
				if len(pod.Containers) > 0 {
					container = pod.Containers[0].Name
				}
				break
			}
		}
	}

	// 交互式模式
	if interactive {
		return c.execInteractive(ctx, sess, kubelet, namespace, podName, container, shellPath)
	}

	// 非交互式执行
	return c.execCommand(ctx, sess, kubelet, namespace, podName, container, command)
}

// execCommand 执行单条命令
func (c *ExecCmd) execCommand(ctx context.Context, sess *session.Session, kubelet interface {
	Exec(ctx context.Context, opts *types.ExecOptions) (*types.ExecResult, error)
}, namespace, podName, container string, command []string) error {
	p := sess.Printer

	opts := &types.ExecOptions{
		Namespace: namespace,
		Pod:       podName,
		Container: container,
		Command:   command,
		Stdin:     false,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}

	result, err := kubelet.Exec(ctx, opts)
	if err != nil {
		return fmt.Errorf("执行命令失败: %w", err)
	}

	if result.Stdout != "" {
		p.Print(result.Stdout)
		if !strings.HasSuffix(result.Stdout, "\n") {
			p.Println()
		}
	}
	if result.Error != "" {
		p.Error(result.Error)
	}

	return nil
}

// execInteractive 交互式 shell
func (c *ExecCmd) execInteractive(ctx context.Context, sess *session.Session, kubelet interface {
	Exec(ctx context.Context, opts *types.ExecOptions) (*types.ExecResult, error)
	ExecInteractive(ctx context.Context, opts *types.ExecOptions) error
}, namespace, podName, container, shellPath string) error {
	p := sess.Printer

	// 如果指定了 shell，直接使用
	if shellPath != "" {
		p.Printf("%s Starting shell: %s\n",
			p.Colored(config.ColorBlue, "[*]"),
			p.Colored(config.ColorGreen, shellPath))
		return c.startShell(ctx, kubelet, namespace, podName, container, shellPath)
	}

	// 探测可用的 shell
	p.Printf("%s Detecting available shells...\n",
		p.Colored(config.ColorBlue, "[*]"))

	availableShells := c.detectShells(ctx, kubelet, namespace, podName, container)

	if len(availableShells) == 0 {
		return fmt.Errorf("未找到可用的 shell，请使用 --shell 指定")
	}

	// 显示可用的 shell
	p.Printf("%s Available shells:\n", p.Colored(config.ColorGreen, "[+]"))
	for i, shell := range availableShells {
		p.Printf("    %s %s\n",
			p.Colored(config.ColorCyan, fmt.Sprintf("[%d]", i+1)),
			shell)
	}

	// 使用第一个可用的 shell
	selectedShell := availableShells[0]
	p.Printf("%s Using: %s\n",
		p.Colored(config.ColorBlue, "[*]"),
		p.Colored(config.ColorGreen, selectedShell))
	p.Printf("%s Press Ctrl+D or type 'exit' to quit\n",
		p.Colored(config.ColorGray, "[*]"))
	p.Println()

	return c.startShell(ctx, kubelet, namespace, podName, container, selectedShell)
}

// detectShells 探测可用的 shell
func (c *ExecCmd) detectShells(ctx context.Context, kubelet interface {
	Exec(ctx context.Context, opts *types.ExecOptions) (*types.ExecResult, error)
}, namespace, podName, container string) []string {
	var available []string

	for _, shell := range defaultShells {
		// 使用 which 或直接测试 shell 是否存在
		opts := &types.ExecOptions{
			Namespace: namespace,
			Pod:       podName,
			Container: container,
			Command:   []string{"test", "-x", shell},
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}

		result, err := kubelet.Exec(ctx, opts)
		if err == nil && result.Error == "" {
			available = append(available, shell)
		}
	}

	// 如果没有找到，尝试 which 命令
	if len(available) == 0 {
		for _, shell := range defaultShells {
			shellName := shell[strings.LastIndex(shell, "/")+1:]
			opts := &types.ExecOptions{
				Namespace: namespace,
				Pod:       podName,
				Container: container,
				Command:   []string{"which", shellName},
				Stdin:     false,
				Stdout:    true,
				Stderr:    true,
				TTY:       false,
			}

			result, err := kubelet.Exec(ctx, opts)
			if err == nil && result.Error == "" && result.Stdout != "" {
				path := strings.TrimSpace(result.Stdout)
				if path != "" {
					available = append(available, path)
				}
			}
		}
	}

	return available
}

// startShell 启动交互式 shell
func (c *ExecCmd) startShell(ctx context.Context, kubelet interface {
	ExecInteractive(ctx context.Context, opts *types.ExecOptions) error
}, namespace, podName, container, shell string) error {
	opts := &types.ExecOptions{
		Namespace: namespace,
		Pod:       podName,
		Container: container,
		Command:   []string{shell},
		Stdin:     true,
		Stdout:    true,
		Stderr:    true,
		TTY:       true,
	}

	return kubelet.ExecInteractive(ctx, opts)
}
