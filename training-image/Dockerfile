# 基础训练镜像示例，默认使用 Python 3.11 + uv
FROM python:3.11-slim

ENV DEBIAN_FRONTEND=noninteractive \
    LANG=C.UTF-8 \
    LC_ALL=C.UTF-8

RUN apt-get update && \
    apt-get install -y --no-install-recommends curl git build-essential ca-certificates && \
    rm -rf /var/lib/apt/lists/* && \
    curl -LsSf https://astral.sh/uv/install.sh | sh

ENV PATH="/root/.cargo/bin:/root/.local/bin:${PATH}"

WORKDIR /workspace
COPY entrypoint.sh /opt/train/entrypoint.sh
RUN chmod +x /opt/train/entrypoint.sh

ENTRYPOINT ["/opt/train/entrypoint.sh"]
