package persist

import (
	"context"
	"fmt"
	"strings"

	"kctl/internal/client/k8s"
	"kctl/pkg/types"
)

// ToolsToDetect 需要检测的工具列表
var ToolsToDetect = []string{
	// Shells
	"sh", "bash", "ash", "zsh",
	// Network tools
	"curl", "wget", "nc", "ncat",
	// Script languages
	"python3", "python", "perl", "ruby", "node",
	// Utilities
	"base64",
}

// DetectTools 检测容器中可用的工具
// 通过在 Pod 中执行 `which <tool>` 命令来检测
func DetectTools(ctx context.Context, client k8s.Client, namespace, podName, container string) ([]types.ToolDetectionResult, error) {
	results := make([]types.ToolDetectionResult, len(ToolsToDetect))

	for i, tool := range ToolsToDetect {
		results[i] = types.ToolDetectionResult{
			Tool:      tool,
			Available: false,
		}

		// 执行 which 命令
		execResult, err := client.ExecInPod(ctx, namespace, podName, container, []string{"which", tool})
		if err != nil {
			// 执行失败，工具不可用
			continue
		}

		// 检查退出码和输出
		if execResult.ExitCode == 0 && execResult.Stdout != "" {
			results[i].Available = true
			results[i].Path = strings.TrimSpace(execResult.Stdout)
		}
	}

	return results, nil
}

// DetectToolsQuick 快速检测工具（只检测关键工具）
func DetectToolsQuick(ctx context.Context, client k8s.Client, namespace, podName, container string) ([]types.ToolDetectionResult, error) {
	// 关键工具列表
	criticalTools := []string{"sh", "bash", "base64", "curl", "wget", "nc", "python3", "python", "perl"}

	results := make([]types.ToolDetectionResult, 0, len(criticalTools))

	for _, tool := range criticalTools {
		result := types.ToolDetectionResult{
			Tool:      tool,
			Available: false,
		}

		// 尝试多种检测方式
		// 方式1: 使用 which
		execResult, err := client.ExecInPod(ctx, namespace, podName, container, []string{"which", tool})
		if err == nil && execResult.ExitCode == 0 && execResult.Stdout != "" {
			result.Available = true
			result.Path = strings.TrimSpace(execResult.Stdout)
			results = append(results, result)
			continue
		}

		// 方式2: 使用 command -v (POSIX 兼容)
		execResult, err = client.ExecInPod(ctx, namespace, podName, container, []string{"sh", "-c", "command -v " + tool})
		if err == nil && execResult.ExitCode == 0 && execResult.Stdout != "" {
			result.Available = true
			result.Path = strings.TrimSpace(execResult.Stdout)
			results = append(results, result)
			continue
		}

		// 方式3: 直接尝试执行 --version 或 --help
		if tool == "sh" || tool == "bash" || tool == "ash" {
			execResult, err = client.ExecInPod(ctx, namespace, podName, container, []string{tool, "-c", "echo ok"})
			if err == nil && execResult.ExitCode == 0 {
				result.Available = true
				result.Path = "/bin/" + tool
				results = append(results, result)
				continue
			}
		}

		results = append(results, result)
	}

	return results, nil
}

// GetAvailableTools 获取可用工具列表
func GetAvailableTools(results []types.ToolDetectionResult) []string {
	var available []string
	for _, r := range results {
		if r.Available {
			available = append(available, r.Tool)
		}
	}
	return available
}

// HasTool 检查是否有指定工具
func HasTool(results []types.ToolDetectionResult, tool string) bool {
	for _, r := range results {
		if r.Tool == tool && r.Available {
			return true
		}
	}
	return false
}

// HasAnyTool 检查是否有任意一个指定工具
func HasAnyTool(results []types.ToolDetectionResult, tools []string) bool {
	for _, tool := range tools {
		if HasTool(results, tool) {
			return true
		}
	}
	return false
}

// HasShell 检查是否有可用的 shell
func HasShell(results []types.ToolDetectionResult) bool {
	return HasAnyTool(results, []string{"sh", "bash", "ash", "zsh"})
}

// HasBase64 检查是否有 base64 工具
func HasBase64(results []types.ToolDetectionResult) bool {
	return HasTool(results, "base64")
}

// GetPreferredShell 获取首选 shell
func GetPreferredShell(results []types.ToolDetectionResult) string {
	// 优先级: bash > sh > ash > zsh
	shells := []string{"bash", "sh", "ash", "zsh"}
	for _, shell := range shells {
		if HasTool(results, shell) {
			return shell
		}
	}
	return ""
}

// ValidateToolsForPayload 验证载荷所需的工具是否可用
func ValidateToolsForPayload(results []types.ToolDetectionResult, requiredTools []string) error {
	missing := []string{}
	for _, tool := range requiredTools {
		if !HasTool(results, tool) {
			missing = append(missing, tool)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("缺少必需的工具: %s", strings.Join(missing, ", "))
	}

	return nil
}

// FormatToolDetectionResults 格式化工具检测结果
func FormatToolDetectionResults(results []types.ToolDetectionResult) string {
	var sb strings.Builder
	sb.WriteString("工具检测结果:\n")

	for _, r := range results {
		status := "✗"
		if r.Available {
			status = "✓"
		}
		if r.Path != "" {
			sb.WriteString(fmt.Sprintf("  [%s] %s (%s)\n", status, r.Tool, r.Path))
		} else {
			sb.WriteString(fmt.Sprintf("  [%s] %s\n", status, r.Tool))
		}
	}

	return sb.String()
}
