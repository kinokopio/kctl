package k8s

import (
	"context"
	"encoding/json"
	"fmt"

	"kctl/config"
	"kctl/pkg/types"
)

// ==================== Webhook 相关 API 结构 ====================

// MutatingWebhookConfigurationList MutatingWebhookConfiguration 列表
type MutatingWebhookConfigurationList struct {
	APIVersion string                         `json:"apiVersion"`
	Kind       string                         `json:"kind"`
	Items      []MutatingWebhookConfiguration `json:"items"`
}

// MutatingWebhookConfiguration MutatingWebhookConfiguration 资源
type MutatingWebhookConfiguration struct {
	APIVersion string           `json:"apiVersion"`
	Kind       string           `json:"kind"`
	Metadata   ResourceMetadata `json:"metadata"`
	Webhooks   []WebhookConfig  `json:"webhooks"`
}

// ValidatingWebhookConfigurationList ValidatingWebhookConfiguration 列表
type ValidatingWebhookConfigurationList struct {
	APIVersion string                           `json:"apiVersion"`
	Kind       string                           `json:"kind"`
	Items      []ValidatingWebhookConfiguration `json:"items"`
}

// ValidatingWebhookConfiguration ValidatingWebhookConfiguration 资源
type ValidatingWebhookConfiguration struct {
	APIVersion string           `json:"apiVersion"`
	Kind       string           `json:"kind"`
	Metadata   ResourceMetadata `json:"metadata"`
	Webhooks   []WebhookConfig  `json:"webhooks"`
}

// WebhookConfig Webhook 配置
type WebhookConfig struct {
	Name                    string              `json:"name"`
	ClientConfig            WebhookClientConfig `json:"clientConfig"`
	Rules                   []WebhookRuleConfig `json:"rules,omitempty"`
	FailurePolicy           string              `json:"failurePolicy,omitempty"`
	MatchPolicy             string              `json:"matchPolicy,omitempty"`
	NamespaceSelector       *LabelSelector      `json:"namespaceSelector,omitempty"`
	ObjectSelector          *LabelSelector      `json:"objectSelector,omitempty"`
	SideEffects             string              `json:"sideEffects,omitempty"`
	AdmissionReviewVersions []string            `json:"admissionReviewVersions,omitempty"`
}

// WebhookClientConfig Webhook 客户端配置
type WebhookClientConfig struct {
	Service  *WebhookServiceRef `json:"service,omitempty"`
	URL      *string            `json:"url,omitempty"`
	CABundle []byte             `json:"caBundle,omitempty"`
}

// WebhookServiceRef Webhook 服务引用
type WebhookServiceRef struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Path      string `json:"path,omitempty"`
	Port      int32  `json:"port,omitempty"`
}

// WebhookRuleConfig Webhook 规则配置
type WebhookRuleConfig struct {
	Operations  []string `json:"operations,omitempty"`
	APIGroups   []string `json:"apiGroups,omitempty"`
	APIVersions []string `json:"apiVersions,omitempty"`
	Resources   []string `json:"resources,omitempty"`
	Scope       string   `json:"scope,omitempty"`
}

// ==================== Webhook API 操作 ====================

// ListMutatingWebhooks 列出所有 MutatingWebhookConfiguration
func (c *k8sClient) ListMutatingWebhooks(ctx context.Context) ([]types.WebhookInfo, error) {
	url := fmt.Sprintf("%s/apis/admissionregistration.k8s.io/v1/mutatingwebhookconfigurations", c.apiServer)

	var list MutatingWebhookConfigurationList
	if err := c.doGet(ctx, url, &list); err != nil {
		return nil, err
	}

	var result []types.WebhookInfo
	for _, item := range list.Items {
		for _, wh := range item.Webhooks {
			info := convertWebhookConfigToInfo(&item.Metadata, &wh, "Mutating")
			result = append(result, info)
		}
	}

	return result, nil
}

// ListValidatingWebhooks 列出所有 ValidatingWebhookConfiguration
func (c *k8sClient) ListValidatingWebhooks(ctx context.Context) ([]types.WebhookInfo, error) {
	url := fmt.Sprintf("%s/apis/admissionregistration.k8s.io/v1/validatingwebhookconfigurations", c.apiServer)

	var list ValidatingWebhookConfigurationList
	if err := c.doGet(ctx, url, &list); err != nil {
		return nil, err
	}

	var result []types.WebhookInfo
	for _, item := range list.Items {
		for _, wh := range item.Webhooks {
			info := convertWebhookConfigToInfo(&item.Metadata, &wh, "Validating")
			result = append(result, info)
		}
	}

	return result, nil
}

// CreateMutatingWebhook 创建 MutatingWebhookConfiguration
func (c *k8sClient) CreateMutatingWebhook(ctx context.Context, webhook []byte) error {
	url := fmt.Sprintf("%s/apis/admissionregistration.k8s.io/v1/mutatingwebhookconfigurations", c.apiServer)
	return c.doPost(ctx, url, webhook)
}

// DeleteMutatingWebhook 删除 MutatingWebhookConfiguration
func (c *k8sClient) DeleteMutatingWebhook(ctx context.Context, name string) error {
	url := fmt.Sprintf("%s/apis/admissionregistration.k8s.io/v1/mutatingwebhookconfigurations/%s", c.apiServer, name)
	return c.doDelete(ctx, url)
}

// ==================== Namespace 操作 ====================

// NamespaceList Namespace 列表
type NamespaceList struct {
	APIVersion string      `json:"apiVersion"`
	Kind       string      `json:"kind"`
	Items      []Namespace `json:"items"`
}

// Namespace Namespace 资源
type Namespace struct {
	APIVersion string           `json:"apiVersion"`
	Kind       string           `json:"kind"`
	Metadata   ResourceMetadata `json:"metadata"`
}

// ListNamespaces 列出所有命名空间
func (c *k8sClient) ListNamespaces(ctx context.Context) ([]string, error) {
	url := fmt.Sprintf("%s/api/v1/namespaces", c.apiServer)

	var list NamespaceList
	if err := c.doGet(ctx, url, &list); err != nil {
		return nil, err
	}

	result := make([]string, len(list.Items))
	for i, ns := range list.Items {
		result[i] = ns.Metadata.Name
	}

	return result, nil
}

// CreateNamespace 创建命名空间
func (c *k8sClient) CreateNamespace(ctx context.Context, name string, labels map[string]string) error {
	ns := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata": map[string]interface{}{
			"name":   name,
			"labels": labels,
		},
	}

	body, err := jsonMarshal(ns)
	if err != nil {
		return fmt.Errorf("序列化 Namespace 失败: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/namespaces", c.apiServer)
	return c.doPost(ctx, url, body)
}

// DeleteNamespace 删除命名空间
func (c *k8sClient) DeleteNamespace(ctx context.Context, name string) error {
	url := fmt.Sprintf("%s/api/v1/namespaces/%s", c.apiServer, name)
	return c.doDelete(ctx, url)
}

// ==================== Secret 操作 ====================

// CreateSecret 创建 Secret
func (c *k8sClient) CreateSecret(ctx context.Context, namespace string, secret []byte) error {
	url := fmt.Sprintf("%s/api/v1/namespaces/%s/secrets", c.apiServer, namespace)
	return c.doPost(ctx, url, secret)
}

// DeleteSecret 删除 Secret
func (c *k8sClient) DeleteSecret(ctx context.Context, namespace, name string) error {
	url := fmt.Sprintf("%s/api/v1/namespaces/%s/secrets/%s", c.apiServer, namespace, name)
	return c.doDelete(ctx, url)
}

// ==================== ConfigMap 操作 ====================

// CreateConfigMap 创建 ConfigMap
func (c *k8sClient) CreateConfigMap(ctx context.Context, namespace string, configMap []byte) error {
	url := fmt.Sprintf("%s/api/v1/namespaces/%s/configmaps", c.apiServer, namespace)
	return c.doPost(ctx, url, configMap)
}

// DeleteConfigMap 删除 ConfigMap
func (c *k8sClient) DeleteConfigMap(ctx context.Context, namespace, name string) error {
	url := fmt.Sprintf("%s/api/v1/namespaces/%s/configmaps/%s", c.apiServer, namespace, name)
	return c.doDelete(ctx, url)
}

// ==================== Service 操作 ====================

// CreateService 创建 Service
func (c *k8sClient) CreateService(ctx context.Context, namespace string, service []byte) error {
	url := fmt.Sprintf("%s/api/v1/namespaces/%s/services", c.apiServer, namespace)
	return c.doPost(ctx, url, service)
}

// DeleteService 删除 Service
func (c *k8sClient) DeleteService(ctx context.Context, namespace, name string) error {
	url := fmt.Sprintf("%s/api/v1/namespaces/%s/services/%s", c.apiServer, namespace, name)
	return c.doDelete(ctx, url)
}

// ==================== Deployment 扩展操作 ====================

// CreateDeployment 创建 Deployment
func (c *k8sClient) CreateDeployment(ctx context.Context, namespace string, deployment []byte) error {
	url := fmt.Sprintf("%s/apis/apps/v1/namespaces/%s/deployments", c.apiServer, namespace)
	return c.doPost(ctx, url, deployment)
}

// DeleteDeployment 删除 Deployment
func (c *k8sClient) DeleteDeployment(ctx context.Context, namespace, name string) error {
	url := fmt.Sprintf("%s/apis/apps/v1/namespaces/%s/deployments/%s", c.apiServer, namespace, name)
	return c.doDelete(ctx, url)
}

// ==================== 辅助函数 ====================

// convertWebhookConfigToInfo 将 WebhookConfig 转换为 WebhookInfo
func convertWebhookConfigToInfo(metadata *ResourceMetadata, wh *WebhookConfig, webhookType string) types.WebhookInfo {
	info := types.WebhookInfo{
		Name:          metadata.Name,
		Type:          webhookType,
		FailurePolicy: wh.FailurePolicy,
		Labels:        metadata.Labels,
	}

	// 检查是否由 kctl 管理
	if metadata.Labels != nil {
		if metadata.Labels[config.WebhookManagedByLabel] == config.WebhookManagedByValue {
			info.IsManagedByKctl = true
		}
	}

	// 提取服务信息
	if wh.ClientConfig.Service != nil {
		info.ServiceNS = wh.ClientConfig.Service.Namespace
		info.ServiceName = wh.ClientConfig.Service.Name
		info.ServicePort = wh.ClientConfig.Service.Port
		info.Path = wh.ClientConfig.Service.Path
	}

	// 转换规则
	for _, rule := range wh.Rules {
		info.Rules = append(info.Rules, types.WebhookRule{
			Operations: rule.Operations,
			APIGroups:  rule.APIGroups,
			Resources:  rule.Resources,
		})
	}

	return info
}

// jsonMarshal JSON 序列化辅助函数
func jsonMarshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}
