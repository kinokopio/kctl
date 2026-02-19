package persist

import (
	"context"
	"fmt"

	"kctl/config"
	"kctl/internal/persist"
	"kctl/internal/session"
	"kctl/pkg/types"
	"kctl/utils/Ask"
)

// WebhookRestoreCmd webhook-restore 子命令
type WebhookRestoreCmd struct{}

func init() {
	Register(&WebhookRestoreCmd{})
}

func (c *WebhookRestoreCmd) Name() string        { return "webhook-restore" }
func (c *WebhookRestoreCmd) Aliases() []string   { return []string{"wr", "webhook-remove"} }
func (c *WebhookRestoreCmd) Description() string { return "移除注入的恶意 Webhook" }

func (c *WebhookRestoreCmd) Usage() string {
	return `persist webhook-restore [选项]

移除由 kctl 注入的恶意 Webhook 及相关资源

此命令会删除 MutatingWebhookConfiguration 以及相关的 Deployment、Service、
Secret 和 ConfigMap。

可选选项:
  --name          指定要移除的 Webhook 名称
  --all           移除所有 kctl 管理的 Webhook
  --server        API Server URL
  --token         认证 Token

示例:
  persist webhook-restore                       # 交互式选择要移除的 Webhook
  persist webhook-restore --name kctl-webhook-xxx
  persist webhook-restore --all                 # 移除所有 kctl 管理的 Webhook`
}

func (c *WebhookRestoreCmd) Execute(sess *session.Session, args []string) error {
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

	// 获取所有 kctl 管理的 Webhook
	p.Printf("%s Fetching kctl-managed webhooks...\n", p.Colored(config.ColorBlue, "[*]"))
	mutating, err := k8sClient.ListMutatingWebhooks(ctx)
	if err != nil {
		return fmt.Errorf("获取 Webhook 列表失败: %w", err)
	}

	// 过滤出 kctl 管理的
	var kctlWebhooks []types.WebhookInfo
	for _, wh := range mutating {
		if wh.IsManagedByKctl {
			kctlWebhooks = append(kctlWebhooks, wh)
		}
	}

	if len(kctlWebhooks) == 0 {
		p.Printf("%s No kctl-managed webhooks found\n", p.Colored(config.ColorYellow, "[!]"))
		return nil
	}

	p.Printf("%s Found %d kctl-managed webhook(s)\n", p.Colored(config.ColorGreen, "[+]"), len(kctlWebhooks))

	// 确定要移除的 Webhook
	var toRemove []types.WebhookInfo

	if opts.removeAll {
		toRemove = kctlWebhooks
	} else if opts.webhookName != "" {
		// 按名称查找
		for _, wh := range kctlWebhooks {
			if wh.Name == opts.webhookName {
				toRemove = append(toRemove, wh)
				break
			}
		}
		if len(toRemove) == 0 {
			return fmt.Errorf("未找到名为 %s 的 kctl 管理的 Webhook", opts.webhookName)
		}
	} else {
		// 交互式选择
		options := make([]Ask.SelectOption, len(kctlWebhooks))
		for i, wh := range kctlWebhooks {
			desc := fmt.Sprintf("%s - %s/%s", wh.Type, wh.ServiceNS, wh.ServiceName)
			options[i] = Ask.SelectOption{
				Label:       wh.Name,
				Value:       wh.Name,
				Description: desc,
			}
		}

		selected, err := Ask.SelectMultiple("选择要移除的 Webhook:", options)
		if err != nil {
			return fmt.Errorf("选择失败: %w", err)
		}

		if len(selected) == 0 {
			p.Printf("%s No webhooks selected\n", p.Colored(config.ColorYellow, "[!]"))
			return nil
		}

		// 根据选择过滤
		selectedMap := make(map[string]bool)
		for _, name := range selected {
			selectedMap[name] = true
		}
		for _, wh := range kctlWebhooks {
			if selectedMap[wh.Name] {
				toRemove = append(toRemove, wh)
			}
		}
	}

	// 显示将要移除的 Webhook
	p.Println()
	p.Printf("%s Will remove the following webhook(s):\n", p.Colored(config.ColorYellow, "[!]"))
	for _, wh := range toRemove {
		p.Printf("  - %s (%s/%s)\n", wh.Name, wh.ServiceNS, wh.ServiceName)
	}
	p.Println()

	// 确认
	if !Ask.ForSure("确认移除这些 Webhook?") {
		p.Printf("%s Operation cancelled\n", p.Colored(config.ColorYellow, "[!]"))
		return nil
	}

	// 执行移除
	for _, wh := range toRemove {
		p.Printf("%s Removing %s...\n", p.Colored(config.ColorBlue, "[*]"), wh.Name)

		if err := persist.RemoveWebhook(ctx, k8sClient, &wh); err != nil {
			p.Printf("%s Failed to remove %s: %s\n", p.Colored(config.ColorRed, "[-]"), wh.Name, err.Error())
		} else {
			p.Printf("%s Removed %s\n", p.Colored(config.ColorGreen, "[+]"), wh.Name)
		}
	}

	p.Println()
	p.Printf("%s Done!\n", p.Colored(config.ColorGreen, "[+]"))

	return nil
}

type webhookRestoreOptions struct {
	webhookName string
	removeAll   bool
	serverURL   string
	token       string
}

func (c *WebhookRestoreCmd) parseArgs(args []string) *webhookRestoreOptions {
	opts := &webhookRestoreOptions{}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--name":
			if i+1 < len(args) {
				i++
				opts.webhookName = args[i]
			}
		case "--all":
			opts.removeAll = true
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
