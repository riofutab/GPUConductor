#!/bin/bash

# GPUConductor 卸载脚本

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

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
    echo -e "${BLUE}  GPUConductor 卸载脚本${NC}"
    echo -e "${BLUE}================================${NC}"
}

# 确认卸载
confirm_uninstall() {
    echo
    print_warning "此操作将完全卸载 GPUConductor 及其所有数据"
    print_warning "包括配置文件、数据库文件和日志文件"
    echo
    read -p "确定要继续吗？(y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_message "取消卸载"
        exit 0
    fi
}

# 停止服务
stop_services() {
    print_message "停止 GPUConductor 服务..."
    
    # 停止并禁用服务
    for service in gpuconductor-server gpuconductor-agent; do
        if systemctl is-active --quiet $service; then
            print_message "停止服务: $service"
            sudo systemctl stop $service
        fi
        
        if systemctl is-enabled --quiet $service 2>/dev/null; then
            print_message "禁用服务: $service"
            sudo systemctl disable $service
        fi
    done
}

# 删除 systemd 服务文件
remove_systemd_files() {
    print_message "删除 systemd 服务文件..."
    
    for service in gpuconductor-server gpuconductor-agent; do
        service_file="/etc/systemd/system/${service}.service"
        if [[ -f $service_file ]]; then
            print_message "删除: $service_file"
            sudo rm -f $service_file
        fi
    done
    
    # 重新加载 systemd
    sudo systemctl daemon-reload
}

# 删除二进制文件
remove_binary() {
    print_message "删除二进制文件..."
    
    if [[ -f /usr/local/bin/gcond ]]; then
        print_message "删除: /usr/local/bin/gcond"
        sudo rm -f /usr/local/bin/gcond
    fi
}

# 删除配置文件
remove_config() {
    print_message "删除配置文件..."
    
    if [[ -d /etc/gpuconductor ]]; then
        print_message "删除配置目录: /etc/gpuconductor"
        sudo rm -rf /etc/gpuconductor
    fi
}

# 删除数据文件
remove_data() {
    print_message "删除数据文件..."
    
    # 数据目录
    if [[ -d /var/lib/gpuconductor ]]; then
        print_warning "删除数据目录: /var/lib/gpuconductor (包含数据库文件)"
        sudo rm -rf /var/lib/gpuconductor
    fi
    
    # 日志目录
    if [[ -d /var/log/gpuconductor ]]; then
        print_message "删除日志目录: /var/log/gpuconductor"
        sudo rm -rf /var/log/gpuconductor
    fi
}

# 清理 Docker 容器和镜像 (可选)
cleanup_docker() {
    print_message "检查 Docker 容器和镜像..."
    
    # 停止并删除相关容器
    containers=$(docker ps -a --filter "name=gpuconductor" --format "{{.Names}}" 2>/dev/null || true)
    if [[ -n "$containers" ]]; then
        print_warning "发现 GPUConductor 相关容器，是否删除？"
        read -p "删除 Docker 容器？(y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            echo "$containers" | xargs -r docker rm -f
            print_message "Docker 容器已删除"
        fi
    fi
    
    # 删除相关镜像
    images=$(docker images --filter "reference=gpuconductor*" --format "{{.Repository}}:{{.Tag}}" 2>/dev/null || true)
    if [[ -n "$images" ]]; then
        print_warning "发现 GPUConductor 相关镜像，是否删除？"
        read -p "删除 Docker 镜像？(y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            echo "$images" | xargs -r docker rmi -f
            print_message "Docker 镜像已删除"
        fi
    fi
}

# 显示卸载完成信息
show_completion_info() {
    print_header
    print_message "GPUConductor 卸载完成！"
    echo
    print_message "已删除的内容:"
    echo "  ✓ 二进制文件: /usr/local/bin/gcond"
    echo "  ✓ 配置文件: /etc/gpuconductor/"
    echo "  ✓ 数据文件: /var/lib/gpuconductor/"
    echo "  ✓ 日志文件: /var/log/gpuconductor/"
    echo "  ✓ systemd 服务文件"
    echo
    print_message "如果您使用了 Docker Compose，请手动清理："
    echo "  docker-compose down -v"
    echo "  docker volume prune"
    echo
    print_message "感谢使用 GPUConductor！"
}

# 主函数
main() {
    print_header
    
    # 检查是否安装了 GPUConductor
    if [[ ! -f /usr/local/bin/gcond ]]; then
        print_error "GPUConductor 似乎没有安装"
        exit 1
    fi
    
    confirm_uninstall
    stop_services
    remove_systemd_files
    remove_binary
    remove_config
    remove_data
    
    # 可选的 Docker 清理
    if command -v docker &> /dev/null; then
        cleanup_docker
    fi
    
    show_completion_info
}

# 运行主函数
main "$@"