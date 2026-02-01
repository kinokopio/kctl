package rbac

import (
	"context"

	"kctl/config"
	"kctl/internal/client/k8s"
	"kctl/pkg/types"
)

// Checker 权限检查器
type Checker struct {
	client k8s.Client
}

// NewChecker 创建权限检查器
func NewChecker(client k8s.Client) *Checker {
	return &Checker{client: client}
}

// CheckAll 检查所有常用权限
func (c *Checker) CheckAll(ctx context.Context, namespace string) ([]types.PermissionCheckResult, error) {
	permissions, err := c.client.CheckCommonPermissions(ctx, namespace)
	if err != nil {
		return nil, err
	}

	var results []types.PermissionCheckResult
	for _, p := range permissions {
		result := types.PermissionCheckResult{
			PermissionCheck: p,
		}

		if p.Allowed {
			result.Level, result.Description = GetPermissionInfo(p)
		}

		results = append(results, result)
	}

	return results, nil
}

// GetPermissionInfo 获取权限的敏感级别和描述
func GetPermissionInfo(p types.PermissionCheck) (config.PermissionLevel, string) {
	for _, rule := range config.PermissionRiskRules {
		if matchRule(p, rule) {
			return rule.Level, rule.Description
		}
	}
	return config.PermLevelNormal, ""
}

// matchRule 检查权限是否匹配规则
func matchRule(p types.PermissionCheck, rule config.PermissionRiskRule) bool {
	// 资源匹配
	if rule.Resource != "*" && rule.Resource != p.Resource {
		return false
	}

	// 操作匹配
	if rule.Verb != "*" && rule.Verb != p.Verb {
		return false
	}

	// API Group 匹配
	if rule.Group != "*" && rule.Group != p.Group {
		return false
	}

	// 子资源匹配
	if rule.Subresource != "*" && rule.Subresource != p.Subresource {
		return false
	}

	return true
}

// GetLevelName 获取级别名称
func GetLevelName(level config.PermissionLevel) string {
	if name, ok := config.PermissionLevelNames[level]; ok {
		return name
	}
	return "普通"
}
