package commands

import (
	"fmt"
	"time"

	"kctl/config"
	"kctl/internal/session"
)

// ShowCmd show 命令
type ShowCmd struct{}

func init() {
	Register(&ShowCmd{})
}

func (c *ShowCmd) Name() string {
	return "show"
}

func (c *ShowCmd) Aliases() []string {
	return nil
}

func (c *ShowCmd) Description() string {
	return "显示配置或状态信息"
}

func (c *ShowCmd) Usage() string {
	return `show <what>

显示配置或状态信息

可用选项：
  options    显示当前配置
  status     显示会话状态
  env        显示环境信息

示例：
  show options
  show status`
}

func (c *ShowCmd) Execute(sess *session.Session, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("用法: show <options|status|env>")
	}

	what := args[0]

	switch what {
	case "options", "opts", "config":
		c.showOptions(sess)

	case "status", "stat":
		c.showStatus(sess)

	case "env":
		c.showEnv(sess)

	default:
		return fmt.Errorf("未知选项: %s (可用: options, status, env)", what)
	}

	return nil
}

func (c *ShowCmd) showOptions(sess *session.Session) {
	p := sess.Printer

	p.Println()
	p.Printf("  %s\n", p.Colored(config.ColorCyan, "Configuration"))
	p.Println("  " + p.Colored(config.ColorGray, "─────────────────────────────────────────"))

	// Kubelet IP
	kubeletIP := sess.Config.KubeletIP
	if kubeletIP == "" {
		kubeletIP = p.Colored(config.ColorGray, "(not set)")
	} else if sess.InPod {
		kubeletIP = kubeletIP + p.Colored(config.ColorGray, " (auto)")
	}
	p.Printf("  %-16s: %s\n", "Kubelet IP", kubeletIP)

	// Kubelet Port
	p.Printf("  %-16s: %d\n", "Kubelet Port", sess.Config.KubeletPort)

	// Token
	tokenStatus := p.Colored(config.ColorGray, "(not set)")
	if sess.Config.Token != "" {
		if sess.Config.TokenFile != "" {
			tokenStatus = sess.Config.TokenFile
		} else {
			tokenStatus = p.Colored(config.ColorGreen, "(set)")
		}
	}
	p.Printf("  %-16s: %s\n", "Token", tokenStatus)

	// API Server
	apiServer := sess.Config.APIServer
	if apiServer == "" {
		apiServer = p.Colored(config.ColorGray, "(not set)")
	}
	p.Printf("  %-16s: %s:%d\n", "API Server", apiServer, sess.Config.APIServerPort)

	// Proxy
	proxy := sess.Config.ProxyURL
	if proxy == "" {
		proxy = p.Colored(config.ColorGray, "(none)")
	}
	p.Printf("  %-16s: %s\n", "Proxy", proxy)

	// Concurrency
	p.Printf("  %-16s: %d\n", "Concurrency", sess.Config.Concurrency)

	p.Println()
}

func (c *ShowCmd) showStatus(sess *session.Session) {
	p := sess.Printer

	p.Println()
	p.Printf("  %s\n", p.Colored(config.ColorCyan, "Session Status"))
	p.Println("  " + p.Colored(config.ColorGray, "─────────────────────────────────────────"))

	// Connected
	connStatus := p.Colored(config.ColorRed, "No")
	if sess.IsConnected {
		connStatus = p.Colored(config.ColorGreen, "Yes")
	}
	p.Printf("  %-16s: %s\n", "Connected", connStatus)

	// Scanned
	scanStatus := p.Colored(config.ColorGray, "No")
	if sess.IsScanned {
		elapsed := time.Since(sess.LastScanTime)
		scanStatus = fmt.Sprintf("%s (%s ago)",
			p.Colored(config.ColorGreen, "Yes"),
			formatDuration(elapsed))
	}
	p.Printf("  %-16s: %s\n", "Scanned", scanStatus)

	// Cached SAs
	saCount := 0
	if sess.SADB != nil {
		if sas, err := sess.SADB.GetAll(); err == nil {
			saCount = len(sas)
		}
	}
	p.Printf("  %-16s: %d\n", "Cached SAs", saCount)

	// Cached Pods
	podCount := len(sess.GetCachedPods())
	p.Printf("  %-16s: %d\n", "Cached Pods", podCount)

	// Current SA
	currentSA := p.Colored(config.ColorGray, "(none)")
	if sa := sess.GetCurrentSA(); sa != nil {
		currentSA = fmt.Sprintf("%s/%s", sa.Namespace, sa.Name)
		if sa.RiskLevel != "" && sa.RiskLevel != string(config.RiskNone) {
			currentSA = fmt.Sprintf("%s %s", currentSA,
				p.Colored(config.ColorRed, sa.RiskLevel))
		}
	}
	p.Printf("  %-16s: %s\n", "Current SA", currentSA)

	// Mode
	p.Printf("  %-16s: %s\n", "Mode", sess.GetModeString())

	p.Println()
}

func (c *ShowCmd) showEnv(sess *session.Session) {
	p := sess.Printer

	p.Println()
	p.Printf("  %s\n", p.Colored(config.ColorCyan, "Environment"))
	p.Println("  " + p.Colored(config.ColorGray, "─────────────────────────────────────────"))

	// In Pod
	inPod := p.Colored(config.ColorRed, "No")
	if sess.InPod {
		inPod = p.Colored(config.ColorGreen, "Yes")
	}
	p.Printf("  %-16s: %s\n", "In Pod", inPod)

	// Database
	dbMode := "Memory"
	if sess.DB != nil && !sess.DB.IsInMemory() {
		dbMode = sess.DB.Path()
	}
	p.Printf("  %-16s: %s\n", "Database", dbMode)

	p.Println()
}

// formatDuration 格式化时间间隔
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	}
	return fmt.Sprintf("%d hours", int(d.Hours()))
}
