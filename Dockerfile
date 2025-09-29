# 多阶段构建 Dockerfile
FROM golang:1.21-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装必要的包
RUN apk add --no-cache git ca-certificates tzdata

# 复制 go mod 文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o gcond ./cmd/gcond

# 运行阶段
FROM alpine:latest

# 安装必要的包
RUN apk --no-cache add ca-certificates tzdata

# 创建非root用户
RUN addgroup -g 1001 -S gcond && \
    adduser -u 1001 -S gcond -G gcond

WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/gcond .

# 创建数据目录
RUN mkdir -p /app/data && chown -R gcond:gcond /app

# 切换到非root用户
USER gcond

# 暴露端口
EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/v1/stats || exit 1

# 启动命令
CMD ["./gcond", "server", "--port", "8080", "--database", "/app/data/gcond.db"]