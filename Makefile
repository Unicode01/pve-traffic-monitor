.PHONY: all build monitor web build-all test clean install uninstall check-deps install-deps install-go-deps install-web-deps

# 默认目标
all: build

# 检查依赖工具
check-deps:
	@echo "检查依赖工具..."
	@command -v go >/dev/null 2>&1 || { echo "错误: 未安装 Go，请访问 https://golang.org/dl/"; exit 1; }
	@if [ -d "web" ]; then \
		command -v node >/dev/null 2>&1 || { echo "错误: 未安装 Node.js，请访问 https://nodejs.org/"; exit 1; }; \
		command -v npm >/dev/null 2>&1 || { echo "错误: 未安装 npm，请访问 https://nodejs.org/"; exit 1; }; \
	fi
	@echo "✓ 依赖工具检查通过"

# 安装 Go 依赖
install-go-deps:
	@echo "检查 Go 模块依赖..."
	@if [ ! -f "go.sum" ] || [ "go.mod" -nt "go.sum" ]; then \
		echo "下载 Go 依赖..."; \
		go mod download; \
		go mod tidy; \
	else \
		echo "✓ Go 依赖已是最新"; \
	fi

# 安装前端依赖
install-web-deps:
	@if [ -d "web" ]; then \
		echo "检查前端依赖..."; \
		cd web && { \
			if [ ! -d "node_modules" ] || [ "package.json" -nt "node_modules" ]; then \
				echo "安装前端依赖（这可能需要几分钟）..."; \
				npm install; \
				echo "✓ 前端依赖安装完成"; \
			else \
				echo "✓ 前端依赖已是最新"; \
			fi; \
		}; \
	fi

# 安装所有依赖
install-deps: check-deps install-go-deps install-web-deps
	@echo "✓ 所有依赖已就绪"

# 编译监控程序
monitor: install-go-deps
	@echo "编译监控程序..."
	@mkdir -p bin
	@go build -o bin/monitor cmd/monitor/main.go cmd/monitor/debug.go
	@echo "✓ 后端编译完成: bin/monitor"

# 编译所有程序
build: monitor
	@echo "编译完成"

# 构建前端
web: install-web-deps
	@echo "构建前端..."
	@cd web && npm run build
	@echo "✓ 前端构建完成: web/dist"

# 编译程序 + 构建前端（自动安装依赖）
build-all: check-deps monitor web
	@echo ""
	@echo "✨ 完整构建完成"
	@echo ""
	@echo "构建产物："
	@echo "  - 后端: bin/monitor"
	@if [ -d "web/dist" ]; then echo "  - 前端: web/dist/"; fi

# 运行测试
test:
	@echo "运行测试..."
	@go test -v ./...

# 清理构建文件
clean:
	@echo "清理构建文件..."
	@rm -rf bin/
	@rm -rf web/dist/
	@rm -rf web/node_modules/

# 安装到 /opt（推荐使用 ./auto.sh install 代替）
install: build
	@echo "安装到 /opt/pve-traffic-monitor..."
	@sudo mkdir -p /opt/pve-traffic-monitor/{bin,data,exports}
	@sudo cp -r bin/ /opt/pve-traffic-monitor/
	@sudo cp config.example.json /opt/pve-traffic-monitor/config.json
	@echo "注意: 推荐使用 './auto.sh install' 进行完整安装"
	@echo "安装完成（服务文件需手动创建或使用 auto.sh）"

# 卸载
uninstall:
	@echo "卸载..."
	@sudo systemctl stop pve-traffic-monitor || true
	@sudo systemctl disable pve-traffic-monitor || true
	@sudo rm -f /etc/systemd/system/pve-traffic-monitor.service
	@sudo rm -rf /opt/pve-traffic-monitor
	@sudo systemctl daemon-reload
	@echo "卸载完成"
