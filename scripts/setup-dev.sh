#!/bin/bash

# GPUConductor 开发环境设置脚本

set -e

# 颜色定义
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_message() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_header() {
    echo -e "${BLUE}================================${NC}"
    echo -e "${BLUE}  GPUConductor 开发环境设置${NC}"
    echo -e "${BLUE}================================${NC}"
}

# 检查 Go 版本
check_go() {
    print_message "检查 Go 环境..."
    
    if ! command -v go &> /dev/null; then
        print_warning "Go 未安装，请先安装 Go 1.21+"
        exit 1
    fi
    
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    print_message "Go 版本: $GO_VERSION"
    
    # 检查 Go 版本是否满足要求
    if [[ $(echo "$GO_VERSION 1.21" | tr " " "\n" | sort -V | head -n1) != "1.21" ]]; then
        print_warning "Go 版本过低，建议使用 1.21+"
    fi
}

# 安装开发工具
install_dev_tools() {
    print_message "安装开发工具..."
    
    # golangci-lint
    if ! command -v golangci-lint &> /dev/null; then
        print_message "安装 golangci-lint..."
        go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
    else
        print_message "golangci-lint 已安装"
    fi
    
    # air (热重载)
    if ! command -v air &> /dev/null; then
        print_message "安装 air..."
        go install github.com/air-verse/air@latest
    else
        print_message "air 已安装"
    fi
    
    # goimports
    if ! command -v goimports &> /dev/null; then
        print_message "安装 goimports..."
        go install golang.org/x/tools/cmd/goimports@latest
    else
        print_message "goimports 已安装"
    fi
    
    # mockgen (可选)
    if ! command -v mockgen &> /dev/null; then
        print_message "安装 mockgen..."
        go install github.com/golang/mock/mockgen@latest
    else
        print_message "mockgen 已安装"
    fi
}

# 设置 Git hooks
setup_git_hooks() {
    print_message "设置 Git hooks..."
    
    # 创建 pre-commit hook
    cat > .git/hooks/pre-commit << 'EOF'
#!/bin/bash

# 运行代码格式化
echo "运行 gofmt..."
gofmt -w .

echo "运行 goimports..."
goimports -w .

# 运行代码检查
echo "运行 golangci-lint..."
golangci-lint run

# 运行测试
echo "运行测试..."
go test ./...

echo "Pre-commit 检查通过"
EOF

    chmod +x .git/hooks/pre-commit
    print_message "Git pre-commit hook 已设置"
}

# 创建开发配置
create_dev_config() {
    print_message "创建开发配置..."
    
    mkdir -p dev-config
    
    # 开发服务器配置
    cat > dev-config/server.yaml << 'EOF'
server:
  port: "8080"
  database: "dev-gcond.db"
  redis: "localhost:6379"

log:
  level: "debug"
  file: "dev-server.log"

scheduler:
  interval: 2
  
monitoring:
  enable_metrics: true
  metrics_port: "9090"
EOF

    # 开发 Agent 配置
    cat > dev-config/agent.yaml << 'EOF'
agent:
  server: "http://localhost:8080"
  name: "dev-agent"
  tags: ["dev", "gpu"]

monitor:
  gpu_interval: 5
  heartbeat_interval: 10

log:
  level: "debug"
  file: "dev-agent.log"
EOF

    print_message "开发配置文件已创建在 dev-config/ 目录"
}

# 启动开发服务
setup_dev_services() {
    print_message "设置开发服务..."
    
    # 检查 Docker
    if command -v docker &> /dev/null; then
        print_message "启动 Redis 容器..."
        docker run -d --name gpuconductor-redis-dev \
            -p 6379:6379 \
            redis:7-alpine \
            redis-server --appendonly yes || true
        print_message "Redis 已启动在端口 6379"
    else
        print_warning "Docker 未安装，请手动启动 Redis"
    fi
}

# 创建 Makefile 别名
create_aliases() {
    print_message "创建开发别名..."
    
    cat >> ~/.bashrc << 'EOF'

# GPUConductor 开发别名
alias gcond-dev='go run ./cmd/gcond'
alias gcond-server='go run ./cmd/gcond server --config dev-config/server.yaml'
alias gcond-agent='go run ./cmd/gcond agent --config dev-config/agent.yaml'
alias gcond-build='make build'
alias gcond-test='make test'
alias gcond-lint='make lint'
EOF

    print_message "别名已添加到 ~/.bashrc"
    print_warning "请运行 'source ~/.bashrc' 或重新打开终端"
}

# 显示开发信息
show_dev_info() {
    print_header
    print_message "开发环境设置完成！"
    echo
    print_message "开发命令:"
    echo "  热重载开发: make dev"
    echo "  构建项目: make build"
    echo "  运行测试: make test"
    echo "  代码检查: make lint"
    echo "  格式化代码: make fmt"
    echo
    print_message "开发服务:"
    echo "  启动服务器: gcond-server"
    echo "  启动 Agent: gcond-agent"
    echo "  直接运行: gcond-dev server"
    echo
    print_message "开发配置:"
    echo "  服务器配置: dev-config/server.yaml"
    echo "  Agent 配置: dev-config/agent.yaml"
    echo
    print_message "开发工具:"
    echo "  代码检查: golangci-lint run"
    echo "  热重载: air"
    echo "  格式化: goimports -w ."
    echo
    print_message "Web 界面: http://localhost:8080"
    print_message "指标接口: http://localhost:9090/metrics"
}

# 主函数
main() {
    print_header
    
    check_go
    install_dev_tools
    
    # 只在 Git 仓库中设置 hooks
    if [[ -d .git ]]; then
        setup_git_hooks
    fi
    
    create_dev_config
    setup_dev_services
    create_aliases
    show_dev_info
}

# 运行主函数
main "$@"