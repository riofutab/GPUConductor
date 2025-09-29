# GPUConductor Makefile

.PHONY: build clean install run-server run-agent test deps

# 构建配置
BINARY_NAME=gcond
BUILD_DIR=build
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X main.Version=$(VERSION)"

# 默认目标
all: build

# 安装依赖
deps:
	go mod tidy
	go mod download

# 构建
build: deps
	@echo "构建 GPUConductor..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/gcond

# 构建所有平台
build-all: deps
	@echo "构建所有平台版本..."
	@mkdir -p $(BUILD_DIR)
	
	# Linux AMD64
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/gcond
	
	# Linux ARM64
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/gcond
	
	# Windows AMD64
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/gcond
	
	# macOS AMD64
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/gcond
	
	# macOS ARM64
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/gcond

# 安装到系统
install: build
	@echo "安装 GPUConductor..."
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/

# 运行服务器
run-server: build
	@echo "启动 GPUConductor 服务器..."
	./$(BUILD_DIR)/$(BINARY_NAME) server --port 8080

# 运行代理
run-agent: build
	@echo "启动 GPUConductor 代理..."
	./$(BUILD_DIR)/$(BINARY_NAME) agent --server http://localhost:8080

# 测试
test:
	@echo "运行测试..."
	go test -v ./...

# 测试覆盖率
test-coverage:
	@echo "运行测试覆盖率..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# 代码格式化
fmt:
	@echo "格式化代码..."
	go fmt ./...

# 代码检查
lint:
	@echo "代码检查..."
	golangci-lint run

# 清理
clean:
	@echo "清理构建文件..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# 开发环境设置
dev-setup:
	@echo "设置开发环境..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/air-verse/air@latest

# 热重载开发
dev: deps
	@echo "启动开发模式 (热重载)..."
	air

# Docker 构建
docker-build:
	@echo "构建 Docker 镜像..."
	docker build -t gpuconductor:$(VERSION) .
	docker build -t gpuconductor:latest .

# Docker 运行
docker-run:
	@echo "运行 Docker 容器..."
	docker run -d --name gpuconductor -p 8080:8080 gpuconductor:latest

# 生成配置文件示例
config:
	@echo "生成配置文件示例..."
	@mkdir -p config
	@cat > config/server.yaml << 'EOF'
# GPUConductor 服务器配置
server:
  port: "8080"
  database: "gcond.db"
  redis: "localhost:6379"

# 日志配置
log:
  level: "info"
  file: "gcond.log"

# 安全配置
security:
  enable_auth: false
  jwt_secret: "your-secret-key"
EOF
	@cat > config/agent.yaml << 'EOF'
# GPUConductor 代理配置
agent:
  server: "http://localhost:8080"
  name: ""  # 留空使用主机名
  tags: ["gpu", "training"]

# 监控配置
monitor:
  gpu_interval: 10  # GPU监控间隔(秒)
  heartbeat_interval: 30  # 心跳间隔(秒)

# Docker配置
docker:
  endpoint: "unix:///var/run/docker.sock"
EOF
	@echo "配置文件已生成到 config/ 目录"

# 帮助
help:
	@echo "GPUConductor 构建工具"
	@echo ""
	@echo "可用命令:"
	@echo "  build         构建二进制文件"
	@echo "  build-all     构建所有平台版本"
	@echo "  install       安装到系统"
	@echo "  run-server    运行服务器"
	@echo "  run-agent     运行代理"
	@echo "  test          运行测试"
	@echo "  test-coverage 运行测试覆盖率"
	@echo "  fmt           格式化代码"
	@echo "  lint          代码检查"
	@echo "  clean         清理构建文件"
	@echo "  dev-setup     设置开发环境"
	@echo "  dev           热重载开发"
	@echo "  docker-build  构建Docker镜像"
	@echo "  docker-run    运行Docker容器"
	@echo "  config        生成配置文件示例"
	@echo "  help          显示帮助信息"