package rbac

import (
	"kctl/config"
	"kctl/pkg/types"
)

// RiskAssessment 风险评估结果
type RiskAssessment struct {
	Level          config.RiskLevel
	IsClusterAdmin bool
	AdminPerms     []types.PermissionCheckResult
	DangerousPerms []types.PermissionCheckResult
	SensitivePerms []types.PermissionCheckResult
	NormalPerms    []types.PermissionCheckResult
}

// AssessRisk 评估权限风险
func AssessRisk(results []types.PermissionCheckResult) *RiskAssessment {
	assessment := &RiskAssessment{
		Level: config.RiskNone,
	}

	for _, r := range results {
		if !r.Allowed {
			continue
		}

		switch r.Level {
		case config.PermLevelAdmin:
			assessment.AdminPerms = append(assessment.AdminPerms, r)
			// 检查是否是 cluster-admin
			if r.Resource == "*" && r.Verb == "*" {
				assessment.IsClusterAdmin = true
			}
		case config.PermLevelDangerous:
			assessment.DangerousPerms = append(assessment.DangerousPerms, r)
		case config.PermLevelSensitive:
			assessment.SensitivePerms = append(assessment.SensitivePerms, r)
		default:
			assessment.NormalPerms = append(assessment.NormalPerms, r)
		}
	}

	// 计算风险等级
	if assessment.IsClusterAdmin {
		assessment.Level = config.RiskAdmin
	} else if len(assessment.AdminPerms) > 0 {
		assessment.Level = config.RiskCritical
	} else if len(assessment.DangerousPerms) > 0 {
		assessment.Level = config.RiskHigh
	} else if len(assessment.SensitivePerms) > 0 {
		assessment.Level = config.RiskMedium
	} else if len(assessment.NormalPerms) > 0 {
		assessment.Level = config.RiskLow
	}

	return assessment
}

// AssessRiskFromPermissions 从权限检查结果评估风险（简化版）
func AssessRiskFromPermissions(permissions []types.PermissionCheck) *RiskAssessment {
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

	return AssessRisk(results)
}

// CalculateRiskLevel 计算权限的风险等级（快速版本）
func CalculateRiskLevel(permissions []types.PermissionCheck) config.RiskLevel {
	// 检查是否是集群管理员
	for _, p := range permissions {
		if p.Allowed && p.Resource == "*" && p.Verb == "*" {
			return config.RiskAdmin
		}
	}

	// 检查 CRITICAL 权限
	for _, p := range permissions {
		if !p.Allowed {
			continue
		}

		resource := p.Resource
		if p.Subresource != "" {
			resource = p.Resource + "/" + p.Subresource
		}

		if verbs, ok := config.CriticalPermissions[resource]; ok {
			for _, v := range verbs {
				if v == p.Verb || v == "*" {
					return config.RiskCritical
				}
			}
		}
		// 通配符资源
		if p.Resource == "*" {
			return config.RiskCritical
		}
	}

	// 检查 HIGH 权限
	for _, p := range permissions {
		if !p.Allowed {
			continue
		}

		resource := p.Resource
		if p.Subresource != "" {
			resource = p.Resource + "/" + p.Subresource
		}

		if verbs, ok := config.HighPermissions[resource]; ok {
			for _, v := range verbs {
				if v == p.Verb || v == "*" {
					return config.RiskHigh
				}
			}
		}
	}

	// 检查 MEDIUM 权限
	for _, p := range permissions {
		if !p.Allowed {
			continue
		}

		resource := p.Resource
		if p.Subresource != "" {
			resource = p.Resource + "/" + p.Subresource
		}

		if verbs, ok := config.MediumPermissions[resource]; ok {
			for _, v := range verbs {
				if v == p.Verb || v == "*" {
					return config.RiskMedium
				}
			}
		}
	}

	// 检查是否有任何允许的权限
	for _, p := range permissions {
		if p.Allowed {
			return config.RiskLow
		}
	}

	return config.RiskNone
}

// IsClusterAdmin 检查是否拥有集群管理员权限
func IsClusterAdmin(permissions []types.PermissionCheck) bool {
	for _, p := range permissions {
		if p.Allowed && p.Resource == "*" && p.Verb == "*" {
			return true
		}
	}
	return false
}
