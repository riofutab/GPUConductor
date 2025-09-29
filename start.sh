#!/bin/bash

# GPUConductor 启动脚本 (Bash)

echo "=== GPUConductor 启动脚本 ==="

# 检查可执行文件是否存在
if [ ! -f "build/gcond" ]; then
    echo "错误: 找不到 gcond 可执行文件，请先运行构建命令:"
    echo "go build -o build/gcond cmd/gcond/main.go"
    exit 1
fi

# 检查配置文件
if [ ! -f "config.yaml" ]; then
    echo "警告: 找不到 config.yaml，将使用默认配置"
fi

# 显示菜单
echo ""
echo "请选择启动方式:"
echo "1. 启动服务器 (默认配置)"
echo "2. 启动服务器 (指定配置文件)"
echo "3. 启动 Agent"
echo "4. 使用 Docker Compose 启动完整环境"
echo "5. 查看帮助信息"
echo "6. 退出"
echo ""

read -p "请输入选择 (1-6): " choice

case $choice in
    1)
        echo "启动服务器 (默认配置)..."
        ./build/gcond server
        ;;
    2)
        echo "启动服务器 (使用配置文件)..."
        ./build/gcond server --config config.yaml
        ;;
    3)
        read -p "请输入服务器地址 (默认: http://localhost:8080): " server_url
        server_url=${server_url:-http://localhost:8080}
        read -p "请输入节点ID (默认: gpu-node-1): " node_id
        node_id=${node_id:-gpu-node-1}
        echo "启动 Agent..."
        ./build/gcond agent --server "$server_url" --node-id "$node_id"
        ;;
    4)
        echo "使用 Docker Compose 启动完整环境..."
        if command -v docker-compose &> /dev/null; then
            docker-compose up -d
            echo "服务启动完成!"
            echo "Web 界面: http://localhost:8080"
            echo "LDAP 管理: http://localhost:8081"
        else
            echo "错误: 找不到 docker-compose 命令"
            exit 1
        fi
        ;;
    5)
        echo "显示帮助信息..."
        ./build/gcond --help
        ;;
    6)
        echo "退出"
        exit 0
        ;;
    *)
        echo "无效选择，退出"
        exit 1
        ;;
esac