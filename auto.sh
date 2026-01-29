#!/bin/bash

# PVE Traffic Monitor - 自动管理脚本
# 支持：安装、启动、停止、调试、后台运行等功能

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 配置
APP_NAME="pve-traffic-monitor"
SERVICE_NAME="pve-traffic-monitor"
INSTALL_DIR="/opt/${APP_NAME}"
BIN_PATH="${INSTALL_DIR}/bin/monitor"
CONFIG_PATH="${INSTALL_DIR}/config.json"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
PID_FILE="/var/run/${APP_NAME}.pid"
LOG_FILE="/var/log/${APP_NAME}.log"

# 打印函数
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查是否为 root
check_root() {
    if [ "$EUID" -ne 0 ]; then
        print_error "请使用 root 权限运行此脚本"
        exit 1
    fi
}

# 检查服务是否已安装
is_service_installed() {
    [ -f "$SERVICE_FILE" ]
}

# 检查服务是否正在运行（systemctl）
is_service_running() {
    systemctl is-active --quiet ${SERVICE_NAME} 2>/dev/null
}

# 检查后台进程是否正在运行（nohup）
is_nohup_running() {
    if [ -f "$PID_FILE" ]; then
        local pid=$(cat "$PID_FILE")
        if ps -p "$pid" > /dev/null 2>&1; then
            return 0
        else
            rm -f "$PID_FILE"
            return 1
        fi
    fi
    return 1
}

# 获取运行状态
get_status() {
    if is_service_installed; then
        if is_service_running; then
            echo "service_running"
        else
            echo "service_stopped"
        fi
    elif is_nohup_running; then
        echo "nohup_running"
    else
        echo "not_running"
    fi
}

# 检查依赖工具
check_dependencies() {
    local missing_deps=()
    
    # 检查 Go
    if ! command -v go &> /dev/null; then
        missing_deps+=("go")
    fi
    
    # 检查前端构建工具（如果有 web 目录）
    if [ -d "web" ]; then
        if ! command -v node &> /dev/null; then
            missing_deps+=("node")
        fi
        if ! command -v npm &> /dev/null; then
            missing_deps+=("npm")
        fi
    fi
    
    # 如果有缺失的依赖，报错退出
    if [ ${#missing_deps[@]} -gt 0 ]; then
        print_error "缺少以下依赖工具: ${missing_deps[*]}"
        echo ""
        print_info "安装建议："
        for dep in "${missing_deps[@]}"; do
            case $dep in
                go)
                    echo "  - Go: https://golang.org/dl/ 或使用系统包管理器安装"
                    ;;
                node|npm)
                    echo "  - Node.js/npm: https://nodejs.org/ 或使用系统包管理器安装"
                    ;;
            esac
        done
        exit 1
    fi
}

# 安装 Go 模块依赖
install_go_deps() {
    print_info "检查 Go 模块依赖..."
    
    if [ ! -f "go.mod" ]; then
        print_error "未找到 go.mod 文件"
        return 1
    fi
    
    # 检查 go.sum 是否存在或是否需要更新
    if [ ! -f "go.sum" ] || [ "go.mod" -nt "go.sum" ]; then
        print_info "下载 Go 依赖..."
        go mod download
        go mod tidy
    else
        print_info "Go 依赖已是最新"
    fi
}

# 安装前端依赖
install_web_deps() {
    if [ ! -d "web" ]; then
        return 0
    fi
    
    print_info "检查前端依赖..."
    cd web
    
    # 检查 package.json 是否存在
    if [ ! -f "package.json" ]; then
        print_warning "未找到 package.json，跳过前端依赖安装"
        cd ..
        return 0
    fi
    
    # 检查是否需要安装依赖
    local need_install=false
    
    if [ ! -d "node_modules" ]; then
        need_install=true
        print_info "未找到 node_modules，需要安装前端依赖"
    elif [ "package.json" -nt "node_modules" ]; then
        need_install=true
        print_info "package.json 已更新，需要重新安装依赖"
    elif [ -f "package-lock.json" ] && [ "package-lock.json" -nt "node_modules" ]; then
        need_install=true
        print_info "package-lock.json 已更新，需要重新安装依赖"
    else
        print_info "前端依赖已是最新"
    fi
    
    # 安装依赖
    if [ "$need_install" = true ]; then
        print_info "安装前端依赖（这可能需要几分钟）..."
        npm install
        
        if [ $? -eq 0 ]; then
            print_success "前端依赖安装完成"
        else
            print_error "前端依赖安装失败"
            cd ..
            return 1
        fi
    fi
    
    cd ..
}

# 编译程序（前后端）
build() {
    print_info "开始完整构建（前后端）..."
    
    if [ ! -f "go.mod" ]; then
        print_error "未找到 go.mod 文件，请在项目根目录运行"
        exit 1
    fi
    
    # 检查必要的工具
    check_dependencies
    
    # 安装 Go 依赖
    install_go_deps || exit 1
    
    # 编译后端
    print_info "编译监控程序（后端）..."
    mkdir -p bin
    go build -o bin/monitor cmd/monitor/main.go cmd/monitor/debug.go
    
    if [ $? -eq 0 ]; then
        print_success "后端编译完成: bin/monitor"
    else
        print_error "后端编译失败"
        exit 1
    fi
    
    # 编译前端
    if [ -d "web" ]; then
        # 安装前端依赖
        install_web_deps || exit 1
        
        print_info "构建前端..."
        cd web
        npm run build
        
        if [ $? -eq 0 ] && [ -d "dist" ]; then
            print_success "前端构建完成: web/dist"
        else
            print_error "前端构建失败"
            cd ..
            exit 1
        fi
        cd ..
    else
        print_warning "未找到 web 目录，跳过前端构建"
    fi
    
    echo ""
    print_success "✨ 完整构建完成（和 make build-all 效果相同）"
    echo ""
    print_info "构建产物："
    print_info "  - 后端: bin/monitor"
    if [ -d "web/dist" ]; then
        print_info "  - 前端: web/dist/"
    fi
}

# 安装服务
install_service() {
    check_root
    
    print_info "开始安装 ${APP_NAME}..."
    
    # 编译程序
    build
    
    # 创建安装目录
    print_info "创建安装目录..."
    mkdir -p ${INSTALL_DIR}/{bin,data,exports}
    
    # 复制文件
    print_info "复制文件..."
    cp bin/monitor ${INSTALL_DIR}/bin/
    chmod +x ${INSTALL_DIR}/bin/monitor
    
    # 复制配置文件
    if [ ! -f "${CONFIG_PATH}" ]; then
        if [ -f "config.json" ]; then
            cp config.json ${CONFIG_PATH}
        elif [ -f "config.example.json" ]; then
            cp config.example.json ${CONFIG_PATH}
        else
            print_error "未找到配置文件"
            exit 1
        fi
        print_info "配置文件已复制到 ${CONFIG_PATH}"
    else
        print_warning "配置文件已存在，跳过"
    fi
    
    # 复制前端文件（如果存在）
    if [ -d "web/dist" ]; then
        print_info "复制前端文件..."
        mkdir -p ${INSTALL_DIR}/web
        cp -r web/dist ${INSTALL_DIR}/web/
    fi
    
    # 创建 systemd 服务文件
    print_info "创建 systemd 服务..."
    cat > ${SERVICE_FILE} << EOF
[Unit]
Description=PVE Traffic Monitor
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=${INSTALL_DIR}
ExecStart=${BIN_PATH} -config ${CONFIG_PATH}
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF
    
    # 重载 systemd
    systemctl daemon-reload
    
    print_success "安装完成！"
    print_info "配置文件: ${CONFIG_PATH}"
    print_info "使用 '$0 start' 启动服务"
}

# 卸载服务
uninstall_service() {
    check_root
    
    print_info "开始卸载 ${APP_NAME}..."
    
    # 停止服务
    if is_service_running; then
        print_info "停止服务..."
        systemctl stop ${SERVICE_NAME}
    fi
    
    # 禁用服务
    if is_service_installed; then
        print_info "禁用服务..."
        systemctl disable ${SERVICE_NAME} 2>/dev/null || true
    fi
    
    # 删除服务文件
    if [ -f "${SERVICE_FILE}" ]; then
        print_info "删除服务文件..."
        rm -f ${SERVICE_FILE}
        systemctl daemon-reload
    fi
    
    # 删除安装目录
    if [ -d "${INSTALL_DIR}" ]; then
        print_warning "删除安装目录: ${INSTALL_DIR}"
        read -p "是否保留数据目录？(y/n): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            print_info "保留数据目录..."
            mv ${INSTALL_DIR}/data ${INSTALL_DIR}_data_backup_$(date +%Y%m%d_%H%M%S) 2>/dev/null || true
        fi
        rm -rf ${INSTALL_DIR}
    fi
    
    print_success "卸载完成！"
}

# 启动服务
start_service() {
    local status=$(get_status)
    
    case $status in
        "service_running")
            print_warning "服务已在运行（systemctl）"
            systemctl status ${SERVICE_NAME} --no-pager
            ;;
        "service_stopped")
            print_info "启动服务（systemctl）..."
            check_root
            systemctl start ${SERVICE_NAME}
            sleep 1
            if is_service_running; then
                print_success "服务启动成功"
                systemctl status ${SERVICE_NAME} --no-pager
            else
                print_error "服务启动失败"
                exit 1
            fi
            ;;
        "nohup_running")
            print_warning "程序已在后台运行（nohup）"
            local pid=$(cat "$PID_FILE")
            print_info "PID: $pid"
            ;;
        "not_running")
            if is_service_installed; then
                print_info "启动服务（systemctl）..."
                check_root
                systemctl start ${SERVICE_NAME}
                sleep 1
                if is_service_running; then
                    print_success "服务启动成功"
                else
                    print_error "服务启动失败，查看日志: journalctl -u ${SERVICE_NAME} -n 50"
                    exit 1
                fi
            else
                print_error "服务未安装，使用 '$0 install' 安装或 '$0 nohup' 后台运行"
                exit 1
            fi
            ;;
    esac
}

# 停止服务
stop_service() {
    local status=$(get_status)
    
    case $status in
        "service_running")
            print_info "停止服务（systemctl）..."
            check_root
            systemctl stop ${SERVICE_NAME}
            print_success "服务已停止"
            ;;
        "nohup_running")
            print_info "停止后台进程（nohup）..."
            local pid=$(cat "$PID_FILE")
            kill $pid 2>/dev/null || true
            sleep 1
            if is_nohup_running; then
                print_warning "进程未响应，强制终止..."
                kill -9 $pid 2>/dev/null || true
            fi
            rm -f "$PID_FILE"
            print_success "后台进程已停止"
            ;;
        *)
            print_warning "程序未运行"
            ;;
    esac
}

# 后台运行（nohup）
start_nohup() {
    if is_service_installed; then
        print_error "已安装为系统服务，请使用 '$0 start' 或先卸载服务"
        exit 1
    fi
    
    if is_nohup_running; then
        print_warning "程序已在后台运行"
        local pid=$(cat "$PID_FILE")
        print_info "PID: $pid"
        return
    fi
    
    print_info "后台启动程序（nohup）..."
    
    # 查找配置文件
    local config="config.json"
    if [ ! -f "$config" ] && [ -f "config.example.json" ]; then
        config="config.example.json"
    fi
    
    if [ ! -f "$config" ]; then
        print_error "未找到配置文件"
        exit 1
    fi
    
    # 确保编译
    if [ ! -f "bin/monitor" ]; then
        build
    fi
    
    # 启动
    nohup ./bin/monitor -config $config > "$LOG_FILE" 2>&1 &
    local pid=$!
    echo $pid > "$PID_FILE"
    
    sleep 1
    
    if is_nohup_running; then
        print_success "程序已启动"
        print_info "PID: $pid"
        print_info "日志: $LOG_FILE"
    else
        print_error "启动失败，查看日志: $LOG_FILE"
        exit 1
    fi
}

# 调试模式
debug_mode() {
    if is_service_running || is_nohup_running; then
        print_error "程序正在运行，请先停止"
        exit 1
    fi
    
    print_info "启动调试模式..."
    
    # 查找配置文件
    local config="config.json"
    if [ ! -f "$config" ] && [ -f "config.example.json" ]; then
        config="config.example.json"
    fi
    
    if [ ! -f "$config" ]; then
        print_error "未找到配置文件"
        exit 1
    fi
    
    # 确保编译
    if [ ! -f "bin/monitor" ]; then
        build
    fi
    
    print_info "配置文件: $config"
    print_info "按 Ctrl+C 停止"
    echo "-----------------------------------"
    
    ./bin/monitor -config $config
}

# 查看状态
show_status() {
    local status=$(get_status)
    
    echo "========================================="
    echo "  PVE Traffic Monitor - 状态"
    echo "========================================="
    
    case $status in
        "service_running")
            print_success "状态: 运行中（systemctl）"
            echo ""
            systemctl status ${SERVICE_NAME} --no-pager
            ;;
        "service_stopped")
            print_warning "状态: 已停止（systemctl）"
            print_info "服务已安装但未启动"
            ;;
        "nohup_running")
            print_success "状态: 运行中（nohup）"
            local pid=$(cat "$PID_FILE")
            print_info "PID: $pid"
            print_info "日志: $LOG_FILE"
            ;;
        "not_running")
            print_warning "状态: 未运行"
            if is_service_installed; then
                print_info "服务已安装: $SERVICE_FILE"
            else
                print_info "服务未安装"
            fi
            ;;
    esac
    
    echo ""
    print_info "安装目录: $INSTALL_DIR"
    if [ -f "$CONFIG_PATH" ]; then
        print_info "配置文件: $CONFIG_PATH"
    fi
}

# 查看日志
show_logs() {
    local status=$(get_status)
    
    case $status in
        "service_running"|"service_stopped")
            if is_service_installed; then
                print_info "查看服务日志（按 Ctrl+C 退出）..."
                journalctl -u ${SERVICE_NAME} -f
            fi
            ;;
        "nohup_running")
            if [ -f "$LOG_FILE" ]; then
                print_info "查看日志文件（按 Ctrl+C 退出）..."
                tail -f "$LOG_FILE"
            else
                print_error "日志文件不存在"
            fi
            ;;
        *)
            print_error "程序未运行"
            ;;
    esac
}

# 启用开机自启
enable_autostart() {
    check_root
    
    if ! is_service_installed; then
        print_error "服务未安装，请先运行 '$0 install'"
        exit 1
    fi
    
    print_info "启用开机自启..."
    systemctl enable ${SERVICE_NAME}
    print_success "已启用开机自启"
}

# 禁用开机自启
disable_autostart() {
    check_root
    
    if ! is_service_installed; then
        print_error "服务未安装"
        exit 1
    fi
    
    print_info "禁用开机自启..."
    systemctl disable ${SERVICE_NAME}
    print_success "已禁用开机自启"
}

# 重启服务
restart_service() {
    stop_service
    sleep 1
    start_service
}

# 显示帮助
show_help() {
    cat << EOF
PVE Traffic Monitor - 自动管理脚本

用法: $0 <命令>

命令:
  install       安装服务（systemd）
  uninstall     卸载服务
  
  start         启动服务（自动判断 systemctl/nohup）
  stop          停止服务（自动判断 systemctl/nohup）
  restart       重启服务
  
  nohup         后台运行（使用 nohup，不安装服务）
  debug         调试模式（前台运行，查看日志）
  
  status        查看运行状态
  logs          查看日志
  
  enable        启用开机自启（需要已安装服务）
  disable       禁用开机自启
  
  build         编译前后端（同 make build-all）
  help          显示此帮助信息

示例:
  # 首次使用 - 安装并启动
  sudo $0 install
  sudo $0 start
  sudo $0 enable

  # 开发调试
  $0 build      # 完整构建（前后端）
  $0 debug

  # 临时运行（不安装服务）
  $0 nohup
  $0 stop

  # 查看状态和日志
  $0 status
  $0 logs

注意:
  - build 命令会同时编译后端和构建前端（等同于 make build-all）
  - 前端需要 Node.js 和 npm 支持
  - 如果没有前端环境，系统会使用内嵌的简化版 Web 界面

EOF
}

# 主函数
main() {
    case "${1:-}" in
        install)
            install_service
            ;;
        uninstall)
            uninstall_service
            ;;
        start)
            start_service
            ;;
        stop)
            stop_service
            ;;
        restart)
            restart_service
            ;;
        nohup)
            start_nohup
            ;;
        debug)
            debug_mode
            ;;
        status)
            show_status
            ;;
        logs)
            show_logs
            ;;
        enable)
            enable_autostart
            ;;
        disable)
            disable_autostart
            ;;
        build)
            build
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            print_error "未知命令: ${1:-}"
            echo ""
            show_help
            exit 1
            ;;
    esac
}

# 执行主函数
main "$@"
