package persist

import (
	"context"
	"fmt"
	"strings"

	"kctl/config"
	"kctl/internal/session"
	"kctl/pkg/types"
)

// ProbeListCmd probe-list 子命令
type ProbeListCmd struct{}

func init() {
	Register(&ProbeListCmd{})
}

func (c *ProbeListCmd) Name() string        { return "probe-list" }
func (c *ProbeListCmd) Aliases() []string   { return []string{"pl", "list"} }
func (c *ProbeListCmd) Description() string { return "列出所有带 exec 探针的工作负载" }

func (c *ProbeListCmd) Usage() string {
	return `persist probe-list [选项]

列出所有带 exec 类型探针的工作负载

此命令扫描集群中的 DaemonSet 和 Deployment，找出所有配置了 exec 类型探针的工作负载。
这可以帮助识别可能被注入恶意命令的工作负载。

可选选项:
  --namespace, -n   目标命名空间 (默认: 所有命名空间)
  --type, -t        工作负载类型 (daemonset/deployment/all，默认: all)

示例:
  persist probe-list                           # 列出所有带 exec 探针的工作负载
  persist probe-list -n kube-system            # 只在 kube-system 命名空间中搜索
  persist probe-list -t daemonset              # 只搜索 DaemonSet`
}

func (c *ProbeListCmd) Execute(sess *session.Session, args []string) error {
	p := sess.Printer
	ctx := context.Background()

	// 检查是否请求帮助
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			p.Println(c.Usage())
			return nil
		}
	}

	// 解析参数
	opts, err := c.parseArgs(args)
	if err != nil {
		return err
	}

	// 创建 K8s 客户端
	k8sClient, err := createK8sClient(sess, "", "")
	if err != nil {
		return err
	}

	p.Printf("%s Scanning workloads for exec probes...\n", p.Colored(config.ColorBlue, "[*]"))

	// 获取所有带探针的工作负载
	workloads, err := k8sClient.ListWorkloadsWithProbes(ctx, opts.Namespace)
	if err != nil {
		return fmt.Errorf("扫描工作负载失败: %w", err)
	}

	// 根据类型过滤
	var filtered []types.WorkloadInfo
	for _, w := range workloads {
		if opts.WorkloadType == "" || opts.WorkloadType == "all" {
			filtered = append(filtered, w)
		} else if opts.WorkloadType == "daemonset" && w.Type == types.WorkloadTypeDaemonSet {
			filtered = append(filtered, w)
		} else if opts.WorkloadType == "deployment" && w.Type == types.WorkloadTypeDeployment {
			filtered = append(filtered, w)
		}
	}

	// 显示结果
	if len(filtered) == 0 {
		p.Printf("%s No workloads with exec probes found\n", p.Colored(config.ColorYellow, "[!]"))
		return nil
	}

	p.Printf("%s Found %d workload(s) with exec probes:\n\n", p.Colored(config.ColorGreen, "[+]"), len(filtered))

	for _, w := range filtered {
		p.Printf("%s %s/%s (%s)\n",
			p.Colored(config.ColorCyan, "►"),
			w.Namespace, w.Name, w.Type)

		for _, container := range w.Containers {
			if container.LivenessProbe != nil {
				cmdStr := formatCommand(container.LivenessProbe.Command)
				p.Printf("  %s [%s] livenessProbe: %s\n",
					p.Colored(config.ColorYellow, "•"),
					container.Name, cmdStr)
			}
			if container.ReadinessProbe != nil {
				cmdStr := formatCommand(container.ReadinessProbe.Command)
				p.Printf("  %s [%s] readinessProbe: %s\n",
					p.Colored(config.ColorYellow, "•"),
					container.Name, cmdStr)
			}
			if container.StartupProbe != nil {
				cmdStr := formatCommand(container.StartupProbe.Command)
				p.Printf("  %s [%s] startupProbe: %s\n",
					p.Colored(config.ColorYellow, "•"),
					container.Name, cmdStr)
			}
		}
		p.Println()
	}

	// 显示恢复提示
	p.Printf("%s To remove a probe, use:\n", p.Colored(config.ColorBlue, "[*]"))
	p.Printf("  persist probe-restore -t <type> -n <namespace> --name <name> --probe <probe-type>\n")

	return nil
}

func (c *ProbeListCmd) parseArgs(args []string) (*probeListOptions, error) {
	opts := &probeListOptions{}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--namespace", "-n":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--namespace 需要参数")
			}
			i++
			opts.Namespace = args[i]
		case "--type", "-t":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--type 需要参数")
			}
			i++
			t := strings.ToLower(args[i])
			switch t {
			case "daemonset", "ds":
				opts.WorkloadType = "daemonset"
			case "deployment", "deploy":
				opts.WorkloadType = "deployment"
			case "all", "":
				opts.WorkloadType = "all"
			default:
				return nil, fmt.Errorf("不支持的工作负载类型: %s", args[i])
			}
		default:
			if strings.HasPrefix(arg, "-") {
				return nil, fmt.Errorf("未知选项: %s", arg)
			}
		}
	}
	return opts, nil
}

// formatCommand 格式化命令显示
func formatCommand(cmd []string) string {
	cmdStr := strings.Join(cmd, " ")
	if len(cmdStr) > 80 {
		cmdStr = cmdStr[:77] + "..."
	}
	return cmdStr
}

type probeListOptions struct {
	Namespace    string
	WorkloadType string
}
