package commands

import (
	"context"
	"fmt"
	"strings"

	"kctl/config"
	"kctl/internal/output"
	"kctl/internal/session"
	"kctl/pkg/types"
)

// PodsCmd pods 命令
type PodsCmd struct{}

func init() {
	Register(&PodsCmd{})
}

func (c *PodsCmd) Name() string {
	return "pods"
}

func (c *PodsCmd) Aliases() []string {
	return []string{"po"}
}

func (c *PodsCmd) Description() string {
	return "列出 Pod"
}

func (c *PodsCmd) Usage() string {
	return `pods [options]

列出节点上的 Pod

选项：
  --detail, -d        显示详细信息
  --privileged, -P    只显示特权 Pod
  --running, -R       只显示 Running 状态的 Pod
  -n <namespace>      按命名空间过滤
  --refresh           强制刷新（重新从 Kubelet 获取）

示例：
  pods                    列出所有 Pod
  pods --detail           显示详细信息
  pods --privileged       只显示特权 Pod
  pods -n kube-system     只显示 kube-system 命名空间的 Pod`
}

func (c *PodsCmd) Execute(sess *session.Session, args []string) error {
	p := sess.Printer
	ctx := context.Background()

	// 解析参数
	showDetail := false
	onlyPrivileged := false
	onlyRunning := false
	namespace := ""
	refresh := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--detail", "-d":
			showDetail = true
		case "--privileged", "-P":
			onlyPrivileged = true
		case "--running", "-R":
			onlyRunning = true
		case "-n":
			if i+1 < len(args) {
				namespace = args[i+1]
				i++
			}
		case "--refresh":
			refresh = true
		}
	}

	// 获取 Pod 列表
	pods := sess.GetCachedPods()

	// 如果没有缓存或需要刷新，从 Kubelet 获取
	if len(pods) == 0 || refresh {
		kubelet, err := sess.GetKubeletClient()
		if err != nil {
			return err
		}

		p.Printf("%s Fetching pods from Kubelet...\n",
			p.Colored(config.ColorBlue, "[*]"))

		pods, err = kubelet.GetPodsWithContainers(ctx)
		if err != nil {
			return fmt.Errorf("获取 Pod 列表失败: %w", err)
		}

		// 缓存
		sess.CachePods(pods)
	}

	if len(pods) == 0 {
		p.Warning("没有找到 Pod")
		return nil
	}

	// 过滤
	var filtered []types.PodContainerInfo
	for _, pod := range pods {
		// 命名空间过滤
		if namespace != "" && pod.Namespace != namespace {
			continue
		}

		// Running 过滤
		if onlyRunning && pod.Status != "Running" {
			continue
		}

		// 特权过滤
		if onlyPrivileged && !pod.SecurityFlags.Privileged {
			continue
		}

		filtered = append(filtered, pod)
	}

	if len(filtered) == 0 {
		p.Warning("没有符合条件的 Pod")
		return nil
	}

	p.Println()

	// 根据是否显示详情选择输出格式
	if showDetail {
		c.printDetail(p, filtered)
	} else {
		c.printTable(p, filtered)
	}

	p.Printf("\n  共 %d 个 Pod\n\n", len(filtered))

	return nil
}

// printTable 表格形式输出
func (c *PodsCmd) printTable(p output.Printer, pods []types.PodContainerInfo) {
	var rows []output.PodRow
	for _, pod := range pods {
		flags := c.buildFlags(p, pod.SecurityFlags)
		rows = append(rows, output.PodRow{
			Name:           pod.PodName,
			Namespace:      pod.Namespace,
			ServiceAccount: pod.ServiceAccount,
			PodIP:          pod.PodIP,
			NodeName:       pod.NodeName,
			Flags:          flags,
		})
	}

	tablePrinter := output.NewTablePrinter()
	tablePrinter.PrintPods(rows)
}

// printDetail 详细信息输出
func (c *PodsCmd) printDetail(p output.Printer, pods []types.PodContainerInfo) {
	for i, pod := range pods {
		// Pod 标题
		statusColor := config.ColorGreen
		if pod.Status != "Running" {
			statusColor = config.ColorYellow
		}

		p.Printf("  %s %s/%s\n",
			p.Colored(config.ColorCyan, fmt.Sprintf("[%d]", i+1)),
			p.Colored(config.ColorWhite, pod.Namespace),
			p.Colored(config.ColorWhite, pod.PodName))

		p.Println("  " + p.Colored(config.ColorGray, strings.Repeat("─", 60)))

		// 基本信息
		p.Printf("    %-18s: %s\n", "Status", p.Colored(statusColor, pod.Status))
		p.Printf("    %-18s: %s\n", "Pod IP", pod.PodIP)
		p.Printf("    %-18s: %s\n", "Host IP", pod.HostIP)
		p.Printf("    %-18s: %s\n", "Node", pod.NodeName)
		p.Printf("    %-18s: %s\n", "ServiceAccount", pod.ServiceAccount)
		if pod.CreatedAt != "" {
			p.Printf("    %-18s: %s\n", "Created", pod.CreatedAt)
		}
		if pod.UID != "" {
			p.Printf("    %-18s: %s\n", "UID", p.Colored(config.ColorGray, pod.UID))
		}

		// 安全标识摘要
		p.Printf("    %-18s: %s\n", "Security Flags", c.buildFlags(p, pod.SecurityFlags))

		// 容器详情
		p.Println()
		p.Printf("    %s (%d)\n", p.Colored(config.ColorYellow, "Containers"), len(pod.Containers))
		for j, container := range pod.Containers {
			c.printContainerDetail(p, container, j+1)
		}

		// Volumes
		if len(pod.Volumes) > 0 {
			p.Println()
			p.Printf("    %s (%d)\n", p.Colored(config.ColorYellow, "Volumes"), len(pod.Volumes))
			for _, vol := range pod.Volumes {
				typeColor := config.ColorGray
				if vol.Type == "hostPath" {
					typeColor = config.ColorRed
				} else if vol.Type == "secret" {
					typeColor = config.ColorYellow
				}
				p.Printf("      %-20s %s",
					vol.Name,
					p.Colored(typeColor, fmt.Sprintf("[%s]", vol.Type)))
				if vol.Source != "" {
					p.Printf(" -> %s", p.Colored(config.ColorCyan, vol.Source))
				}
				p.Println()
			}
		}

		p.Println()
	}
}

// printContainerDetail 打印容器详情
func (c *PodsCmd) printContainerDetail(p output.Printer, container types.ContainerDetail, index int) {
	// 容器名称和状态
	stateColor := config.ColorGreen
	if !strings.HasPrefix(container.State, "Running") {
		stateColor = config.ColorYellow
	}

	p.Printf("      %s %s\n",
		p.Colored(config.ColorCyan, fmt.Sprintf("[%d]", index)),
		p.Colored(config.ColorWhite, container.Name))

	p.Printf("          %-14s: %s\n", "Image", p.Colored(config.ColorGray, container.Image))
	p.Printf("          %-14s: %s\n", "State", p.Colored(stateColor, container.State))

	if container.StartedAt != "" {
		p.Printf("          %-14s: %s\n", "Started", container.StartedAt)
	}

	// 安全上下文
	if container.Privileged || container.AllowPE {
		p.Printf("          %-14s: ", "Security")
		var secFlags []string
		if container.Privileged {
			secFlags = append(secFlags, p.Colored(config.ColorRed, "Privileged"))
		}
		if container.AllowPE {
			secFlags = append(secFlags, p.Colored(config.ColorYellow, "AllowPrivilegeEscalation"))
		}
		p.Println(strings.Join(secFlags, ", "))
	}

	// 挂载点
	if len(container.VolumeMounts) > 0 {
		p.Printf("          %-14s:\n", "Mounts")
		for _, vm := range container.VolumeMounts {
			// 根据类型着色
			typeColor := config.ColorGray
			mountColor := config.ColorWhite
			if vm.Type == "hostPath" {
				typeColor = config.ColorRed
				mountColor = config.ColorRed
			} else if vm.Type == "secret" {
				typeColor = config.ColorYellow
			}

			readOnlyStr := ""
			if vm.ReadOnly {
				readOnlyStr = p.Colored(config.ColorGray, " (ro)")
			}

			p.Printf("            %s%s\n",
				p.Colored(mountColor, vm.MountPath),
				readOnlyStr)

			// 显示源
			if vm.Source != "" {
				p.Printf("              └─ %s: %s\n",
					p.Colored(typeColor, vm.Type),
					p.Colored(config.ColorCyan, vm.Source))
			} else if vm.Type != "" {
				p.Printf("              └─ %s\n",
					p.Colored(typeColor, vm.Type))
			}
		}
	}
}

// buildFlags 构建简短的 flags 字符串
func (c *PodsCmd) buildFlags(p output.Printer, flags types.SecurityFlags) string {
	var result []string

	if flags.Privileged {
		result = append(result, p.Colored(config.ColorRed, "PRIV"))
	}
	if flags.AllowPrivilegeEscalation {
		result = append(result, p.Colored(config.ColorYellow, "PE"))
	}
	if flags.HasHostPath {
		result = append(result, p.Colored(config.ColorRed, "HP"))
	}
	if flags.HasSecretMount {
		result = append(result, p.Colored(config.ColorYellow, "SEC"))
	}
	if flags.HasSATokenMount {
		result = append(result, p.Colored(config.ColorGreen, "SA"))
	}

	if len(result) == 0 {
		return "-"
	}
	return strings.Join(result, ",")
}
