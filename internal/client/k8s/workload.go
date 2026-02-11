package k8s

import (
	"context"
	"fmt"

	"kctl/pkg/types"
)

// ListWorkloadsWithProbes 列出所有带探针的工作负载
func (c *k8sClient) ListWorkloadsWithProbes(ctx context.Context, namespace string) ([]types.WorkloadInfo, error) {
	var results []types.WorkloadInfo

	// 获取 DaemonSets
	daemonSets, err := c.listDaemonSetsWithProbes(ctx, namespace)
	if err == nil {
		results = append(results, daemonSets...)
	}

	// 获取 Deployments
	deployments, err := c.listDeploymentsWithProbes(ctx, namespace)
	if err == nil {
		results = append(results, deployments...)
	}

	return results, nil
}

// listDaemonSetsWithProbes 列出带探针信息的 DaemonSets
func (c *k8sClient) listDaemonSetsWithProbes(ctx context.Context, namespace string) ([]types.WorkloadInfo, error) {
	dsList, err := c.ListDaemonSets(ctx, namespace)
	if err != nil {
		return nil, err
	}

	var results []types.WorkloadInfo
	for _, ds := range dsList {
		// 获取完整的 DaemonSet 信息
		fullDS, err := c.getDaemonSetRaw(ctx, ds.Namespace, ds.Name)
		if err != nil {
			continue
		}

		workload := types.WorkloadInfo{
			Type:          types.WorkloadTypeDaemonSet,
			Name:          ds.Name,
			Namespace:     ds.Namespace,
			Replicas:      ds.DesiredReplicas,
			ReadyReplicas: ds.ReadyReplicas,
			Labels:        ds.Labels,
			Containers:    extractContainerProbeInfo(fullDS.Spec.Template.Spec.Containers),
		}

		// 只添加有 exec 探针的工作负载
		if hasExecProbe(workload.Containers) {
			results = append(results, workload)
		}
	}

	return results, nil
}

// listDeploymentsWithProbes 列出带探针信息的 Deployments
func (c *k8sClient) listDeploymentsWithProbes(ctx context.Context, namespace string) ([]types.WorkloadInfo, error) {
	deployList, err := c.ListDeployments(ctx, namespace)
	if err != nil {
		return nil, err
	}

	var results []types.WorkloadInfo
	for _, deploy := range deployList {
		// 获取完整的 Deployment 信息
		fullDeploy, err := c.getDeploymentRaw(ctx, deploy.Namespace, deploy.Name)
		if err != nil {
			continue
		}

		workload := types.WorkloadInfo{
			Type:          types.WorkloadTypeDeployment,
			Name:          deploy.Name,
			Namespace:     deploy.Namespace,
			Replicas:      deploy.Replicas,
			ReadyReplicas: deploy.ReadyReplicas,
			Labels:        deploy.Labels,
			Selector:      deploy.Selector,
			Containers:    extractContainerProbeInfo(fullDeploy.Spec.Template.Spec.Containers),
		}

		// 只添加有 exec 探针的工作负载
		if hasExecProbe(workload.Containers) {
			results = append(results, workload)
		}
	}

	return results, nil
}

// extractContainerProbeInfo 从容器列表中提取探针信息
func extractContainerProbeInfo(containers []Container) []types.WorkloadContainerInfo {
	result := make([]types.WorkloadContainerInfo, 0, len(containers))
	for _, container := range containers {
		containerInfo := types.WorkloadContainerInfo{
			Name:  container.Name,
			Image: container.Image,
		}

		if container.LivenessProbe != nil {
			containerInfo.HasLivenessProbe = true
			containerInfo.LivenessProbe = extractProbeInfo(container.LivenessProbe)
		}
		if container.ReadinessProbe != nil {
			containerInfo.HasReadinessProbe = true
			containerInfo.ReadinessProbe = extractProbeInfo(container.ReadinessProbe)
		}
		if container.StartupProbe != nil {
			containerInfo.HasStartupProbe = true
			containerInfo.StartupProbe = extractProbeInfo(container.StartupProbe)
		}

		result = append(result, containerInfo)
	}
	return result
}

// extractProbeInfo 从探针中提取信息 (仅 exec 类型)
func extractProbeInfo(probe *Probe) *types.ProbeInfo {
	if probe == nil || probe.Exec == nil {
		return nil
	}
	return &types.ProbeInfo{
		Type:    "exec",
		Command: probe.Exec.Command,
	}
}

// hasExecProbe 检查容器列表中是否有 exec 探针
func hasExecProbe(containers []types.WorkloadContainerInfo) bool {
	for _, c := range containers {
		if c.LivenessProbe != nil || c.ReadinessProbe != nil || c.StartupProbe != nil {
			return true
		}
	}
	return false
}

// getDaemonSetRaw 获取原始 DaemonSet 对象
func (c *k8sClient) getDaemonSetRaw(ctx context.Context, namespace, name string) (*DaemonSet, error) {
	url := fmt.Sprintf("%s/apis/apps/v1/namespaces/%s/daemonsets/%s", c.apiServer, namespace, name)

	var ds DaemonSet
	if err := c.doGet(ctx, url, &ds); err != nil {
		return nil, err
	}

	return &ds, nil
}

// getDeploymentRaw 获取原始 Deployment 对象
func (c *k8sClient) getDeploymentRaw(ctx context.Context, namespace, name string) (*Deployment, error) {
	url := fmt.Sprintf("%s/apis/apps/v1/namespaces/%s/deployments/%s", c.apiServer, namespace, name)

	var deploy Deployment
	if err := c.doGet(ctx, url, &deploy); err != nil {
		return nil, err
	}

	return &deploy, nil
}
