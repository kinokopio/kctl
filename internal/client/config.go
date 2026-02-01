package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/net/proxy"
	"kctl/config"
)

// Config 客户端通用配置
type Config struct {
	// 代理设置
	ProxyURL string

	// 超时设置
	Timeout        time.Duration
	ConnectTimeout time.Duration

	// TLS 设置
	SkipTLSVerify bool
	CACertPath    string

	// 重试设置
	MaxRetries    int
	RetryInterval time.Duration
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Timeout:        config.DefaultHTTPTimeout,
		ConnectTimeout: config.DefaultConnectTimeout,
		SkipTLSVerify:  true,
		MaxRetries:     config.DefaultMaxRetries,
		RetryInterval:  time.Second,
	}
}

// WithProxy 设置代理
func (c *Config) WithProxy(proxyURL string) *Config {
	c.ProxyURL = proxyURL
	return c
}

// WithTimeout 设置超时
func (c *Config) WithTimeout(timeout time.Duration) *Config {
	c.Timeout = timeout
	return c
}

// NewHTTPClient 创建 HTTP 客户端
func NewHTTPClient(cfg *Config) (*http.Client, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.SkipTLSVerify,
		},
	}

	// 配置代理
	if cfg.ProxyURL != "" {
		dialer, err := createSOCKS5Dialer(cfg.ProxyURL)
		if err != nil {
			return nil, err
		}
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		}
	}

	return &http.Client{
		Transport: transport,
		Timeout:   cfg.Timeout,
	}, nil
}

// NewWebSocketDialer 创建 WebSocket 拨号器
func NewWebSocketDialer(cfg *Config) (*websocket.Dialer, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	dialer := &websocket.Dialer{
		TLSClientConfig:  &tls.Config{InsecureSkipVerify: cfg.SkipTLSVerify},
		Subprotocols:     []string{"v4.channel.k8s.io"},
		HandshakeTimeout: config.DefaultWebSocketTimeout,
	}

	// 配置代理
	if cfg.ProxyURL != "" {
		socksDialer, err := createSOCKS5Dialer(cfg.ProxyURL)
		if err != nil {
			return nil, err
		}
		dialer.NetDial = func(network, addr string) (net.Conn, error) {
			return socksDialer.Dial(network, addr)
		}
	}

	return dialer, nil
}

// createSOCKS5Dialer 创建 SOCKS5 代理拨号器
func createSOCKS5Dialer(proxyURL string) (proxy.Dialer, error) {
	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("解析代理 URL 失败: %w", err)
	}

	if u.Scheme != "socks5" && u.Scheme != "socks5h" {
		return nil, fmt.Errorf("不支持的代理协议: %s，仅支持 socks5 或 socks5h", u.Scheme)
	}

	return proxy.SOCKS5("tcp", u.Host, nil, proxy.Direct)
}
