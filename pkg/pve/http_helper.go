package pve

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// doPost 执行 POST 请求（不使用 chunked transfer encoding）
func (c *Client) doPost(path string, data map[string]string) ([]byte, error) {
	// 构建表单数据
	formData := url.Values{}
	for key, value := range data {
		formData.Set(key, value)
	}
	bodyBytes := []byte(formData.Encode())

	// 创建请求
	fullURL := c.baseURL + path
	req, err := http.NewRequest("POST", fullURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头（关键：设置 Content-Length 避免 chunked）
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(bodyBytes)))
	req.Header.Set("Accept", "application/json")

	// 从resty客户端复制Authorization header（API Token认证）
	if auth := c.client.Header.Get("Authorization"); auth != "" {
		req.Header.Set("Authorization", auth)
	}

	debugLog("POST %s, Content-Length=%d", fullURL, len(bodyBytes))

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	debugLog("响应状态码: %d, Content-Length=%d", resp.StatusCode, len(body))

	// 检查状态码
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d, 响应: %s", resp.StatusCode, string(body))
	}

	return body, nil
}
