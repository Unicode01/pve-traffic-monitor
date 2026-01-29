package pve

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"pve-traffic-monitor/pkg/models"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

// Client PVE API 客户端
type Client struct {
	config     models.PVEConfig
	client     *resty.Client
	httpClient *http.Client
	baseURL    string
}

// NewClient 创建新的 PVE 客户端（本地访问模式）
func NewClient(config models.PVEConfig) *Client {
	// 创建自定义的 HTTP Transport
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // 本地访问不验证证书
		},
		DisableCompression: true, // 禁用压缩
		DisableKeepAlives:  false,
		MaxIdleConns:       10,
		IdleConnTimeout:    30 * time.Second,
	}

	// 创建原生 HTTP 客户端（用于 POST 请求，避免 chunked encoding）
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	// 创建 resty 客户端（用于 GET 请求）
	client := resty.New()
	client.SetTransport(transport)
	client.SetDisableWarn(true)

	// 设置请求头
	client.SetHeaders(map[string]string{
		"Accept": "application/json",
	})

	// 禁用自动重定向
	client.SetRedirectPolicy(resty.NoRedirectPolicy())

	// 使用本地 Unix socket 或 localhost
	baseURL := fmt.Sprintf("https://%s:%d/api2/json", config.Host, config.Port)
	client.SetBaseURL(baseURL)

	return &Client{
		config:     config,
		client:     client,
		httpClient: httpClient,
		baseURL:    baseURL,
	}
}

// Login 登录 PVE（使用API Token认证）
func (c *Client) Login() error {
	// 从环境变量读取 API Token
	apiTokenID := os.Getenv("PVE_API_TOKEN_ID")
	apiTokenSecret := os.Getenv("PVE_API_TOKEN_SECRET")

	// 如果环境变量不存在，使用配置文件
	if apiTokenID == "" {
		apiTokenID = c.config.APITokenID
	}
	if apiTokenSecret == "" {
		apiTokenSecret = c.config.APITokenSecret
	}

	// 验证API Token配置
	if apiTokenID == "" || apiTokenSecret == "" {
		return fmt.Errorf("必须配置API Token (api_token_id 和 api_token_secret)")
	}

	log.Printf("[PVE] 使用API Token认证: %s", apiTokenID)

	// API Token格式: PVEAPIToken=USER@REALM!TOKENID=UUID
	authHeader := fmt.Sprintf("PVEAPIToken=%s=%s", apiTokenID, apiTokenSecret)

	// 设置Authorization header
	c.client.SetHeader("Authorization", authHeader)

	log.Printf("[PVE] API Token认证配置完成")
	return nil
}

// GetAllVMs 获取所有虚拟机（默认过滤模板）
func (c *Client) GetAllVMs() ([]models.VMInfo, error) {
	return c.GetAllVMsWithFilter(false)
}

// GetAllVMsWithFilter 获取所有虚拟机（可选是否包含模板）
func (c *Client) GetAllVMsWithFilter(includeTemplates bool) ([]models.VMInfo, error) {
	resp, err := c.client.R().
		Get(fmt.Sprintf("/nodes/%s/qemu", c.config.Node))

	if err != nil {
		return nil, fmt.Errorf("获取虚拟机列表失败: %w", err)
	}

	// 检查响应状态码
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("PVE API 返回错误状态码: %d, 响应: %s", resp.StatusCode(), string(resp.Body()))
	}

	// 检查响应体是否为空
	if len(resp.Body()) == 0 {
		return nil, fmt.Errorf("PVE API 返回空响应")
	}

	var result struct {
		Data []struct {
			VMID     int    `json:"vmid"`
			Name     string `json:"name"`
			Status   string `json:"status"`
			NetIn    uint64 `json:"netin"`
			NetOut   uint64 `json:"netout"`
			Tags     string `json:"tags"`
			Template int    `json:"template"` // PVE 返回 0 或 1
		} `json:"data"`
	}

	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		// 输出响应内容以便调试
		return nil, fmt.Errorf("解析虚拟机列表失败: %w\n响应内容: %s", err, string(resp.Body()))
	}

	// 过滤掉模板虚拟机（如果配置不包含）
	vms := make([]models.VMInfo, 0, len(result.Data))
	for _, vm := range result.Data {
		// 检查是否为模板
		isTemplate := vm.Template == 1

		// 如果不包含模板且当前是模板，则跳过
		if !includeTemplates && isTemplate {
			continue
		}

		tags := []string{}
		if vm.Tags != "" {
			// 分割标签并去除空格
			rawTags := strings.Split(vm.Tags, ";")
			for _, tag := range rawTags {
				trimmed := strings.TrimSpace(tag)
				if trimmed != "" {
					tags = append(tags, trimmed)
				}
			}
		}

		vms = append(vms, models.VMInfo{
			VMID:      vm.VMID,
			Name:      vm.Name,
			Status:    vm.Status,
			Tags:      tags,
			NetworkRX: vm.NetIn,
			NetworkTX: vm.NetOut,
			Template:  isTemplate,
		})
	}

	return vms, nil
}

// GetVMStatus 获取虚拟机状态
func (c *Client) GetVMStatus(vmid int) (*models.VMInfo, error) {
	resp, err := c.client.R().
		Get(fmt.Sprintf("/nodes/%s/qemu/%d/status/current", c.config.Node, vmid))

	if err != nil {
		return nil, fmt.Errorf("获取虚拟机状态失败: %w", err)
	}

	// 检查响应状态码
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("PVE API 返回错误状态码: %d, 响应: %s", resp.StatusCode(), string(resp.Body()))
	}

	// 检查响应体是否为空
	if len(resp.Body()) == 0 {
		return nil, fmt.Errorf("PVE API 返回空响应")
	}

	var result struct {
		Data struct {
			VMID   int    `json:"vmid"`
			Name   string `json:"name"`
			Status string `json:"status"`
			NetIn  uint64 `json:"netin"`
			NetOut uint64 `json:"netout"`
		} `json:"data"`
	}

	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("解析虚拟机状态失败: %w\n响应内容: %s", err, string(resp.Body()))
	}

	return &models.VMInfo{
		VMID:      result.Data.VMID,
		Name:      result.Data.Name,
		Status:    result.Data.Status,
		NetworkRX: result.Data.NetIn,
		NetworkTX: result.Data.NetOut,
	}, nil
}

// ShutdownVM 关闭虚拟机（优雅关机，需要虚拟机支持 ACPI）
func (c *Client) ShutdownVM(vmid int) error {
	// 使用原生 HTTP 客户端避免 chunked encoding
	_, err := c.doPost(fmt.Sprintf("/nodes/%s/qemu/%d/status/shutdown", c.config.Node, vmid), map[string]string{})
	if err != nil {
		return fmt.Errorf("关闭虚拟机失败: %w", err)
	}

	return nil
}

// StopVM 强制停止虚拟机（立即停止，不等待虚拟机响应）
func (c *Client) StopVM(vmid int) error {
	// 使用原生 HTTP 客户端避免 chunked encoding
	_, err := c.doPost(fmt.Sprintf("/nodes/%s/qemu/%d/status/stop", c.config.Node, vmid), map[string]string{})
	if err != nil {
		return fmt.Errorf("停止虚拟机失败: %w", err)
	}

	return nil
}

// StartVM 启动虚拟机
func (c *Client) StartVM(vmid int) error {
	// 使用原生 HTTP 客户端避免 chunked encoding
	_, err := c.doPost(fmt.Sprintf("/nodes/%s/qemu/%d/status/start", c.config.Node, vmid), map[string]string{})
	if err != nil {
		return fmt.Errorf("启动虚拟机失败: %w", err)
	}

	return nil
}

// RemoveNetworkRateLimit 移除网络速率限制
func (c *Client) RemoveNetworkRateLimit(vmid int) error {
	// 获取虚拟机配置
	resp, err := c.client.R().
		Get(fmt.Sprintf("/nodes/%s/qemu/%d/config", c.config.Node, vmid))

	if err != nil {
		return fmt.Errorf("获取虚拟机配置失败: %w", err)
	}

	var config struct {
		Data map[string]interface{} `json:"data"`
	}

	if err := json.Unmarshal(resp.Body(), &config); err != nil {
		return fmt.Errorf("解析虚拟机配置失败: %w", err)
	}

	// 查找网络接口并移除速率限制
	updated := false
	for key, value := range config.Data {
		if strings.HasPrefix(key, "net") {
			netConfig := value.(string)
			// 移除 rate 参数
			parts := strings.Split(netConfig, ",")
			newParts := []string{}
			for _, part := range parts {
				if !strings.HasPrefix(strings.TrimSpace(part), "rate=") {
					newParts = append(newParts, part)
				}
			}
			newNetConfig := strings.Join(newParts, ",")

			// 更新配置
			_, err := c.client.R().
				SetFormData(map[string]string{
					key: newNetConfig,
				}).
				Put(fmt.Sprintf("/nodes/%s/qemu/%d/config", c.config.Node, vmid))

			if err != nil {
				return fmt.Errorf("移除网络速率限制失败: %w", err)
			}
			updated = true
			break
		}
	}

	if !updated {
		return fmt.Errorf("未找到网络接口配置")
	}

	return nil
}

// DisconnectNetwork 断开虚拟机网络连接
func (c *Client) DisconnectNetwork(vmid int) error {
	// 获取虚拟机配置
	resp, err := c.client.R().
		Get(fmt.Sprintf("/nodes/%s/qemu/%d/config", c.config.Node, vmid))

	if err != nil {
		return fmt.Errorf("获取虚拟机配置失败: %w", err)
	}

	var config struct {
		Data map[string]interface{} `json:"data"`
	}

	if err := json.Unmarshal(resp.Body(), &config); err != nil {
		return fmt.Errorf("解析虚拟机配置失败: %w", err)
	}

	// 查找网络接口并设置 link_down
	updated := false
	for key, value := range config.Data {
		if strings.HasPrefix(key, "net") {
			netConfig := value.(string)

			// 移除旧的 link_down 参数（如果有）
			parts := strings.Split(netConfig, ",")
			newParts := []string{}
			for _, part := range parts {
				trimmed := strings.TrimSpace(part)
				if !strings.HasPrefix(trimmed, "link_down=") {
					newParts = append(newParts, part)
				}
			}

			// 添加 link_down=1
			newParts = append(newParts, "link_down=1")
			newNetConfig := strings.Join(newParts, ",")

			// 更新配置
			_, err := c.client.R().
				SetFormData(map[string]string{
					key: newNetConfig,
				}).
				Put(fmt.Sprintf("/nodes/%s/qemu/%d/config", c.config.Node, vmid))

			if err != nil {
				return fmt.Errorf("断开网络失败: %w", err)
			}
			updated = true
			break
		}
	}

	if !updated {
		return fmt.Errorf("未找到网络接口配置")
	}

	return nil
}

// ConnectNetwork 连接虚拟机网络
func (c *Client) ConnectNetwork(vmid int) error {
	// 获取虚拟机配置
	resp, err := c.client.R().
		Get(fmt.Sprintf("/nodes/%s/qemu/%d/config", c.config.Node, vmid))

	if err != nil {
		return fmt.Errorf("获取虚拟机配置失败: %w", err)
	}

	var config struct {
		Data map[string]interface{} `json:"data"`
	}

	if err := json.Unmarshal(resp.Body(), &config); err != nil {
		return fmt.Errorf("解析虚拟机配置失败: %w", err)
	}

	// 查找网络接口并移除 link_down
	updated := false
	for key, value := range config.Data {
		if strings.HasPrefix(key, "net") {
			netConfig := value.(string)

			// 移除 link_down 参数
			parts := strings.Split(netConfig, ",")
			newParts := []string{}
			for _, part := range parts {
				trimmed := strings.TrimSpace(part)
				if !strings.HasPrefix(trimmed, "link_down=") {
					newParts = append(newParts, part)
				}
			}
			newNetConfig := strings.Join(newParts, ",")

			// 更新配置
			_, err := c.client.R().
				SetFormData(map[string]string{
					key: newNetConfig,
				}).
				Put(fmt.Sprintf("/nodes/%s/qemu/%d/config", c.config.Node, vmid))

			if err != nil {
				return fmt.Errorf("连接网络失败: %w", err)
			}
			updated = true
			break
		}
	}

	if !updated {
		return fmt.Errorf("未找到网络接口配置")
	}

	return nil
}

// SetNetworkRateLimit 设置网络速率限制（单位：MB/s，支持小数）
func (c *Client) SetNetworkRateLimit(vmid int, rateMB float64) error {
	// 获取虚拟机配置
	resp, err := c.client.R().
		Get(fmt.Sprintf("/nodes/%s/qemu/%d/config", c.config.Node, vmid))

	if err != nil {
		return fmt.Errorf("获取虚拟机配置失败: %w", err)
	}

	var config struct {
		Data map[string]interface{} `json:"data"`
	}

	if err := json.Unmarshal(resp.Body(), &config); err != nil {
		return fmt.Errorf("解析虚拟机配置失败: %w", err)
	}

	// 查找网络接口并设置速率限制
	updated := false
	for key, value := range config.Data {
		if strings.HasPrefix(key, "net") {
			netConfig := value.(string)
			// 移除旧的 rate 参数
			parts := strings.Split(netConfig, ",")
			newParts := []string{}
			for _, part := range parts {
				if !strings.HasPrefix(strings.TrimSpace(part), "rate=") {
					newParts = append(newParts, part)
				}
			}
			// 添加新的 rate 参数 (MB/s，支持小数)
			newParts = append(newParts, fmt.Sprintf("rate=%.2f", rateMB))
			newNetConfig := strings.Join(newParts, ",")

			// 更新配置
			_, err := c.client.R().
				SetFormData(map[string]string{
					key: newNetConfig,
				}).
				Put(fmt.Sprintf("/nodes/%s/qemu/%d/config", c.config.Node, vmid))

			if err != nil {
				return fmt.Errorf("设置网络速率限制失败: %w", err)
			}
			updated = true
			break
		}
	}

	if !updated {
		return fmt.Errorf("未找到网络接口配置")
	}

	return nil
}

// GetVMConfig 获取虚拟机配置
func (c *Client) GetVMConfig(vmid int) (map[string]interface{}, error) {
	resp, err := c.client.R().
		Get(fmt.Sprintf("/nodes/%s/qemu/%d/config", c.config.Node, vmid))

	if err != nil {
		return nil, fmt.Errorf("获取虚拟机配置失败: %w", err)
	}

	var result struct {
		Data map[string]interface{} `json:"data"`
	}

	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("解析虚拟机配置失败: %w", err)
	}

	return result.Data, nil
}

// GetVMTags 获取虚拟机标签
func (c *Client) GetVMTags(vmid int) ([]string, error) {
	config, err := c.GetVMConfig(vmid)
	if err != nil {
		return nil, err
	}

	if tags, ok := config["tags"].(string); ok && tags != "" {
		return strings.Split(tags, ";"), nil
	}

	return []string{}, nil
}

// SetVMTags 设置虚拟机标签
func (c *Client) SetVMTags(vmid int, tags []string) error {
	// PVE 标签必须是小写，自动转换
	lowerTags := make([]string, len(tags))
	for i, tag := range tags {
		lowerTags[i] = strings.ToLower(tag)
	}

	// 将标签数组转换为分号分隔的字符串
	tagsStr := strings.Join(lowerTags, ";")

	// 更新虚拟机配置
	_, err := c.client.R().
		SetFormData(map[string]string{
			"tags": tagsStr,
		}).
		Put(fmt.Sprintf("/nodes/%s/qemu/%d/config", c.config.Node, vmid))

	if err != nil {
		return fmt.Errorf("设置虚拟机标签失败: %w", err)
	}

	return nil
}

// AddVMTag 为虚拟机添加单个标签（不覆盖现有标签）
func (c *Client) AddVMTag(vmid int, tag string) error {
	// PVE 标签必须是小写
	tag = strings.ToLower(tag)

	// 获取现有标签
	existingTags, err := c.GetVMTags(vmid)
	if err != nil {
		return fmt.Errorf("获取现有标签失败: %w", err)
	}

	// 检查标签是否已存在（不区分大小写）
	for _, existingTag := range existingTags {
		if strings.EqualFold(existingTag, tag) {
			return nil // 标签已存在，无需添加
		}
	}

	// 添加新标签
	existingTags = append(existingTags, tag)
	return c.SetVMTags(vmid, existingTags)
}

// RemoveVMTag 移除虚拟机的指定标签
func (c *Client) RemoveVMTag(vmid int, tag string) error {
	// PVE 标签必须是小写
	tag = strings.ToLower(tag)

	// 获取现有标签
	existingTags, err := c.GetVMTags(vmid)
	if err != nil {
		return fmt.Errorf("获取现有标签失败: %w", err)
	}

	// 过滤掉要移除的标签（不区分大小写）
	newTags := []string{}
	for _, existingTag := range existingTags {
		if !strings.EqualFold(existingTag, tag) {
			newTags = append(newTags, existingTag)
		}
	}

	return c.SetVMTags(vmid, newTags)
}

// AutoTagByTraffic 根据流量使用情况自动打标签（简单版本，已废弃，请使用 AutoTagByTrafficWithRule）
func (c *Client) AutoTagByTraffic(vmid int, trafficGB float64, threshold float64) error {
	// 移除旧标签
	c.RemoveVMTag(vmid, "traffic-limit")

	// 只在超限时打标签
	if trafficGB > threshold {
		return c.AddVMTag(vmid, "traffic-limit")
	}

	// 未超限：不打标签
	return nil
}

// AutoTagByTrafficWithRule 根据流量使用情况为特定规则打标签（每个规则独立标签）
func (c *Client) AutoTagByTrafficWithRule(vmid int, trafficGB float64, threshold float64, ruleName string) error {
	// 清理规则名，用于标签（移除空格，转小写）
	safeRuleName := strings.ToLower(strings.ReplaceAll(ruleName, " ", "-"))

	// 先移除该规则的旧标签
	c.RemoveVMTag(vmid, fmt.Sprintf("traffic-limit-%s", safeRuleName))

	// 只在超限时打标签
	if trafficGB > threshold {
		tag := fmt.Sprintf("traffic-limit-%s", safeRuleName)
		return c.AddVMTag(vmid, tag)
	}

	// 未超限：不打标签
	return nil
}

// GetVMCreationTime 获取虚拟机创建时间
func (c *Client) GetVMCreationTime(vmid int) (time.Time, error) {
	config, err := c.GetVMConfig(vmid)
	if err != nil {
		return time.Time{}, err
	}

	// PVE 在配置中有多个时间戳字段
	// 1. meta: creation timestamp (最准确)
	// 2. smbios1: uuid timestamp
	// 3. 配置文件的修改时间

	// 优先使用 meta 字段中的创建时间
	if meta, ok := config["meta"].(string); ok {
		// meta 格式: "creation-qemu=8.1.2,ctime=1703145600"
		parts := strings.Split(meta, ",")
		for _, part := range parts {
			if strings.HasPrefix(part, "ctime=") {
				timestampStr := strings.TrimPrefix(part, "ctime=")
				if timestamp, err := strconv.ParseInt(timestampStr, 10, 64); err == nil {
					return time.Unix(timestamp, 0), nil
				}
			}
		}
	}

	// 如果没有 meta 字段，尝试从其他来源获取
	// 使用虚拟机的配置文件修改时间作为后备
	resp, err := c.client.R().
		Get(fmt.Sprintf("/nodes/%s/qemu/%d/status/current", c.config.Node, vmid))

	if err == nil {
		var result struct {
			Data struct {
				Uptime int64 `json:"uptime"`
			} `json:"data"`
		}

		if json.Unmarshal(resp.Body(), &result) == nil && result.Data.Uptime > 0 {
			// 如果虚拟机正在运行，可以用当前时间减去运行时间作为估算
			// 但这不准确，仅作为最后的后备方案
		}
	}

	// 如果都获取不到，返回 Unix 纪元时间作为标记
	return time.Unix(0, 0), fmt.Errorf("无法获取虚拟机创建时间，请在虚拟机配置中添加 meta 标签")
}

// ParseVMID 解析字符串为 VMID
func ParseVMID(s string) (int, error) {
	return strconv.Atoi(s)
}

// ApplyRulesToVMs 为VM列表应用规则匹配
func ApplyRulesToVMs(vms []models.VMInfo, rules []models.Rule) []models.VMInfo {
	result := make([]models.VMInfo, len(vms))
	for i, vm := range vms {
		result[i] = vm
		result[i].MatchedRules = GetMatchedRulesForVM(vm, rules)
	}
	return result
}

// GetMatchedRulesForVM 获取VM匹配的规则名称列表
func GetMatchedRulesForVM(vm models.VMInfo, rules []models.Rule) []string {
	var matchedRules []string

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		if VMMatchesRule(vm, rule) {
			matchedRules = append(matchedRules, rule.Name)
		}
	}

	return matchedRules
}

// VMMatchesRule 检查VM是否匹配规则
func VMMatchesRule(vm models.VMInfo, rule models.Rule) bool {
	// 检查排除列表
	for _, excludeID := range rule.ExcludeVMIDs {
		if vm.VMID == excludeID {
			return false
		}
	}

	// 检查 VM ID 列表
	if len(rule.VMIDs) > 0 {
		matched := false
		for _, vmid := range rule.VMIDs {
			if vm.VMID == vmid {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// 检查标签（PVE 标签不区分大小写，统一转换为小写比较）
	if len(rule.VMTags) > 0 {
		matched := false
		for _, ruleTag := range rule.VMTags {
			ruleTagLower := strings.ToLower(ruleTag)

			for _, vmTag := range vm.Tags {
				vmTagLower := strings.ToLower(vmTag)

				// PVE 标签不区分大小写
				if vmTagLower == ruleTagLower {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}
