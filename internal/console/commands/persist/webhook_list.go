package persist

import (
	"context"
	"fmt"
	"strings"

	"kctl/config"
	"kctl/internal/output"
	"kctl/internal/session"
	"kctl/pkg/types"
)

// WebhookListCmd webhook-list 子命令
type WebhookListCmd struct{}

func init() {
	Register(&WebhookListCmd{})
}

func (c *WebhookListCmd) Name() string        { return "webhook-list" }
func (c *WebhookListCmd) Aliases() []string   { return []string{"wl", "webhooks"} }
func (c *WebhookListCmd) Description() string { return "列出所有 Admission Webhook 配置" }

func (c *WebhookListCmd) Usage() string {
	return `persist webhook-list [选项]

列出集群中的 Admission Webhook 配置

此命令会列出所有 MutatingWebhookConfiguration 和 ValidatingWebhookConfiguration，
并标记由 kctl 注入的 Webhook。

可选选项:
  --all, -a       显示所有 Webhook (默认只显示 kctl 管理的)
  --type, -t      过滤类型 (mutating/validating/all，默认: all)
  --server        API Server URL
  --token         认证 Token

示例:
  persist webhook-list                  # 列出 kctl 管理的 Webhook
  persist webhook-list --all            # 列出所有 Webhook
  persist webhook-list -t mutating      # 只列出 MutatingWebhook`
}

func (c *WebhookListCmd) Execute(sess *session.Session, args []string) error {
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
	opts := c.parseArgs(args)

	// 创建 K8s 客户端
	k8sClient, err := createK8sClient(sess, opts.serverURL, opts.token)
	if err != nil {
		return err
	}

	var allWebhooks []types.WebhookInfo

	// 获取 MutatingWebhook
	if opts.webhookType == "" || opts.webhookType == "all" || opts.webhookType == "mutating" {
		p.Printf("%s Fetching MutatingWebhookConfigurations...\n", p.Colored(config.ColorBlue, "[*]"))
		mutating, err := k8sClient.ListMutatingWebhooks(ctx)
		if err != nil {
			p.Warning("获取 MutatingWebhook 失败: " + err.Error())
		} else {
			allWebhooks = append(allWebhooks, mutating...)
		}
	}

	// 获取 ValidatingWebhook
	if opts.webhookType == "" || opts.webhookType == "all" || opts.webhookType == "validating" {
		p.Printf("%s Fetching ValidatingWebhookConfigurations...\n", p.Colored(config.ColorBlue, "[*]"))
		validating, err := k8sClient.ListValidatingWebhooks(ctx)
		if err != nil {
			p.Warning("获取 ValidatingWebhook 失败: " + err.Error())
		} else {
			allWebhooks = append(allWebhooks, validating...)
		}
	}

	// 过滤
	var filtered []types.WebhookInfo
	for _, wh := range allWebhooks {
		if opts.showAll || wh.IsManagedByKctl {
			filtered = append(filtered, wh)
		}
	}

	if len(filtered) == 0 {
		if opts.showAll {
			p.Printf("%s No webhooks found\n", p.Colored(config.ColorYellow, "[!]"))
		} else {
			p.Printf("%s No kctl-managed webhooks found. Use --all to show all webhooks.\n", p.Colored(config.ColorYellow, "[!]"))
		}
		return nil
	}

	// 显示结果
	p.Println()
	p.Printf("%s Found %d webhook(s):\n\n", p.Colored(config.ColorGreen, "[+]"), len(filtered))

	for i, wh := range filtered {
		c.printWebhookInfo(p, &wh, i+1)
	}

	return nil
}

func (c *WebhookListCmd) printWebhookInfo(p output.Printer, wh *types.WebhookInfo, index int) {
	// 标题
	title := fmt.Sprintf("[%d] %s", index, wh.Name)
	if wh.IsManagedByKctl {
		title += p.Colored(config.ColorRed, " [kctl]")
	}
	p.Println(p.Colored(config.ColorCyan, title))

	// 基本信息
	p.Printf("    Type: %s\n", wh.Type)
	if wh.ServiceNS != "" && wh.ServiceName != "" {
		p.Printf("    Service: %s/%s:%d\n", wh.ServiceNS, wh.ServiceName, wh.ServicePort)
	}
	if wh.Path != "" {
		p.Printf("    Path: %s\n", wh.Path)
	}
	p.Printf("    FailurePolicy: %s\n", wh.FailurePolicy)

	// 规则
	if len(wh.Rules) > 0 {
		p.Printf("    Rules:\n")
		for _, rule := range wh.Rules {
			ops := strings.Join(rule.Operations, ",")
			resources := strings.Join(rule.Resources, ",")
			groups := strings.Join(rule.APIGroups, ",")
			if groups == "" {
				groups = "core"
			}
			p.Printf("      - %s %s/%s\n", ops, groups, resources)
		}
	}

	p.Println()
}

type webhookListOptions struct {
	showAll     bool
	webhookType string
	serverURL   string
	token       string
}

func (c *WebhookListCmd) parseArgs(args []string) *webhookListOptions {
	opts := &webhookListOptions{}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--all", "-a":
			opts.showAll = true
		case "--type", "-t":
			if i+1 < len(args) {
				i++
				opts.webhookType = strings.ToLower(args[i])
			}
		case "--server":
			if i+1 < len(args) {
				i++
				opts.serverURL = args[i]
			}
		case "--token":
			if i+1 < len(args) {
				i++
				opts.token = args[i]
			}
		}
	}

	return opts
}
