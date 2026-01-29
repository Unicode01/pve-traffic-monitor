# PVE 虚拟机流量监控系统

一个轻量级的 Proxmox VE 流量监控系统，完全通过 PVE API 和 API Token 运行，支持自动流量限制、可选的 Web 可视化监控和图表导出。

## ✨ 核心功能

- 🔌 **完全基于 PVE API**: 使用 PVE API Token 认证，无需 root 权限或密码
- 🔍 **自动发现虚拟机**: 通过 PVE API 自动获取节点上的所有虚拟机
- 📊 **实时流量监控**: 持续监控虚拟机网络流量（上传/下载）
- ⏰ **多周期限制**: 支持按小时/天/月设置流量限制
- 🚫 **自动操作**: 超出流量限制后自动关机或限速
- 🌐 **可选 Web 界面**: Vue 3 + Element Plus，支持明暗主题和中英文切换（可禁用）
- 📈 **多格式导出**: 支持导出 JSON/PNG/HTML（go-echarts）格式
- 🎨 **统一配色**: 前后端使用一致的配色方案
- 🏷️ **标签过滤**: 支持根据虚拟机标签应用不同规则
- 💾 **数据持久化**: 支持文件存储和数据库（MySQL/PostgreSQL/SQLite）
- 🖥️ **无头运行**: 可以完全无 Web 界面运行，仅通过 CLI 导出数据

## 🚀 快速开始

### 1. 下载依赖

```bash
go mod download
```

### 2. 配置

#### 方式一：交互式配置生成（推荐）

```bash
# 使用配置生成向导
./auto.sh config
```

配置向导会引导你完成以下配置：
- **PVE 连接配置**: 主机地址、端口、节点名称、API Token
- **监控配置**: 监控间隔、数据保留天数
- **存储配置**: 支持文件/SQLite/MySQL/PostgreSQL
- **Web API 配置**: 是否启用 Web 界面
- **流量规则配置**: 支持创建多条规则，每条规则可配置：
  - 规则名称和周期（小时/天/月）
  - 流量方向（双向/上传/下载）
  - 流量限制和超限操作（关机/断网/限速）
  - 匹配的虚拟机标签

#### 方式二：手动复制配置

```bash
# 复制配置文件
cp config.example.json config.json

# 编辑配置
nano config.json
```

### 3. 使用管理脚本

本项目提供统一的管理脚本 `auto.sh`，支持多种运行方式：

#### 安装为系统服务（推荐）

```bash
# 安装服务
sudo ./auto.sh install

# 启动服务
sudo ./auto.sh start

# 启用开机自启
sudo ./auto.sh enable

# 查看状态
./auto.sh status

# 查看日志
./auto.sh logs
```

#### 临时运行（不安装服务）

```bash
# 后台运行（使用 nohup）
./auto.sh nohup

# 停止后台运行
./auto.sh stop

# 查看状态
./auto.sh status
```

#### 开发调试

```bash
# 编译程序（前后端完整构建）
./auto.sh build

# 调试模式（前台运行）
./auto.sh debug
```

#### 其他操作

```bash
# 重启服务
sudo ./auto.sh restart

# 卸载服务
sudo ./auto.sh uninstall

# 查看帮助
./auto.sh help
```

### 4. 访问 Web 界面（可选）

如果启用了 Web API (`api.enabled: true`)，可以在浏览器中访问：**http://服务器IP:8080**

**Web 界面功能：**
- ✨ 现代化 UI 设计（Vue 3 + Element Plus）
- 🌓 明暗主题切换
- 🌍 中英文切换
- 📊 ECharts 交互式图表
- 📱 响应式布局

**无 Web 界面运行**：
```json
{
  "api": {
    "enabled": false    // 禁用 Web API，仅后台监控
  }
}
```

**前端开发（可选）：**
```bash
cd web
npm install      # 安装依赖
npm run dev      # 开发模式
npm run build    # 构建生产版本
```

## 📋 详细配置

### PVE 连接配置

```json
{
  "pve": {
    "host": "localhost",                      // PVE 服务器地址
    "port": 8006,                             // PVE API 端口
    "node": "pve",                            // 节点名称
    "api_token_id": "monitor@pve!token",      // API Token ID
    "api_token_secret": "xxxxxxxx-xxxx-..."   // API Token Secret
  }
}
```

**认证方式**（使用 API Token，推荐）:
1. **配置文件**: 在配置文件中填写 `api_token_id` 和 `api_token_secret`
2. **环境变量**: 设置 `PVE_API_TOKEN_ID` 和 `PVE_API_TOKEN_SECRET` 环境变量

**创建 API Token**:
```bash
# 在 PVE Web 界面中：
# 数据中心 → 权限 → API Tokens → 添加
# 用户: monitor@pve
# Token ID: traffic-monitor
# 权限分离: 不勾选（使用完整权限）
```

### 监控配置

```json
{
  "monitor": {
    "interval_seconds": 60,         // 监控间隔（秒），建议 60-300
    "export_path": "./exports",     // 图表导出路径
    "include_templates": false,     // 是否包含模板虚拟机
    "data_retention_days": 90       // 数据保留天数（0=永久保留）
  }
}
```

### 存储配置

```json
{
  "storage": {
    "type": "sqlite",                    // 存储类型: file, mysql, postgresql, sqlite
    "dsn": "./data/pve_traffic.db",      // 数据库连接字符串
    "max_open_conns": 10,                // 最大打开连接数
    "max_idle_conns": 5,                 // 最大空闲连接数
    "conn_max_lifetime": 3600            // 连接最大生命周期（秒）
  }
}
```

**存储类型说明**:
- `file`: 文件存储（JSON格式）
- `sqlite`: SQLite 数据库（推荐，轻量级）
- `mysql`: MySQL/MariaDB 数据库
- `postgresql`: PostgreSQL 数据库

### API 配置

```json
{
  "api": {
    "enabled": true,        // 是否启用 Web API（可选，仅用于 Web 界面）
    "host": "0.0.0.0",      // 监听地址，0.0.0.0 表示所有接口
    "port": 8080            // 监听端口
  }
}
```

**说明**: 
- `api.enabled: true` - 启用 Web 界面和 HTTP API
- `api.enabled: false` - 禁用 Web 界面，系统仍然正常运行，仅通过后台监控和 CLI 导出
- 系统核心功能完全通过 **PVE API** 运行，本配置的 API 仅用于 Web 可视化

### 流量规则配置

```json
{
  "rules": [
    {
      "name": "monthly_limit",          // 规则名称
      "enabled": true,                  // 是否启用
      "period": "month",                // 周期: hour/day/month
      "limit_gb": 1000,                 // 流量限制（GB）
      "action": "shutdown",             // 操作: shutdown/rate_limit
      "rate_limit_mb": 10,              // 限速值 MB/s（仅 rate_limit 操作）
      "vm_ids": [100, 101],             // 指定虚拟机 ID（空=所有）
      "vm_tags": ["monitored"],         // 指定标签（空=不过滤）
      "exclude_vm_ids": [999]           // 排除的虚拟机
    }
  ]
}
```

**规则匹配逻辑**:
- 如果虚拟机在 `exclude_vm_ids` 中，跳过（优先级最高）
- 如果 `vm_ids` 非空，虚拟机必须在列表中
- 如果 `vm_tags` 非空，虚拟机必须包含至少一个标签
- 如果 `vm_ids` 和 `vm_tags` 都为空，匹配所有虚拟机（除排除列表）

## 🌐 Web API 接口（可选）

启用 Web API (`api.enabled: true`) 后，可以通过 HTTP 访问以下接口：

### 页面

- `GET /` - Web 监控界面

### API 端点

- `GET /api/vms` - 获取所有虚拟机列表
- `GET /api/vm/{vmid}` - 获取单个虚拟机详情
- `GET /api/stats?period=day` - 获取流量统计
- `GET /api/logs` - 获取操作日志
- `GET /api/rules` - 获取规则列表

**示例**:
```bash
# 获取所有虚拟机
curl http://localhost:8080/api/vms

# 获取虚拟机 100 的详情
curl http://localhost:8080/api/vm/100

# 获取日流量统计
curl http://localhost:8080/api/stats?period=day
```

**注意**: Web API 仅用于可视化监控，系统核心功能（流量监控、规则执行）完全基于 PVE API，即使禁用 Web API 也能正常运行。

## 📊 导出流量图表

CLI 工具支持多种导出格式：**JSON**、**PNG**、**HTML**（默认）

### 快速示例

```bash
# 导出为交互式 HTML（默认格式，推荐）
./bin/monitor -config config.json -export 100 -period day

# 导出为 HTML（暗色主题）
./bin/monitor -config config.json -export 100 -period day -format html -dark

# 导出为 JSON（数据分析）
./bin/monitor -config config.json -export 100 -period day -format json

# 导出为 PNG（报告文档）
./bin/monitor -config config.json -export 100 -period day -format png

# 导出所有虚拟机的汇总（HTML）
./bin/monitor -config config.json -export all -period day -format html

# 导出指定日期的数据
./bin/monitor -config config.json -export 100 -date 2024-01-15 -format html

# 导出指定时间范围
./bin/monitor -config config.json -export 100 -start "2024-01-01" -end "2024-01-31" -format html
```

**参数说明：**
- `-format`: 导出格式（json/png/html），默认：html
- `-dark`: 使用暗色主题（仅 HTML 格式）
- `-direction`: 流量方向（both/rx/tx）
- `-period`: 统计周期（hour/day/month）
- `-date`: 指定日期（格式：2006-01-02）
- `-start` 和 `-end`: 自定义时间范围

图表保存在配置的 `export_path` 目录中。

## 🗑️ 清除历史数据

系统提供了灵活的数据清除功能，支持清除指定时间段或VM的历史数据。

### 基本用法

```bash
# 清除指定时间段的数据（所有VM）
./bin/monitor -config config.json -cleanup range -start "2024-01-01" -end "2024-01-31" -dry-run

# 清除指定VM某天的数据
./bin/monitor -config config.json -cleanup vm -vmid 100 -date 2024-01-15 -dry-run

# 清除指定日期之前的所有数据
./bin/monitor -config config.json -cleanup before -before 2024-01-01 -dry-run
```

**参数说明**:
- `-cleanup`: 清除类型 (`range`/`vm`/`before`)
- `-vmid`: 虚拟机ID（cleanup=vm时必需）
- `-date`: 指定日期（格式: 2006-01-02）
- `-start` / `-end`: 时间范围
- `-before`: 删除此日期之前的数据
- `-dry-run`: 预览模式，不实际删除

**注意**: 建议先使用 `-dry-run` 预览，删除操作不可恢复。

## 📁 数据存储

### 文件存储模式 (type: file)
```
data/
├── vm_100/
│   ├── traffic_2024-01.json      # 2024年1月流量记录
│   └── traffic_2024-02.json
├── logs/
│   ├── actions_2024-01-15.json   # 操作日志
│   └── actions_2024-01-16.json
└── states/
    └── vm_100_state.json          # 虚拟机状态
```

### 数据库存储模式 (type: sqlite/mysql/postgresql)
- `traffic_records`: 流量记录表
- `action_logs`: 操作日志表
- `vm_states`: 虚拟机状态表

## 🛠️ 管理脚本命令

```bash
# 查看所有命令
./auto.sh help

# 配置生成
./auto.sh config           # 交互式配置生成向导（支持多规则）

# 服务管理
sudo ./auto.sh install      # 安装服务
sudo ./auto.sh start        # 启动服务
sudo ./auto.sh stop         # 停止服务
sudo ./auto.sh restart      # 重启服务
sudo ./auto.sh uninstall    # 卸载服务

# 开机自启
sudo ./auto.sh enable       # 启用开机自启
sudo ./auto.sh disable      # 禁用开机自启

# 后台运行（不安装服务）
./auto.sh nohup            # 后台运行
./auto.sh stop             # 停止后台运行

# 状态和日志
./auto.sh status           # 查看状态
./auto.sh logs             # 查看日志

# 开发
./auto.sh build            # 完整构建（前后端，同 make build-all）
./auto.sh debug            # 调试模式
```

## 🔧 编译和构建

```bash
# 使用管理脚本（推荐）
./auto.sh build             # 完整构建（前后端，等同于 make build-all）

# 或使用 Makefile
make build                  # 仅编译后端
make web                    # 仅构建前端（需要 npm）
make build-all              # 完整构建（前后端）
make clean                  # 清理构建文件

# 前端开发
cd web
npm install                 # 安装依赖
npm run dev                 # 开发模式
npm run build               # 构建生产版本
```

**说明**: `./auto.sh build` 现在会同时编译后端和构建前端，效果等同于 `make build-all`。

## ❓ 常见问题

### 1. 服务无法启动

```bash
# 查看详细日志
./auto.sh logs

# 检查配置文件
cat /opt/pve-traffic-monitor/config.json

# 调试模式运行
./auto.sh debug
```

### 2. Web 界面无法访问

- 检查配置 `api.enabled` 是否为 `true`
- 确认端口未被占用：`netstat -tlnp | grep 8080`
- 检查防火墙设置
- 注意：Web 界面是可选功能，禁用后系统仍然正常运行

### 3. 虚拟机未被监控

- 检查 PVE API Token 权限是否正确
- 检查规则配置中的 `vm_ids` 和 `vm_tags`
- 查看日志: `./auto.sh logs`
- 确认虚拟机在 PVE 中存在
- 验证 API Token: 在配置文件或环境变量中正确设置

### 4. 前端显示异常

- 检查是否构建了前端: `ls -la /opt/pve-traffic-monitor/web/dist/`
- 重新构建: `cd web && npm run build`
- 未构建会使用内嵌的简化版本

### 5. API Token 认证失败

- 检查 Token 格式: `user@realm!tokenid`（例如：`monitor@pve!traffic-monitor`）
- 确认 Token 未过期且权限正确
- 在 PVE Web 界面重新生成 Token
- 检查配置文件或环境变量中的 Token 是否正确

## 🔒 安全建议

1. **保护 API Token**: API Token 拥有完整权限，请妥善保管
   ```bash
   chmod 600 config.json
   ```

2. **限制 API 访问**: 修改 `api.host` 为 `127.0.0.1` 仅允许本地访问

3. **使用防火墙**: 限制 API 端口的访问来源
   ```bash
   # 仅允许特定 IP 访问
   iptables -A INPUT -p tcp --dport 8080 -s 192.168.1.0/24 -j ACCEPT
   iptables -A INPUT -p tcp --dport 8080 -j DROP
   ```

4. **定期备份**: 备份数据库或 `data` 目录中的数据

5. **最小权限原则**: 为 API Token 分配所需的最小权限集

## 📝 系统要求

- **操作系统**: Linux (Debian/Ubuntu/CentOS) 或任何支持 Go 的系统
- **Proxmox VE**: 6.0 或更高版本
- **Go**: 1.23+ (仅编译时需要)
- **Node.js**: 16+ (仅前端开发/构建时需要)
- **权限**: 需要 PVE API Token（在 PVE Web 界面中创建）
- **网络**: 能够访问 PVE API（默认端口 8006）

## 📄 许可证

MIT License - 详见 [LICENSE](LICENSE)

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

---

**提示**: 
- 首次使用建议先在测试虚拟机上验证功能
- 系统核心功能完全基于 **PVE API** 运行，使用 API Token 认证
- Web API (`api.enabled`) 是可选的，仅用于 Web 界面，可以完全无 Web 运行
- `./auto.sh build` 会同时编译后端和构建前端
- 即使禁用 Web API，仍可使用 CLI 导出图表和查看数据
