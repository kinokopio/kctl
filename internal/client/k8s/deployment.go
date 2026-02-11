package k8s

import (
	"context"
	"fmt"

	"kctl/pkg/types"
)

// Deployment K8s Deployment 资源结构
type Deployment struct {
	APIVersion string           `json:"apiVersion"`
	Kind       string           `json:"kind"`
	Metadata   ResourceMetadata `json:"metadata"`
	Spec       DeploymentSpec   `json:"spec"`
	Status     DeploymentStatus `json:"status"`
}

// DeploymentSpec Deployment 规格
type DeploymentSpec struct {
	Replicas int32           `json:"replicas,omitempty"`
	Selector *LabelSelector  `json:"selector,omitempty"`
	Template PodTemplateSpec `json:"template"`
}

// DeploymentStatus Deployment 状态
type DeploymentStatus struct {
	Replicas          int32 `json:"replicas"`
	ReadyReplicas     int32 `json:"readyReplicas"`
	AvailableReplicas int32 `json:"availableReplicas"`
}

// DeploymentList Deployment 列表
type DeploymentList struct {
	APIVersion string       `json:"apiVersion"`
	Kind       string       `json:"kind"`
	Items      []Deployment `json:"items"`
}

// ListDeployments 列出指定命名空间的 Deployment
func (c *k8sClient) ListDeployments(ctx context.Context, namespace string) ([]types.DeploymentInfo, error) {
	url := fmt.Sprintf("%s/apis/apps/v1/namespaces/%s/deployments", c.apiServer, namespace)
	if namespace == "" {
		url = fmt.Sprintf("%s/apis/apps/v1/deployments", c.apiServer)
	}

	var deployList DeploymentList
	if err := c.doGet(ctx, url, &deployList); err != nil {
		return nil, err
	}

	result := make([]types.DeploymentInfo, len(deployList.Items))
	for i, deploy := range deployList.Items {
		result[i] = convertDeploymentToInfo(&deploy)
	}

	return result, nil
}

// GetDeployment 获取指定的 Deployment
func (c *k8sClient) GetDeployment(ctx context.Context, namespace, name string) (*types.DeploymentInfo, error) {
	url := fmt.Sprintf("%s/apis/apps/v1/namespaces/%s/deployments/%s", c.apiServer, namespace, name)

	var deploy Deployment
	found, err := c.doGetWithNotFound(ctx, url, &deploy)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("Deployment %s/%s 不存在", namespace, name)
	}

	info := convertDeploymentToInfo(&deploy)
	return &info, nil
}

// PatchDeployment 使用 Strategic Merge Patch 更新 Deployment
func (c *k8sClient) PatchDeployment(ctx context.Context, namespace, name string, patch []byte) error {
	url := fmt.Sprintf("%s/apis/apps/v1/namespaces/%s/deployments/%s", c.apiServer, namespace, name)
	return c.doPatch(ctx, url, patch)
}

// convertDeploymentToInfo 将 Deployment 转换为 DeploymentInfo
func convertDeploymentToInfo(deploy *Deployment) types.DeploymentInfo {
	info := types.DeploymentInfo{
		Name:          deploy.Metadata.Name,
		Namespace:     deploy.Metadata.Namespace,
		Replicas:      deploy.Spec.Replicas,
		ReadyReplicas: deploy.Status.ReadyReplicas,
		Labels:        deploy.Metadata.Labels,
		Containers:    make([]types.BasicContainerInfo, len(deploy.Spec.Template.Spec.Containers)),
	}

	if deploy.Spec.Selector != nil {
		info.Selector = deploy.Spec.Selector.MatchLabels
	}

	for i, container := range deploy.Spec.Template.Spec.Containers {
		info.Containers[i] = types.BasicContainerInfo{
			Name:  container.Name,
			Image: container.Image,
		}

		// 检查探针
		if container.LivenessProbe != nil {
			info.HasLivenessProbe = true
		}
		if container.ReadinessProbe != nil {
			info.HasReadinessProbe = true
		}
		if container.StartupProbe != nil {
			info.HasStartupProbe = true
		}
	}

	return info
}
