package token

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"kctl/config"
	"kctl/pkg/types"
)

// Read 从指定路径读取 Token
func Read(path string) (string, error) {
	if path == "" {
		path = config.DefaultTokenPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("读取 Token 文件失败: %w", err)
	}

	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", fmt.Errorf("token 文件为空")
	}

	return token, nil
}

// Parse 解析 JWT Token 获取基本信息
func Parse(token string) (*types.TokenInfo, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("无效的 JWT Token 格式")
	}

	// 解码 payload（第二部分）
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// 尝试标准 base64 解码
		payload, err = base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return nil, fmt.Errorf("解码 Token payload 失败: %w", err)
		}
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("解析 Token claims 失败: %w", err)
	}

	info := &types.TokenInfo{}

	// 提取 issuer
	if iss, ok := claims["iss"].(string); ok {
		info.Issuer = iss
	}

	// 提取过期时间
	if exp, ok := claims["exp"].(float64); ok {
		info.Expiration = time.Unix(int64(exp), 0)
		info.IsExpired = time.Now().After(info.Expiration)
	}

	// 提取 Kubernetes ServiceAccount 信息
	// 格式可能是 kubernetes.io 的标准格式
	if k8s, ok := claims["kubernetes.io"].(map[string]interface{}); ok {
		if ns, ok := k8s["namespace"].(string); ok {
			info.Namespace = ns
		}
		if sa, ok := k8s["serviceaccount"].(map[string]interface{}); ok {
			if name, ok := sa["name"].(string); ok {
				info.ServiceAccount = name
			}
		}
	}

	// 备用：从 sub 字段提取
	if info.ServiceAccount == "" {
		if sub, ok := claims["sub"].(string); ok {
			// 格式: system:serviceaccount:namespace:name
			parts := strings.Split(sub, ":")
			if len(parts) >= 4 && parts[0] == "system" && parts[1] == "serviceaccount" {
				info.Namespace = parts[2]
				info.ServiceAccount = parts[3]
			}
		}
	}

	return info, nil
}

// Truncate 截断 Token 用于显示
func Truncate(token string, maxLen int) string {
	if len(token) <= maxLen {
		return token
	}
	return token[:maxLen] + "..."
}

// GetDefaultPath 返回默认的 Token 文件路径
func GetDefaultPath() string {
	return config.DefaultTokenPath
}
