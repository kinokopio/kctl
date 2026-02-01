package commands

import (
	"encoding/json"
	"fmt"
	"strings"

	"kctl/config"
	"kctl/internal/session"
	"kctl/pkg/types"
)

// UseCmd use 命令
type UseCmd struct{}

func init() {
	Register(&UseCmd{})
}

func (c *UseCmd) Name() string {
	return "use"
}

func (c *UseCmd) Aliases() []string {
	return nil
}

func (c *UseCmd) Description() string {
	return "选择 ServiceAccount"
}

func (c *UseCmd) Usage() string {
	return `use <namespace/name>

选择一个 ServiceAccount 作为当前操作目标

选择后：
  - 提示符会显示当前 SA 和风险等级
  - exec 命令会默认使用该 SA 关联的 Pod

示例：
  use kube-system/cluster-admin
  use default/nginx`
}

func (c *UseCmd) Execute(sess *session.Session, args []string) error {
	p := sess.Printer

	if len(args) == 0 {
		// 没有参数时，列出可用的 SA
		return c.listAvailableSAs(sess)
	}

	// 解析 namespace/name
	target := args[0]
	parts := strings.SplitN(target, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("格式错误，请使用 namespace/sa-name 格式")
	}

	namespace := parts[0]
	name := parts[1]

	// 从数据库查找
	sa, err := sess.SADB.GetByName(namespace, name)
	if err != nil {
		return fmt.Errorf("查找 ServiceAccount 失败: %w", err)
	}

	if sa == nil {
		// 未找到，显示可用的 SA
		p.Error(fmt.Sprintf("未找到 ServiceAccount: %s/%s", namespace, name))
		p.Println()
		return c.listAvailableSAs(sess)
	}

	// 设置当前 SA
	sess.SetCurrentSA(sa)

	// 显示信息
	p.Printf("%s Selected: %s/%s\n",
		p.Colored(config.ColorBlue, "[*]"),
		sa.Namespace, sa.Name)

	// 显示风险等级
	if sa.IsClusterAdmin {
		p.Printf("%s Risk Level: %s (cluster-admin)\n",
			p.Colored(config.ColorBlue, "[*]"),
			p.Colored(config.ColorRed, "ADMIN"))
	} else if sa.RiskLevel != "" && sa.RiskLevel != string(config.RiskNone) {
		riskLevel := config.RiskLevel(sa.RiskLevel)
		display := config.RiskLevelDisplayConfig[riskLevel]
		p.Printf("%s Risk Level: %s\n",
			p.Colored(config.ColorBlue, "[*]"),
			p.Colored(display.Color, display.Label))
	}

	// 显示关联的 Pod
	if sa.Pods != "" && sa.Pods != "[]" {
		p.Printf("%s Associated Pods: %s\n",
			p.Colored(config.ColorBlue, "[*]"),
			c.formatPods(sa.Pods))
	}

	return nil
}

// listAvailableSAs 列出可用的 ServiceAccount
func (c *UseCmd) listAvailableSAs(sess *session.Session) error {
	p := sess.Printer

	sas, err := sess.SADB.GetAll()
	if err != nil {
		return fmt.Errorf("获取 ServiceAccount 列表失败: %w", err)
	}

	if len(sas) == 0 {
		return fmt.Errorf("没有可用的 ServiceAccount，请先执行 'scan'")
	}

	p.Printf("  %s\n\n", p.Colored(config.ColorYellow, "可用的 ServiceAccount:"))

	for _, sa := range sas {
		// 风险等级
		var riskLabel string
		if sa.IsClusterAdmin {
			riskLabel = p.Colored(config.ColorRed, "ADMIN")
		} else {
			riskLevel := config.RiskLevel(sa.RiskLevel)
			display := config.RiskLevelDisplayConfig[riskLevel]
			riskLabel = p.Colored(display.Color, display.Label)
		}

		p.Printf("    %s/%s  %s\n",
			sa.Namespace, sa.Name, riskLabel)
	}

	p.Println()
	p.Printf("  用法: %s\n\n", p.Colored(config.ColorCyan, "use <namespace/sa-name>"))

	return nil
}

func (c *UseCmd) formatPods(podsJSON string) string {
	if podsJSON == "" || podsJSON == "[]" {
		return "-"
	}

	var pods []types.SAPodInfo
	if err := json.Unmarshal([]byte(podsJSON), &pods); err != nil {
		// JSON 解析失败，返回原始内容的简化版
		return podsJSON
	}

	var result []string
	for _, pod := range pods {
		result = append(result, fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))
	}

	if len(result) == 0 {
		return "-"
	}
	return strings.Join(result, ", ")
}
