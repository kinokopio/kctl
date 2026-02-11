package k8s

import (
	"context"
	"encoding/json"
	"fmt"

	"kctl/pkg/types"
)

// DaemonSet K8s DaemonSet 资源结构
type DaemonSet struct {
	APIVersion string           `json:"apiVersion"`
	Kind       string           `json:"kind"`
	Metadata   ResourceMetadata `json:"metadata"`
	Spec       DaemonSetSpec    `json:"spec"`
	Status     DaemonSetStatus  `json:"status"`
}

// ResourceMetadata 通用资源元数据
type ResourceMetadata struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// DaemonSetSpec DaemonSet 规格
type DaemonSetSpec struct {
	Selector *LabelSelector  `json:"selector,omitempty"`
	Template PodTemplateSpec `json:"template"`
}

// LabelSelector 标签选择器
type LabelSelector struct {
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
}

// PodTemplateSpec Pod 模板规格
type PodTemplateSpec struct {
	Metadata PodTemplateMetadata `json:"metadata,omitempty"`
	Spec     PodSpec             `json:"spec"`
}

// PodTemplateMetadata Pod 模板元数据
type PodTemplateMetadata struct {
	Labels map[string]string `json:"labels,omitempty"`
}

// PodSpec Pod 规格
type PodSpec struct {
	Containers []Container `json:"containers"`
}

// Container 容器定义
type Container struct {
	Name           string `json:"name"`
	Image          string `json:"image"`
	LivenessProbe  *Probe `json:"livenessProbe,omitempty"`
	ReadinessProbe *Probe `json:"readinessProbe,omitempty"`
	StartupProbe   *Probe `json:"startupProbe,omitempty"`
}

// Probe 探针定义
type Probe struct {
	Exec                *ExecAction      `json:"exec,omitempty"`
	HTTPGet             *HTTPGetAction   `json:"httpGet,omitempty"`
	TCPSocket           *TCPSocketAction `json:"tcpSocket,omitempty"`
	InitialDelaySeconds int32            `json:"initialDelaySeconds,omitempty"`
	PeriodSeconds       int32            `json:"periodSeconds,omitempty"`
	TimeoutSeconds      int32            `json:"timeoutSeconds,omitempty"`
	SuccessThreshold    int32            `json:"successThreshold,omitempty"`
	FailureThreshold    int32            `json:"failureThreshold,omitempty"`
}

// ExecAction exec 探针动作
type ExecAction struct {
	Command []string `json:"command"`
}

// HTTPGetAction HTTP GET 探针动作
type HTTPGetAction struct {
	Path   string      `json:"path,omitempty"`
	Port   IntOrString `json:"port"`
	Scheme string      `json:"scheme,omitempty"`
}

// TCPSocketAction TCP Socket 探针动作
type TCPSocketAction struct {
	Port IntOrString `json:"port"`
}

// IntOrString 可以是整数或字符串的类型（用于 Kubernetes API 兼容）
type IntOrString struct {
	IntVal int
	StrVal string
	IsStr  bool
}

// UnmarshalJSON 实现 json.Unmarshaler 接口
func (i *IntOrString) UnmarshalJSON(data []byte) error {
	// 尝试解析为整数
	var intVal int
	if err := json.Unmarshal(data, &intVal); err == nil {
		i.IntVal = intVal
		i.IsStr = false
		return nil
	}

	// 尝试解析为字符串
	var strVal string
	if err := json.Unmarshal(data, &strVal); err == nil {
		i.StrVal = strVal
		i.IsStr = true
		return nil
	}

	return fmt.Errorf("IntOrString: cannot unmarshal %s", string(data))
}

// MarshalJSON 实现 json.Marshaler 接口
func (i IntOrString) MarshalJSON() ([]byte, error) {
	if i.IsStr {
		return json.Marshal(i.StrVal)
	}
	return json.Marshal(i.IntVal)
}

// Int 返回整数值
func (i IntOrString) Int() int {
	return i.IntVal
}

// String 返回字符串值
func (i IntOrString) String() string {
	if i.IsStr {
		return i.StrVal
	}
	return fmt.Sprintf("%d", i.IntVal)
}

// DaemonSetStatus DaemonSet 状态
type DaemonSetStatus struct {
	DesiredNumberScheduled int32 `json:"desiredNumberScheduled"`
	NumberReady            int32 `json:"numberReady"`
	CurrentNumberScheduled int32 `json:"currentNumberScheduled"`
}

// DaemonSetList DaemonSet 列表
type DaemonSetList struct {
	APIVersion string      `json:"apiVersion"`
	Kind       string      `json:"kind"`
	Items      []DaemonSet `json:"items"`
}

// ListDaemonSets 列出指定命名空间的 DaemonSet
func (c *k8sClient) ListDaemonSets(ctx context.Context, namespace string) ([]types.DaemonSetInfo, error) {
	url := fmt.Sprintf("%s/apis/apps/v1/namespaces/%s/daemonsets", c.apiServer, namespace)
	if namespace == "" {
		url = fmt.Sprintf("%s/apis/apps/v1/daemonsets", c.apiServer)
	}

	var dsList DaemonSetList
	if err := c.doGet(ctx, url, &dsList); err != nil {
		return nil, err
	}

	result := make([]types.DaemonSetInfo, len(dsList.Items))
	for i, ds := range dsList.Items {
		result[i] = convertDaemonSetToInfo(&ds)
	}

	return result, nil
}

// GetDaemonSet 获取指定的 DaemonSet
func (c *k8sClient) GetDaemonSet(ctx context.Context, namespace, name string) (*types.DaemonSetInfo, error) {
	url := fmt.Sprintf("%s/apis/apps/v1/namespaces/%s/daemonsets/%s", c.apiServer, namespace, name)

	var ds DaemonSet
	found, err := c.doGetWithNotFound(ctx, url, &ds)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("DaemonSet %s/%s 不存在", namespace, name)
	}

	info := convertDaemonSetToInfo(&ds)
	return &info, nil
}

// PatchDaemonSet 使用 Strategic Merge Patch 更新 DaemonSet
func (c *k8sClient) PatchDaemonSet(ctx context.Context, namespace, name string, patch []byte) error {
	url := fmt.Sprintf("%s/apis/apps/v1/namespaces/%s/daemonsets/%s", c.apiServer, namespace, name)
	return c.doPatch(ctx, url, patch)
}

// convertDaemonSetToInfo 将 DaemonSet 转换为 DaemonSetInfo
func convertDaemonSetToInfo(ds *DaemonSet) types.DaemonSetInfo {
	info := types.DaemonSetInfo{
		Name:            ds.Metadata.Name,
		Namespace:       ds.Metadata.Namespace,
		DesiredReplicas: ds.Status.DesiredNumberScheduled,
		ReadyReplicas:   ds.Status.NumberReady,
		Labels:          ds.Metadata.Labels,
		Containers:      make([]types.BasicContainerInfo, len(ds.Spec.Template.Spec.Containers)),
	}

	for i, container := range ds.Spec.Template.Spec.Containers {
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
