package kubeconfig

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config kubeconfig 文件结构
type Config struct {
	APIVersion     string         `yaml:"apiVersion"`
	Kind           string         `yaml:"kind"`
	CurrentContext string         `yaml:"current-context"`
	Clusters       []ClusterEntry `yaml:"clusters"`
	Contexts       []ContextEntry `yaml:"contexts"`
	Users          []UserEntry    `yaml:"users"`
}

// ClusterEntry 集群配置条目
type ClusterEntry struct {
	Name    string  `yaml:"name"`
	Cluster Cluster `yaml:"cluster"`
}

// Cluster 集群配置
type Cluster struct {
	Server                   string `yaml:"server"`
	CertificateAuthority     string `yaml:"certificate-authority,omitempty"`
	CertificateAuthorityData string `yaml:"certificate-authority-data,omitempty"`
	InsecureSkipTLSVerify    bool   `yaml:"insecure-skip-tls-verify,omitempty"`
}

// ContextEntry 上下文配置条目
type ContextEntry struct {
	Name    string  `yaml:"name"`
	Context Context `yaml:"context"`
}

// Context 上下文配置
type Context struct {
	Cluster   string `yaml:"cluster"`
	User      string `yaml:"user"`
	Namespace string `yaml:"namespace,omitempty"`
}

// UserEntry 用户配置条目
type UserEntry struct {
	Name string `yaml:"name"`
	User User   `yaml:"user"`
}

// User 用户配置
type User struct {
	Token                 string `yaml:"token,omitempty"`
	ClientCertificate     string `yaml:"client-certificate,omitempty"`
	ClientCertificateData string `yaml:"client-certificate-data,omitempty"`
	ClientKey             string `yaml:"client-key,omitempty"`
	ClientKeyData         string `yaml:"client-key-data,omitempty"`
	Username              string `yaml:"username,omitempty"`
	Password              string `yaml:"password,omitempty"`
}

// ParsedConfig 解析后的配置
type ParsedConfig struct {
	Server      string // API Server URL
	Token       string // Bearer Token
	CACert      []byte // CA 证书数据
	ClientCert  []byte // 客户端证书数据
	ClientKey   []byte // 客户端私钥数据
	Namespace   string // 默认命名空间
	ContextName string // 上下文名称
}

// Load 从文件加载 kubeconfig
func Load(path string) (*Config, error) {
	// 展开 ~ 路径
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("获取用户目录失败: %w", err)
		}
		path = filepath.Join(home, path[1:])
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取 kubeconfig 文件失败: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析 kubeconfig 文件失败: %w", err)
	}

	return &config, nil
}

// Parse 解析 kubeconfig 并返回当前上下文的配置
func Parse(path string) (*ParsedConfig, error) {
	config, err := Load(path)
	if err != nil {
		return nil, err
	}

	return config.GetCurrentContext()
}

// GetCurrentContext 获取当前上下文的配置
func (c *Config) GetCurrentContext() (*ParsedConfig, error) {
	if c.CurrentContext == "" {
		return nil, fmt.Errorf("kubeconfig 中未设置 current-context")
	}

	return c.GetContext(c.CurrentContext)
}

// GetContext 获取指定上下文的配置
func (c *Config) GetContext(contextName string) (*ParsedConfig, error) {
	// 查找上下文
	var ctx *Context
	for _, entry := range c.Contexts {
		if entry.Name == contextName {
			ctx = &entry.Context
			break
		}
	}
	if ctx == nil {
		return nil, fmt.Errorf("未找到上下文: %s", contextName)
	}

	// 查找集群
	var cluster *Cluster
	for _, entry := range c.Clusters {
		if entry.Name == ctx.Cluster {
			cluster = &entry.Cluster
			break
		}
	}
	if cluster == nil {
		return nil, fmt.Errorf("未找到集群: %s", ctx.Cluster)
	}

	// 查找用户
	var user *User
	for _, entry := range c.Users {
		if entry.Name == ctx.User {
			user = &entry.User
			break
		}
	}
	if user == nil {
		return nil, fmt.Errorf("未找到用户: %s", ctx.User)
	}

	result := &ParsedConfig{
		Server:      cluster.Server,
		Namespace:   ctx.Namespace,
		ContextName: contextName,
	}

	// 解析 Token
	if user.Token != "" {
		result.Token = user.Token
	}

	// 解析 CA 证书
	if cluster.CertificateAuthorityData != "" {
		data, err := base64.StdEncoding.DecodeString(cluster.CertificateAuthorityData)
		if err != nil {
			return nil, fmt.Errorf("解码 CA 证书失败: %w", err)
		}
		result.CACert = data
	}

	// 解析客户端证书
	if user.ClientCertificateData != "" {
		data, err := base64.StdEncoding.DecodeString(user.ClientCertificateData)
		if err != nil {
			return nil, fmt.Errorf("解码客户端证书失败: %w", err)
		}
		result.ClientCert = data
	}

	// 解析客户端私钥
	if user.ClientKeyData != "" {
		data, err := base64.StdEncoding.DecodeString(user.ClientKeyData)
		if err != nil {
			return nil, fmt.Errorf("解码客户端私钥失败: %w", err)
		}
		result.ClientKey = data
	}

	return result, nil
}

// ListContexts 列出所有上下文
func (c *Config) ListContexts() []string {
	var names []string
	for _, entry := range c.Contexts {
		names = append(names, entry.Name)
	}
	return names
}

// HasToken 检查是否有 Token 认证
func (p *ParsedConfig) HasToken() bool {
	return p.Token != ""
}

// HasClientCert 检查是否有客户端证书认证
func (p *ParsedConfig) HasClientCert() bool {
	return len(p.ClientCert) > 0 && len(p.ClientKey) > 0
}
