package commands

import (
	"encoding/json"
	"fmt"
	"strings"

	"kctl/config"
	"kctl/internal/output"
	"kctl/internal/session"
	"kctl/pkg/types"
)

// InfoCmd info 命令
type InfoCmd struct{}

func init() {
	Register(&InfoCmd{})
}

func (c *InfoCmd) Name() string {
	return "info"
}

func (c *InfoCmd) Aliases() []string {
	return nil
}

func (c *InfoCmd) Description() string {
	return "显示当前 SA 详情"
}

func (c *InfoCmd) Usage() string {
	return `info

显示当前 ServiceAccount 的详细信息

进入控制台时会自动设置当前 SA，也可以使用 'use' 命令切换

运行 'scan' 后可以获取更详细的权限信息`
}

func (c *InfoCmd) Execute(sess *session.Session, args []string) error {
	p := sess.Printer

	sa := sess.GetCurrentSA()
	if sa == nil {
		return fmt.Errorf("未选择 ServiceAccount，请先使用 'use <namespace/name>' 选择")
	}

	p.Println()
	p.Printf("  %s\n", p.Colored(config.ColorCyan, "ServiceAccount Information"))
	p.Println("  " + p.Colored(config.ColorGray, "─────────────────────────────────────────"))

	// 基本信息
	p.Printf("  %-16s: %s\n", "Name", sa.Name)
	p.Printf("  %-16s: %s\n", "Namespace", sa.Namespace)

	// 风险等级
	var riskDisplay string
	if sa.IsClusterAdmin {
		riskDisplay = p.Colored(config.ColorRed, "ADMIN (cluster-admin)")
	} else {
		riskLevel := config.RiskLevel(sa.RiskLevel)
		display := config.RiskLevelDisplayConfig[riskLevel]
		riskDisplay = p.Colored(display.Color, display.Label)
	}
	p.Printf("  %-16s: %s\n", "Risk Level", riskDisplay)

	// Token 状态
	tokenStatus := p.Colored(config.ColorGreen, "Valid")
	if sa.IsExpired {
		tokenStatus = p.Colored(config.ColorRed, "Expired")
	}
	if sa.TokenExpiration != "" {
		tokenStatus = fmt.Sprintf("%s (expires: %s)", tokenStatus, sa.TokenExpiration)
	}
	p.Printf("  %-16s: %s\n", "Token Status", tokenStatus)

	p.Println()

	// 权限
	p.Printf("  %s:\n", p.Colored(config.ColorYellow, "Permissions"))
	if sa.IsClusterAdmin {
		p.Printf("    %s\n", p.Colored(config.ColorRed, "*/* (cluster-admin)"))
	} else if sa.Permissions != "" && sa.Permissions != "[]" {
		c.printPermissions(p, sa.Permissions)
	} else {
		p.Printf("    %s\n", p.Colored(config.ColorGray, "(not scanned - run 'scan' to check permissions)"))
	}

	p.Println()

	// 安全标识
	p.Printf("  %s:\n", p.Colored(config.ColorYellow, "Security Flags"))
	c.printSecurityFlags(p, sa.SecurityFlags)

	p.Println()

	// 关联的 Pod
	p.Printf("  %s:\n", p.Colored(config.ColorYellow, "Associated Pods"))
	c.printPods(p, sa.Pods)

	p.Println()

	return nil
}

func (c *InfoCmd) printPermissions(p output.Printer, permissionsJSON string) {
	var perms []types.SAPermission
	if err := json.Unmarshal([]byte(permissionsJSON), &perms); err != nil {
		// 简单解析
		parts := strings.Split(permissionsJSON, `"resource":"`)
		for i := 1; i < len(parts); i++ {
			endIdx := strings.Index(parts[i], `"`)
			if endIdx > 0 {
				resource := parts[i][:endIdx]
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
						p.Printf("    - %s\n", permStr)
					}
				}
			}
		}
		return
	}

	for _, perm := range perms {
		resource := perm.Resource
		if perm.Subresource != "" {
			resource = perm.Resource + "/" + perm.Subresource
		}
		permStr := fmt.Sprintf("%s:%s", resource, perm.Verb)
		if config.IsCriticalPermission(resource, perm.Verb) {
			permStr = p.Colored(config.ColorRed, permStr)
		} else if config.IsHighPermission(resource, perm.Verb) {
			permStr = p.Colored(config.ColorYellow, permStr)
		}
		p.Printf("    - %s\n", permStr)
	}
}

func (c *InfoCmd) printSecurityFlags(p output.Printer, flagsJSON string) {
	if flagsJSON == "" {
		p.Printf("    %s\n", p.Colored(config.ColorGray, "(none)"))
		return
	}

	var flags types.SASecurityFlags
	if err := json.Unmarshal([]byte(flagsJSON), &flags); err != nil {
		p.Printf("    %s\n", p.Colored(config.ColorGray, "(parse error)"))
		return
	}

	hasFlags := false
	if flags.Privileged {
		p.Printf("    - %s\n", p.Colored(config.ColorRed, "Privileged Container"))
		hasFlags = true
	}
	if flags.AllowPrivilegeEscalation {
		p.Printf("    - %s\n", p.Colored(config.ColorYellow, "Allow Privilege Escalation"))
		hasFlags = true
	}
	if flags.HasHostPath {
		p.Printf("    - %s\n", p.Colored(config.ColorRed, "HostPath Mount"))
		hasFlags = true
	}
	if flags.HasSecretMount {
		p.Printf("    - %s\n", p.Colored(config.ColorYellow, "Secret Mount"))
		hasFlags = true
	}
	if flags.HasSATokenMount {
		p.Printf("    - %s\n", p.Colored(config.ColorGreen, "ServiceAccount Token Mount"))
		hasFlags = true
	}

	if !hasFlags {
		p.Printf("    %s\n", p.Colored(config.ColorGray, "(none)"))
	}
}

func (c *InfoCmd) printPods(p output.Printer, podsJSON string) {
	if podsJSON == "" || podsJSON == "[]" {
		p.Printf("    %s\n", p.Colored(config.ColorGray, "(none)"))
		return
	}

	var pods []types.SAPodInfo
	if err := json.Unmarshal([]byte(podsJSON), &pods); err != nil {
		// 简单解析
		parts := strings.Split(podsJSON, `"name":"`)
		for i := 1; i < len(parts); i++ {
			endIdx := strings.Index(parts[i], `"`)
			if endIdx > 0 {
				name := parts[i][:endIdx]
				p.Printf("    - %s\n", name)
			}
		}
		return
	}

	for _, pod := range pods {
		p.Printf("    - %s/%s", pod.Namespace, pod.Name)
		if pod.Container != "" {
			p.Printf(" (%s)", pod.Container)
		}
		p.Println()
	}
}
