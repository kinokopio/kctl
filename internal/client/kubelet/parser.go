package kubelet

import (
	"encoding/json"
	"time"

	"kctl/pkg/types"
)

// ExtractPodRecords 从原始数据中提取有安全价值的信息
func ExtractPodRecords(rawData []byte, kubeletIP string) ([]*types.PodRecord, error) {
	var response types.KubeletPodsFullResponse
	if err := json.Unmarshal(rawData, &response); err != nil {
		return nil, err
	}

	var records []*types.PodRecord
	now := time.Now()

	for _, item := range response.Items {
		record := &types.PodRecord{
			Name:              item.Metadata.Name,
			Namespace:         item.Metadata.Namespace,
			UID:               item.Metadata.UID,
			NodeName:          item.Spec.NodeName,
			PodIP:             item.Status.PodIP,
			HostIP:            item.Status.HostIP,
			Phase:             item.Status.Phase,
			ServiceAccount:    item.Spec.ServiceAccountName,
			CreationTimestamp: item.Metadata.CreationTimestamp,
			CollectedAt:       now,
			KubeletIP:         kubeletIP,
		}

		// 提取容器安全信息
		containers := extractContainerInfo(item.Spec.Containers)
		if len(containers) > 0 {
			containersJSON, _ := json.Marshal(containers)
			record.Containers = string(containersJSON)
		}

		// 提取敏感卷信息
		volumes := extractSensitiveVolumes(item.Spec.Volumes, item.Spec.Containers)
		if len(volumes) > 0 {
			volumesJSON, _ := json.Marshal(volumes)
			record.Volumes = string(volumesJSON)
		}

		// 提取 Pod 级安全上下文
		if item.Spec.SecurityContext != nil {
			secCtxJSON, _ := json.Marshal(item.Spec.SecurityContext)
			record.SecurityContext = string(secCtxJSON)
		}

		records = append(records, record)
	}

	return records, nil
}

// extractContainerInfo 提取容器安全信息
func extractContainerInfo(containers []types.ContainerSpec) []types.ContainerInfo {
	var infos []types.ContainerInfo

	for _, c := range containers {
		info := types.ContainerInfo{
			Name:  c.Name,
			Image: c.Image,
		}

		// 提取挂载路径
		for _, vm := range c.VolumeMounts {
			info.VolumeMounts = append(info.VolumeMounts, vm.MountPath)
		}

		// 提取安全上下文
		if c.SecurityContext != nil {
			info.RunAsUser = c.SecurityContext.RunAsUser
			info.RunAsGroup = c.SecurityContext.RunAsGroup

			if c.SecurityContext.Privileged != nil {
				info.Privileged = *c.SecurityContext.Privileged
			}
			if c.SecurityContext.AllowPrivilegeEscalation != nil {
				info.AllowPrivilegeEscalation = *c.SecurityContext.AllowPrivilegeEscalation
			}
			if c.SecurityContext.ReadOnlyRootFilesystem != nil {
				info.ReadOnlyRootFilesystem = *c.SecurityContext.ReadOnlyRootFilesystem
			}
		}

		infos = append(infos, info)
	}

	return infos
}

// extractSensitiveVolumes 提取敏感卷信息
func extractSensitiveVolumes(volumes []types.VolumeSpec, containers []types.ContainerSpec) []types.SensitiveVolume {
	var sensitiveVols []types.SensitiveVolume

	// 构建卷名到挂载路径的映射
	mountPaths := make(map[string]string)
	for _, c := range containers {
		for _, vm := range c.VolumeMounts {
			mountPaths[vm.Name] = vm.MountPath
		}
	}

	for _, v := range volumes {
		var sv *types.SensitiveVolume

		if v.Secret != nil {
			sv = &types.SensitiveVolume{
				Name:       v.Name,
				Type:       "secret",
				SecretName: v.Secret.SecretName,
			}
		} else if v.HostPath != nil {
			sv = &types.SensitiveVolume{
				Name:     v.Name,
				Type:     "hostPath",
				HostPath: v.HostPath.Path,
			}
		} else if v.Projected != nil {
			// 检查 projected 卷是否包含 ServiceAccount Token
			for _, src := range v.Projected.Sources {
				if src.ServiceAccountToken != nil {
					sv = &types.SensitiveVolume{
						Name: v.Name,
						Type: "projected-sa-token",
					}
					break
				}
				if src.Secret != nil {
					sv = &types.SensitiveVolume{
						Name:       v.Name,
						Type:       "projected-secret",
						SecretName: src.Secret.SecretName,
					}
					break
				}
			}
		}

		if sv != nil {
			if mp, ok := mountPaths[v.Name]; ok {
				sv.MountPath = mp
			}
			sensitiveVols = append(sensitiveVols, *sv)
		}
	}

	return sensitiveVols
}
