# GPUConductor 部署指南

## 🚀 快速开始

### 1. 构建项目

```bash
# 克隆项目
git clone <your-repo-url>
cd GPUConductor

# 安装依赖并构建
go mod tidy
go build -o build/gcond ./cmd/gcond

# 或使用 Makefile
make build
```

### 2. 启动服务器

```bash
# 基本启动
./build/gcond server

# 自定义配置
./build/gcond server --port 8080 --database ./data/gcond.db --redis localhost:6379
```

服务器启动后访问: http://localhost:8080

### 3. 启动 Agent 节点

在每台 GPU 机器上运行：

```bash
# 连接到服务器
./build/gcond agent --server http://server-ip:8080 --name gpu-node-1 --tags gpu,training
```

## 📋 系统要求

### 服务器节点
- Go 1.21+
- Redis (消息队列)
- 8080 端口可用

### Agent 节点
- Go 1.21+
- NVIDIA GPU + nvidia-smi (GPU 监控)
- Docker (任务执行，可选)

## 🔧 配置说明

### 服务器配置

创建 `server.yaml`:
```yaml
server:
  port: "8080"
  database: "gcond.db"
  redis: "localhost:6379"

log:
  level: "info"
  file: "gcond.log"
```

### Agent 配置

创建 `agent.yaml`:
```yaml
agent:
  server: "http://localhost:8080"
  name: "gpu-node-1"
  tags: ["gpu", "training"]

monitor:
  gpu_interval: 10
  heartbeat_interval: 30
```

## 🐳 Docker 部署

### 使用 Docker Compose

```bash
# 启动完整环境
docker-compose up -d

# 仅启动服务器和 Redis
docker-compose up -d server redis
```

### 手动 Docker 部署

```bash
# 构建镜像
docker build -t gpuconductor .

# 启动 Redis
docker run -d --name redis -p 6379:6379 redis:7-alpine

# 启动服务器
docker run -d --name gpuconductor-server \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  gpuconductor ./gcond server --redis redis:6379

# 在 GPU 节点启动 Agent
docker run -d --name gpuconductor-agent \
  --gpus all \
  -v /var/run/docker.sock:/var/run/docker.sock \
  gpuconductor ./gcond agent --server http://server-ip:8080
```

## 📊 功能特性

### ✅ 已实现功能

1. **基础架构**
   - ✅ 命令行工具 (gcond)
   - ✅ 服务器/Agent 架构
   - ✅ RESTful API
   - ✅ 内嵌 Web 界面

2. **节点管理**
   - ✅ 节点注册和心跳
   - ✅ GPU 状态监控
   - ✅ 节点标签系统

3. **任务管理**
   - ✅ 任务提交和队列
   - ✅ 优先级调度
   - ✅ 任务状态跟踪
   - ✅ 超时控制

4. **Web 界面**
   - ✅ 系统概览仪表板
   - ✅ GPU 节点监控
   - ✅ 任务管理界面
   - ✅ 任务创建表单

### 🚧 待完善功能

1. **Docker 集成**
   - ⏳ 容器任务执行 (当前为模拟模式)
   - ⏳ 镜像管理
   - ⏳ 资源限制

2. **高级功能**
   - ⏳ 分布式训练支持
   - ⏳ 任务日志收集
   - ⏳ 指标监控
   - ⏳ 告警通知

## 🔍 故障排除

### 常见问题

1. **构建失败**
   ```bash
   # 清理并重新构建
   go clean -modcache
   go mod tidy
   go build -o build/gcond ./cmd/gcond
   ```

2. **服务器无法启动**
   - 检查端口是否被占用
   - 确认 Redis 是否运行
   - 查看日志文件

3. **Agent 连接失败**
   - 检查服务器地址是否正确
   - 确认网络连接
   - 查看防火墙设置

4. **GPU 监控失败**
   - 确认安装了 nvidia-smi
   - 检查 NVIDIA 驱动
   - 验证 GPU 可见性

### 日志查看

```bash
# 查看服务器日志
tail -f gcond.log

# 查看系统日志 (Linux)
journalctl -u gpuconductor-server -f
```

## 📈 性能优化

### 服务器优化

1. **数据库优化**
   - 定期清理旧任务记录
   - 添加适当的索引
   - 考虑使用 PostgreSQL

2. **Redis 优化**
   - 配置持久化
   - 设置内存限制
   - 启用集群模式

### Agent 优化

1. **监控频率**
   - 根据需要调整 GPU 监控间隔
   - 优化心跳频率
   - 减少不必要的网络请求

2. **资源管理**
   - 设置合理的并发任务数
   - 配置资源限制
   - 监控系统负载

## 🔒 安全建议

1. **网络安全**
   - 使用 HTTPS (生产环境)
   - 配置防火墙规则
   - 限制 API 访问

2. **容器安全**
   - 使用非 root 用户
   - 限制容器权限
   - 扫描镜像漏洞

3. **数据安全**
   - 定期备份数据库
   - 加密敏感配置
   - 审计日志记录

## 📞 技术支持

如遇问题，请：

1. 查看本文档的故障排除部分
2. 检查 GitHub Issues
3. 提交新的 Issue 并附上：
   - 系统信息
   - 错误日志
   - 复现步骤

---

**GPUConductor** - 让 GPU 资源调度更智能 🚀