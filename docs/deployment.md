# Cowork 部署指南

## 1. 本地开发部署

### 1.1 环境要求

| 工具 | 版本 | 用途 |
|------|------|------|
| Go | 1.21+ | 后端编译 |
| Node.js | 18+ | 前端构建 |
| SQLite | 3.x | 数据库 |

### 1.2 快速启动

```bash
# 克隆项目
cd ~/projects/cowork

# 启动 Coordinator
go run ./cmd/coordinator

# 启动 Worker（另一个终端）
go run ./cmd/worker --name=worker-1 --tags=dev --model=gpt-4

# 启动前端（另一个终端）
cd web
npm install
npm run dev
```

### 1.3 配置文件

```yaml
# config.yaml
server:
  host: "0.0.0.0"
  port: 8080
  
database:
  type: "sqlite"
  path: "./data/cowork.db"
  
websocket:
  path: "/ws"
  heartbeat_interval: 30s
  
agent:
  default_model: "gpt-4"
  models:
    - id: "gpt-4"
      provider: "openai"
      api_key: "${OPENAI_API_KEY}"
    - id: "claude-3"
      provider: "anthropic"
      api_key: "${ANTHROPIC_API_KEY}"
    - id: "glm-4"
      provider: "zhipu"
      api_key: "${ZHIPU_API_KEY}"

storage:
  work_dir: "/tmp/cowork"
  max_file_size: 100MB
  
log:
  level: "info"
  format: "json"
```

---

## 2. Docker 部署

### 2.1 Dockerfile - Coordinator

```dockerfile
# docker/Dockerfile.coordinator
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o coordinator ./cmd/coordinator

FROM alpine:3.19

RUN apk add --no-cache ca-certificates sqlite
WORKDIR /app

COPY --from=builder /app/coordinator .
COPY --from=builder /app/web/dist ./web/dist

EXPOSE 8080

CMD ["./coordinator"]
```

### 2.2 Dockerfile - Worker

```dockerfile
# docker/Dockerfile.worker
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o worker ./cmd/worker

FROM alpine:3.19

RUN apk add --no-cache ca-certificates git
WORKDIR /app

COPY --from=builder /app/worker .

# 可选：安装 Docker CLI（用于容器执行模式）
# RUN apk add --no-cache docker-cli

CMD ["./worker"]
```

### 2.3 Docker Compose

```yaml
# docker/docker-compose.yml
version: '3.8'

services:
  coordinator:
    build:
      context: ..
      dockerfile: docker/Dockerfile.coordinator
    ports:
      - "8080:8080"
    volumes:
      - cowork-data:/app/data
      - cowork-workdir:/tmp/cowork
    environment:
      - COORDINATOR_URL=http://coordinator:8080
    restart: unless-stopped

  worker-dev:
    build:
      context: ..
      dockerfile: docker/Dockerfile.worker
    depends_on:
      - coordinator
    environment:
      - COORDINATOR_URL=http://coordinator:8080
      - WORKER_NAME=worker-dev
      - WORKER_TAGS=dev,coding
      - WORKER_MODEL=gpt-4
    volumes:
      - cowork-workdir:/tmp/cowork
    restart: unless-stopped

  worker-test:
    build:
      context: ..
      dockerfile: docker/Dockerfile.worker
    depends_on:
      - coordinator
    environment:
      - COORDINATOR_URL=http://coordinator:8080
      - WORKER_NAME=worker-test
      - WORKER_TAGS=test,qa
      - WORKER_MODEL=claude-3
    volumes:
      - cowork-workdir:/tmp/cowork
    restart: unless-stopped

volumes:
  cowork-data:
  cowork-workdir:
```

### 2.4 启动命令

```bash
# 构建镜像
docker-compose build

# 启动服务
docker-compose up -d

# 查看日志
docker-compose logs -f coordinator

# 停止服务
docker-compose down

# 停止并删除数据
docker-compose down -v
```

---

## 3. 生产部署

### 3.1 Systemd 服务

```ini
# /etc/systemd/system/cowork-coordinator.service
[Unit]
Description=Cowork Coordinator
After=network.target

[Service]
Type=simple
User=cowork
Group=cowork
WorkingDirectory=/opt/cowork
ExecStart=/opt/cowork/coordinator
Restart=on-failure
RestartSec=5s

# 环境变量
EnvironmentFile=/opt/cowork/.env

# 资源限制
LimitNOFILE=65536
MemoryMax=2G

[Install]
WantedBy=multi-user.target
```

```ini
# /etc/systemd/system/cowork-worker.service
[Unit]
Description=Cowork Worker
After=network.target cowork-coordinator.service

[Service]
Type=simple
User=cowork
Group=cowork
WorkingDirectory=/opt/cowork
ExecStart=/opt/cowork/worker --name=worker-1 --tags=dev
Restart=on-failure
RestartSec=5s

EnvironmentFile=/opt/cowork/.env

[Install]
WantedBy=multi-user.target
```

**启用服务**：
```bash
sudo systemctl daemon-reload
sudo systemctl enable cowork-coordinator
sudo systemctl enable cowork-worker
sudo systemctl start cowork-coordinator
sudo systemctl start cowork-worker
```

### 3.2 Nginx 反向代理

```nginx
# /etc/nginx/sites-available/cowork
server {
    listen 80;
    server_name cowork.example.com;
    
    # 重定向到 HTTPS
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name cowork.example.com;
    
    ssl_certificate /etc/letsencrypt/live/cowork.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/cowork.example.com/privkey.pem;
    
    # API 代理
    location /api {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
    
    # WebSocket 代理
    location /ws {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_read_timeout 86400;
    }
    
    # 静态文件
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
    }
    
    # 文件上传大小限制
    client_max_body_size 100M;
}
```

### 3.3 PostgreSQL 迁移（可选）

如果需要更高性能，可以迁移到 PostgreSQL：

```yaml
# docker-compose.yml 添加 PostgreSQL
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: cowork
      POSTGRES_USER: cowork
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - postgres-data:/var/lib/postgresql/data
    restart: unless-stopped

  coordinator:
    # ...
    environment:
      - DB_TYPE=postgres
      - DB_HOST=postgres
      - DB_PORT=5432
      - DB_NAME=cowork
      - DB_USER=cowork
      - DB_PASSWORD=${DB_PASSWORD}
    depends_on:
      - postgres

volumes:
  postgres-data:
```

---

## 4. 监控与日志

### 4.1 日志轮转

```bash
# /etc/logrotate.d/cowork
/var/log/cowork/*.log {
    daily
    rotate 30
    compress
    delaycompress
    missingok
    notifempty
    create 0640 cowork cowork
}
```

### 4.2 Prometheus 监控（可选）

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'cowork-coordinator'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'
```

### 4.3 健康检查

```bash
# 健康检查脚本
#!/bin/bash
# scripts/healthcheck.sh

COORDINATOR_URL="http://localhost:8080"

# 检查 Coordinator
if ! curl -sf "$COORDINATOR_URL/api/system/stats" > /dev/null; then
    echo "Coordinator is not responding"
    exit 1
fi

# 检查 WebSocket
if ! timeout 5 bash -c "echo '{\"type\":\"ping\"}' | websocat -n1 ws://localhost:8080/ws" > /dev/null; then
    echo "WebSocket is not responding"
    exit 1
fi

echo "All services healthy"
exit 0
```

---

## 5. 备份策略

### 5.1 数据备份

```bash
#!/bin/bash
# scripts/backup.sh

BACKUP_DIR="/backup/cowork"
DATE=$(date +%Y%m%d_%H%M%S)

# 创建备份目录
mkdir -p $BACKUP_DIR

# 备份 SQLite 数据库
cp /opt/cowork/data/cowork.db "$BACKUP_DIR/cowork_$DATE.db"

# 备份配置
cp /opt/cowork/config.yaml "$BACKUP_DIR/config_$DATE.yaml"

# 清理旧备份（保留 30 天）
find $BACKUP_DIR -name "*.db" -mtime +30 -delete

echo "Backup completed: cowork_$DATE.db"
```

### 5.2 自动备份（Cron）

```bash
# 每天凌晨 2 点备份
0 2 * * * /opt/cowork/scripts/backup.sh >> /var/log/cowork/backup.log 2>&1
```

---

## 6. 安全加固

### 6.1 防火墙配置

```bash
# 只允许本地访问 Coordinator
sudo ufw allow 80/tcp   # Nginx
sudo ufw allow 443/tcp  # Nginx HTTPS
sudo ufw enable
```

### 6.2 环境变量管理

```bash
# /opt/cowork/.env
# 敏感配置通过环境变量注入
OPENAI_API_KEY=sk-xxx
ANTHROPIC_API_KEY=sk-xxx
JWT_SECRET=your-secret-key
```

### 6.3 文件权限

```bash
# 设置正确的文件权限
sudo chown -R cowork:cowork /opt/cowork
sudo chmod 700 /opt/cowork
sudo chmod 600 /opt/cowork/.env
```

---

## 7. 故障排查

### 7.1 常见问题

| 问题 | 可能原因 | 解决方案 |
|------|----------|----------|
| Worker 离线 | 网络问题/心跳超时 | 检查网络连接，重启 Worker |
| 任务卡住 | Worker 崩溃 | 检查日志，重启 Worker |
| 数据库锁定 | SQLite 并发限制 | 迁移到 PostgreSQL |
| WebSocket 断开 | 代理超时 | 增加 Nginx proxy_read_timeout |

### 7.2 诊断命令

```bash
# 检查服务状态
sudo systemctl status cowork-coordinator
sudo systemctl status cowork-worker

# 查看日志
journalctl -u cowork-coordinator -f
journalctl -u cowork-worker -f

# 检查端口占用
netstat -tlnp | grep 8080

# 检查数据库
sqlite3 /opt/cowork/data/cowork.db ".tables"
```