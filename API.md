# API 接口文档

PVE 流量监控系统提供了 RESTful API 接口，支持通过 HTTP 访问监控数据。

## 基础信息

- **默认地址**: `http://localhost:8080`
- **协议**: HTTP
- **数据格式**: JSON
- **CORS**: 已启用（支持跨域访问）

## 启用 API

在配置文件 `config.json` 中设置：

```json
{
  "api": {
    "enabled": true,
    "host": "0.0.0.0",
    "port": 8080
  }
}
```

## Web 界面

### GET /

访问 Web 监控界面。

**示例**:
```
http://localhost:8080/
```

浏览器访问后可查看：
- 虚拟机总数统计
- 运行中虚拟机数量
- 总流量统计
- 虚拟机列表（实时刷新）

---

## API 端点

### 1. 获取所有虚拟机

**请求**:
```
GET /api/vms
```

**响应**:
```json
{
  "success": true,
  "data": [
    {
      "vmid": 100,
      "name": "vm-100",
      "status": "running",
      "tags": ["web", "production"],
      "netrx": 1073741824,
      "nettx": 536870912,
      "last_updated": "0001-01-01T00:00:00Z"
    }
  ]
}
```

**字段说明**:
- `vmid`: 虚拟机 ID
- `name`: 虚拟机名称
- `status`: 状态（running/stopped）
- `tags`: 标签列表
- `netrx`: 接收字节数（累计）
- `nettx`: 发送字节数（累计）

**curl 示例**:
```bash
curl http://localhost:8080/api/vms
```

---

### 2. 获取单个虚拟机详情

**请求**:
```
GET /api/vm/{vmid}
```

**参数**:
- `vmid`: 虚拟机 ID

**响应**:
```json
{
  "success": true,
  "data": {
    "vm": {
      "vmid": 100,
      "name": "vm-100",
      "status": "running",
      "tags": ["web"],
      "netrx": 1073741824,
      "nettx": 536870912,
      "last_updated": "0001-01-01T00:00:00Z"
    },
    "stats": {
      "hour": {
        "vmid": 100,
        "period": "hour",
        "start_time": "2024-01-24T10:00:00Z",
        "end_time": "2024-01-24T11:00:00Z",
        "total_bytes": 10485760,
        "total_gb": 0.01
      },
      "day": {
        "vmid": 100,
        "period": "day",
        "start_time": "2024-01-24T00:00:00Z",
        "end_time": "2024-01-24T11:00:00Z",
        "total_bytes": 1073741824,
        "total_gb": 1.0
      },
      "month": {
        "vmid": 100,
        "period": "month",
        "start_time": "2024-01-01T00:00:00Z",
        "end_time": "2024-01-24T11:00:00Z",
        "total_bytes": 107374182400,
        "total_gb": 100.0
      }
    }
  }
}
```

**curl 示例**:
```bash
curl http://localhost:8080/api/vm/100
```

---

### 3. 获取流量统计

**请求**:
```
GET /api/stats?period={period}
```

**参数**:
- `period`: 统计周期（hour/day/month），默认 day

**响应**:
```json
{
  "success": true,
  "data": [
    {
      "vmid": 100,
      "period": "day",
      "start_time": "2024-01-24T00:00:00Z",
      "end_time": "2024-01-24T11:00:00Z",
      "total_bytes": 1073741824,
      "total_gb": 1.0
    },
    {
      "vmid": 101,
      "period": "day",
      "start_time": "2024-01-24T00:00:00Z",
      "end_time": "2024-01-24T11:00:00Z",
      "total_bytes": 536870912,
      "total_gb": 0.5
    }
  ]
}
```

**curl 示例**:
```bash
# 获取日统计
curl http://localhost:8080/api/stats?period=day

# 获取月统计
curl http://localhost:8080/api/stats?period=month
```

---

### 4. 获取操作日志

**请求**:
```
GET /api/logs?start={start}&end={end}
```

**参数**（可选）:
- `start`: 开始时间（RFC3339 格式）
- `end`: 结束时间（RFC3339 格式）
- 默认：最近 7 天

**响应**:
```json
{
  "success": true,
  "data": [
    {
      "vmid": 100,
      "rule_name": "monthly_limit",
      "action": "shutdown",
      "reason": "超出流量限制: 1050.00 GB / 1000.00 GB",
      "timestamp": "2024-01-24T10:30:00Z",
      "success": true,
      "error": ""
    }
  ]
}
```

**字段说明**:
- `action`: 执行的操作（shutdown/rate_limit）
- `reason`: 操作原因
- `success`: 是否执行成功
- `error`: 错误信息（如果有）

**curl 示例**:
```bash
# 获取最近 7 天日志
curl http://localhost:8080/api/logs

# 获取指定时间范围
curl "http://localhost:8080/api/logs?start=2024-01-20T00:00:00Z&end=2024-01-24T23:59:59Z"
```

---

### 5. 获取规则列表

**请求**:
```
GET /api/rules
```

**响应**:
```json
{
  "success": true,
  "data": [
    {
      "name": "monthly_limit",
      "enabled": true,
      "period": "month",
      "limit_gb": 1000,
      "action": "shutdown",
      "rate_limit_mb": 0,
      "vm_ids": [100, 101],
      "vm_tags": ["monitored"],
      "exclude_vm_ids": []
    }
  ]
}
```

**curl 示例**:
```bash
curl http://localhost:8080/api/rules
```

---

## 错误响应

当发生错误时，API 返回：

```json
{
  "success": false,
  "error": "错误信息"
}
```

**HTTP 状态码**:
- `200 OK`: 请求成功
- `400 Bad Request`: 请求参数错误
- `500 Internal Server Error`: 服务器内部错误

---

## 前端集成示例

### JavaScript (Fetch API)

```javascript
// 获取虚拟机列表
async function getVMs() {
    const response = await fetch('http://localhost:8080/api/vms');
    const data = await response.json();
    
    if (data.success) {
        console.log('虚拟机列表:', data.data);
    }
}

// 获取流量统计
async function getStats(period = 'day') {
    const response = await fetch(`http://localhost:8080/api/stats?period=${period}`);
    const data = await response.json();
    
    if (data.success) {
        data.data.forEach(stat => {
            console.log(`VM ${stat.vmid}: ${stat.total_gb} GB`);
        });
    }
}
```

### Python

```python
import requests

# 获取虚拟机列表
response = requests.get('http://localhost:8080/api/vms')
data = response.json()

if data['success']:
    for vm in data['data']:
        print(f"VM {vm['vmid']}: {vm['name']} - {vm['status']}")

# 获取流量统计
response = requests.get('http://localhost:8080/api/stats?period=day')
data = response.json()

if data['success']:
    for stat in data['data']:
        print(f"VM {stat['vmid']}: {stat['total_gb']:.2f} GB")
```

### 使用 jq 处理 JSON

```bash
# 获取所有运行中的虚拟机
curl -s http://localhost:8080/api/vms | jq '.data[] | select(.status=="running")'

# 获取流量最大的虚拟机
curl -s http://localhost:8080/api/stats | jq '.data | sort_by(.total_gb) | reverse | .[0]'

# 统计总流量
curl -s http://localhost:8080/api/stats | jq '[.data[].total_gb] | add'
```

---

## 安全建议

### 1. 限制访问地址

仅允许本地访问：

```json
{
  "api": {
    "host": "127.0.0.1",
    "port": 8080
  }
}
```

### 2. 使用 Nginx 反向代理 + 认证

```nginx
server {
    listen 80;
    server_name monitor.example.com;
    
    location / {
        auth_basic "Restricted Access";
        auth_basic_user_file /etc/nginx/.htpasswd;
        
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
    }
}
```

### 3. 防火墙规则

```bash
# 仅允许特定 IP 访问
iptables -A INPUT -p tcp --dport 8080 -s 192.168.1.0/24 -j ACCEPT
iptables -A INPUT -p tcp --dport 8080 -j DROP
```

---

## 开发调试

启用详细日志：

```bash
# 查看 API 请求日志
sudo journalctl -u pve-traffic-monitor -f | grep API
```

测试所有端点：

```bash
#!/bin/bash
BASE_URL="http://localhost:8080"

echo "测试 API 端点..."
echo ""

echo "1. 获取虚拟机列表"
curl -s "$BASE_URL/api/vms" | jq .
echo ""

echo "2. 获取虚拟机 100 详情"
curl -s "$BASE_URL/api/vm/100" | jq .
echo ""

echo "3. 获取流量统计"
curl -s "$BASE_URL/api/stats?period=day" | jq .
echo ""

echo "4. 获取日志"
curl -s "$BASE_URL/api/logs" | jq .
echo ""

echo "5. 获取规则"
curl -s "$BASE_URL/api/rules" | jq .
```
