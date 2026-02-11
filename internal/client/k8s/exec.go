package k8s

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/moby/spdystream"
	"golang.org/x/net/proxy"

	"kctl/internal/client"
	"kctl/pkg/types"
)

// ExecInPod 在 Pod 中执行命令
// 使用 SPDY 协议连接到 K8s API Server 的 exec 端点
func (c *k8sClient) ExecInPod(ctx context.Context, namespace, podName, container string, command []string) (*types.PodExecResult, error) {
	result := &types.PodExecResult{
		PodName:   podName,
		Namespace: namespace,
		Container: container,
	}

	// 构建 exec URL
	execURL, err := c.buildExecURL(namespace, podName, container, command)
	if err != nil {
		result.Error = err
		return result, err
	}

	// 创建 SPDY 连接
	stdout, stderr, exitCode, err := c.execViaSPDY(ctx, execURL)
	if err != nil {
		result.Error = err
		return result, err
	}

	result.Stdout = stdout
	result.Stderr = stderr
	result.ExitCode = exitCode

	return result, nil
}

// buildExecURL 构建 exec API URL
func (c *k8sClient) buildExecURL(namespace, podName, container string, command []string) (string, error) {
	// 基础 URL
	baseURL := fmt.Sprintf("%s/api/v1/namespaces/%s/pods/%s/exec", c.apiServer, namespace, podName)

	// 构建查询参数
	params := url.Values{}
	params.Add("stdout", "true")
	params.Add("stderr", "true")
	if container != "" {
		params.Add("container", container)
	}
	for _, cmd := range command {
		params.Add("command", cmd)
	}

	return baseURL + "?" + params.Encode(), nil
}

// execViaSPDY 通过 SPDY 协议执行命令
func (c *k8sClient) execViaSPDY(ctx context.Context, execURL string) (stdout, stderr string, exitCode int, err error) {
	// 解析 URL
	u, err := url.Parse(execURL)
	if err != nil {
		return "", "", -1, fmt.Errorf("解析 URL 失败: %w", err)
	}

	// 确定主机和端口
	host := u.Host
	if !strings.Contains(host, ":") {
		if u.Scheme == "https" {
			host += ":443"
		} else {
			host += ":80"
		}
	}

	// 创建 TCP 连接
	var conn net.Conn
	if c.config != nil && c.config.ProxyURL != "" {
		// 通过代理连接
		conn, err = c.dialViaProxy(host)
	} else {
		// 直接连接
		conn, err = net.Dial("tcp", host)
	}
	if err != nil {
		return "", "", -1, fmt.Errorf("连接失败: %w", err)
	}

	// 如果是 HTTPS，包装为 TLS 连接
	if u.Scheme == "https" {
		// 提取主机名（不含端口）
		serverName := u.Hostname()

		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         serverName,
		}

		// 配置客户端证书
		if c.config != nil && c.config.HasClientCert() {
			cert, err := tls.X509KeyPair(c.config.ClientCert, c.config.ClientKey)
			if err != nil {
				_ = conn.Close()
				return "", "", -1, fmt.Errorf("加载客户端证书失败: %w", err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}

		// 配置 CA 证书
		if c.config != nil && len(c.config.CACert) > 0 {
			caCertPool := x509.NewCertPool()
			if caCertPool.AppendCertsFromPEM(c.config.CACert) {
				tlsConfig.RootCAs = caCertPool
				tlsConfig.InsecureSkipVerify = false
			}
		}

		tlsConn := tls.Client(conn, tlsConfig)
		if err := tlsConn.Handshake(); err != nil {
			_ = conn.Close()
			return "", "", -1, fmt.Errorf("TLS 握手失败: %w", err)
		}
		conn = tlsConn
	}

	// 发送 HTTP 升级请求
	reqPath := u.RequestURI()

	// 构建请求头
	var httpReqBuilder strings.Builder
	httpReqBuilder.WriteString(fmt.Sprintf("POST %s HTTP/1.1\r\n", reqPath))
	httpReqBuilder.WriteString(fmt.Sprintf("Host: %s\r\n", u.Host))

	// 如果有 token，添加 Authorization 头
	if c.token != "" {
		httpReqBuilder.WriteString(fmt.Sprintf("Authorization: Bearer %s\r\n", c.token))
	}

	httpReqBuilder.WriteString("Connection: Upgrade\r\n")
	httpReqBuilder.WriteString("Upgrade: SPDY/3.1\r\n")
	httpReqBuilder.WriteString("X-Stream-Protocol-Version: v4.channel.k8s.io\r\n")
	httpReqBuilder.WriteString("X-Stream-Protocol-Version: v3.channel.k8s.io\r\n")
	httpReqBuilder.WriteString("X-Stream-Protocol-Version: v2.channel.k8s.io\r\n")
	httpReqBuilder.WriteString("X-Stream-Protocol-Version: channel.k8s.io\r\n")
	httpReqBuilder.WriteString("\r\n")

	if _, err := conn.Write([]byte(httpReqBuilder.String())); err != nil {
		_ = conn.Close()
		return "", "", -1, fmt.Errorf("发送请求失败: %w", err)
	}

	// 读取响应头
	respBuf := make([]byte, 4096)
	n, err := conn.Read(respBuf)
	if err != nil {
		_ = conn.Close()
		return "", "", -1, fmt.Errorf("读取响应失败: %w", err)
	}

	respStr := string(respBuf[:n])
	if !strings.Contains(respStr, "101 Switching Protocols") {
		_ = conn.Close()
		return "", "", -1, fmt.Errorf("升级协议失败: %s", respStr)
	}

	// 创建 SPDY 连接
	spdyConn, err := spdystream.NewConnection(conn, false)
	if err != nil {
		_ = conn.Close()
		return "", "", -1, fmt.Errorf("创建 SPDY 连接失败: %w", err)
	}
	go spdyConn.Serve(spdystream.NoOpStreamHandler)

	// 创建流
	var stdoutBuf, stderrBuf bytes.Buffer

	// 创建 stdout 流 (streamID = 1)
	stdoutStream, err := spdyConn.CreateStream(http.Header{
		"streamType": []string{"stdout"},
	}, nil, false)
	if err != nil {
		_ = spdyConn.Close()
		return "", "", -1, fmt.Errorf("创建 stdout 流失败: %w", err)
	}

	// 创建 stderr 流 (streamID = 2)
	stderrStream, err := spdyConn.CreateStream(http.Header{
		"streamType": []string{"stderr"},
	}, nil, false)
	if err != nil {
		_ = spdyConn.Close()
		return "", "", -1, fmt.Errorf("创建 stderr 流失败: %w", err)
	}

	// 创建 error 流用于获取退出码
	errorStream, err := spdyConn.CreateStream(http.Header{
		"streamType": []string{"error"},
	}, nil, false)
	if err != nil {
		_ = spdyConn.Close()
		return "", "", -1, fmt.Errorf("创建 error 流失败: %w", err)
	}

	// 读取输出
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&stdoutBuf, stdoutStream)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(&stderrBuf, stderrStream)
		done <- struct{}{}
	}()

	// 读取错误/退出码
	var errorBuf bytes.Buffer
	go func() {
		_, _ = io.Copy(&errorBuf, errorStream)
		done <- struct{}{}
	}()

	// 等待所有流完成
	for i := 0; i < 3; i++ {
		<-done
	}

	_ = spdyConn.Close()

	// 解析退出码
	exitCode = 0
	if errorBuf.Len() > 0 {
		var execStatus struct {
			Status string `json:"status"`
			Code   int    `json:"code,omitempty"`
		}
		if err := json.Unmarshal(errorBuf.Bytes(), &execStatus); err == nil {
			if execStatus.Status != "Success" {
				exitCode = execStatus.Code
				if exitCode == 0 {
					exitCode = 1 // 非成功状态但没有退出码，设为 1
				}
			}
		}
	}

	return stdoutBuf.String(), stderrBuf.String(), exitCode, nil
}

// dialViaProxy 通过代理连接
func (c *k8sClient) dialViaProxy(addr string) (net.Conn, error) {
	u, err := url.Parse(c.config.ProxyURL)
	if err != nil {
		return nil, fmt.Errorf("解析代理 URL 失败: %w", err)
	}

	dialer, err := proxy.SOCKS5("tcp", u.Host, nil, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("创建代理拨号器失败: %w", err)
	}

	return dialer.Dial("tcp", addr)
}

// GetAPIServer 获取 API Server 地址
func (c *k8sClient) GetAPIServer() string {
	return c.apiServer
}

// GetToken 获取 Token
func (c *k8sClient) GetToken() string {
	return c.token
}

// GetConfig 获取配置
func (c *k8sClient) GetConfig() *client.Config {
	return c.config
}
