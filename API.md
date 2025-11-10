# GPUConductor API 文档

所有接口统一前缀为 `/api/v1`，默认返回 JSON，除登录以外均需在 `Authorization` Header 中携带 `Bearer <token>`。

## 认证

### POST /api/v1/auth/login
请求体：
```json
{ "mobile": "13800000000", "password": "******" }
```
响应：
```json
{
  "token": "JWT",
  "user": {
    "id": "uuid",
    "username": "admin",
    "mobile": "13800000000",
    "role": "admin"
  }
}
```

## 用户

### GET /api/v1/users/profile
返回当前用户信息。

## 任务

### GET /api/v1/tasks
查询参数：`page`、`pageSize`、`status`.  
响应：
```json
{
  "tasks": [ { "id": "uuid", "name": "训练任务", ... } ],
  "total": 12,
  "page": 1,
  "size": 20
}
```

### POST /api/v1/tasks
请求体示例：
```json
{
  "name": "图像分类",
  "image": "pytorch/pytorch:latest",
  "command": "python train.py",
  "dataset_path": "/data/dataset01",
  "minio_endpoint": "https://minio.example.com",
  "minio_bucket": "datasets",
  "minio_access_key": "ak",
  "minio_secret_key": "sk",
  "model_output_path": "/data/output",
  "script_path": "scripts/train.sh",
  "iterations": 100,
  "priority": 5,
  "gpu_count": 1,
  "gpu_memory": 0,
  "max_duration": 3600,
  "node_id": "",
  "description": ""
}
```
字段说明：
- `dataset_path`: Agent 可挂载的数据集目录
- `minio_endpoint`/`minio_bucket`/`minio_access_key`/`minio_secret_key`: MinIO 连接信息，用于分发或回传模型文件
- `model_output_path`: 模型产物输出路径
- `script_path`: 镜像内执行的脚本（相对路径）
- `iterations`: 希望运行的迭代/epoch 数，用于排程参考
- `gpu_count`、`gpu_memory`、`max_duration`: 控制资源和最长运行时长（秒）

响应：创建后的任务对象。

### GET /api/v1/tasks/{id}
返回任务详情。

### POST /api/v1/tasks/{id}/cancel
取消任务，响应 `{ "message": "任务已取消" }`。

### GET /api/v1/tasks/{id}/logs
返回任务日志数组。

### PUT /api/v1/tasks/{id}
供 Agent 上报状态，字段：`status`、`assigned_node_id`、`container_id` 等。

## 节点

### GET /api/v1/nodes
返回所有节点及 GPU 信息。

### PUT /api/v1/nodes/{id}/status
更新节点状态（管理员）。

### POST /api/v1/nodes/{id}/heartbeat
Agent 心跳：
```json
{
  "name": "node-1",
  "address": "10.0.0.2",
  "tags": ["gpu-a100"],
  "status": "online",
  "gpus": [ { "index": 0, "name": "A100", ... } ]
}
```

## GPU 及统计

### GET /api/v1/gpu/stats
返回所有 GPU 指标。

### GET /api/v1/stats
返回节点/任务/GPU 汇总。

## 配置 / 其他

### GET /api/v1/config/redis
响应：`{ "redis": "host:port" }`。

### GET /api/v1/health
健康检查。

### 根路径 `/`
返回一个 JSON，列出可用 API，而不再提供内置前端。
