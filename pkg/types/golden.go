package types

import "time"

// ==================== Golden Ticket 相关类型 ====================

// GoldenForgeType 伪造类型
type GoldenForgeType string

const (
	ForgeTypeUserCert GoldenForgeType = "user-cert"
	ForgeTypeNodeCert GoldenForgeType = "node-cert"
	ForgeTypeSAToken  GoldenForgeType = "sa-token"
)

// UIDCache UID 缓存结构
type UIDCache struct {
	Entries   map[string]string `json:"entries"`   // namespace/name -> UID
	ServerURL string            `json:"serverUrl"` // API Server URL
	UpdatedAt time.Time         `json:"updatedAt"` // 更新时间
}

// ForgeResult 伪造结果
type ForgeResult struct {
	Type           GoldenForgeType // 伪造类型
	CertPath       string          // 证书路径 (证书类型)
	KeyPath        string          // 私钥路径 (证书类型)
	Token          string          // Token 内容 (Token 类型)
	KubeconfigPath string          // kubeconfig 路径
	Identity       string          // 身份标识
	ExpiresAt      time.Time       // 过期时间
}

// UserCertOptions 用户证书伪造选项
type UserCertOptions struct {
	CACertPath   string // CA 证书路径
	CAKeyPath    string // CA 私钥路径
	Role         string // 集群角色
	Username     string // 用户名
	Days         int    // 有效天数
	ServerURL    string // API Server URL
	OutputDir    string // 输出目录
	NoKubeconfig bool   // 不生成 kubeconfig
	Force        bool   // 覆盖已存在文件
}

// NodeCertOptions 节点证书伪造选项
type NodeCertOptions struct {
	CACertPath   string // CA 证书路径
	CAKeyPath    string // CA 私钥路径
	NodeName     string // 节点名称
	Days         int    // 有效天数
	ServerURL    string // API Server URL
	OutputDir    string // 输出目录
	NoKubeconfig bool   // 不生成 kubeconfig
	Force        bool   // 覆盖已存在文件
}

// SATokenOptions SA Token 伪造选项
type SATokenOptions struct {
	SAKeyPath    string // SA 私钥路径
	CACertPath   string // CA 证书路径 (用于 kubeconfig)
	Namespace    string // ServiceAccount 命名空间
	Name         string // ServiceAccount 名称
	UID          string // ServiceAccount UID
	UIDCachePath string // UID 缓存文件路径
	TTL          int    // Token 有效期 (秒)
	Audience     string // Token 受众 URL
	ServerURL    string // API Server URL
	OutputDir    string // 输出目录
	NoKubeconfig bool   // 不生成 kubeconfig
	Force        bool   // 覆盖已存在文件
}

// UpdateUIDOptions 更新 UID 缓存选项
type UpdateUIDOptions struct {
	CACertPath string // CA 证书路径
	CAKeyPath  string // CA 私钥路径
	Token      string // 认证 Token
	ServerURL  string // API Server URL
	OutputPath string // 输出路径
	Force      bool   // 覆盖已存在文件
}

// TestOptions 测试选项
type TestOptions struct {
	CACertPath   string // CA 证书路径
	CAKeyPath    string // CA 私钥路径
	SAKeyPath    string // SA 私钥路径
	UIDCachePath string // UID 缓存文件路径
}

// TestResult 测试结果
type TestResult struct {
	Name    string // 测试项名称
	Path    string // 文件路径
	Valid   bool   // 是否有效
	Message string // 消息
}
