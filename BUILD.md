# GPUConductor 构建指南

## 🚀 本地构建

### Windows PowerShell
```powershell
# 运行构建脚本
.\build.ps1

# 或者手动构建特定平台
$env:GOOS="linux"
$env:GOARCH="amd64"
go build -ldflags="-s -w -X 'GPUConductor/internal/cmd.version=v1.0.0' -X 'GPUConductor/internal/cmd.commit=$(git rev-parse HEAD)' -X 'GPUConductor/internal/cmd.buildTime=$(Get-Date -Format "yyyy-MM-dd HH:mm:ss")'" -o build/gcond-linux-amd64 cmd/gcond/main.go
```

### Linux/macOS Bash
```bash
# 运行构建脚本
chmod +x build.sh
./build.sh

# 或者手动构建
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X 'GPUConductor/internal/cmd.version=v1.0.0' -X 'GPUConductor/internal/cmd.commit=$(git rev-parse HEAD)' -X 'GPUConductor/internal/cmd.buildTime=$(date)'" -o build/gcond-linux-amd64 cmd/gcond/main.go
```

## 🔧 支持的平台

| 操作系统 | 架构 | 文件名 |
|---------|------|--------|
| Windows | AMD64 | gcond-windows-amd64.exe |
| Linux | AMD64 | gcond-linux-amd64 |
| Linux | ARM64 | gcond-linux-arm64 |
| macOS | AMD64 | gcond-darwin-amd64 |
| macOS | ARM64 | gcond-darwin-arm64 |

## 🤖 GitHub Actions 自动构建

### 触发方式

1. **标签推送** (推荐)
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

2. **手动触发**
   - 在 GitHub 仓库页面
   - 点击 "Actions" 标签
   - 选择 "Build and Release" 工作流
   - 点击 "Run workflow"

3. **代码推送** (CI 检查)
   ```bash
   git push origin main
   ```

### 工作流说明

#### `.github/workflows/release.yml`
- **触发条件**: 推送标签 (v*)
- **功能**: 
  - 多平台交叉编译
  - 自动创建 GitHub Release
  - 上传所有平台的二进制文件
  - 生成 Release Notes

#### `.github/workflows/ci.yml`
- **触发条件**: 推送到 main/master 分支或 PR
- **功能**:
  - 代码质量检查
  - 运行测试
  - Go vet 检查
  - golangci-lint 代码规范检查

## 📦 构建产物

构建完成后，在 `build/` 目录下会生成：

```
build/
├── gcond-windows-amd64.exe    # Windows 64位
├── gcond-linux-amd64          # Linux 64位
├── gcond-linux-arm64          # Linux ARM64
├── gcond-darwin-amd64         # macOS Intel
└── gcond-darwin-arm64         # macOS Apple Silicon
```

## 🐛 常见问题

### Linux 提示 "cannot execute binary file"

**原因**: 架构不匹配

**解决方案**:
1. 检查目标系统架构：
   ```bash
   uname -m
   # x86_64  -> 使用 gcond-linux-amd64
   # aarch64 -> 使用 gcond-linux-arm64
   ```

2. 添加执行权限：
   ```bash
   chmod +x gcond-linux-amd64
   ```

3. 验证文件类型：
   ```bash
   file gcond-linux-amd64
   # 应该显示: ELF 64-bit LSB executable, x86-64
   ```

### 构建失败

1. **依赖问题**:
   ```bash
   go mod tidy
   go mod download
   ```

2. **Go 版本**:
   确保使用 Go 1.21 或更高版本

3. **权限问题**:
   ```bash
   # Linux/macOS
   chmod +x build.sh
   
   # Windows (以管理员身份运行 PowerShell)
   Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
   ```

## 🚀 发布流程

1. **完成开发**
   ```bash
   git add .
   git commit -m "feat: 新功能"
   git push
   ```

2. **创建标签**
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

3. **自动构建**
   - GitHub Actions 自动触发
   - 构建所有平台版本
   - 创建 GitHub Release
   - 上传二进制文件

4. **下载使用**
   - 访问 GitHub Releases 页面
   - 下载对应平台的二进制文件
   - 解压并运行

## 📋 版本信息

构建的二进制文件包含版本信息：

```bash
./gcond version
# 输出:
# GPUConductor v1.0.0
# Commit: abc123...
# Build Time: 2024-01-01 12:00:00
```

## 🔍 调试构建

启用详细输出：
```bash
# 查看构建过程
go build -v -x cmd/gcond/main.go

# 查看依赖
go list -m all

# 检查交叉编译支持
go tool dist list