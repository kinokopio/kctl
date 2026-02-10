package golden

import (
	"fmt"
	"sort"
	"strings"

	"kctl/config"
	"kctl/internal/golden"
	"kctl/internal/session"
	"kctl/pkg/types"
)

// UIDListCmd uid-list 子命令
type UIDListCmd struct{}

func init() {
	Register(&UIDListCmd{})
}

func (c *UIDListCmd) Name() string        { return "uid-list" }
func (c *UIDListCmd) Aliases() []string   { return []string{"uidlist", "uids"} }
func (c *UIDListCmd) Description() string { return "列出 UID 缓存" }

func (c *UIDListCmd) Usage() string {
	return `golden uid-list [选项]

列出内存或文件中的 ServiceAccount UID 缓存

选项:
  --file, -f        从文件加载 UID 缓存 (默认: 使用内存缓存)
  --namespace, -n   按命名空间过滤
  --name            按 SA 名称过滤 (支持前缀匹配)
  --output, -o      保存到文件
  --force           覆盖已存在的文件

示例:
  golden uid-list                           # 列出内存中的 UID 缓存
  golden uid-list -f ./uid_cache.json       # 从文件加载并列出
  golden uid-list -n kube-system            # 只显示 kube-system 命名空间
  golden uid-list --name default            # 只显示名称包含 default 的 SA
  golden uid-list -o ./filtered.json        # 保存到文件`
}

type uidListOptions struct {
	FilePath   string
	Namespace  string
	NameFilter string
	OutputPath string
	Force      bool
}

func (c *UIDListCmd) Execute(sess *session.Session, args []string) error {
	p := sess.Printer

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

	// 获取 UID 缓存
	var cache *types.UIDCache
	if opts.FilePath != "" {
		// 从文件加载
		p.Printf("%s Loading UID cache from: %s\n", p.Colored(config.ColorBlue, "[*]"), opts.FilePath)
		loadedCache, err := golden.LoadUIDCache(opts.FilePath)
		if err != nil {
			return fmt.Errorf("加载 UID 缓存失败: %w", err)
		}
		cache = loadedCache
	} else {
		// 使用内存缓存
		if sess.UIDCache == nil || len(sess.UIDCache.Entries) == 0 {
			return fmt.Errorf("内存中没有 UID 缓存，请先执行 'golden update-uid' 或使用 --file 指定缓存文件")
		}
		cache = sess.UIDCache
		p.Printf("%s Using in-memory UID cache (%d entries)\n", p.Colored(config.ColorBlue, "[*]"), len(sess.UIDCache.Entries))
	}

	// 过滤
	filtered := c.filterEntries(cache.Entries, opts)

	// 如果需要保存到文件
	if opts.OutputPath != "" {
		if err := golden.CheckFileOverwrite(opts.OutputPath, opts.Force); err != nil {
			return err
		}
		filteredCache := *cache
		filteredCache.Entries = filtered
		if err := golden.SaveUIDCache(opts.OutputPath, &filteredCache); err != nil {
			return fmt.Errorf("保存 UID 缓存失败: %w", err)
		}
		p.Printf("%s Saved %d entries to: %s\n", p.Colored(config.ColorGreen, "[+]"), len(filtered), opts.OutputPath)
	}

	// 显示结果
	c.printResult(p, filtered, cache.ServerURL, opts)

	return nil
}

func (c *UIDListCmd) parseArgs(args []string) (*uidListOptions, error) {
	opts := &uidListOptions{}

	for i := 0; i < len(args); i++ {
		arg := args[i]

		switch arg {
		case "--file", "-f":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--file 需要参数")
			}
			i++
			opts.FilePath = args[i]

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
			opts.NameFilter = args[i]

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

	return opts, nil
}

func (c *UIDListCmd) filterEntries(entries map[string]string, opts *uidListOptions) map[string]string {
	if opts.Namespace == "" && opts.NameFilter == "" {
		return entries
	}

	filtered := make(map[string]string)
	for key, uid := range entries {
		parts := strings.SplitN(key, "/", 2)
		if len(parts) != 2 {
			continue
		}
		ns, name := parts[0], parts[1]

		// 命名空间过滤
		if opts.Namespace != "" && ns != opts.Namespace {
			continue
		}

		// 名称过滤（前缀匹配）
		if opts.NameFilter != "" && !strings.Contains(name, opts.NameFilter) {
			continue
		}

		filtered[key] = uid
	}

	return filtered
}

func (c *UIDListCmd) printResult(p interface {
	Printf(format string, a ...interface{})
	Println(a ...interface{})
	Colored(colorName config.ColorName, text string) string
}, entries map[string]string, serverURL string, opts *uidListOptions) {
	p.Println()

	if len(entries) == 0 {
		p.Printf("%s No entries found\n", p.Colored(config.ColorYellow, "[!]"))
		return
	}

	// 按 namespace/name 排序
	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 显示过滤条件
	if opts.Namespace != "" || opts.NameFilter != "" {
		p.Printf("%s Filter: ", p.Colored(config.ColorBlue, "[*]"))
		if opts.Namespace != "" {
			p.Printf("namespace=%s ", opts.Namespace)
		}
		if opts.NameFilter != "" {
			p.Printf("name contains '%s'", opts.NameFilter)
		}
		p.Println()
	}

	p.Printf("%s Found %d ServiceAccount(s):\n", p.Colored(config.ColorGreen, "[+]"), len(entries))
	p.Println()

	// 按命名空间分组显示
	currentNS := ""
	for _, key := range keys {
		parts := strings.SplitN(key, "/", 2)
		if len(parts) != 2 {
			continue
		}
		ns, name := parts[0], parts[1]
		uid := entries[key]

		if ns != currentNS {
			if currentNS != "" {
				p.Println()
			}
			p.Printf("  %s\n", p.Colored(config.ColorCyan, "["+ns+"]"))
			currentNS = ns
		}

		p.Printf("    %-40s %s\n", name, uid)
	}

	p.Println()
	if serverURL != "" {
		p.Printf("%s Server: %s\n", p.Colored(config.ColorBlue, "[*]"), serverURL)
	}
	p.Printf("%s Use with: golden sa-token --uid <uid> --namespace <ns> --name <name>\n",
		p.Colored(config.ColorCyan, "[*]"))
}
