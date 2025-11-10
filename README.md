# GPUConductor

一个分布式GPU任务调度系统，支持多机器部署、统一界面访问，可以根据GPU使用率进行算法模型训练任务排队。

## 功能特性

- 🚀 **分布式部署**: 主控 + 多 Agent 结构，按节点标签调度
- 📊 **GPU监控**: 实时采集节点 GPU 使用率、温度、显存等指标
- 🎯 **智能调度**: 按优先级、标签、资源需求分配任务
- ⏱️ **时间控制**: 约束最大运行时间并支持任务取消
- 🐳 **容器化执行**: 自动挂载数据集/输出目录，记录容器日志
- ☁️ **模型归档**: 任务完成后自动上传模型到 MinIO/S3
- 🔐 **安全认证**: JWT + LDAP（可选）

## 系统架构

```
GPUConductor
├── 中央服务器 (Master)
│   ├── API服务
│   ├── 调度器
│   └── 数据库/Redis
└── 节点代理 (Agent)
    ├── GPU监控
    ├── 容器管理
    └── 心跳检测
```

## 快速开始

### 环境要求

- Go 1.19+
- Docker
- NVIDIA驱动和nvidia-smi
- Redis (可选)

### 安装部署

1. **克隆项目**
```bash
git clone <repository-url>
cd GPUConductor
```

2. **构建项目**
```bash
# Windows
build.bat

# Linux/Mac
chmod +x build.sh
./build.sh
```

3. **启动服务器**
```bash
# 启动中央服务器
gcond.exe

# 启动节点代理 (在其他机器上)
set GCOND_MODE=agent
gcond.exe
```

4. **验证 API / 启动 Agent**
```bash
# 验证健康状态
curl http://localhost:8080/api/v1/health

# 在节点机器上启动 Agent
./gcond agent --config config/agent.yaml
```
默认管理员账号: `admin` / `admin123`

## 配置说明

### 主要配置项

```yaml
# 服务器配置
server:
  host: "0.0.0.0"
  port: 8080

# GPU监控
gpu:
  monitor_interval: 5
  utilization_threshold: 20

# 任务调度
scheduler:
  max_concurrent_tasks: 4
  task_timeout: 3600
```

### 节点标签

节点支持标签系统，用于任务绑定：

- `gpu-v100`: V100 GPU节点
- `gpu-a100`: A100 GPU节点  
- `high-memory`: 大内存节点
- `fast-network`: 高速网络节点

## API接口

### 认证接口
- `POST /api/login` - 用户登录
- `POST /api/register` - 用户注册

### 任务管理
- `GET /api/tasks` - 获取任务列表
- `POST /api/tasks` - 创建新任务
- `GET /api/tasks/:id` - 获取任务详情
- `POST /api/tasks/:id/cancel` - 取消任务

### GPU监控
- `GET /api/gpu/stats` - 获取GPU状态
- `GET /api/nodes` - 获取节点列表

## 任务配置示例

### 创建训练任务

```json
{
  "name": "图像分类训练",
  "image": "gpuconductor/train:latest",
  "command": "python main.py",
  "dataset_path": "/data/datasets/imagenet",
  "model_output_path": "/data/output/run-001",
  "minio_endpoint": "https://minio.example.com",
  "minio_bucket": "ml-artifacts",
  "minio_access_key": "AK",
  "minio_secret_key": "SK",
  "script_path": "scripts/train.sh",
  "iterations": 200,
  "gpu_count": 2,
  "max_duration": 7200,
  "node_tags": ["gpu-4090", "linux"]
}
```

字段说明：

| 字段 | 说明 |
|------|------|
| `dataset_path` | 节点可访问的数据集目录，会被挂载到容器 `/workspace/dataset` |
| `model_output_path` | 模型/日志输出目录，任务完成后上传至 MinIO |
| `script_path` | 容器内需要执行的脚本路径（传给 entrypoint） |
| `iterations` | 训练迭代或 epoch 数，会注入 `TRAINING_ITERATIONS` |
| `minio_*` | MinIO 访问参数，配合模型上传使用 |
| `gpu_count`、`max_duration` 等 | 控制资源和最长运行时长 |

### 任务状态

- `pending`: 等待中
- `running`: 运行中  
- `completed`: 已完成
- `failed`: 失败
- `cancelled`: 已取消

## 开发指南

### 项目结构

```
GPUConductor/
├── internal/
│   ├── api/          # API处理器
│   ├── agent/        # 节点代理
│   ├── docker/       # Docker管理
│   ├── models/       # 数据模型
│   └── scheduler/    # 任务调度
├── config/           # server/agent 配置
├── training-image/   # 训练镜像模板
└── main.go           # 程序入口

### 训练镜像模板

`training-image/` 目录提供了默认镜像（Python 3.11 + uv）：

```bash
cd training-image
docker build -t gpuconductor/train:latest .
```

镜像会读取以下环境变量：

- `DATASET_PATH`（默认 `/workspace/dataset`）
- `MODEL_OUTPUT_PATH`（默认 `/workspace/output`）
- `TRAINING_SCRIPT`（优先执行）
- `TRAINING_ITERATIONS`
- `MINIO_*`（用于脚本自行上传）

你可以基于此镜像添加 `requirements.txt`、`scripts/train.sh` 等自定义逻辑。
```

### 添加新的API接口

1. 在 `internal/api/` 创建新的处理器
2. 在 `main.go` 中注册路由
3. 更新前端界面调用新接口

## 故障排除

### 常见问题

1. **GPU监控不工作**
   - 检查nvidia-smi是否可用
   - 验证NVIDIA驱动版本

2. **Docker容器启动失败**
   - 检查Docker服务状态
   - 验证训练镜像是否存在

3. **节点连接失败**
   - 检查网络连通性
   - 验证防火墙设置

### 日志查看

日志文件位置: `gcond.log`

```bash
# 查看实时日志
tail -f gcond.log
```

## 许可证

MIT License

## 贡献

欢迎提交Issue和Pull Request！

## 联系方式

- 项目主页: [GitHub Repository]
- 问题反馈: [Issues]
- 文档: [Wiki]

---

**GPUConductor** - 让GPU任务调度更简单高效！
