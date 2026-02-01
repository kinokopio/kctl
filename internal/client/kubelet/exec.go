package kubelet

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"golang.org/x/term"
	"kctl/pkg/types"
)

// WebSocket 子协议通道编号
const (
	StreamStdin  = 0 // stdin 通道
	StreamStdout = 1 // stdout 通道
	StreamStderr = 2 // stderr 通道
	StreamError  = 3 // error 通道
	StreamResize = 4 // resize 通道 (TTY)
)

// Exec 在 Pod 中执行命令（非交互式）
func (c *kubeletClient) Exec(ctx context.Context, opts *types.ExecOptions) (*types.ExecResult, error) {
	// 构建 exec URL
	execURL := c.buildExecURL(opts)

	// 设置请求头
	headers := http.Header{}
	headers.Set("Authorization", c.authHeader())

	// 建立 WebSocket 连接
	conn, resp, err := c.wsDialer.DialContext(ctx, execURL, headers)
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("WebSocket 连接失败 (HTTP %d): %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("WebSocket 连接失败: %w", err)
	}
	defer func() { _ = conn.Close() }()

	return c.readExecOutput(conn)
}

// ExecInteractive 在 Pod 中交互式执行命令
func (c *kubeletClient) ExecInteractive(ctx context.Context, opts *types.ExecOptions) error {
	// 构建 exec URL
	execURL := c.buildExecURL(opts)

	// 设置请求头
	headers := http.Header{}
	headers.Set("Authorization", c.authHeader())

	// 建立 WebSocket 连接
	conn, resp, err := c.wsDialer.DialContext(ctx, execURL, headers)
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("WebSocket 连接失败 (HTTP %d): %s", resp.StatusCode, string(body))
		}
		return fmt.Errorf("WebSocket 连接失败: %w", err)
	}
	defer func() { _ = conn.Close() }()

	// 如果启用了 TTY，将终端设置为 raw 模式
	if opts.TTY {
		fd := int(os.Stdin.Fd())
		if term.IsTerminal(fd) {
			oldState, err := term.MakeRaw(fd)
			if err != nil {
				return fmt.Errorf("设置终端 raw 模式失败: %w", err)
			}
			defer func() { _ = term.Restore(fd, oldState) }()
		}
	}

	var wg sync.WaitGroup
	done := make(chan struct{})

	// 读取输出
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				return
			default:
				_, message, err := conn.ReadMessage()
				if err != nil {
					return
				}

				if len(message) < 1 {
					continue
				}

				channel := message[0]
				data := message[1:]

				switch channel {
				case StreamStdout:
					_, _ = os.Stdout.Write(data)
				case StreamStderr:
					_, _ = os.Stderr.Write(data)
				case StreamError:
					fmt.Fprintf(os.Stderr, "\n[Error] %s\n", string(data))
				}
			}
		}
	}()

	// 如果启用了 stdin，从标准输入读取
	if opts.Stdin {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := make([]byte, 1024)
			for {
				select {
				case <-done:
					return
				default:
					n, err := os.Stdin.Read(buf)
					if err != nil {
						if err != io.EOF {
							return
						}
						return
					}
					if n > 0 {
						// 发送数据，第一个字节是通道编号 (stdin = 0)
						msg := append([]byte{StreamStdin}, buf[:n]...)
						if err := conn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
							return
						}
					}
				}
			}
		}()
	}

	wg.Wait()
	return nil
}

// buildExecURL 构建 exec WebSocket URL
func (c *kubeletClient) buildExecURL(opts *types.ExecOptions) string {
	// 基础 URL
	baseURL := fmt.Sprintf("wss://%s:%d/exec/%s/%s/%s",
		c.ip, c.port, opts.Namespace, opts.Pod, opts.Container)

	// 构建查询参数
	// 注意: Kubelet API 使用 input/output/error 而不是 stdin/stdout/stderr
	params := url.Values{}

	if opts.Stdin {
		params.Add("input", "1")
	}
	if opts.Stdout {
		params.Add("output", "1")
	}
	if opts.Stderr {
		params.Add("error", "1")
	}
	if opts.TTY {
		params.Add("tty", "1")
	}

	// 添加命令参数
	for _, cmd := range opts.Command {
		params.Add("command", cmd)
	}

	return baseURL + "?" + params.Encode()
}

// readExecOutput 读取 exec 输出
func (c *kubeletClient) readExecOutput(conn *websocket.Conn) (*types.ExecResult, error) {
	result := &types.ExecResult{}
	var mu sync.Mutex

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				break
			}
			if result.Error == "" && !strings.Contains(err.Error(), "close") {
				result.Error = err.Error()
			}
			break
		}

		if len(message) < 1 {
			continue
		}

		// 第一个字节是通道编号
		channel := message[0]
		data := string(message[1:])

		mu.Lock()
		switch channel {
		case StreamStdout:
			result.Stdout += data
		case StreamStderr:
			result.Stderr += data
		case StreamError:
			// 解析 exec 状态响应
			var execStatus types.ExecStatus
			if err := json.Unmarshal([]byte(data), &execStatus); err == nil {
				// 只有当 status 不是 Success 时才认为是错误
				if execStatus.Status != "Success" {
					result.Error = execStatus.Message
					if result.Error == "" {
						result.Error = data
					}
				}
			} else {
				// 无法解析为 JSON，作为原始错误处理
				result.Error = data
			}
		}
		mu.Unlock()
	}

	return result, nil
}
