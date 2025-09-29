#!/bin/bash

# GPUConductor 安装脚本

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 打印带颜色的消息
print_message() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_header() {
    echo -e "${BLUE}================================${NC}"
    echo -e "${BLUE}  GPUConductor 安装脚本${NC}"
    echo -e "${BLUE}================================${NC}"
}

# 检查系统要求
check_requirements() {
    print_message "检查系统要求..."
    
    # 检查操作系统
    if [[ "$OSTYPE" != "linux-gnu"* ]]; then
        print_error "此脚本仅支持 Linux 系统"
        exit 1
    fi
    
    # 检查是否为 root 用户
    if [[ $EUID -eq 0 ]]; then
        print_warning "建议不要使用 root 用户运行此脚本"
    fi
    
    # 检查必要命令
    for cmd in curl wget tar; do
        if ! command -v $cmd &> /dev/null; then
            print_error "缺少必要命令: $cmd"
            exit 1
        fi
    done
    
    print_message "系统要求检查通过"
}

# 检测系统架构
detect_arch() {
    local arch=$(uname -m)
    case $arch in
        x86_64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        *)
            print_error "不支持的系统架构: $arch"
            exit 1
            ;;
    esac
    print_message "检测到系统架构: $ARCH"
}

# 下载并安装 GPUConductor
install_gpuconductor() {
    print_message "开始安装 GPUConductor..."
    
    # 设置版本和下载URL
    VERSION=${VERSION:-"latest"}
    BINARY_NAME="gcond-linux-${ARCH}"
    DOWNLOAD_URL="https://github.com/your-org/GPUConductor/releases/download/${VERSION}/${BINARY_NAME}"
    
    # 创建临时目录
    TEMP_DIR=$(mktemp -d)
    cd $TEMP_DIR
    
    print_message "下载 GPUConductor ${VERSION}..."
    if ! wget -q "$DOWNLOAD_URL" -O "$BINARY_NAME"; then
        print_error "下载失败，请检查网络连接或版本号"
        exit 1
    fi
    
    # 设置执行权限
    chmod +x "$BINARY_NAME"
    
    # 安装到系统目录
    print_message "安装到 /usr/local/bin/gcond..."
    sudo mv "$BINARY_NAME" /usr/local/bin/gcond
    
    # 清理临时文件
    cd - > /dev/null
    rm -rf $TEMP_DIR
    
    print_message "GPUConductor 安装完成"
}

# 创建配置目录和文件
setup_config() {
    print_message "设置配置文件..."
    
    # 创建配置目录
    CONFIG_DIR="/etc/gpuconductor"
    sudo mkdir -p $CONFIG_DIR
    
    # 创建服务器配置文件
    sudo tee $CONFIG_DIR/server.yaml > /dev/null <<EOF
# GPUConductor 服务器配置
server:
  port: "8080"
  database: "/var/lib/gpuconductor/gcond.db"
  redis: "localhost:6379"

# 日志配置
log:
  level: "info"
  file: "/var/log/gpuconductor/server.log"

# 安全配置
security:
  enable_auth: false
  jwt_secret: "$(openssl rand -base64 32)"
EOF

    # 创建 Agent 配置文件
    sudo tee $CONFIG_DIR/agent.yaml > /dev/null <<EOF
# GPUConductor Agent 配置
agent:
  server: "http://localhost:8080"
  name: "$(hostname)"
  tags: ["gpu", "training"]

# 监控配置
monitor:
  gpu_interval: 10
  heartbeat_interval: 30

# Docker 配置
docker:
  endpoint: "unix:///var/run/docker.sock"
EOF

    # 创建数据和日志目录
    sudo mkdir -p /var/lib/gpuconductor
    sudo mkdir -p /var/log/gpuconductor
    
    # 设置权限
    sudo chown -R $USER:$USER /var/lib/gpuconductor
    sudo chown -R $USER:$USER /var/log/gpuconductor
    
    print_message "配置文件已创建在 $CONFIG_DIR"
}

# 创建 systemd 服务文件
setup_systemd() {
    print_message "设置 systemd 服务..."
    
    # 服务器服务文件
    sudo tee /etc/systemd/system/gpuconductor-server.service > /dev/null <<EOF
[Unit]
Description=GPUConductor Server
After=network.target redis.service
Wants=redis.service

[Service]
Type=simple
User=$USER
Group=$USER
ExecStart=/usr/local/bin/gcond server --config /etc/gpuconductor/server.yaml
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=gpuconductor-server

[Install]
WantedBy=multi-user.target
EOF

    # Agent 服务文件
    sudo tee /etc/systemd/system/gpuconductor-agent.service > /dev/null <<EOF
[Unit]
Description=GPUConductor Agent
After=network.target docker.service
Requires=docker.service

[Service]
Type=simple
User=$USER
Group=$USER
ExecStart=/usr/local/bin/gcond agent --config /etc/gpuconductor/agent.yaml
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=gpuconductor-agent

[Install]
WantedBy=multi-user.target
EOF

    # 重新加载 systemd
    sudo systemctl daemon-reload
    
    print_message "systemd 服务文件已创建"
}

# 安装依赖
install_dependencies() {
    print_message "检查并安装依赖..."
    
    # 检查 Docker
    if ! command -v docker &> /dev/null; then
        print_warning "Docker 未安装，请手动安装 Docker"
        print_message "安装命令: curl -fsSL https://get.docker.com | sh"
    else
        print_message "Docker 已安装: $(docker --version)"
    fi
    
    # 检查 Redis
    if ! command -v redis-server &> /dev/null; then
        print_warning "Redis 未安装，正在安装..."
        if command -v apt-get &> /dev/null; then
            sudo apt-get update && sudo apt-get install -y redis-server
        elif command -v yum &> /dev/null; then
            sudo yum install -y redis
        else
            print_warning "无法自动安装 Redis，请手动安装"
        fi
    else
        print_message "Redis 已安装: $(redis-server --version)"
    fi
    
    # 检查 nvidia-smi (GPU 节点需要)
    if command -v nvidia-smi &> /dev/null; then
        print_message "NVIDIA 驱动已安装: $(nvidia-smi --version | head -1)"
    else
        print_warning "未检测到 NVIDIA 驱动，GPU 监控功能将不可用"
    fi
}

# 显示安装后信息
show_post_install_info() {
    print_header
    print_message "GPUConductor 安装完成！"
    echo
    print_message "配置文件位置:"
    echo "  服务器配置: /etc/gpuconductor/server.yaml"
    echo "  Agent 配置: /etc/gpuconductor/agent.yaml"
    echo
    print_message "启动服务:"
    echo "  启动服务器: sudo systemctl start gpuconductor-server"
    echo "  启动 Agent: sudo systemctl start gpuconductor-agent"
    echo "  开机自启: sudo systemctl enable gpuconductor-server"
    echo
    print_message "查看状态:"
    echo "  服务状态: sudo systemctl status gpuconductor-server"
    echo "  查看日志: journalctl -u gpuconductor-server -f"
    echo
    print_message "Web 界面:"
    echo "  访问地址: http://localhost:8080"
    echo
    print_message "命令行工具:"
    echo "  查看帮助: gcond --help"
    echo "  手动启动: gcond server"
    echo "  Agent 连接: gcond agent --server http://server-ip:8080"
    echo
    print_warning "请根据需要修改配置文件，然后启动相应服务"
}

# 主函数
main() {
    print_header
    
    check_requirements
    detect_arch
    install_gpuconductor
    setup_config
    setup_systemd
    install_dependencies
    show_post_install_info
}

# 运行主函数
main "$@"