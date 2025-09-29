# GPUConductor

GPUConductor 是一个分布式 GPU 任务调度系统，支持多台机器部署、统一界面访问，可以监控 GPU 使用率并根据使用率进行算法模型训练任务排队。

## 功能特性

- 🖥️ **多机器部署支持** - 支持多台 GPU 机器的统一管理
- 📊 **GPU 监控** - 实时监控 GPU 使用率、内存、温度等指标
- 🔄 **智能任务调度** - 基于 GPU 使用率的自动任务排队和调度
- 🎯 **优先级管理** - 支持任务优先级设置和机器绑定
- ⏱️ **超时控制** - 任务最大执行时间限制
- 🐳 **Docker 支持** - 支持使用 Docker 镜像执行训练任务
- 🔐 **LDAP 认证** - 集成 LDAP 用户认证系统
- 🌐 **Web 界面** - 基于 Vue.js 的现代化 Web 管理界面
- 📡 **实时通信** - WebSocket 实时数据推送
- 🗄️ **PostgreSQL** - 使用 PostgreSQL 作为主数据库

## 系统架构

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Web Browser   │    │     LDAP        │    │   PostgreSQL    │
│                 │    │   Directory     │    │    Database     │
└─────────┬───────┘    └─────────┬───────┘    └─────────┬───────┘
          │                      │                      │
          │ HTTP/WebSocket       │ LDAP Auth           │ SQL
          │                      │                      │
┌─────────▼──────────────────────▼──────────────────────▼───────┐
│                    GPUConductor Server                        │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────────────────┐  │
│  │   Web UI    │ │    API      │ │      Scheduler          │  │
│  │   (Vue.js)  │ │  (Gin/Go)   │ │    (Task Queue)         │  │
│  └─────────────┘ └─────────────┘ └─────────────────────────┘  │
└─────────┬──────────────────────────────────────────────┬─────┘
          │                                              │
          │ Redis (Message Queue)                        │
          │                                              │
┌─────────▼───────┐                           ┌─────────▼───────┐
│  GPU Node 1     │                           │  GPU Node N     │
│ ┌─────────────┐ │                           │ ┌─────────────┐ │
│ │   Agent     │ │            ...            │ │   Agent     │ │
│ │   (Go)      │ │                           │ │   (Go)      │ │
│ └─────────────┘ │                           │ └─────────────┘ │
│ ┌─────────────┐ │                           │ ┌─────────────┐ │
│ │   Docker    │ │                           │ │   Docker    │ │
│ │  Containers │ │                           │ │  Containers │ │
│ └─────────────┘ │                           │ └─────────────┘ │
│ ┌─────────────┐ │                           │ ┌─────────────┐ │
│ │    GPUs     │ │                           │ │    GPUs     │ │
│ └─────────────┘ │                           │ └─────────────┘ │
└─────────────────┘                           └─────────────────┘
```

## 快速开始

### 1. 构建项目

```bash
# 下载依赖
go mod tidy

# 构建可执行文件
go build -o build/gcond.exe cmd/gcond/main.go  # Windows
go build -o build/gcond cmd/gcond/main.go      # Linux/macOS
```

### 2. 配置系统

复制并编辑配置文件：

```bash
cp config/server.example.yaml config.yaml
```

主要配置项：

```yaml
# 数据库配置
server:
  database: "host=localhost user=gcond password=gcond dbname=gcond port=5432 sslmode=disable"

# LDAP 配置
ldap:
  host: "your-ldap-server"
  base_dn: "dc=yourcompany,dc=com"
  bind_dn: "cn=admin,dc=yourcompany,dc=com"
  bind_pass: "your-admin-password"

# JWT 配置
jwt:
  secret: "your-secret-key-change-in-production"
```

### 3. 启动系统

#### 方式一：使用启动脚本

**Windows:**
```powershell
.\start.ps1
```

**Linux/macOS:**
```bash
chmod +x start.sh
./start.sh
```

#### 方式二：手动启动

**启动服务器:**
```bash
.\build\gcond.exe server --config config.yaml
```

**启动 Agent (在 GPU 机器上):**
```bash
.\build\gcond.exe agent --server http://your-server:8080 --node-id gpu-node-1
```

#### 方式三：使用 Docker Compose

```bash
docker-compose up -d
```

这将启动完整的环境，包括：
- GPUConductor 服务器 (端口 8080)
- PostgreSQL 数据库 (端口 5432)
- Redis 缓存 (端口 6379)
- LDAP 服务器 (端口 389)
- LDAP 管理界面 (端口 8081)

### 4. 访问系统

- **Web 管理界面**: http://localhost:8080
- **LDAP 管理界面**: http://localhost:8081 (用户名: cn=admin,dc=gpuconductor,dc=local, 密码: admin_password)

## 使用指南

### 创建训练任务

1. 登录 Web 界面
2. 进入"任务管理"页面
3. 点击"创建任务"
4. 填写任务信息：
   - 任务名称和描述
   - Docker 镜像
   - 执行命令
   - 优先级 (低/普通/高/紧急)
   - 最大执行时间
   - 指定节点 (可选)

### 监控 GPU 状态

1. 进入"仪表板"查看整体状态
2. 进入"节点管理"查看详细的 GPU 信息
3. 实时数据通过 WebSocket 自动更新

### 管理用户

用户通过 LDAP 进行认证，首次登录时会自动创建用户记录。

## API 文档

### 认证 API

```bash
# 用户登录
POST /api/v1/auth/login
{
  "username": "your-username",
  "password": "your-password"
}

# 刷新令牌
POST /api/v1/auth/refresh
{
  "refresh_token": "your-refresh-token"
}
```

### 任务 API

```bash
# 获取任务列表
GET /api/v1/tasks

# 创建任务
POST /api/v1/tasks
{
  "name": "训练任务",
  "description": "模型训练",
  "image": "pytorch/pytorch:latest",
  "command": "python train.py",
  "priority": "high",
  "timeout": 3600,
  "node_id": "gpu-node-1"
}

# 取消任务
POST /api/v1/tasks/{id}/cancel
```

### 节点 API

```bash
# 获取节点列表
GET /api/v1/nodes

# 节点心跳
POST /api/v1/nodes/{id}/heartbeat
```

## 开发指南

### 项目结构

```
GPUConductor/
├── cmd/gcond/           # 主程序入口
├── internal/
│   ├── api/            # API 处理器
│   ├── auth/           # 认证模块
│   ├── cmd/            # 命令行处理
│   ├── middleware/     # 中间件
│   ├── models/         # 数据模型
│   ├── scheduler/      # 任务调度器
│   ├── server/         # HTTP 服务器
│   └── agent/          # Agent 客户端
├── web/                # 前端文件
├── config/             # 配置文件
├── build/              # 构建输出
└── docker-compose.yml  # Docker 编排
```

### 添加新功能

1. 在 `internal/models/` 中定义数据模型
2. 在 `internal/api/` 中添加 API 处理器
3. 在 `internal/server/` 中注册路由
4. 在 `web/dist/index.html` 中添加前端界面

## 部署指南

### 生产环境部署

1. **安全配置**
   - 更改默认的 JWT 密钥
   - 配置 HTTPS
   - 设置防火墙规则

2. **数据库配置**
   - 使用独立的 PostgreSQL 服务器
   - 配置数据库备份
   - 优化数据库性能

3. **监控配置**
   - 启用指标收集
   - 配置日志轮转
   - 设置告警通知

### 高可用部署

1. **负载均衡**
   - 使用 Nginx 或 HAProxy
   - 配置多个服务器实例

2. **数据库高可用**
   - PostgreSQL 主从复制
   - 连接池配置

3. **缓存高可用**
   - Redis 集群或哨兵模式

## 故障排除

### 常见问题

1. **无法连接数据库**
   - 检查 PostgreSQL 服务是否运行
   - 验证连接字符串配置
   - 检查网络连接

2. **LDAP 认证失败**
   - 验证 LDAP 服务器配置
   - 检查用户 DN 格式
   - 确认管理员凭据

3. **GPU 监控异常**
   - 确认 nvidia-smi 可用
   - 检查 Agent 与服务器连接
   - 验证 Docker 权限

### 日志查看

```bash
# 查看服务器日志
tail -f gcond.log

# 查看 Docker 容器日志
docker-compose logs -f gcond
```

## 贡献指南

1. Fork 项目
2. 创建功能分支
3. 提交更改
4. 创建 Pull Request

## 许可证

MIT License

## 联系方式

如有问题或建议，请创建 Issue 或联系项目维护者。