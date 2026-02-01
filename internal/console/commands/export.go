package commands

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"kctl/internal/session"
)

// ExportCmd export 命令
type ExportCmd struct{}

func init() {
	Register(&ExportCmd{})
}

func (c *ExportCmd) Name() string {
	return "export"
}

func (c *ExportCmd) Aliases() []string {
	return nil
}

func (c *ExportCmd) Description() string {
	return "导出结果"
}

func (c *ExportCmd) Usage() string {
	return `export <format>

导出扫描结果

格式：
  json    JSON 格式
  csv     CSV 格式

示例：
  export json
  export csv`
}

// ExportData 导出数据结构
type ExportData struct {
	ScanTime        string      `json:"scanTime"`
	KubeletIP       string      `json:"kubeletIP"`
	ServiceAccounts []ExportSA  `json:"serviceAccounts"`
	Pods            []ExportPod `json:"pods"`
}

type ExportSA struct {
	Namespace      string   `json:"namespace"`
	Name           string   `json:"name"`
	RiskLevel      string   `json:"riskLevel"`
	IsClusterAdmin bool     `json:"isClusterAdmin"`
	Permissions    []string `json:"permissions"`
	Pods           []string `json:"pods"`
}

type ExportPod struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	PodIP     string `json:"podIP"`
	Flags     string `json:"flags"`
}

func (c *ExportCmd) Execute(sess *session.Session, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("用法: export <json|csv>")
	}

	format := strings.ToLower(args[0])

	// 检查是否有数据
	if !sess.IsScanned {
		return fmt.Errorf("没有扫描数据，请先执行 'scan'")
	}

	switch format {
	case "json":
		return c.exportJSON(sess)
	case "csv":
		return c.exportCSV(sess)
	default:
		return fmt.Errorf("不支持的格式: %s (可用: json, csv)", format)
	}
}

func (c *ExportCmd) exportJSON(sess *session.Session) error {
	p := sess.Printer

	data := ExportData{
		ScanTime:  sess.LastScanTime.Format(time.RFC3339),
		KubeletIP: sess.Config.KubeletIP,
	}

	// 获取 SA
	sas, err := sess.SADB.GetAll()
	if err != nil {
		return fmt.Errorf("获取 ServiceAccount 失败: %w", err)
	}

	for _, sa := range sas {
		exportSA := ExportSA{
			Namespace:      sa.Namespace,
			Name:           sa.Name,
			RiskLevel:      sa.RiskLevel,
			IsClusterAdmin: sa.IsClusterAdmin,
		}

		// 解析权限
		if sa.Permissions != "" && sa.Permissions != "[]" {
			var perms []struct {
				Resource string `json:"resource"`
				Verb     string `json:"verb"`
			}
			if err := json.Unmarshal([]byte(sa.Permissions), &perms); err == nil {
				for _, perm := range perms {
					exportSA.Permissions = append(exportSA.Permissions, perm.Resource+":"+perm.Verb)
				}
			}
		}

		// 解析 Pod
		if sa.Pods != "" && sa.Pods != "[]" {
			var pods []struct {
				Namespace string `json:"namespace"`
				Name      string `json:"name"`
			}
			if err := json.Unmarshal([]byte(sa.Pods), &pods); err == nil {
				for _, pod := range pods {
					exportSA.Pods = append(exportSA.Pods, pod.Namespace+"/"+pod.Name)
				}
			}
		}

		data.ServiceAccounts = append(data.ServiceAccounts, exportSA)
	}

	// 获取 Pod
	pods := sess.GetCachedPods()
	for _, pod := range pods {
		data.Pods = append(data.Pods, ExportPod{
			Namespace: pod.Namespace,
			Name:      pod.PodName,
			Status:    pod.Status,
			PodIP:     pod.PodIP,
		})
	}

	// 输出 JSON
	output, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 JSON 失败: %w", err)
	}

	p.Println(string(output))
	return nil
}

func (c *ExportCmd) exportCSV(sess *session.Session) error {
	p := sess.Printer

	// 获取 SA
	sas, err := sess.SADB.GetAll()
	if err != nil {
		return fmt.Errorf("获取 ServiceAccount 失败: %w", err)
	}

	// 输出 CSV 头
	p.Println("namespace,name,risk_level,is_cluster_admin,permissions")

	for _, sa := range sas {
		// 解析权限
		perms := ""
		if sa.Permissions != "" && sa.Permissions != "[]" {
			var permList []struct {
				Resource string `json:"resource"`
				Verb     string `json:"verb"`
			}
			if err := json.Unmarshal([]byte(sa.Permissions), &permList); err == nil {
				var permStrs []string
				for _, perm := range permList {
					permStrs = append(permStrs, perm.Resource+":"+perm.Verb)
				}
				perms = strings.Join(permStrs, ";")
			}
		}

		// 输出 CSV 行
		p.Printf("%s,%s,%s,%t,\"%s\"\n",
			sa.Namespace,
			sa.Name,
			sa.RiskLevel,
			sa.IsClusterAdmin,
			perms)
	}

	return nil
}
