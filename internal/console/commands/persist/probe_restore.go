package persist

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"kctl/config"
	"kctl/internal/session"
	"kctl/pkg/types"
	"kctl/utils/Ask"
)

// ProbeRestoreCmd probe-restore 子命令
type ProbeRestoreCmd struct{}

func init() {
	Register(&ProbeRestoreCmd{})
}

func (c *ProbeRestoreCmd) Name() string        { return "probe-restore" }
func (c *ProbeRestoreCmd) Aliases() []string   { return []string{"pr", "restore"} }
func (c *ProbeRestoreCmd) Description() string { return "移除注入的探针" }

func (c *ProbeRestoreCmd) Usage() string {
	return `persist probe-restore [选项]

移除注入的探针，恢复工作负载原始配置

此命令可以移除之前通过 probe-inject 注入的探针。

可选选项:
  --namespace, -n   目标命名空间 (必需，除非交互选择)
  --name            工作负载名称 (必需，除非交互选择)
  --type, -t        工作负载类型 (daemonset/deployment)
  --probe, -p       要移除的探针类型 (liveness/readiness/startup)
  --container, -c   容器名称 (默认: 第一个容器)
  --dry-run         仅预览，不实际执行

示例:
  persist probe-restore                                    # 交互式选择
  persist probe-restore -t daemonset -n kube-system --name kube-proxy -p liveness
  persist probe-restore --dry-run -t deployment -n default --name nginx -p readiness`
}

func (c *ProbeRestoreCmd) Execute(sess *session.Session, args []string) error {
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

	// 如果没有指定工作负载，交互式选择
	if opts.Name == "" {
		p.Printf("%s Scanning workloads with exec probes...\n", p.Colored(config.ColorBlue, "[*]"))

		workloads, err := k8sClient.ListWorkloadsWithProbes(ctx, opts.Namespace)
		if err != nil {
			return fmt.Errorf("扫描工作负载失败: %w", err)
		}

		if len(workloads) == 0 {
			p.Printf("%s No workloads with exec probes found\n", p.Colored(config.ColorYellow, "[!]"))
			return nil
		}

		// 构建选项列表
		workloadOptions := make([]Ask.SelectOption, len(workloads))
		for i, w := range workloads {
			var probeTypes []string
			for _, c := range w.Containers {
				if c.LivenessProbe != nil {
					probeTypes = append(probeTypes, "liveness")
				}
				if c.ReadinessProbe != nil {
					probeTypes = append(probeTypes, "readiness")
				}
				if c.StartupProbe != nil {
					probeTypes = append(probeTypes, "startup")
				}
			}
			workloadOptions[i] = Ask.SelectOption{
				Label:       fmt.Sprintf("%s/%s (%s)", w.Namespace, w.Name, w.Type),
				Value:       fmt.Sprintf("%d", i),
				Description: fmt.Sprintf("Probes: %s", strings.Join(probeTypes, ", ")),
			}
		}

		selectedIdx, err := Ask.SelectOne("选择要恢复的工作负载:", workloadOptions)
		if err != nil {
			return fmt.Errorf("选择工作负载失败: %w", err)
		}

		idx := 0
		fmt.Sscanf(selectedIdx, "%d", &idx)
		selected := workloads[idx]

		opts.Name = selected.Name
		opts.Namespace = selected.Namespace
		opts.WorkloadType = string(selected.Type)

		// 选择要移除的探针
		var probeOptions []Ask.SelectOption
		for _, container := range selected.Containers {
			if container.LivenessProbe != nil {
				probeOptions = append(probeOptions, Ask.SelectOption{
					Label:       fmt.Sprintf("[%s] livenessProbe", container.Name),
					Value:       fmt.Sprintf("%s:liveness", container.Name),
					Description: formatCommand(container.LivenessProbe.Command),
				})
			}
			if container.ReadinessProbe != nil {
				probeOptions = append(probeOptions, Ask.SelectOption{
					Label:       fmt.Sprintf("[%s] readinessProbe", container.Name),
					Value:       fmt.Sprintf("%s:readiness", container.Name),
					Description: formatCommand(container.ReadinessProbe.Command),
				})
			}
			if container.StartupProbe != nil {
				probeOptions = append(probeOptions, Ask.SelectOption{
					Label:       fmt.Sprintf("[%s] startupProbe", container.Name),
					Value:       fmt.Sprintf("%s:startup", container.Name),
					Description: formatCommand(container.StartupProbe.Command),
				})
			}
		}

		if len(probeOptions) == 0 {
			return fmt.Errorf("该工作负载没有 exec 探针")
		}

		selectedProbe, err := Ask.SelectOne("选择要移除的探针:", probeOptions)
		if err != nil {
			return fmt.Errorf("选择探针失败: %w", err)
		}

		parts := strings.Split(selectedProbe, ":")
		opts.Container = parts[0]
		opts.ProbeType = parts[1]
	}

	// 验证必需参数
	if opts.Name == "" {
		return fmt.Errorf("未指定工作负载名称 (--name)")
	}
	if opts.Namespace == "" {
		return fmt.Errorf("未指定命名空间 (--namespace)")
	}
	if opts.ProbeType == "" {
		return fmt.Errorf("未指定探针类型 (--probe)")
	}
	if opts.WorkloadType == "" {
		return fmt.Errorf("未指定工作负载类型 (--type)")
	}

	// 如果没有指定容器名称，获取第一个容器的名称
	if opts.Container == "" {
		containerName, err := c.getFirstContainerName(ctx, k8sClient, opts.Namespace, opts.Name, opts.WorkloadType)
		if err != nil {
			return fmt.Errorf("获取容器名称失败: %w", err)
		}
		opts.Container = containerName
	}

	// 构建移除探针的 Patch
	patch, err := buildRemoveProbePatch(opts.Container, opts.ProbeType)
	if err != nil {
		return fmt.Errorf("构建 Patch 失败: %w", err)
	}

	// 显示预览
	p.Println()
	p.Printf("%s Remove Probe Preview:\n", p.Colored(config.ColorBlue, "[*]"))
	p.Printf("  Workload: %s/%s (%s)\n", opts.Namespace, opts.Name, opts.WorkloadType)
	p.Printf("  Container: %s\n", opts.Container)
	p.Printf("  Probe: %sProbe\n", opts.ProbeType)
	p.Printf("  Patch: %s\n", string(patch))
	p.Println()

	if opts.DryRun {
		p.Printf("%s Dry run mode - no changes applied\n", p.Colored(config.ColorYellow, "[!]"))
		return nil
	}

	// 确认
	if !Ask.ForSure("确认移除探针?") {
		p.Printf("%s Operation cancelled\n", p.Colored(config.ColorYellow, "[!]"))
		return nil
	}

	// 执行 Patch
	p.Printf("%s Removing probe...\n", p.Colored(config.ColorBlue, "[*]"))

	workloadType := types.WorkloadType(opts.WorkloadType)
	switch workloadType {
	case types.WorkloadTypeDaemonSet:
		err = k8sClient.PatchDaemonSet(ctx, opts.Namespace, opts.Name, patch)
	case types.WorkloadTypeDeployment:
		err = k8sClient.PatchDeployment(ctx, opts.Namespace, opts.Name, patch)
	default:
		return fmt.Errorf("不支持的工作负载类型: %s", opts.WorkloadType)
	}

	if err != nil {
		return fmt.Errorf("移除探针失败: %w", err)
	}

	p.Printf("%s Probe removed successfully!\n", p.Colored(config.ColorGreen, "[+]"))
	p.Printf("%s The %s will restart its Pods automatically.\n", p.Colored(config.ColorBlue, "[*]"), opts.WorkloadType)

	return nil
}

func (c *ProbeRestoreCmd) parseArgs(args []string) (*probeRestoreOptions, error) {
	opts := &probeRestoreOptions{
		Container: "", // 默认为空，后续会设为第一个容器
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--namespace", "-n":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--namespace 需要参数")
			}
			i++
			opts.Namespace = args[i]
		case "--name":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--name 需要参数")
			}
			i++
			opts.Name = args[i]
		case "--type", "-t":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--type 需要参数")
			}
			i++
			t := strings.ToLower(args[i])
			switch t {
			case "daemonset", "ds":
				opts.WorkloadType = string(types.WorkloadTypeDaemonSet)
			case "deployment", "deploy":
				opts.WorkloadType = string(types.WorkloadTypeDeployment)
			default:
				return nil, fmt.Errorf("不支持的工作负载类型: %s", args[i])
			}
		case "--probe", "-p":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--probe 需要参数")
			}
			i++
			t := strings.ToLower(args[i])
			switch t {
			case "liveness", "livenessProbe":
				opts.ProbeType = "liveness"
			case "readiness", "readinessProbe":
				opts.ProbeType = "readiness"
			case "startup", "startupProbe":
				opts.ProbeType = "startup"
			default:
				return nil, fmt.Errorf("不支持的探针类型: %s", args[i])
			}
		case "--container", "-c":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--container 需要参数")
			}
			i++
			opts.Container = args[i]
		case "--dry-run":
			opts.DryRun = true
		default:
			if strings.HasPrefix(arg, "-") {
				return nil, fmt.Errorf("未知选项: %s", arg)
			}
		}
	}
	return opts, nil
}

// buildRemoveProbePatch 构建移除探针的 Patch
func buildRemoveProbePatch(containerName, probeType string) ([]byte, error) {
	// 使用 strategic merge patch 将探针设为 null
	probeField := probeType + "Probe"

	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name":     containerName,
							probeField: nil,
						},
					},
				},
			},
		},
	}

	return json.MarshalIndent(patch, "", "  ")
}

type probeRestoreOptions struct {
	Namespace    string
	Name         string
	WorkloadType string
	ProbeType    string
	Container    string
	DryRun       bool
}

// getFirstContainerName 获取工作负载的第一个容器名称
func (c *ProbeRestoreCmd) getFirstContainerName(ctx context.Context, k8sClient interface{}, namespace, name, workloadType string) (string, error) {
	type daemonSetGetter interface {
		GetDaemonSet(ctx context.Context, namespace, name string) (*types.DaemonSetInfo, error)
	}
	type deploymentGetter interface {
		GetDeployment(ctx context.Context, namespace, name string) (*types.DeploymentInfo, error)
	}

	switch types.WorkloadType(workloadType) {
	case types.WorkloadTypeDaemonSet:
		if client, ok := k8sClient.(daemonSetGetter); ok {
			ds, err := client.GetDaemonSet(ctx, namespace, name)
			if err != nil {
				return "", err
			}
			if len(ds.Containers) == 0 {
				return "", fmt.Errorf("DaemonSet %s/%s 没有容器", namespace, name)
			}
			return ds.Containers[0].Name, nil
		}
	case types.WorkloadTypeDeployment:
		if client, ok := k8sClient.(deploymentGetter); ok {
			deploy, err := client.GetDeployment(ctx, namespace, name)
			if err != nil {
				return "", err
			}
			if len(deploy.Containers) == 0 {
				return "", fmt.Errorf("Deployment %s/%s 没有容器", namespace, name)
			}
			return deploy.Containers[0].Name, nil
		}
	}

	return "", fmt.Errorf("不支持的工作负载类型: %s", workloadType)
}
