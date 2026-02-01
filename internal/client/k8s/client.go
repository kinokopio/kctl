package k8s

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"kctl/config"
	"kctl/internal/client"
	"kctl/pkg/types"
)

// Client K8s API Server 客户端接口
type Client interface {
	// RBAC 权限检查
	CheckPermission(ctx context.Context, req *PermissionRequest) (bool, error)
	CheckPermissions(ctx context.Context, reqs []PermissionRequest) ([]types.PermissionCheck, error)
	CheckCommonPermissions(ctx context.Context, namespace string) ([]types.PermissionCheck, error)
}

// PermissionRequest 权限检查请求
type PermissionRequest struct {
	Resource    string
	Verb        string
	Namespace   string
	Group       string
	Subresource string
}

// k8sClient K8s API 客户端实现
type k8sClient struct {
	apiServer  string
	token      string
	httpClient *http.Client
	config     *client.Config
}

// NewClient 创建 K8s API 客户端
func NewClient(apiServer, token string, cfg *client.Config) (Client, error) {
	if cfg == nil {
		cfg = client.DefaultConfig()
	}

	if apiServer == "" {
		apiServer = config.DefaultK8sAPIServer
	}

	httpClient, err := client.NewHTTPClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("创建 HTTP 客户端失败: %w", err)
	}

	return &k8sClient{
		apiServer:  apiServer,
		token:      token,
		httpClient: httpClient,
		config:     cfg,
	}, nil
}

// SelfSubjectAccessReviewRequest 请求结构
type SelfSubjectAccessReviewRequest struct {
	APIVersion string                  `json:"apiVersion"`
	Kind       string                  `json:"kind"`
	Spec       AccessReviewRequestSpec `json:"spec"`
}

type AccessReviewRequestSpec struct {
	ResourceAttributes *ResourceAttributes `json:"resourceAttributes,omitempty"`
}

type ResourceAttributes struct {
	Namespace   string `json:"namespace,omitempty"`
	Verb        string `json:"verb"`
	Group       string `json:"group,omitempty"`
	Resource    string `json:"resource"`
	Subresource string `json:"subresource,omitempty"`
}

// SelfSubjectAccessReviewResponse 响应结构
type SelfSubjectAccessReviewResponse struct {
	Status AccessReviewStatus `json:"status"`
}

type AccessReviewStatus struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

// CheckPermission 检查单个权限
func (c *k8sClient) CheckPermission(ctx context.Context, req *PermissionRequest) (bool, error) {
	reviewReq := &SelfSubjectAccessReviewRequest{
		APIVersion: "authorization.k8s.io/v1",
		Kind:       "SelfSubjectAccessReview",
		Spec: AccessReviewRequestSpec{
			ResourceAttributes: &ResourceAttributes{
				Namespace:   req.Namespace,
				Verb:        req.Verb,
				Group:       req.Group,
				Resource:    req.Resource,
				Subresource: req.Subresource,
			},
		},
	}

	body, err := json.Marshal(reviewReq)
	if err != nil {
		return false, fmt.Errorf("序列化请求失败: %w", err)
	}

	url := c.apiServer + "/apis/authorization.k8s.io/v1/selfsubjectaccessreviews"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return false, fmt.Errorf("创建请求失败: %w", err)
	}

	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return false, fmt.Errorf("请求 K8s API Server 失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("K8s API Server 返回错误状态: %d", resp.StatusCode)
	}

	var response SelfSubjectAccessReviewResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return false, fmt.Errorf("解析响应失败: %w", err)
	}

	return response.Status.Allowed, nil
}

// CheckPermissions 批量检查权限
func (c *k8sClient) CheckPermissions(ctx context.Context, reqs []PermissionRequest) ([]types.PermissionCheck, error) {
	results := make([]types.PermissionCheck, len(reqs))

	for i, req := range reqs {
		allowed, err := c.CheckPermission(ctx, &req)
		results[i] = types.PermissionCheck{
			Resource:    req.Resource,
			Verb:        req.Verb,
			Group:       req.Group,
			Subresource: req.Subresource,
			Allowed:     allowed,
		}
		if err != nil {
			// 记录错误但继续检查其他权限
			results[i].Allowed = false
		}
	}

	return results, nil
}

// CheckCommonPermissions 检查常用资源的权限
func (c *k8sClient) CheckCommonPermissions(ctx context.Context, namespace string) ([]types.PermissionCheck, error) {
	var reqs []PermissionRequest

	for _, perm := range config.PermissionsToCheck {
		reqs = append(reqs, PermissionRequest{
			Resource:    perm.Resource,
			Verb:        perm.Verb,
			Group:       perm.Group,
			Subresource: perm.Subresource,
			Namespace:   namespace,
		})
	}

	return c.CheckPermissions(ctx, reqs)
}
