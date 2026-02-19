package console

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/c-bata/go-prompt"

	"kctl/config"
	"kctl/internal/console/commands"
	"kctl/internal/session"
	"kctl/pkg/kubeconfig"
	"kctl/pkg/token"
)

// Options 控制台启动选项
type Options struct {
	Target     string // Kubelet IP
	Port       int    // Kubelet 端口
	TokenFile  string // Token 文件路径
	Token      string // Token 字符串
	Proxy      string // SOCKS5 代理
	APIServer  string // API Server 地址
	APIPort    int    // API Server 端口
	Kubeconfig string // kubeconfig 文件路径
}

// Console 交互式控制台
type Console struct {
	session  *session.Session
	executor *Executor
	exitFlag bool
}

// New 创建控制台（使用默认选项）
func New() (*Console, error) {
	return NewWithOptions(Options{})
}

// NewWithOptions 使用指定选项创建控制台
func NewWithOptions(opts Options) (*Console, error) {
	sess, err := session.NewSession()
	if err != nil {
		return nil, fmt.Errorf("创建会话失败: %w", err)
	}

	// 处理 kubeconfig (优先级最高)
	if opts.Kubeconfig != "" {
		parsed, err := kubeconfig.Parse(opts.Kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("解析 kubeconfig 失败: %w", err)
		}

		// 设置 API Server
		if parsed.Server != "" {
			sess.Config.APIServer = parsed.Server
			// 解析端口
			if strings.Contains(parsed.Server, ":") {
				parts := strings.Split(parsed.Server, ":")
				if len(parts) >= 3 {
					portStr := parts[len(parts)-1]
					if idx := strings.Index(portStr, "/"); idx != -1 {
						portStr = portStr[:idx]
					}
					if port, err := strconv.Atoi(portStr); err == nil {
						sess.Config.APIServerPort = port
					}
				}
			}
		}

		// 设置认证信息
		if parsed.HasToken() {
			sess.Config.Token = parsed.Token
		} else if parsed.HasClientCert() {
			// 使用客户端证书认证
			sess.Config.ClientCert = parsed.ClientCert
			sess.Config.ClientKey = parsed.ClientKey
			if len(parsed.CACert) > 0 {
				sess.Config.CACert = parsed.CACert
			}
		} else {
			return nil, fmt.Errorf("kubeconfig 中没有可用的认证信息")
		}

		// 自动切换到 kubernetes 模式
		sess.Mode = session.ModeKubernetes
	}

	// 应用命令行参数覆盖 (kubeconfig 之后的参数可以覆盖)
	if opts.Target != "" {
		sess.Config.KubeletIP = opts.Target
	}
	if opts.Port > 0 {
		sess.Config.KubeletPort = opts.Port
	}
	if opts.TokenFile != "" {
		if tokenStr, err := token.Read(opts.TokenFile); err == nil {
			sess.Config.Token = tokenStr
			sess.Config.TokenFile = opts.TokenFile
		}
	}
	if opts.Token != "" {
		sess.Config.Token = opts.Token
	}
	if opts.Proxy != "" {
		sess.Config.ProxyURL = opts.Proxy
	}
	if opts.APIServer != "" {
		sess.Config.APIServer = opts.APIServer
	}
	if opts.APIPort > 0 {
		sess.Config.APIServerPort = opts.APIPort
	}

	c := &Console{
		session:  sess,
		executor: NewExecutor(sess),
	}

	return c, nil
}

// Run 运行控制台主循环
func (c *Console) Run() {
	// 打印 Banner
	PrintBanner(c.session)

	// 自动连接
	c.autoConnect()

	// 创建 prompt
	p := prompt.New(
		c.executorWrapper,
		c.completer,
		prompt.OptionPrefix(c.getPrompt()),
		prompt.OptionLivePrefix(c.getLivePrefix),
		prompt.OptionTitle("kctl console"),
		prompt.OptionPrefixTextColor(prompt.Cyan),
		prompt.OptionPreviewSuggestionTextColor(prompt.Blue),
		prompt.OptionSelectedSuggestionBGColor(prompt.LightGray),
		prompt.OptionSuggestionBGColor(prompt.DarkGray),
	)

	// 运行主循环
	p.Run()
}

// executorWrapper 命令执行包装器
func (c *Console) executorWrapper(input string) {
	c.executor.Execute(input)
}

// completer 自动补全
func (c *Console) completer(d prompt.Document) []prompt.Suggest {
	// 获取当前输入
	text := d.TextBeforeCursor()
	if text == "" {
		return nil
	}

	args := parseArgs(text)
	if len(args) == 0 {
		return c.getCommandSuggestions("")
	}

	cmd := args[0]
	word := d.GetWordBeforeCursor()

	// 如果只有一个词且没有空格，补全命令
	if len(args) == 1 && !strings.HasSuffix(text, " ") {
		return c.getCommandSuggestions(word)
	}

	// 根据命令补全参数
	switch cmd {
	case "mode":
		return c.getModeSuggestions(word)
	case "exec":
		return c.getExecSuggestions(args, word)
	case "set":
		return c.getSetSuggestions(word)
	case "show":
		return c.getShowSuggestions(word)
	case "export":
		return c.getExportSuggestions(word)
	case "help", "?", "h":
		return c.getCommandSuggestions(word)
	case "sa":
		return c.getSASuggestions(args, word)
	case "pods", "po":
		return c.getPodsFlagSuggestions(word)
	case "scan":
		return c.getScanFlagSuggestions(word)
	case "discover", "disc":
		return c.getDiscoverSuggestions(args, word)
	case "run":
		return c.getRunSuggestions(args, word)
	case "portforward", "pf":
		return c.getPortForwardSuggestions(args, word)
	case "pid2pod", "p2p":
		return c.getPid2PodSuggestions(word)
	case "golden", "gt":
		return c.getGoldenSuggestions(args, word)
	case "persist", "ps":
		return c.getPersistSuggestions(args, word)
	}

	return nil
}

// getCommandSuggestions 获取命令建议（根据当前模式过滤）
func (c *Console) getCommandSuggestions(prefix string) []prompt.Suggest {
	mode := c.session.GetMode()
	var suggestions []prompt.Suggest

	// 通用命令
	commonCmds := []prompt.Suggest{
		{Text: "help", Description: "显示帮助信息"},
		{Text: "mode", Description: "查看或切换运行模式"},
		{Text: "set", Description: "设置配置"},
		{Text: "show", Description: "显示信息"},
		{Text: "export", Description: "导出结果"},
		{Text: "clear", Description: "清除缓存"},
		{Text: "exit", Description: "退出控制台"},
	}

	// Kubelet 模式命令
	kubeletCmds := []prompt.Suggest{
		{Text: "connect", Description: "连接到 Kubelet"},
		{Text: "discover", Description: "扫描网络发现 Kubelet"},
		{Text: "sa", Description: "ServiceAccount 操作"},
		{Text: "pods", Description: "列出 Pod"},
		{Text: "exec", Description: "执行命令 (WebSocket)"},
		{Text: "run", Description: "执行命令 (/run API)"},
		{Text: "portforward", Description: "端口转发"},
		{Text: "pid2pod", Description: "将 PID 映射到 Pod"},
	}

	// Kubernetes 模式命令
	kubernetesCmds := []prompt.Suggest{
		{Text: "golden", Description: "Golden Ticket 伪造"},
		{Text: "persist", Description: "持久化攻击工具"},
	}

	// 添加通用命令
	suggestions = append(suggestions, commonCmds...)

	// 根据模式添加特定命令
	if mode == session.ModeKubelet {
		suggestions = append(suggestions, kubeletCmds...)
	} else {
		suggestions = append(suggestions, kubernetesCmds...)
	}

	return prompt.FilterHasPrefix(suggestions, prefix, true)
}

// getUseSuggestions 获取 use 命令的 SA 补全
func (c *Console) getUseSuggestions(word string) []prompt.Suggest {
	var suggestions []prompt.Suggest

	// 从数据库获取已扫描的 SA
	if c.session.SADB != nil {
		sas, err := c.session.SADB.GetAll()
		if err == nil {
			for _, sa := range sas {
				saName := fmt.Sprintf("%s/%s", sa.Namespace, sa.Name)
				desc := sa.RiskLevel
				if sa.IsClusterAdmin {
					desc = "ADMIN"
				}
				suggestions = append(suggestions, prompt.Suggest{
					Text:        saName,
					Description: desc,
				})
			}
		}
	}

	if len(suggestions) == 0 {
		suggestions = []prompt.Suggest{
			{Text: "<namespace/sa-name>", Description: "先执行 scan 扫描 SA"},
		}
	}

	return prompt.FilterHasPrefix(suggestions, word, true)
}

// getExecSuggestions 获取 exec 命令的补全
func (c *Console) getExecSuggestions(args []string, word string) []prompt.Suggest {
	// 检查是否已经有 -- 分隔符
	for _, arg := range args {
		if arg == "--" {
			return nil // -- 之后不补全
		}
	}

	// 检查上一个参数，决定补全什么
	if len(args) >= 2 {
		lastArg := args[len(args)-1]
		// 如果当前正在输入（word 不为空），检查倒数第二个参数
		if word != "" && len(args) >= 2 {
			lastArg = args[len(args)-2]
		}

		switch lastArg {
		case "--shell":
			// 补全 shell 路径
			return c.getShellSuggestions(word)
		case "-n":
			// 补全命名空间
			return c.getNamespaceSuggestions(word)
		case "-c":
			// 补全容器名
			return c.getContainerSuggestions(args, word)
		case "--filter":
			// 补全要排除的 Pod
			return c.getFilterPodSuggestions(word)
		case "--filter-ns":
			// 补全要排除的命名空间
			return c.getNamespaceSuggestions(word)
		case "--concurrency":
			// 补全并发数
			return c.getConcurrencySuggestions(word)
		}
	}

	var suggestions []prompt.Suggest

	// 补全选项
	suggestions = append(suggestions,
		prompt.Suggest{Text: "-it", Description: "交互式 shell"},
		prompt.Suggest{Text: "--shell", Description: "指定 shell 路径"},
		prompt.Suggest{Text: "-n", Description: "指定命名空间"},
		prompt.Suggest{Text: "-c", Description: "指定容器"},
		prompt.Suggest{Text: "--all-pods", Description: "在所有 Pod 中执行"},
		prompt.Suggest{Text: "--filter", Description: "排除指定 Pod（逗号分隔）"},
		prompt.Suggest{Text: "--filter-ns", Description: "排除指定命名空间（逗号分隔）"},
		prompt.Suggest{Text: "--concurrency", Description: "并发数（默认: 10）"},
		prompt.Suggest{Text: "--", Description: "命令分隔符"},
	)

	// 补全 Pod 名称
	pods := c.session.GetCachedPods()
	for _, pod := range pods {
		if pod.Status == "Running" {
			suggestions = append(suggestions, prompt.Suggest{
				Text:        pod.PodName,
				Description: pod.Namespace,
			})
		}
	}

	return prompt.FilterHasPrefix(suggestions, word, true)
}

// getShellSuggestions 获取 shell 路径补全
func (c *Console) getShellSuggestions(word string) []prompt.Suggest {
	suggestions := []prompt.Suggest{
		{Text: "/bin/bash", Description: "Bash shell"},
		{Text: "/bin/sh", Description: "Bourne shell"},
		{Text: "/bin/ash", Description: "Alpine shell"},
		{Text: "/bin/zsh", Description: "Z shell"},
		{Text: "/usr/bin/bash", Description: "Bash shell"},
		{Text: "/usr/bin/zsh", Description: "Z shell"},
	}
	return prompt.FilterHasPrefix(suggestions, word, true)
}

// getNamespaceSuggestions 获取命名空间补全
func (c *Console) getNamespaceSuggestions(word string) []prompt.Suggest {
	var suggestions []prompt.Suggest
	seen := make(map[string]bool)

	// 从缓存的 Pod 中获取命名空间
	pods := c.session.GetCachedPods()
	for _, pod := range pods {
		if !seen[pod.Namespace] {
			seen[pod.Namespace] = true
			suggestions = append(suggestions, prompt.Suggest{
				Text:        pod.Namespace,
				Description: "namespace",
			})
		}
	}

	return prompt.FilterHasPrefix(suggestions, word, true)
}

// getContainerSuggestions 获取容器名补全
func (c *Console) getContainerSuggestions(args []string, word string) []prompt.Suggest {
	var suggestions []prompt.Suggest

	// 尝试找到指定的 Pod
	podName := ""
	namespace := ""
	for i, arg := range args {
		if arg == "-n" && i+1 < len(args) {
			namespace = args[i+1]
		}
		if !strings.HasPrefix(arg, "-") && arg != "exec" {
			podName = arg
		}
	}

	pods := c.session.GetCachedPods()
	for _, pod := range pods {
		// 如果指定了 Pod 名，只显示该 Pod 的容器
		if podName != "" && pod.PodName != podName {
			continue
		}
		// 如果指定了命名空间，过滤
		if namespace != "" && pod.Namespace != namespace {
			continue
		}

		for _, container := range pod.Containers {
			suggestions = append(suggestions, prompt.Suggest{
				Text:        container.Name,
				Description: fmt.Sprintf("%s/%s", pod.Namespace, pod.PodName),
			})
		}
	}

	return prompt.FilterHasPrefix(suggestions, word, true)
}

// getFilterPodSuggestions 获取 filter 的 Pod 补全
func (c *Console) getFilterPodSuggestions(word string) []prompt.Suggest {
	var suggestions []prompt.Suggest
	pods := c.session.GetCachedPods()
	for _, pod := range pods {
		if pod.Status == "Running" {
			suggestions = append(suggestions, prompt.Suggest{
				Text:        pod.PodName,
				Description: fmt.Sprintf("排除 %s", pod.Namespace),
			})
		}
	}
	return prompt.FilterHasPrefix(suggestions, word, true)
}

// getSetSuggestions 获取 set 命令建议
func (c *Console) getSetSuggestions(word string) []prompt.Suggest {
	suggestions := []prompt.Suggest{
		{Text: "target", Description: "Kubelet IP 地址"},
		{Text: "port", Description: "Kubelet 端口"},
		{Text: "token", Description: "Token 字符串"},
		{Text: "token-file", Description: "Token 文件路径"},
		{Text: "kubeconfig", Description: "kubeconfig 文件路径"},
		{Text: "api-server", Description: "API Server 地址"},
		{Text: "api-port", Description: "API Server 端口"},
		{Text: "proxy", Description: "SOCKS5 代理地址"},
		{Text: "concurrency", Description: "扫描并发数"},
	}
	return prompt.FilterHasPrefix(suggestions, word, true)
}

// getShowSuggestions 获取 show 命令建议
func (c *Console) getShowSuggestions(word string) []prompt.Suggest {
	suggestions := []prompt.Suggest{
		{Text: "options", Description: "显示当前配置"},
		{Text: "status", Description: "显示会话状态"},
		{Text: "env", Description: "显示环境信息"},
	}
	return prompt.FilterHasPrefix(suggestions, word, true)
}

// getExportSuggestions 获取 export 命令建议
func (c *Console) getExportSuggestions(word string) []prompt.Suggest {
	suggestions := []prompt.Suggest{
		{Text: "json", Description: "JSON 格式"},
		{Text: "csv", Description: "CSV 格式"},
	}
	return prompt.FilterHasPrefix(suggestions, word, true)
}

func (c *Console) getModeSuggestions(word string) []prompt.Suggest {
	suggestions := []prompt.Suggest{
		{Text: "kubelet", Description: "Kubelet API (10250)"},
		{Text: "kubernetes", Description: "API Server (6443)"},
	}
	return prompt.FilterHasPrefix(suggestions, word, true)
}

// getSAFlagSuggestions 获取 sa 命令的选项补全
func (c *Console) getSAFlagSuggestions(word string) []prompt.Suggest {
	suggestions := []prompt.Suggest{
		{Text: "scan", Description: "扫描所有 Pod 的 SA Token"},
		{Text: "list", Description: "列出已扫描的 SA"},
		{Text: "use", Description: "选择 SA 作为当前身份"},
		{Text: "info", Description: "显示当前 SA 详情"},
		{Text: "--admin", Description: "只显示 cluster-admin"},
		{Text: "--risky", Description: "只显示有风险的 SA"},
		{Text: "-n", Description: "按命名空间过滤"},
		{Text: "--perms", Description: "显示权限"},
		{Text: "--token", Description: "显示 Token"},
	}
	return prompt.FilterHasPrefix(suggestions, word, true)
}

func (c *Console) getSASuggestions(args []string, word string) []prompt.Suggest {
	if len(args) >= 2 {
		subCmd := args[1]
		if word != "" && len(args) == 2 {
			return c.getSAFlagSuggestions(word)
		}
		switch subCmd {
		case "use":
			return c.getUseSuggestions(word)
		case "scan":
			return c.getScanFlagSuggestions(word)
		case "list":
			return c.getSAListFlagSuggestions(word)
		}
	}
	return c.getSAFlagSuggestions(word)
}

func (c *Console) getSAListFlagSuggestions(word string) []prompt.Suggest {
	suggestions := []prompt.Suggest{
		{Text: "--admin", Description: "只显示 cluster-admin"},
		{Text: "--risky", Description: "只显示有风险的 SA"},
		{Text: "-n", Description: "按命名空间过滤"},
		{Text: "--perms", Description: "显示权限"},
		{Text: "--token", Description: "显示 Token"},
	}
	return prompt.FilterHasPrefix(suggestions, word, true)
}

// getPodsFlagSuggestions 获取 pods 命令的选项补全
func (c *Console) getPodsFlagSuggestions(word string) []prompt.Suggest {
	suggestions := []prompt.Suggest{
		{Text: "--detail", Description: "显示详细信息"},
		{Text: "--privileged", Description: "只显示特权 Pod"},
		{Text: "--running", Description: "只显示 Running 状态"},
		{Text: "-n", Description: "按命名空间过滤"},
		{Text: "--refresh", Description: "强制刷新"},
	}
	return prompt.FilterHasPrefix(suggestions, word, true)
}

// getScanFlagSuggestions 获取 scan 命令的选项补全
func (c *Console) getScanFlagSuggestions(word string) []prompt.Suggest {
	suggestions := []prompt.Suggest{
		{Text: "--risky", Description: "只显示有风险的 SA"},
		{Text: "--perms", Description: "显示权限"},
		{Text: "--token", Description: "显示 Token"},
	}
	return prompt.FilterHasPrefix(suggestions, word, true)
}

// getDiscoverSuggestions 获取 discover 命令的选项补全
func (c *Console) getDiscoverSuggestions(args []string, word string) []prompt.Suggest {
	// 检查上一个参数，决定补全什么
	if len(args) >= 2 {
		lastArg := args[len(args)-1]
		// 如果当前正在输入（word 不为空），检查倒数第二个参数
		if word != "" && len(args) >= 2 {
			lastArg = args[len(args)-2]
		}

		switch lastArg {
		case "-p", "--port":
			// 补全端口
			return c.getPortSuggestions(word)
		case "-c", "--concurrency":
			// 补全并发数
			return c.getConcurrencySuggestions(word)
		case "-t", "--timeout":
			// 补全超时
			return c.getTimeoutSuggestions(word)
		}
	}

	suggestions := []prompt.Suggest{
		{Text: "-p", Description: "端口 (默认: 10250)"},
		{Text: "--port", Description: "端口 (默认: 10250)"},
		{Text: "-c", Description: "并发数 (默认: 100)"},
		{Text: "--concurrency", Description: "并发数 (默认: 100)"},
		{Text: "-t", Description: "超时秒数 (默认: 3)"},
		{Text: "--timeout", Description: "超时秒数 (默认: 3)"},
		{Text: "--all", Description: "显示所有开放端口"},
	}
	return prompt.FilterHasPrefix(suggestions, word, true)
}

// getRunSuggestions 获取 run 命令的补全
func (c *Console) getRunSuggestions(args []string, word string) []prompt.Suggest {
	// 检查上一个参数，决定补全什么
	if len(args) >= 2 {
		lastArg := args[len(args)-1]
		// 如果当前正在输入（word 不为空），检查倒数第二个参数
		if word != "" && len(args) >= 2 {
			lastArg = args[len(args)-2]
		}

		switch lastArg {
		case "-n":
			// 补全命名空间
			return c.getNamespaceSuggestions(word)
		case "-c":
			// 补全容器名
			return c.getContainerSuggestions(args, word)
		case "--filter":
			// 补全要排除的 Pod
			return c.getFilterPodSuggestions(word)
		case "--filter-ns":
			// 补全要排除的命名空间
			return c.getNamespaceSuggestions(word)
		case "--concurrency":
			// 补全并发数
			return c.getConcurrencySuggestions(word)
		case "--cmd":
			// 命令参数，不补全
			return nil
		}
	}

	var suggestions []prompt.Suggest

	// 补全选项
	suggestions = append(suggestions,
		prompt.Suggest{Text: "--cmd", Description: "要执行的命令（必需）"},
		prompt.Suggest{Text: "-n", Description: "指定命名空间"},
		prompt.Suggest{Text: "-c", Description: "指定容器"},
		prompt.Suggest{Text: "--all-pods", Description: "在所有 Pod 中执行"},
		prompt.Suggest{Text: "--filter", Description: "排除指定 Pod（逗号分隔）"},
		prompt.Suggest{Text: "--filter-ns", Description: "排除指定命名空间（逗号分隔）"},
		prompt.Suggest{Text: "--concurrency", Description: "并发数（默认: 10）"},
	)

	// 补全 Pod 名称
	pods := c.session.GetCachedPods()
	for _, pod := range pods {
		if pod.Status == "Running" {
			suggestions = append(suggestions, prompt.Suggest{
				Text:        pod.PodName,
				Description: pod.Namespace,
			})
		}
	}

	return prompt.FilterHasPrefix(suggestions, word, true)
}

// getPortSuggestions 获取端口补全
func (c *Console) getPortSuggestions(word string) []prompt.Suggest {
	suggestions := []prompt.Suggest{
		{Text: "10250", Description: "Kubelet API (默认)"},
		{Text: "10255", Description: "Kubelet 只读端口"},
		{Text: "10250,10255", Description: "常用 Kubelet 端口"},
	}
	return prompt.FilterHasPrefix(suggestions, word, true)
}

// getConcurrencySuggestions 获取并发数补全
func (c *Console) getConcurrencySuggestions(word string) []prompt.Suggest {
	suggestions := []prompt.Suggest{
		{Text: "50", Description: "低并发"},
		{Text: "100", Description: "默认"},
		{Text: "200", Description: "高并发"},
	}
	return prompt.FilterHasPrefix(suggestions, word, true)
}

// getTimeoutSuggestions 获取超时补全
func (c *Console) getTimeoutSuggestions(word string) []prompt.Suggest {
	suggestions := []prompt.Suggest{
		{Text: "1", Description: "1 秒"},
		{Text: "3", Description: "3 秒 (默认)"},
		{Text: "5", Description: "5 秒"},
	}
	return prompt.FilterHasPrefix(suggestions, word, true)
}

// getPid2PodSuggestions 获取 pid2pod 命令的补全
func (c *Console) getPid2PodSuggestions(word string) []prompt.Suggest {
	suggestions := []prompt.Suggest{
		{Text: "--pid", Description: "只查看指定 PID"},
		{Text: "--all", Description: "显示所有进程（包括非容器进程）"},
	}
	return prompt.FilterHasPrefix(suggestions, word, true)
}

// getGoldenSuggestions 获取 golden 命令的补全
func (c *Console) getGoldenSuggestions(args []string, word string) []prompt.Suggest {
	// 子命令补全
	if len(args) == 1 || (len(args) == 2 && word != "" && !strings.HasPrefix(word, "-")) {
		suggestions := []prompt.Suggest{
			{Text: "user-cert", Description: "伪造用户证书"},
			{Text: "node-cert", Description: "伪造节点证书"},
			{Text: "sa-token", Description: "伪造 SA Token"},
			{Text: "update-uid", Description: "更新 UID 缓存到内存"},
			{Text: "uid-list", Description: "列出 UID 缓存"},
			{Text: "test", Description: "测试密钥文件"},
		}
		return prompt.FilterHasPrefix(suggestions, word, true)
	}

	// 根据子命令提供不同的参数补全
	if len(args) >= 2 {
		subCmd := args[1]
		switch subCmd {
		case "user-cert":
			return c.getGoldenUserCertSuggestions(word)
		case "node-cert":
			return c.getGoldenNodeCertSuggestions(word)
		case "sa-token":
			return c.getGoldenSATokenSuggestions(word)
		case "update-uid":
			return c.getGoldenUpdateUIDSuggestions(word)
		case "uid-list", "uidlist", "uids":
			return c.getGoldenUIDListSuggestions(word)
		case "test":
			return c.getGoldenTestSuggestions(word)
		}
	}

	// 通用参数补全
	suggestions := []prompt.Suggest{
		{Text: "--ca-cert", Description: "CA 证书路径"},
		{Text: "--ca-key", Description: "CA 私钥路径"},
		{Text: "--sa-key", Description: "SA 私钥路径"},
		{Text: "--server", Description: "API Server URL"},
		{Text: "--output", Description: "输出目录"},
		{Text: "--force", Description: "覆盖已存在文件"},
	}
	return prompt.FilterHasPrefix(suggestions, word, true)
}

// getGoldenUserCertSuggestions 获取 golden user-cert 命令的补全
func (c *Console) getGoldenUserCertSuggestions(word string) []prompt.Suggest {
	suggestions := []prompt.Suggest{
		{Text: "--ca-cert", Description: "CA 证书路径（必需）"},
		{Text: "--ca-key", Description: "CA 私钥路径（必需）"},
		{Text: "--role", Description: "集群角色（默认: cluster-admins）"},
		{Text: "--user", Description: "用户名（默认: golden-admin）"},
		{Text: "--days", Description: "证书有效天数（默认: 365）"},
		{Text: "--server", Description: "API Server URL"},
		{Text: "--output", Description: "输出目录"},
		{Text: "--force", Description: "覆盖已存在文件"},
		{Text: "--no-kubeconfig", Description: "不生成 kubeconfig"},
	}
	return prompt.FilterHasPrefix(suggestions, word, true)
}

// getGoldenNodeCertSuggestions 获取 golden node-cert 命令的补全
func (c *Console) getGoldenNodeCertSuggestions(word string) []prompt.Suggest {
	suggestions := []prompt.Suggest{
		{Text: "--ca-cert", Description: "CA 证书路径（必需）"},
		{Text: "--ca-key", Description: "CA 私钥路径（必需）"},
		{Text: "--node", Description: "节点名称（必需）"},
		{Text: "--days", Description: "证书有效天数（默认: 365）"},
		{Text: "--server", Description: "API Server URL"},
		{Text: "--output", Description: "输出目录"},
		{Text: "--force", Description: "覆盖已存在文件"},
		{Text: "--no-kubeconfig", Description: "不生成 kubeconfig"},
	}
	return prompt.FilterHasPrefix(suggestions, word, true)
}

// getGoldenSATokenSuggestions 获取 golden sa-token 命令的补全
func (c *Console) getGoldenSATokenSuggestions(word string) []prompt.Suggest {
	suggestions := []prompt.Suggest{
		{Text: "--sa-key", Description: "SA 私钥路径（必需）"},
		{Text: "--namespace", Description: "命名空间（必需）"},
		{Text: "--name", Description: "SA 名称（必需）"},
		{Text: "--uid", Description: "SA UID"},
		{Text: "--uid-cache", Description: "UID 缓存文件"},
		{Text: "--ttl", Description: "Token 有效期（默认: 8760h）"},
		{Text: "--audience", Description: "Token 受众 URL"},
		{Text: "--server", Description: "API Server URL"},
		{Text: "--ca-cert", Description: "CA 证书路径（用于 kubeconfig）"},
		{Text: "--output", Description: "输出目录"},
		{Text: "--force", Description: "覆盖已存在文件"},
		{Text: "--no-kubeconfig", Description: "不生成 kubeconfig"},
	}
	return prompt.FilterHasPrefix(suggestions, word, true)
}

// getGoldenUpdateUIDSuggestions 获取 golden update-uid 命令的补全
func (c *Console) getGoldenUpdateUIDSuggestions(word string) []prompt.Suggest {
	suggestions := []prompt.Suggest{
		{Text: "--ca-cert", Description: "CA 证书路径"},
		{Text: "--ca-key", Description: "CA 私钥路径"},
		{Text: "--token", Description: "认证 Token"},
		{Text: "--server", Description: "API Server URL"},
		{Text: "--output", Description: "同时保存到文件"},
		{Text: "--force", Description: "覆盖已存在文件"},
	}
	return prompt.FilterHasPrefix(suggestions, word, true)
}

// getGoldenUIDListSuggestions 获取 golden uid-list 命令的补全
func (c *Console) getGoldenUIDListSuggestions(word string) []prompt.Suggest {
	suggestions := []prompt.Suggest{
		{Text: "--file", Description: "从文件加载 UID 缓存"},
		{Text: "--namespace", Description: "按命名空间过滤"},
		{Text: "--name", Description: "按 SA 名称过滤"},
		{Text: "--output", Description: "保存到文件"},
		{Text: "--force", Description: "覆盖已存在文件"},
	}
	return prompt.FilterHasPrefix(suggestions, word, true)
}

// getGoldenTestSuggestions 获取 golden test 命令的补全
func (c *Console) getGoldenTestSuggestions(word string) []prompt.Suggest {
	suggestions := []prompt.Suggest{
		{Text: "--ca-cert", Description: "CA 证书路径"},
		{Text: "--ca-key", Description: "CA 私钥路径"},
		{Text: "--sa-key", Description: "SA 私钥路径"},
		{Text: "--uid-cache", Description: "UID 缓存文件"},
	}
	return prompt.FilterHasPrefix(suggestions, word, true)
}

// getPortForwardSuggestions 获取 portforward 命令的补全
func (c *Console) getPortForwardSuggestions(args []string, word string) []prompt.Suggest {
	// 检查上一个参数
	if len(args) >= 2 {
		lastArg := args[len(args)-1]
		if word != "" && len(args) >= 2 {
			lastArg = args[len(args)-2]
		}

		switch lastArg {
		case "-n":
			return c.getNamespaceSuggestions(word)
		case "--address":
			return []prompt.Suggest{
				{Text: "127.0.0.1", Description: "仅本地访问（默认）"},
				{Text: "0.0.0.0", Description: "所有接口"},
			}
		}
	}

	var suggestions []prompt.Suggest

	// 补全选项
	suggestions = append(suggestions,
		prompt.Suggest{Text: "-n", Description: "指定命名空间"},
		prompt.Suggest{Text: "--address", Description: "监听地址"},
	)

	// 补全 Pod 名称
	pods := c.session.GetCachedPods()
	for _, pod := range pods {
		if pod.Status == "Running" {
			suggestions = append(suggestions, prompt.Suggest{
				Text:        pod.PodName,
				Description: pod.Namespace,
			})
		}
	}

	return prompt.FilterHasPrefix(suggestions, word, true)
}

// getPersistSuggestions 获取 persist 命令的补全
func (c *Console) getPersistSuggestions(args []string, word string) []prompt.Suggest {
	// 子命令补全
	if len(args) == 1 || (len(args) == 2 && word != "" && !strings.HasPrefix(word, "-")) {
		suggestions := []prompt.Suggest{
			{Text: "probe-inject", Description: "通过探针注入实现持久化"},
			{Text: "probe-list", Description: "列出所有带有 exec 探针的工作负载"},
			{Text: "probe-restore", Description: "移除注入的探针"},
			{Text: "webhook-inject", Description: "通过恶意 Admission Webhook 实现持久化"},
			{Text: "webhook-list", Description: "列出所有 Admission Webhook 配置"},
			{Text: "webhook-restore", Description: "移除注入的恶意 Webhook"},
		}
		return prompt.FilterHasPrefix(suggestions, word, true)
	}

	// 根据子命令提供不同的参数补全
	if len(args) >= 2 {
		subCmd := args[1]
		switch subCmd {
		case "probe-inject", "pi", "inject":
			return c.getPersistProbeInjectSuggestions(word)
		case "probe-list", "pl", "list":
			return c.getPersistProbeListSuggestions(word)
		case "probe-restore", "pr", "restore":
			return c.getPersistProbeRestoreSuggestions(word)
		case "webhook-inject", "wi", "webhook":
			return c.getPersistWebhookInjectSuggestions(word)
		case "webhook-list", "wl", "webhooks":
			return c.getPersistWebhookListSuggestions(word)
		case "webhook-restore", "wr", "webhook-remove":
			return c.getPersistWebhookRestoreSuggestions(word)
		}
	}

	return nil
}

// getPersistProbeInjectSuggestions 获取 persist probe-inject 命令的补全
func (c *Console) getPersistProbeInjectSuggestions(word string) []prompt.Suggest {
	suggestions := []prompt.Suggest{
		{Text: "--namespace", Description: "目标命名空间"},
		{Text: "-n", Description: "目标命名空间"},
		{Text: "--type", Description: "工作负载类型 (daemonset/deployment)"},
		{Text: "-t", Description: "工作负载类型 (daemonset/deployment)"},
		{Text: "--server", Description: "API Server URL"},
		{Text: "--token", Description: "认证 Token"},
		{Text: "--dry-run", Description: "仅预览，不实际执行"},
	}
	return prompt.FilterHasPrefix(suggestions, word, true)
}

// getPersistProbeListSuggestions 获取 persist probe-list 命令的补全
func (c *Console) getPersistProbeListSuggestions(word string) []prompt.Suggest {
	suggestions := []prompt.Suggest{
		{Text: "--namespace", Description: "按命名空间过滤"},
		{Text: "-n", Description: "按命名空间过滤"},
		{Text: "--server", Description: "API Server URL"},
		{Text: "--token", Description: "认证 Token"},
	}
	return prompt.FilterHasPrefix(suggestions, word, true)
}

// getPersistProbeRestoreSuggestions 获取 persist probe-restore 命令的补全
func (c *Console) getPersistProbeRestoreSuggestions(word string) []prompt.Suggest {
	suggestions := []prompt.Suggest{
		{Text: "--namespace", Description: "目标命名空间"},
		{Text: "-n", Description: "目标命名空间"},
		{Text: "--name", Description: "工作负载名称"},
		{Text: "--type", Description: "工作负载类型 (daemonset/deployment)"},
		{Text: "-t", Description: "工作负载类型 (daemonset/deployment)"},
		{Text: "--probe", Description: "探针类型 (liveness/readiness/startup)"},
		{Text: "-p", Description: "探针类型 (liveness/readiness/startup)"},
		{Text: "--container", Description: "容器名称"},
		{Text: "-c", Description: "容器名称"},
		{Text: "--server", Description: "API Server URL"},
		{Text: "--token", Description: "认证 Token"},
		{Text: "--dry-run", Description: "仅预览，不实际执行"},
	}
	return prompt.FilterHasPrefix(suggestions, word, true)
}

// getPersistWebhookInjectSuggestions 获取 persist webhook-inject 命令的补全
func (c *Console) getPersistWebhookInjectSuggestions(word string) []prompt.Suggest {
	suggestions := []prompt.Suggest{
		{Text: "--type", Description: "攻击类型 (secret-exfil/pod-backdoor)"},
		{Text: "-t", Description: "攻击类型 (secret-exfil/pod-backdoor)"},
		{Text: "--namespace", Description: "Webhook 服务部署命名空间"},
		{Text: "-n", Description: "Webhook 服务部署命名空间"},
		{Text: "--exfil-url", Description: "外泄目标 URL (secret-exfil 类型)"},
		{Text: "--image", Description: "后门容器镜像 (pod-backdoor 类型)"},
		{Text: "--command", Description: "后门容器命令，逗号分隔"},
		{Text: "--target-ns", Description: "目标命名空间，逗号分隔"},
		{Text: "--webhook-image", Description: "Webhook 服务镜像 (默认: ghcr.io/kctl/webhook)"},
		{Text: "--external-url", Description: "使用外部 URL 作为 Webhook 端点"},
		{Text: "--server", Description: "API Server URL"},
		{Text: "--token", Description: "认证 Token"},
		{Text: "--dry-run", Description: "仅预览，不实际执行"},
	}
	return prompt.FilterHasPrefix(suggestions, word, true)
}

// getPersistWebhookListSuggestions 获取 persist webhook-list 命令的补全
func (c *Console) getPersistWebhookListSuggestions(word string) []prompt.Suggest {
	suggestions := []prompt.Suggest{
		{Text: "--all", Description: "显示所有 Webhook"},
		{Text: "-a", Description: "显示所有 Webhook"},
		{Text: "--type", Description: "过滤类型 (mutating/validating/all)"},
		{Text: "-t", Description: "过滤类型 (mutating/validating/all)"},
		{Text: "--server", Description: "API Server URL"},
		{Text: "--token", Description: "认证 Token"},
	}
	return prompt.FilterHasPrefix(suggestions, word, true)
}

// getPersistWebhookRestoreSuggestions 获取 persist webhook-restore 命令的补全
func (c *Console) getPersistWebhookRestoreSuggestions(word string) []prompt.Suggest {
	suggestions := []prompt.Suggest{
		{Text: "--name", Description: "指定要移除的 Webhook 名称"},
		{Text: "--all", Description: "移除所有 kctl 管理的 Webhook"},
		{Text: "--server", Description: "API Server URL"},
		{Text: "--token", Description: "认证 Token"},
	}
	return prompt.FilterHasPrefix(suggestions, word, true)
}

// getPrompt 获取提示符
func (c *Console) getPrompt() string {
	return fmt.Sprintf("kctl [%s]> ", c.session.GetPromptDisplay())
}

// getLivePrefix 动态获取提示符
// 注意：go-prompt 不支持在提示符中使用 ANSI 颜色代码，所以这里不着色
func (c *Console) getLivePrefix() (string, bool) {
	return fmt.Sprintf("kctl [%s]> ", c.session.GetPromptDisplay()), true
}

// autoConnect 自动连接到 Kubelet
func (c *Console) autoConnect() {
	p := c.session.Printer
	ctx := context.Background()

	// 检查是否有足够的配置信息
	if c.session.Config.KubeletIP == "" {
		p.Warning("未检测到 Kubelet IP，请使用 'set target <ip>' 设置后执行 'connect'")
		return
	}

	if c.session.Config.Token == "" {
		p.Warning("未检测到 Token，请使用 'set token <token>' 设置后执行 'connect'")
		return
	}

	p.Printf("%s Auto-connecting to Kubelet %s:%d...\n",
		p.Colored(config.ColorBlue, "[*]"),
		c.session.Config.KubeletIP,
		c.session.Config.KubeletPort)

	// 连接
	if err := c.session.Connect(); err != nil {
		p.Error(fmt.Sprintf("自动连接失败: %v", err))
		p.Info("请检查配置后手动执行 'connect'")
		return
	}

	// 验证连接
	kubelet, err := c.session.GetKubeletClient()
	if err != nil {
		p.Error(fmt.Sprintf("获取客户端失败: %v", err))
		return
	}

	// 尝试验证 Kubelet 端口
	result, err := kubelet.ValidatePort(ctx)
	if err != nil {
		p.Warning(fmt.Sprintf("连接成功，但无法验证 Kubelet 端口: %v", err))
	} else if result.IsKubelet {
		p.Success("Connected successfully")
	} else {
		p.Warning("连接成功，但目标可能不是 Kubelet")
	}

	// 解析当前 Token 并设置为当前 SA
	if err := c.session.SetupCurrentSA(); err != nil {
		p.Warning(fmt.Sprintf("设置 SA 失败: %v", err))
	}

	fmt.Println() // 空行分隔
}

// GetSession 获取会话
func (c *Console) GetSession() *session.Session {
	return c.session
}

// Close 关闭控制台
func (c *Console) Close() {
	if c.session != nil {
		_ = c.session.Close()
	}
}

// SetExitFlag 设置退出标志
func (c *Console) SetExitFlag() {
	c.exitFlag = true
	c.session.Printer.Info("Clearing memory...")
	c.session.Printer.Info("Goodbye!")
	c.Close()
	resetTerminal()
	os.Exit(0)
}

// resetTerminal 重置终端设置
func resetTerminal() {
	// 使用 stty sane 恢复终端到正常状态
	cmd := exec.Command("stty", "sane")
	cmd.Stdin = os.Stdin
	_ = cmd.Run()
}

// RegisterCommands 注册所有命令
func RegisterCommands() {
	// 在这里注册所有命令
	// 命令会在各自的 init() 函数中自动注册
	_ = commands.All() // 触发 init
}
