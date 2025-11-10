# GPUConductor 快速开始指南

## 🚀 快速部署

### 1. 下载二进制文件

从 [GitHub Releases](https://github.com/your-username/GPUConductor/releases) 下载最新版本：

```bash
# 下载 Linux AMD64 版本
wget https://github.com/your-username/GPUConductor/releases/latest/download/gcond-linux-amd64

# 或者下载 Linux ARM64 版本
wget https://github.com/your-username/GPUConductor/releases/latest/download/gcond-linux-arm64

# 添加执行权限
chmod +x gcond-linux-amd64
```

### 2. 验证安装

```bash
# 查看版本信息
./gcond-linux-amd64 version

# 查看帮助信息
./gcond-linux-amd64 --help
```

### 3. 配置文件

创建配置文件 `config.yaml`：

```yaml
# 服务器配置
server:
  port: "8080"
  database: "host=localhost user=gcond password=gcond dbname=gcond port=5432 sslmode=disable"
  redis: "localhost:6379"

# LDAP 认证配置
ldap:
  host: "localhost"
  port: 389
  base_dn: "dc=example,dc=com"
  user_dn: "mobile=%s,ou=users,dc=example,dc=com"
  bind_dn: "cn=admin,dc=example,dc=com"
  bind_pass: "admin"

# JWT 配置
jwt:
  secret: "your-secret-key-change-this-in-production"
  expiration_hours: 24
  refresh_expiration_days: 7
```

### 4. 启动服务

#### 方式一：使用 Docker Compose（推荐）

```bash
# 下载 docker-compose.yml
wget https://raw.githubusercontent.com/your-username/GPUConductor/main/docker-compose.yml

# 启动完整环境
docker-compose up -d

# 查看服务状态
docker-compose ps
```

#### 方式二：手动启动

```bash
# 启动服务器
./gcond-linux-amd64 server --config config.yaml

# 在其他 GPU 机器上启动 Agent
./gcond-linux-amd64 agent --server http://server-ip:8080 --node-id gpu-node-1
```

### 5. 访问系统

- **Web 界面**: http://localhost:8080
- **LDAP 管理**: http://localhost:8081 (Docker 环境)

## 🔧 常用命令

### 服务器命令

```bash
# 启动服务器
./gcond-linux-amd64 server

# 指定端口和配置
./gcond-linux-amd64 server --port 8080 --config config.yaml

# 调试模式
./gcond-linux-amd64 server --debug
```

### Agent 命令

```bash
# 启动 Agent
./gcond-linux-amd64 agent --server http://server:8080 --node-id gpu-node-1

# 指定 GPU 设备
./gcond-linux-amd64 agent --server http://server:8080 --node-id gpu-node-1 --gpu-devices 0,1

# 设置资源限制
./gcond-linux-amd64 agent --server http://server:8080 --node-id gpu-node-1 --max-tasks 4
```

## 🐛 故障排除

### 1. "cannot execute binary file" 错误

```bash
# 检查系统架构
uname -m
# x86_64  -> 使用 gcond-linux-amd64
# aarch64 -> 使用 gcond-linux-arm64

# 检查文件权限
ls -la gcond-linux-amd64
chmod +x gcond-linux-amd64

# 检查文件类型
file gcond-linux-amd64
```

### 2. 数据库连接失败

```bash
# 检查 PostgreSQL 服务
sudo systemctl status postgresql

# 测试数据库连接
psql -h localhost -U gcond -d gcond

# 使用 Docker 启动 PostgreSQL
docker run -d --name postgres \
  -e POSTGRES_USER=gcond \
  -e POSTGRES_PASSWORD=gcond \
  -e POSTGRES_DB=gcond \
  -p 5432:5432 \
  postgres:15
```

### 3. LDAP 认证失败

```bash
# 测试 LDAP 连接
ldapsearch -x -H ldap://localhost:389 -D "cn=admin,dc=example,dc=com" -W

# 使用 Docker 启动 LDAP 服务
docker run -d --name openldap \
  -e LDAP_ADMIN_USERNAME=admin \
  -e LDAP_ADMIN_PASSWORD=admin \
  -e LDAP_ROOT=dc=example,dc=com \
  -p 389:1389 \
  bitnami/openldap:latest
```

### 4. 端口被占用

```bash
# 检查端口占用
netstat -tlnp | grep 8080

# 使用其他端口
./gcond-linux-amd64 server --port 8081
```

## 📊 监控和日志

### 查看日志

```bash
# 启动时输出详细日志
./gcond-linux-amd64 server --debug

# 使用 systemd 管理服务
sudo systemctl status gcond
sudo journalctl -u gcond -f
```

### 系统监控

```bash
# 查看 GPU 使用情况
nvidia-smi

# 查看系统资源
htop

# 查看网络连接
ss -tlnp | grep 8080
```

## 🔄 更新升级

```bash
# 停止服务
sudo systemctl stop gcond

# 备份配置
cp config.yaml config.yaml.bak

# 下载新版本
wget https://github.com/your-username/GPUConductor/releases/latest/download/gcond-linux-amd64

# 替换二进制文件
chmod +x gcond-linux-amd64
sudo cp gcond-linux-amd64 /usr/local/bin/gcond

# 重启服务
sudo systemctl start gcond
```

## 📞 获取帮助

- **GitHub Issues**: https://github.com/your-username/GPUConductor/issues
- **文档**: [README.md](README.md)
- **构建指南**: [BUILD.md](BUILD.md)
- **发布说明**: [RELEASE.md](RELEASE.md)