# GPUConductor 启动脚本 (PowerShell)

Write-Host "=== GPUConductor 启动脚本 ===" -ForegroundColor Green

# 检查可执行文件是否存在
if (-not (Test-Path "build\gcond.exe")) {
    Write-Host "错误: 找不到 gcond.exe，请先运行构建命令:" -ForegroundColor Red
    Write-Host "go build -o build\gcond.exe cmd\gcond\main.go" -ForegroundColor Yellow
    exit 1
}

# 检查配置文件
if (-not (Test-Path "config.yaml")) {
    Write-Host "警告: 找不到 config.yaml，将使用默认配置" -ForegroundColor Yellow
}

# 显示菜单
Write-Host ""
Write-Host "请选择启动方式:" -ForegroundColor Cyan
Write-Host "1. 启动服务器 (默认配置)" -ForegroundColor White
Write-Host "2. 启动服务器 (指定配置文件)" -ForegroundColor White
Write-Host "3. 启动 Agent" -ForegroundColor White
Write-Host "4. 使用 Docker Compose 启动完整环境" -ForegroundColor White
Write-Host "5. 查看帮助信息" -ForegroundColor White
Write-Host "6. 退出" -ForegroundColor White
Write-Host ""

$choice = Read-Host "请输入选择 (1-6)"

switch ($choice) {
    "1" {
        Write-Host "启动服务器 (默认配置)..." -ForegroundColor Green
        .\build\gcond.exe server
    }
    "2" {
        Write-Host "启动服务器 (使用配置文件)..." -ForegroundColor Green
        .\build\gcond.exe server --config config.yaml
    }
    "3" {
        $serverUrl = Read-Host "请输入服务器地址 (默认: http://localhost:8080)"
        if ([string]::IsNullOrEmpty($serverUrl)) {
            $serverUrl = "http://localhost:8080"
        }
        $nodeId = Read-Host "请输入节点ID (默认: gpu-node-1)"
        if ([string]::IsNullOrEmpty($nodeId)) {
            $nodeId = "gpu-node-1"
        }
        Write-Host "启动 Agent..." -ForegroundColor Green
        .\build\gcond.exe agent --server $serverUrl --node-id $nodeId
    }
    "4" {
        Write-Host "使用 Docker Compose 启动完整环境..." -ForegroundColor Green
        if (Get-Command docker-compose -ErrorAction SilentlyContinue) {
            docker-compose up -d
            Write-Host "服务启动完成!" -ForegroundColor Green
            Write-Host "Web 界面: http://localhost:8080" -ForegroundColor Cyan
            Write-Host "LDAP 管理: http://localhost:8081" -ForegroundColor Cyan
        } else {
            Write-Host "错误: 找不到 docker-compose 命令" -ForegroundColor Red
        }
    }
    "5" {
        Write-Host "显示帮助信息..." -ForegroundColor Green
        .\build\gcond.exe --help
    }
    "6" {
        Write-Host "退出" -ForegroundColor Yellow
        exit 0
    }
    default {
        Write-Host "无效选择，退出" -ForegroundColor Red
        exit 1
    }
}