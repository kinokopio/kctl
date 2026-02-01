package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"kctl/config"
	k8sclient "kctl/internal/client/k8s"
	"kctl/internal/output"
	"kctl/internal/rbac"
	"kctl/internal/session"
	"kctl/pkg/token"
	"kctl/pkg/types"
)

// ScanCmd scan 命令
type ScanCmd struct{}

func init() {
	Register(&ScanCmd{})
}

func (c *ScanCmd) Name() string {
	return "scan"
}

func (c *ScanCmd) Aliases() []string {
	return nil
}

func (c *ScanCmd) Description() string {
	return "扫描所有 SA 权限"
}

func (c *ScanCmd) Usage() string {
	return `scan [options]

扫描所有 Pod 中的 ServiceAccount Token 权限

选项：
  --risky, -r     只显示有风险权限的 SA
  --perms, -p     显示完整权限列表
  --token, -t     显示 Token

示例：
  scan              扫描所有 SA
  scan --risky      只显示有风险的 SA
  scan --perms      显示完整权限`
}

// SATokenResult 扫描结果
type SATokenResult struct {
	Namespace      string
	PodName        string
	Container      string
	ServiceAccount string
	Token          string
	TokenInfo      *types.TokenInfo
	Permissions    []types.PermissionCheck
	SecurityFlags  types.SecurityFlags
	RiskLevel      config.RiskLevel
	IsClusterAdmin bool
	Error          string
}

func (c *ScanCmd) Execute(sess *session.Session, args []string) error {
	p := sess.Printer
	ctx := context.Background()

	// 解析参数
	onlyRisky := false
	showPerms := false
	showToken := false

	for _, arg := range args {
		switch arg {
		case "--risky", "-r":
			onlyRisky = true
		case "--perms", "-p":
			showPerms = true
		case "--token", "-t":
			showToken = true
		}
	}

	// 检查连接
	kubelet, err := sess.GetKubeletClient()
	if err != nil {
		return err
	}

	p.Printf("%s Scanning ServiceAccount tokens...\n",
		p.Colored(config.ColorBlue, "[*]"))

	// 获取所有 Pod
	pods, err := kubelet.GetPodsWithContainers(ctx)
	if err != nil {
		return fmt.Errorf("获取 Pod 列表失败: %w", err)
	}

	// 缓存 Pod 列表
	sess.CachePods(pods)

	// 过滤 Running 状态且挂载了 SA Token 的 Pod
	var targetPods []types.PodContainerInfo
	for _, pod := range pods {
		if pod.Status == "Running" && pod.SecurityFlags.HasSATokenMount {
			targetPods = append(targetPods, pod)
		}
	}

	if len(targetPods) == 0 {
		p.Warning("没有找到挂载 SA Token 的 Running Pod")
		return nil
	}

	p.Printf("%s Found %d pods with SA tokens\n",
		p.Colored(config.ColorBlue, "[*]"),
		len(targetPods))
	p.Printf("%s Checking permissions... (%d concurrent)\n",
		p.Colored(config.ColorBlue, "[*]"),
		sess.Config.Concurrency)

	// 并发扫描
	results := make(chan SATokenResult, len(targetPods))
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, sess.Config.Concurrency)

	for _, pod := range targetPods {
		wg.Add(1)
		go func(pod types.PodContainerInfo) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result := c.scanPodToken(ctx, sess, kubelet, pod)
			results <- result
		}(pod)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// 收集结果
	var allResults []SATokenResult
	for result := range results {
		allResults = append(allResults, result)
	}

	// 按风险等级排序
	c.sortByRisk(allResults)

	// 保存到数据库
	savedCount := c.saveResults(sess, allResults)

	// 标记已扫描
	sess.MarkScanned()

	// 过滤显示结果
	var displayResults []SATokenResult
	for _, result := range allResults {
		if result.Error != "" {
			continue
		}
		if onlyRisky && result.RiskLevel == config.RiskNone && !result.IsClusterAdmin {
			continue
		}
		displayResults = append(displayResults, result)
	}

	// 打印结果
	p.Println()
	tablePrinter := output.NewTablePrinter()
	var rows []output.ScanResultRow
	for _, result := range displayResults {
		rows = append(rows, c.buildResultRow(p, result))
	}
	tablePrinter.PrintScanResults(rows, showPerms, showToken)

	// 统计
	adminCount := 0
	criticalCount := 0
	highCount := 0
	for _, r := range allResults {
		if r.IsClusterAdmin {
			adminCount++
		} else {
			switch r.RiskLevel {
			case config.RiskCritical:
				criticalCount++
			case config.RiskHigh:
				highCount++
			}
		}
	}

	p.Println()
	p.Printf("%s Scan complete: %d SAs",
		p.Colored(config.ColorGreen, "[+]"),
		savedCount)
	if adminCount > 0 {
		p.Printf(", %s ADMIN", p.Colored(config.ColorRed, fmt.Sprintf("%d", adminCount)))
	}
	if criticalCount > 0 {
		p.Printf(", %s CRITICAL", p.Colored(config.ColorRed, fmt.Sprintf("%d", criticalCount)))
	}
	if highCount > 0 {
		p.Printf(", %s HIGH", p.Colored(config.ColorYellow, fmt.Sprintf("%d", highCount)))
	}
	p.Println()
	p.Printf("%s Results cached in memory\n",
		p.Colored(config.ColorGreen, "[+]"))

	return nil
}

func (c *ScanCmd) scanPodToken(ctx context.Context, sess *session.Session, kubelet interface {
	Exec(ctx context.Context, opts *types.ExecOptions) (*types.ExecResult, error)
}, pod types.PodContainerInfo) SATokenResult {
	result := SATokenResult{
		Namespace:     pod.Namespace,
		PodName:       pod.PodName,
		RiskLevel:     config.RiskNone,
		SecurityFlags: pod.SecurityFlags,
	}

	if len(pod.Containers) == 0 {
		result.Error = "Pod 没有容器"
		return result
	}
	result.Container = pod.Containers[0].Name

	// 读取 Token
	command := []string{"cat", "/var/run/secrets/kubernetes.io/serviceaccount/token"}
	opts := &types.ExecOptions{
		Namespace: pod.Namespace,
		Pod:       pod.PodName,
		Container: result.Container,
		Command:   command,
		Stdin:     false,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}

	execResult, err := kubelet.Exec(ctx, opts)
	if err != nil {
		result.Error = fmt.Sprintf("exec 失败: %v", err)
		return result
	}

	if execResult.Error != "" {
		result.Error = fmt.Sprintf("读取 Token 失败: %s", execResult.Error)
		return result
	}

	result.Token = strings.TrimSpace(execResult.Stdout)
	if result.Token == "" {
		result.Error = "Token 为空"
		return result
	}

	// 解析 Token
	tokenInfo, err := token.Parse(result.Token)
	if err != nil {
		result.Error = fmt.Sprintf("解析 Token 失败: %v", err)
		return result
	}
	result.TokenInfo = tokenInfo
	result.ServiceAccount = tokenInfo.ServiceAccount

	// 检查权限
	k8s, err := sess.GetK8sClient(result.Token)
	if err != nil {
		result.Error = fmt.Sprintf("创建 K8s 客户端失败: %v", err)
		return result
	}

	permissions, err := k8s.CheckCommonPermissions(ctx, tokenInfo.Namespace)
	if err != nil {
		result.Error = fmt.Sprintf("检查权限失败: %v", err)
		return result
	}
	result.Permissions = permissions

	// 检查是否是集群管理员
	result.IsClusterAdmin = rbac.IsClusterAdmin(permissions)

	// 计算风险等级
	if result.IsClusterAdmin {
		result.RiskLevel = config.RiskAdmin
	} else {
		result.RiskLevel = rbac.CalculateRiskLevel(permissions)
	}

	return result
}

func (c *ScanCmd) sortByRisk(results []SATokenResult) {
	sort.Slice(results, func(i, j int) bool {
		if results[i].IsClusterAdmin != results[j].IsClusterAdmin {
			return results[i].IsClusterAdmin
		}
		return config.RiskLevelOrder[results[i].RiskLevel] < config.RiskLevelOrder[results[j].RiskLevel]
	})
}

func (c *ScanCmd) saveResults(sess *session.Session, results []SATokenResult) int {
	saMap := make(map[string]*types.ServiceAccountRecord)

	for _, result := range results {
		if result.Error != "" || result.ServiceAccount == "" {
			continue
		}

		key := fmt.Sprintf("%s/%s", result.TokenInfo.Namespace, result.ServiceAccount)

		if existing, ok := saMap[key]; ok {
			var pods []types.SAPodInfo
			if existing.Pods != "" {
				_ = json.Unmarshal([]byte(existing.Pods), &pods)
			}
			pods = append(pods, types.SAPodInfo{
				Name:      result.PodName,
				Namespace: result.Namespace,
				Container: result.Container,
			})
			podsJSON, _ := json.Marshal(pods)
			existing.Pods = string(podsJSON)
		} else {
			record := &types.ServiceAccountRecord{
				Name:           result.ServiceAccount,
				Namespace:      result.TokenInfo.Namespace,
				Token:          result.Token,
				IsClusterAdmin: result.IsClusterAdmin,
				CollectedAt:    time.Now(),
				KubeletIP:      sess.Config.KubeletIP,
			}

			if result.TokenInfo != nil && !result.TokenInfo.Expiration.IsZero() {
				record.TokenExpiration = result.TokenInfo.Expiration.Format(time.RFC3339)
				record.IsExpired = result.TokenInfo.IsExpired
			}

			if result.IsClusterAdmin {
				record.RiskLevel = string(config.RiskAdmin)
			} else {
				record.RiskLevel = string(result.RiskLevel)
			}

			var permissions []types.SAPermission
			for _, p := range result.Permissions {
				if p.Allowed {
					permissions = append(permissions, types.SAPermission{
						Resource:    p.Resource,
						Verb:        p.Verb,
						Group:       p.Group,
						Subresource: p.Subresource,
						Allowed:     p.Allowed,
					})
				}
			}
			permJSON, _ := json.Marshal(permissions)
			record.Permissions = string(permJSON)

			secFlags := types.SASecurityFlags{
				Privileged:               result.SecurityFlags.Privileged,
				AllowPrivilegeEscalation: result.SecurityFlags.AllowPrivilegeEscalation,
				HasHostPath:              result.SecurityFlags.HasHostPath,
				HasSecretMount:           result.SecurityFlags.HasSecretMount,
				HasSATokenMount:          result.SecurityFlags.HasSATokenMount,
			}
			secFlagsJSON, _ := json.Marshal(secFlags)
			record.SecurityFlags = string(secFlagsJSON)

			pods := []types.SAPodInfo{{
				Name:      result.PodName,
				Namespace: result.Namespace,
				Container: result.Container,
			}}
			podsJSON, _ := json.Marshal(pods)
			record.Pods = string(podsJSON)

			saMap[key] = record
		}
	}

	var records []*types.ServiceAccountRecord
	for _, record := range saMap {
		records = append(records, record)
	}

	if sess.SADB != nil {
		count, _ := sess.SADB.SaveBatch(records)
		return count
	}

	return len(records)
}

func (c *ScanCmd) buildResultRow(p output.Printer, result SATokenResult) output.ScanResultRow {
	var riskLabel string
	if result.IsClusterAdmin {
		riskLabel = p.Colored(config.ColorRed, "ADMIN")
	} else {
		display := config.RiskLevelDisplayConfig[result.RiskLevel]
		riskLabel = p.Colored(display.Color, display.Label)
	}

	tokenStatus := p.Colored(config.ColorGreen, "有效")
	if result.TokenInfo != nil && result.TokenInfo.IsExpired {
		tokenStatus = p.Colored(config.ColorRed, "已过期")
	}

	permissions := c.buildPermissionsString(p, result.Permissions, result.IsClusterAdmin)
	flags := c.buildFlags(p, result)

	return output.ScanResultRow{
		Risk:           riskLabel,
		Namespace:      result.Namespace,
		Pod:            result.PodName,
		ServiceAccount: result.ServiceAccount,
		TokenStatus:    tokenStatus,
		Flags:          flags,
		Permissions:    permissions,
		Token:          result.Token,
	}
}

func (c *ScanCmd) buildFlags(p output.Printer, result SATokenResult) string {
	var flags []string

	if result.SecurityFlags.Privileged {
		flags = append(flags, p.Colored(config.ColorRed, "PRIV"))
	}
	if result.SecurityFlags.AllowPrivilegeEscalation {
		flags = append(flags, p.Colored(config.ColorYellow, "PE"))
	}
	if result.SecurityFlags.HasHostPath {
		flags = append(flags, p.Colored(config.ColorRed, "HP"))
	}
	if result.SecurityFlags.HasSecretMount {
		flags = append(flags, p.Colored(config.ColorYellow, "SEC"))
	}

	for _, perm := range result.Permissions {
		if !perm.Allowed {
			continue
		}
		resource := perm.Resource
		if perm.Subresource != "" {
			resource = perm.Resource + "/" + perm.Subresource
		}
		if config.IsPrivilegeEquivalent(resource, perm.Verb) {
			if !c.containsFlag(flags, "PRIV") {
				flags = append(flags, p.Colored(config.ColorRed, "PRIV"))
			}
			break
		}
	}

	if len(flags) == 0 {
		return "-"
	}
	return strings.Join(flags, ",")
}

func (c *ScanCmd) containsFlag(flags []string, flag string) bool {
	for _, f := range flags {
		if strings.Contains(f, flag) {
			return true
		}
	}
	return false
}

func (c *ScanCmd) buildPermissionsString(p output.Printer, permissions []types.PermissionCheck, isClusterAdmin bool) string {
	if isClusterAdmin {
		return p.Colored(config.ColorRed, "*/* (cluster-admin)")
	}

	var permList []string
	for _, perm := range permissions {
		if !perm.Allowed {
			continue
		}

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

		permList = append(permList, permStr)
	}

	if len(permList) == 0 {
		return "-"
	}
	return strings.Join(permList, "\n")
}

// 确保 k8sclient.Client 实现了需要的接口
var _ interface {
	CheckCommonPermissions(ctx context.Context, namespace string) ([]types.PermissionCheck, error)
} = (k8sclient.Client)(nil)
