package golden

import (
	"context"
	"fmt"
	"os"
	"strings"

	"kctl/config"
	"kctl/internal/golden"
	"kctl/internal/session"
	"kctl/pkg/types"
)

// UpdateUIDCmd update-uid 子命令
type UpdateUIDCmd struct{}

func init() {
	Register(&UpdateUIDCmd{})
}

func (c *UpdateUIDCmd) Name() string        { return "update-uid" }
func (c *UpdateUIDCmd) Aliases() []string   { return []string{"uid", "updateuid"} }
func (c *UpdateUIDCmd) Description() string { return "更新 UID 缓存" }

func (c *UpdateUIDCmd) Usage() string {
	return `golden update-uid [选项]

从 API Server 获取所有 ServiceAccount 的 UID 并缓存到内存

必需选项 (二选一):
  方式一: 使用 CA 密钥生成临时管理员证书
    --ca-cert, -c   CA 证书路径
    --ca-key, -k    CA 私钥路径
  
  方式二: 使用现有 Token
    --token, -t     认证 Token (支持直接传入 token 或文件路径)

可选选项:
  --server          API Server URL (默认: session 中的 api-server)
  --output, -o      同时保存到文件 (默认: 只保存到内存)
  --force           覆盖已存在的文件

示例:
  golden update-uid -c ./ca.crt -k ./ca.key                    # 保存到内存
  golden update-uid -c ./ca.crt -k ./ca.key -o ./uid_cache.json # 同时保存到文件
  golden update-uid --token "eyJhbG..." --server https://10.0.0.1:6443
  golden update-uid --token /path/to/token.txt`
}

func (c *UpdateUIDCmd) Execute(sess *session.Session, args []string) error {
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

	// 如果指定了输出文件，检查是否已存在
	if opts.OutputPath != "" {
		if err := golden.CheckFileOverwrite(opts.OutputPath, opts.Force); err != nil {
			return err
		}
	}

	// 显示使用的 API Server
	p.Printf("%s Using API Server: %s\n", p.Colored(config.ColorBlue, "[*]"), opts.ServerURL)

	var cache *types.UIDCache

	// 根据认证方式获取 SA 列表
	if opts.Token != "" {
		// 使用 Token 认证
		p.Printf("%s Requesting ServiceAccount list with token...\n", p.Colored(config.ColorBlue, "[*]"))
		cache, err = golden.FetchAllServiceAccounts(ctx, opts.ServerURL, opts.Token, sess.GetClientConfig())
	} else {
		// 使用证书认证
		p.Printf("%s Creating temporary admin certificate...\n", p.Colored(config.ColorBlue, "[*]"))
		p.Printf("%s Requesting ServiceAccount list...\n", p.Colored(config.ColorBlue, "[*]"))
		cache, err = golden.FetchServiceAccountsWithCert(ctx, opts.ServerURL, opts, sess.GetClientConfig())
	}

	if err != nil {
		return fmt.Errorf("获取 ServiceAccount 列表失败: %w", err)
	}

	// 保存到内存
	sess.UIDCache = cache

	// 如果指定了输出文件，同时保存到文件
	if opts.OutputPath != "" {
		if err := golden.SaveUIDCache(opts.OutputPath, cache); err != nil {
			return fmt.Errorf("保存 UID 缓存到文件失败: %w", err)
		}
	}

	// 输出结果
	c.printResult(p, cache, opts)

	return nil
}

func (c *UpdateUIDCmd) parseArgs(sess *session.Session, args []string) (*types.UpdateUIDOptions, error) {
	opts := &types.UpdateUIDOptions{
		ServerURL:  BuildAPIServerURL(sess.Config.APIServer, sess.Config.APIServerPort),
		OutputPath: "", // 默认不保存到文件，只保存到内存
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

		case "--token", "-t":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--token 需要参数")
			}
			i++
			tokenArg := args[i]
			// 检查是否是文件路径
			if _, err := os.Stat(tokenArg); err == nil {
				// 是文件，读取内容
				data, err := os.ReadFile(tokenArg)
				if err != nil {
					return nil, fmt.Errorf("读取 token 文件失败: %w", err)
				}
				content := strings.TrimSpace(string(data))
				// 检查是否是私钥文件（不是 token）
				if strings.Contains(content, "-----BEGIN") {
					return nil, fmt.Errorf("文件 %s 是私钥文件，不是 JWT token。\n"+
						"  如果要使用 CA 证书认证，请使用: --ca-cert 和 --ca-key\n"+
						"  如果要使用 token 认证，请提供 JWT token 文件或字符串", tokenArg)
				}
				opts.Token = content
			} else {
				// 不是文件，直接使用
				opts.Token = tokenArg
			}

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
			opts.OutputPath = args[i]

		case "--force":
			opts.Force = true

		default:
			if strings.HasPrefix(arg, "-") {
				return nil, fmt.Errorf("未知选项: %s", arg)
			}
		}
	}

	// 验证必需参数
	if opts.ServerURL == "" {
		return nil, fmt.Errorf("未指定 API Server URL (--server 或使用 'set api-server')")
	}

	// 验证认证方式
	hasToken := opts.Token != ""
	hasCert := opts.CACertPath != "" && opts.CAKeyPath != ""

	if !hasToken && !hasCert {
		return nil, fmt.Errorf("需要指定认证方式: --token 或 (--ca-cert 和 --ca-key)")
	}

	// 如果使用证书认证，验证文件
	if hasCert {
		if err := golden.ValidateCACertFile(opts.CACertPath); err != nil {
			return nil, err
		}
		if err := golden.ValidateCAKeyFile(opts.CAKeyPath); err != nil {
			return nil, err
		}
	}

	return opts, nil
}

func (c *UpdateUIDCmd) printResult(p interface {
	Printf(format string, a ...interface{})
	Println(a ...interface{})
	Colored(colorName config.ColorName, text string) string
}, cache *types.UIDCache, opts *types.UpdateUIDOptions) {
	p.Println()
	p.Printf("%s Received %d ServiceAccounts\n", p.Colored(config.ColorGreen, "[+]"), len(cache.Entries))
	p.Printf("%s UID cache saved to memory\n", p.Colored(config.ColorGreen, "[+]"))
	if opts.OutputPath != "" {
		p.Printf("%s UID cache also saved to: %s\n", p.Colored(config.ColorGreen, "[+]"), opts.OutputPath)
	}
	p.Println()

	// 显示一些示例
	if len(cache.Entries) > 0 {
		p.Printf("%s Sample entries:\n", p.Colored(config.ColorBlue, "[*]"))
		count := 0
		for key, uid := range cache.Entries {
			if count >= 5 {
				p.Printf("  ... and %d more\n", len(cache.Entries)-5)
				break
			}
			p.Printf("  %s -> %s\n", key, uid)
			count++
		}
	}

	p.Println()
	p.Printf("%s Use 'golden uid-list' to view all entries\n", p.Colored(config.ColorCyan, "[*]"))
	p.Printf("%s Use 'golden sa-token --namespace <ns> --name <name>' to forge token (UID auto-lookup from memory)\n",
		p.Colored(config.ColorCyan, "[*]"))
}
