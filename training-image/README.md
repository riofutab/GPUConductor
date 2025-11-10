# 示例训练镜像

本目录包含一个可直接用于 GPUConductor 的训练镜像示例：

- 基于 `python:3.11-slim`，使用 [uv](https://github.com/astral-sh/uv) 管理 Python 依赖；
- 默认挂载 `/workspace/dataset` 与 `/workspace/output`；
- 通过环境变量控制：
  - `DATASET_PATH`：数据集所在目录（由 Agent 注入，默认 `/workspace/dataset`）
  - `MODEL_OUTPUT_PATH`：模型产物输出目录（默认 `/workspace/output`）
  - `TRAINING_SCRIPT`：要执行的脚本（传给 `bash -c`）
  - `TRAINING_ITERATIONS`：训练迭代次数

构建镜像：

```bash
cd training-image
docker build -t gpuconductor/train:latest .
```

镜像入口 `entrypoint.sh` 会检测 `requirements.txt` 并使用 uv 安装依赖。若提供 `TRAINING_SCRIPT` 环境变量则直接执行脚本，否则回退到 `python train.py`。
