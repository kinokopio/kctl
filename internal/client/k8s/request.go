package k8s

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// doRequest 执行 HTTP 请求的通用方法
func (c *k8sClient) doRequest(ctx context.Context, method, url string, body []byte, contentType string) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	c.setAuthHeader(req)
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if body != nil {
		req.ContentLength = int64(len(body))
	}

	return c.httpClient.Do(req)
}

// doGet 执行 GET 请求并解析 JSON 响应
func (c *k8sClient) doGet(ctx context.Context, url string, result interface{}) error {
	resp, err := c.doRequest(ctx, "GET", url, nil, "")
	if err != nil {
		return fmt.Errorf("请求 K8s API Server 失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("K8s API Server 返回错误状态: %d, body: %s", resp.StatusCode, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}

	return nil
}

// doGetWithNotFound 执行 GET 请求，支持 404 返回 nil
func (c *k8sClient) doGetWithNotFound(ctx context.Context, url string, result interface{}) (bool, error) {
	resp, err := c.doRequest(ctx, "GET", url, nil, "")
	if err != nil {
		return false, fmt.Errorf("请求 K8s API Server 失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("K8s API Server 返回错误状态: %d, body: %s", resp.StatusCode, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return false, fmt.Errorf("解析响应失败: %w", err)
	}

	return true, nil
}

// doPatch 执行 PATCH 请求
func (c *k8sClient) doPatch(ctx context.Context, url string, patch []byte) error {
	resp, err := c.doRequest(ctx, "PATCH", url, patch, "application/strategic-merge-patch+json")
	if err != nil {
		return fmt.Errorf("请求 K8s API Server 失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("K8s API Server 返回错误状态: %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// doPost 执行 POST 请求 (创建资源)
func (c *k8sClient) doPost(ctx context.Context, url string, body []byte) error {
	resp, err := c.doRequest(ctx, "POST", url, body, "application/json")
	if err != nil {
		return fmt.Errorf("请求 K8s API Server 失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("K8s API Server 返回错误状态: %d, body: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// doDelete 执行 DELETE 请求
func (c *k8sClient) doDelete(ctx context.Context, url string) error {
	resp, err := c.doRequest(ctx, "DELETE", url, nil, "")
	if err != nil {
		return fmt.Errorf("请求 K8s API Server 失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("K8s API Server 返回错误状态: %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}
