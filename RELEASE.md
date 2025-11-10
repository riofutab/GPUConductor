# 发布说明

## 如何发布新版本

### 1. 准备发布

```bash
# 确保代码是最新的
git pull origin main

# 运行测试
go test ./...

# 本地构建测试
./build.ps1  # Windows
# 或
./build.sh   # Linux/macOS
```

### 2. 创建版本标签

```bash
# 创建标签 (遵循语义化版本)
git tag v1.0.0

# 推送标签到远程仓库
git push origin v1.0.0
```

### 3. 自动构建和发布

推送标签后，GitHub Actions 会自动：

1. **构建多平台二进制文件**
   - Windows (AMD64)
   - Linux (AMD64, ARM64)  
   - macOS (AMD64, ARM64)

2. **创建 GitHub Release**
   - 自动生成 Release Notes
   - 上传所有二进制文件
   - 设置为正式版本

3. **文件命名规则**
   ```
   gcond-{os}-{arch}[.exe]
   
   例如:
   - gcond-windows-amd64.exe
   - gcond-linux-amd64
   - gcond-darwin-arm64
   ```

### 4. 验证发布

1. 检查 GitHub Releases 页面
2. 下载并测试二进制文件
3. 验证版本信息：
   ```bash
   ./gcond version
   ```

## 版本规范

遵循 [语义化版本](https://semver.org/lang/zh-CN/) 规范：

- **主版本号**: 不兼容的 API 修改
- **次版本号**: 向下兼容的功能性新增
- **修订号**: 向下兼容的问题修正

### 示例

- `v1.0.0` - 首个稳定版本
- `v1.1.0` - 新增功能
- `v1.1.1` - 修复 bug
- `v2.0.0` - 重大更新，可能不向下兼容

## 预发布版本

对于测试版本，可以使用预发布标签：

```bash
# 创建预发布版本
git tag v1.1.0-beta.1
git push origin v1.1.0-beta.1

# 或者发布候选版本
git tag v1.1.0-rc.1
git push origin v1.1.0-rc.1
```

## 发布检查清单

- [ ] 代码已合并到 main 分支
- [ ] 所有测试通过
- [ ] 文档已更新
- [ ] CHANGELOG.md 已更新
- [ ] 版本号符合语义化版本规范
- [ ] 本地构建测试成功
- [ ] 标签已推送到远程仓库
- [ ] GitHub Actions 构建成功
- [ ] Release 页面信息正确
- [ ] 二进制文件可正常下载和运行