package pve

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

// DebugMode 是否启用调试模式
var DebugMode = false

func init() {
	// 从环境变量读取调试模式设置
	if os.Getenv("PVE_DEBUG") == "1" || os.Getenv("PVE_DEBUG") == "true" {
		DebugMode = true
	}
}

// EnableDebug 启用调试模式
func EnableDebug() {
	DebugMode = true
}

// DisableDebug 禁用调试模式
func DisableDebug() {
	DebugMode = false
}

// debugLog 输出调试日志
func debugLog(format string, args ...interface{}) {
	if DebugMode {
		log.Printf("[PVE DEBUG] "+format, args...)
	}
}

// TestConnection 测试 PVE 连接
func (c *Client) TestConnection() error {
	debugLog("测试 PVE 连接...")
	debugLog("  主机: %s:%d", c.config.Host, c.config.Port)
	debugLog("  节点: %s", c.config.Node)

	// 测试基本连接
	resp, err := c.client.R().Get("/version")
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}

	debugLog("  响应状态码: %d", resp.StatusCode())
	debugLog("  响应内容: %s", string(resp.Body()))

	if resp.StatusCode() != 200 {
		return fmt.Errorf("连接测试失败: HTTP %d", resp.StatusCode())
	}

	return nil
}

// GetNodeInfo 获取节点信息（用于调试）
func (c *Client) GetNodeInfo() (map[string]interface{}, error) {
	debugLog("获取节点信息...")

	resp, err := c.client.R().
		Get(fmt.Sprintf("/nodes/%s/status", c.config.Node))

	if err != nil {
		return nil, fmt.Errorf("获取节点信息失败: %w", err)
	}

	debugLog("  响应状态码: %d", resp.StatusCode())
	debugLog("  响应内容: %s", string(resp.Body()))

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("获取节点信息失败: HTTP %d", resp.StatusCode())
	}

	var result struct {
		Data map[string]interface{} `json:"data"`
	}

	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("解析节点信息失败: %w", err)
	}

	return result.Data, nil
}

// PrintDebugInfo 打印调试信息
func (c *Client) PrintDebugInfo() {
	fmt.Println("========================================")
	fmt.Println("PVE 客户端调试信息")
	fmt.Println("========================================")
	fmt.Printf("主机: %s:%d\n", c.config.Host, c.config.Port)
	fmt.Printf("节点: %s\n", c.config.Node)
	fmt.Printf("API Token ID: %s\n", c.config.APITokenID)
	fmt.Println("========================================")

	// 测试连接
	if err := c.TestConnection(); err != nil {
		fmt.Printf("❌ 连接测试失败: %v\n", err)
	}

	// 获取节点信息
	if info, err := c.GetNodeInfo(); err != nil {
		fmt.Printf("❌ 获取节点信息失败: %v\n", err)
	} else {
		fmt.Println("\n节点信息:")
		for key, value := range info {
			fmt.Printf("  %s: %v\n", key, value)
		}
	}

	// 尝试获取虚拟机列表
	fmt.Println("\n尝试获取虚拟机列表...")
	if vms, err := c.GetAllVMs(); err != nil {
		fmt.Printf("❌ 获取虚拟机列表失败: %v\n", err)
	} else {
		fmt.Printf("✓ 成功获取 %d 个虚拟机\n", len(vms))
		for i, vm := range vms {
			if i < 5 { // 只显示前 5 个
				fmt.Printf("  - VM %d: %s (%s)\n", vm.VMID, vm.Name, vm.Status)
			}
		}
		if len(vms) > 5 {
			fmt.Printf("  ... 还有 %d 个虚拟机\n", len(vms)-5)
		}
	}

	fmt.Println("========================================")
}
