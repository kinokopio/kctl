package k8s

import (
	"context"
	"fmt"
	"net/url"
)

// Pod K8s Pod 资源结构
type Pod struct {
	APIVersion string           `json:"apiVersion"`
	Kind       string           `json:"kind"`
	Metadata   ResourceMetadata `json:"metadata"`
	Spec       PodSpecFull      `json:"spec"`
	Status     PodStatus        `json:"status"`
}

// PodSpecFull Pod 完整规格
type PodSpecFull struct {
	Containers []ContainerSpec `json:"containers"`
	NodeName   string          `json:"nodeName,omitempty"`
}

// ContainerSpec 容器规格
type ContainerSpec struct {
	Name  string `json:"name"`
	Image string `json:"image"`
}

// PodStatus Pod 状态
type PodStatus struct {
	Phase  string `json:"phase"`
	PodIP  string `json:"podIP,omitempty"`
	HostIP string `json:"hostIP,omitempty"`
}

// PodList Pod 列表
type PodList struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Items      []Pod  `json:"items"`
}

// ListPods 列出指定命名空间的 Pod
func (c *k8sClient) ListPods(ctx context.Context, namespace string, labelSelector string) ([]PodInfo, error) {
	baseURL := fmt.Sprintf("%s/api/v1/namespaces/%s/pods", c.apiServer, namespace)
	if namespace == "" {
		baseURL = fmt.Sprintf("%s/api/v1/pods", c.apiServer)
	}

	// 添加标签选择器
	if labelSelector != "" {
		params := url.Values{}
		params.Add("labelSelector", labelSelector)
		baseURL += "?" + params.Encode()
	}

	var podList PodList
	if err := c.doGet(ctx, baseURL, &podList); err != nil {
		return nil, err
	}

	result := make([]PodInfo, len(podList.Items))
	for i, pod := range podList.Items {
		result[i] = convertPodToInfo(&pod)
	}

	return result, nil
}

// GetPod 获取指定的 Pod
func (c *k8sClient) GetPod(ctx context.Context, namespace, name string) (*PodInfo, error) {
	reqURL := fmt.Sprintf("%s/api/v1/namespaces/%s/pods/%s", c.apiServer, namespace, name)

	var pod Pod
	found, err := c.doGetWithNotFound(ctx, reqURL, &pod)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("Pod %s/%s 不存在", namespace, name)
	}

	info := convertPodToInfo(&pod)
	return &info, nil
}

// convertPodToInfo 将 Pod 转换为 PodInfo
func convertPodToInfo(pod *Pod) PodInfo {
	containers := make([]string, len(pod.Spec.Containers))
	for i, c := range pod.Spec.Containers {
		containers[i] = c.Name
	}

	return PodInfo{
		Name:       pod.Metadata.Name,
		Namespace:  pod.Metadata.Namespace,
		Status:     pod.Status.Phase,
		PodIP:      pod.Status.PodIP,
		NodeName:   pod.Spec.NodeName,
		Containers: containers,
	}
}
