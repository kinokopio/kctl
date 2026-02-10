package golden

import (
	"fmt"
	"strings"

	"kctl/config"
	"kctl/internal/golden"
	"kctl/internal/session"
	"kctl/pkg/types"
)

// TestCmd test 子命令
type TestCmd struct{}

func init() {
	Register(&TestCmd{})
}

func (c *TestCmd) Name() string        { return "test" }
func (c *TestCmd) Aliases() []string   { return []string{"check", "verify"} }
func (c *TestCmd) Description() string { return "测试密钥文件" }

func (c *TestCmd) Usage() string {
	return `golden test [选项]

测试密钥文件是否完整有效

选项:
  --ca-cert, -c     CA 证书路径
  --ca-key, -k      CA 私钥路径
  --sa-key, -s      SA 私钥路径
  --uid-cache       UID 缓存文件路径

示例:
  golden test -c ./ca.crt -k ./ca.key -s ./sa.key
  golden test -c ./ca.crt -k ./ca.key -s ./sa.key --uid-cache ./uid_cache.json`
}

func (c *TestCmd) Execute(sess *session.Session, args []string) error {
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

	// 如果没有指定任何文件，显示帮助
	if opts.CACertPath == "" && opts.CAKeyPath == "" && opts.SAKeyPath == "" && opts.UIDCachePath == "" {
		p.Println(c.Usage())
		return nil
	}

	p.Println()
	p.Printf("%s Testing key files...\n", p.Colored(config.ColorBlue, "[*]"))
	p.Println()

	var results []types.TestResult
	allValid := true

	// 测试 CA 证书
	if opts.CACertPath != "" {
		result := c.testCACert(opts.CACertPath)
		results = append(results, result)
		if !result.Valid {
			allValid = false
		}
	}

	// 测试 CA 私钥
	if opts.CAKeyPath != "" {
		result := c.testCAKey(opts.CAKeyPath)
		results = append(results, result)
		if !result.Valid {
			allValid = false
		}
	}

	// 测试 CA 证书和私钥是否匹配
	if opts.CACertPath != "" && opts.CAKeyPath != "" {
		result := c.testCertKeyPair(opts.CACertPath, opts.CAKeyPath)
		results = append(results, result)
		if !result.Valid {
			allValid = false
		}
	}

	// 测试 SA 私钥
	if opts.SAKeyPath != "" {
		result := c.testSAKey(opts.SAKeyPath)
		results = append(results, result)
		if !result.Valid {
			allValid = false
		}
	}

	// 测试 UID 缓存
	if opts.UIDCachePath != "" {
		result := c.testUIDCache(opts.UIDCachePath)
		results = append(results, result)
		if !result.Valid {
			allValid = false
		}
	}

	// 输出结果
	c.printResults(p, results)

	// 总结
	p.Println()
	if allValid {
		p.Printf("%s All tests passed!\n", p.Colored(config.ColorGreen, "[+]"))
	} else {
		p.Printf("%s Some tests failed. See above for details.\n", p.Colored(config.ColorRed, "[-]"))
	}

	return nil
}

func (c *TestCmd) parseArgs(args []string) (*types.TestOptions, error) {
	opts := &types.TestOptions{}

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

		case "--sa-key", "-s":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--sa-key 需要参数")
			}
			i++
			opts.SAKeyPath = args[i]

		case "--uid-cache":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--uid-cache 需要参数")
			}
			i++
			opts.UIDCachePath = args[i]

		default:
			if strings.HasPrefix(arg, "-") {
				return nil, fmt.Errorf("未知选项: %s", arg)
			}
		}
	}

	return opts, nil
}

func (c *TestCmd) testCACert(path string) types.TestResult {
	result := types.TestResult{
		Name: "CA Certificate",
		Path: path,
	}

	if !golden.FileExists(path) {
		result.Valid = false
		result.Message = "文件不存在"
		return result
	}

	cert, _, err := golden.LoadCACert(path)
	if err != nil {
		result.Valid = false
		result.Message = err.Error()
		return result
	}

	result.Valid = true
	result.Message = fmt.Sprintf("有效 (Subject: %s)", cert.Subject.CommonName)
	return result
}

func (c *TestCmd) testCAKey(path string) types.TestResult {
	result := types.TestResult{
		Name: "CA Private Key",
		Path: path,
	}

	if !golden.FileExists(path) {
		result.Valid = false
		result.Message = "文件不存在"
		return result
	}

	key, err := golden.LoadRSAPrivateKey(path)
	if err != nil {
		result.Valid = false
		result.Message = err.Error()
		return result
	}

	result.Valid = true
	result.Message = fmt.Sprintf("有效 (RSA %d-bit)", key.N.BitLen())
	return result
}

func (c *TestCmd) testCertKeyPair(certPath, keyPath string) types.TestResult {
	result := types.TestResult{
		Name: "Cert-Key Pair",
		Path: fmt.Sprintf("%s + %s", certPath, keyPath),
	}

	cert, _, err := golden.LoadCACert(certPath)
	if err != nil {
		result.Valid = false
		result.Message = "无法加载证书"
		return result
	}

	key, err := golden.LoadRSAPrivateKey(keyPath)
	if err != nil {
		result.Valid = false
		result.Message = "无法加载私钥"
		return result
	}

	if err := golden.ValidateRSAKeyPair(cert, key); err != nil {
		result.Valid = false
		result.Message = "证书和私钥不匹配"
		return result
	}

	result.Valid = true
	result.Message = "匹配"
	return result
}

func (c *TestCmd) testSAKey(path string) types.TestResult {
	result := types.TestResult{
		Name: "SA Private Key",
		Path: path,
	}

	if !golden.FileExists(path) {
		result.Valid = false
		result.Message = "文件不存在"
		return result
	}

	key, err := golden.LoadRSAPrivateKey(path)
	if err != nil {
		result.Valid = false
		result.Message = err.Error()
		return result
	}

	result.Valid = true
	result.Message = fmt.Sprintf("有效 (RSA %d-bit)", key.N.BitLen())
	return result
}

func (c *TestCmd) testUIDCache(path string) types.TestResult {
	result := types.TestResult{
		Name: "UID Cache",
		Path: path,
	}

	if !golden.FileExists(path) {
		result.Valid = false
		result.Message = "文件不存在"
		return result
	}

	cache, err := golden.LoadUIDCache(path)
	if err != nil {
		result.Valid = false
		result.Message = err.Error()
		return result
	}

	if len(cache.Entries) == 0 {
		result.Valid = false
		result.Message = "缓存为空"
		return result
	}

	result.Valid = true
	result.Message = fmt.Sprintf("有效 (%d entries)", len(cache.Entries))
	return result
}

func (c *TestCmd) printResults(p interface {
	Printf(format string, a ...interface{})
	Println(a ...interface{})
	Colored(colorName config.ColorName, text string) string
}, results []types.TestResult) {
	for _, result := range results {
		var status string
		if result.Valid {
			status = p.Colored(config.ColorGreen, "[+]")
		} else {
			status = p.Colored(config.ColorRed, "[-]")
		}

		p.Printf("  %s %s (%s)\n", status, result.Name, result.Path)
		p.Printf("      %s\n", result.Message)
	}
}
