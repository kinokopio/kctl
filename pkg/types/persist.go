package types

// ==================== Persist Probe Inject 相关类型 ====================

// ProbeType 探针类型
type ProbeType string

const (
	ProbeTypeLiveness  ProbeType = "liveness"
	ProbeTypeReadiness ProbeType = "readiness"
	ProbeTypeStartup   ProbeType = "startup"
)

// String 返回探针类型的字符串表示
func (p ProbeType) String() string {
	return string(p)
}

// PayloadType 载荷类型
type PayloadType string

const (
	PayloadReverseShellBash   PayloadType = "reverse-shell-bash"
	PayloadReverseShellNC     PayloadType = "reverse-shell-nc"
	PayloadReverseShellNCPipe PayloadType = "reverse-shell-nc-pipe"
	PayloadReverseShellPython PayloadType = "reverse-shell-python"
	PayloadReverseShellPerl   PayloadType = "reverse-shell-perl"
	PayloadReverseShellRuby   PayloadType = "reverse-shell-ruby"
	PayloadHTTPBeaconCurl     PayloadType = "http-beacon-curl"
	PayloadHTTPBeaconWget     PayloadType = "http-beacon-wget"
	PayloadCustom             PayloadType = "custom"
)

// PayloadTemplate 载荷模板
type PayloadTemplate struct {
	Type          PayloadType        // 载荷类型
	Name          string             // 显示名称
	Description   string             // 描述
	Template      string             // 命令模板 (使用 {Host}, {Port}, {URL} 等占位符)
	RequiredTools []string           // 需要的工具
	Parameters    []PayloadParameter // 需要的参数
}

// PayloadParameter 载荷参数
type PayloadParameter struct {
	Name        string // 参数名 (如 Host, Port, URL)
	Description string // 描述
	Default     string // 默认值
	Required    bool   // 是否必需
}

// BasicContainerInfo 基础容器信息 (仅名称和镜像)
type BasicContainerInfo struct {
	Name  string // 容器名称
	Image string // 镜像
}

// DaemonSetInfo DaemonSet 信息
type DaemonSetInfo struct {
	Name              string               // DaemonSet 名称
	Namespace         string               // 命名空间
	DesiredReplicas   int32                // 期望副本数
	ReadyReplicas     int32                // 就绪副本数
	Labels            map[string]string    // 标签
	Containers        []BasicContainerInfo // 容器列表
	HasLivenessProbe  bool                 // 是否已有 liveness 探针
	HasReadinessProbe bool                 // 是否已有 readiness 探针
	HasStartupProbe   bool                 // 是否已有 startup 探针
}

// WorkloadType 工作负载类型
type WorkloadType string

const (
	WorkloadTypeDaemonSet  WorkloadType = "DaemonSet"
	WorkloadTypeDeployment WorkloadType = "Deployment"
)

// DeploymentInfo Deployment 信息
type DeploymentInfo struct {
	Name              string               // Deployment 名称
	Namespace         string               // 命名空间
	Replicas          int32                // 副本数
	ReadyReplicas     int32                // 就绪副本数
	Labels            map[string]string    // 标签
	Selector          map[string]string    // Pod 选择器
	Containers        []BasicContainerInfo // 容器列表
	HasLivenessProbe  bool                 // 是否已有 liveness 探针
	HasReadinessProbe bool                 // 是否已有 readiness 探针
	HasStartupProbe   bool                 // 是否已有 startup 探针
}

// WorkloadInfo 通用工作负载信息 (用于列表显示)
type WorkloadInfo struct {
	Type          WorkloadType            // 工作负载类型
	Name          string                  // 名称
	Namespace     string                  // 命名空间
	Replicas      int32                   // 副本数
	ReadyReplicas int32                   // 就绪副本数
	Labels        map[string]string       // 标签
	Selector      map[string]string       // Pod 选择器
	Containers    []WorkloadContainerInfo // 容器列表
}

// WorkloadContainerInfo 工作负载容器信息 (包含探针详情)
type WorkloadContainerInfo struct {
	Name              string     // 容器名称
	Image             string     // 镜像
	HasLivenessProbe  bool       // 是否有 liveness 探针
	HasReadinessProbe bool       // 是否有 readiness 探针
	HasStartupProbe   bool       // 是否有 startup 探针
	LivenessProbe     *ProbeInfo // liveness 探针详情
	ReadinessProbe    *ProbeInfo // readiness 探针详情
	StartupProbe      *ProbeInfo // startup 探针详情
}

// ProbeInfo 探针信息
type ProbeInfo struct {
	Type    string   // exec, httpGet, tcpSocket
	Command []string // exec 命令 (如果是 exec 类型)
}

// ToolDetectionResult 工具检测结果
type ToolDetectionResult struct {
	Tool      string // 工具名称
	Available bool   // 是否可用
	Path      string // 工具路径 (如果可用)
}

// ProbeConfig 探针配置
type ProbeConfig struct {
	Type                ProbeType // 探针类型
	Command             []string  // 执行命令
	PeriodSeconds       int32     // 执行间隔 (秒)
	InitialDelaySeconds int32     // 初始延迟 (秒)
	TimeoutSeconds      int32     // 超时时间 (秒)
	FailureThreshold    int32     // 失败阈值
	SuccessThreshold    int32     // 成功阈值
}

// ProbeInjectOptions probe-inject 命令选项
type ProbeInjectOptions struct {
	Namespace           string      // 目标命名空间
	DaemonSetName       string      // 目标 DaemonSet 名称
	ContainerName       string      // 目标容器名称
	ProbeType           ProbeType   // 探针类型
	PayloadType         PayloadType // 载荷类型
	PayloadCommand      string      // 最终载荷命令
	PeriodSeconds       int32       // 执行间隔
	InitialDelaySeconds int32       // 初始延迟
	TimeoutSeconds      int32       // 超时时间
	FailureThreshold    int32       // 失败阈值
	DryRun              bool        // 仅预览，不执行
	ServerURL           string      // API Server URL (可选覆盖)
	Token               string      // 认证 Token (可选覆盖)
}

// ProbeInjectResult probe-inject 执行结果
type ProbeInjectResult struct {
	Success       bool      // 是否成功
	DaemonSetName string    // DaemonSet 名称
	Namespace     string    // 命名空间
	ContainerName string    // 容器名称
	ProbeType     ProbeType // 探针类型
	Message       string    // 消息
	PatchedSpec   string    // 已应用的 Patch (JSON)
}

// PodExecResult Pod exec 执行结果
type PodExecResult struct {
	PodName   string // Pod 名称
	Namespace string // 命名空间
	Container string // 容器名称
	Stdout    string // 标准输出
	Stderr    string // 标准错误
	ExitCode  int    // 退出码
	Error     error  // 错误
}
