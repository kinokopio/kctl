package sa

import (
	"fmt"
	"strings"

	"kctl/config"
	"kctl/internal/output"
	"kctl/internal/session"
)

// ListCmd list 子命令
type ListCmd struct{}

func init() {
	Register(&ListCmd{})
}

func (c *ListCmd) Name() string {
	return "list"
}

func (c *ListCmd) Aliases() []string {
	return []string{"ls"}
}

func (c *ListCmd) Description() string {
	return "列出已扫描的 ServiceAccount"
}

func (c *ListCmd) Usage() string {
	return `sa list [options]

列出已扫描的 ServiceAccount

选项：
  --admin, -a     只显示 cluster-admin
  --risky, -r     只显示有风险权限的 SA
  -n <namespace>  按命名空间过滤
  --perms, -p     显示权限
  --token, -t     显示 Token

示例：
  sa list                 列出所有 SA
  sa list --admin         只显示 cluster-admin
  sa list --risky         只显示有风险的 SA
  sa list -n kube-system  只显示 kube-system 命名空间的 SA`
}

func (c *ListCmd) Execute(sess *session.Session, args []string) error {
	p := sess.Printer

	// 检查是否已扫描
	if !sess.IsScanned {
		return fmt.Errorf("请先执行 'sa scan' 扫描 ServiceAccount")
	}

	// 解析参数
	onlyAdmin := false
	onlyRisky := false
	namespace := ""
	showPerms := false
	showToken := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--admin", "-a":
			onlyAdmin = true
		case "--risky", "-r":
			onlyRisky = true
		case "-n":
			if i+1 < len(args) {
				namespace = args[i+1]
				i++
			}
		case "--perms", "-p":
			showPerms = true
		case "--token", "-t":
			showToken = true
		}
	}

	// 从数据库获取 SA
	sas, err := sess.SADB.GetAll()
	if err != nil {
		return fmt.Errorf("获取 ServiceAccount 失败: %w", err)
	}

	if len(sas) == 0 {
		p.Warning("没有找到 ServiceAccount，请先执行 'sa scan'")
		return nil
	}

	// 过滤
	var filtered []*struct {
		Namespace      string
		Name           string
		RiskLevel      string
		IsClusterAdmin bool
		Token          string
		Permissions    string
		Flags          string
	}

	for _, sa := range sas {
		// 命名空间过滤
		if namespace != "" && sa.Namespace != namespace {
			continue
		}

		// admin 过滤
		if onlyAdmin && !sa.IsClusterAdmin {
			continue
		}

		// risky 过滤
		if onlyRisky {
			if sa.RiskLevel == string(config.RiskNone) && !sa.IsClusterAdmin {
				continue
			}
		}

		// 构建 flags
		flags := c.buildFlags(p, sa.SecurityFlags, sa.IsClusterAdmin)

		filtered = append(filtered, &struct {
			Namespace      string
			Name           string
			RiskLevel      string
			IsClusterAdmin bool
			Token          string
			Permissions    string
			Flags          string
		}{
			Namespace:      sa.Namespace,
			Name:           sa.Name,
			RiskLevel:      sa.RiskLevel,
			IsClusterAdmin: sa.IsClusterAdmin,
			Token:          sa.Token,
			Permissions:    sa.Permissions,
			Flags:          flags,
		})
	}

	if len(filtered) == 0 {
		p.Warning("没有符合条件的 ServiceAccount")
		return nil
	}

	// 打印表格
	p.Println()
	tablePrinter := output.NewTablePrinter()
	var rows []output.SARow

	for _, sa := range filtered {
		var riskLabel string
		if sa.IsClusterAdmin {
			riskLabel = p.Colored(config.ColorRed, "ADMIN")
		} else {
			riskLevel := config.RiskLevel(sa.RiskLevel)
			display := config.RiskLevelDisplayConfig[riskLevel]
			riskLabel = p.Colored(display.Color, display.Label)
		}

		// 权限字符串
		perms := "-"
		if sa.IsClusterAdmin {
			perms = p.Colored(config.ColorRed, "*/* (cluster-admin)")
		} else if sa.Permissions != "" {
			perms = c.formatPermissions(p, sa.Permissions)
		}

		rows = append(rows, output.SARow{
			Risk:        riskLabel,
			Namespace:   sa.Namespace,
			Name:        sa.Name,
			TokenStatus: p.Colored(config.ColorGreen, "有效"),
			Flags:       sa.Flags,
			Permissions: perms,
			Token:       sa.Token,
		})
	}

	tablePrinter.PrintServiceAccounts(rows, showPerms, showToken)

	p.Printf("\n  共 %d 个 ServiceAccount\n\n", len(filtered))

	return nil
}

func (c *ListCmd) buildFlags(p output.Printer, securityFlagsJSON string, isClusterAdmin bool) string {
	var flags []string

	// 解析安全标识
	if securityFlagsJSON != "" {
		// 简单解析 JSON
		if strings.Contains(securityFlagsJSON, `"privileged":true`) {
			flags = append(flags, p.Colored(config.ColorRed, "PRIV"))
		}
		if strings.Contains(securityFlagsJSON, `"allowPrivilegeEscalation":true`) {
			flags = append(flags, p.Colored(config.ColorYellow, "PE"))
		}
		if strings.Contains(securityFlagsJSON, `"hasHostPath":true`) {
			flags = append(flags, p.Colored(config.ColorRed, "HP"))
		}
		if strings.Contains(securityFlagsJSON, `"hasSecretMount":true`) {
			flags = append(flags, p.Colored(config.ColorYellow, "SEC"))
		}
	}

	if len(flags) == 0 {
		return "-"
	}
	return strings.Join(flags, ",")
}

func (c *ListCmd) formatPermissions(p output.Printer, permissionsJSON string) string {
	// 简单格式化权限
	if permissionsJSON == "" || permissionsJSON == "[]" {
		return "-"
	}

	// 提取权限
	var perms []string
	// 简单解析
	parts := strings.Split(permissionsJSON, `"resource":"`)
	for i := 1; i < len(parts); i++ {
		endIdx := strings.Index(parts[i], `"`)
		if endIdx > 0 {
			resource := parts[i][:endIdx]
			// 查找对应的 verb
			verbStart := strings.Index(parts[i], `"verb":"`)
			if verbStart > 0 {
				verbPart := parts[i][verbStart+8:]
				verbEnd := strings.Index(verbPart, `"`)
				if verbEnd > 0 {
					verb := verbPart[:verbEnd]
					permStr := fmt.Sprintf("%s:%s", resource, verb)

					if config.IsCriticalPermission(resource, verb) {
						permStr = p.Colored(config.ColorRed, permStr)
					} else if config.IsHighPermission(resource, verb) {
						permStr = p.Colored(config.ColorYellow, permStr)
					}

					perms = append(perms, permStr)
				}
			}
		}
	}

	if len(perms) == 0 {
		return "-"
	}
	return strings.Join(perms, "\n")
}
