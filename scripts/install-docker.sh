#!/bin/bash
# Docker 安装脚本 - Ubuntu/Debian
# 用法: sudo ./scripts/install-docker.sh

set -e

echo "=== 安装 Docker ==="

# 检查是否已安装
if command -v docker &> /dev/null; then
    echo "Docker 已安装: $(docker --version)"
    exit 0
fi

# 更新包索引
echo "更新包索引..."
apt-get update -qq

# 安装依赖
echo "安装依赖..."
apt-get install -y -qq \
    ca-certificates \
    curl \
    gnupg \
    lsb-release

# 添加 Docker 官方 GPG key
echo "添加 Docker GPG key..."
mkdir -p /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg

# 添加 Docker 仓库
echo "添加 Docker 仓库..."
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null

# 安装 Docker
echo "安装 Docker..."
apt-get update -qq
apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

# 启动 Docker 服务
echo "启动 Docker 服务..."
systemctl enable docker
systemctl start docker

# 将当前用户添加到 docker 组（可选）
if [ -n "$SUDO_USER" ]; then
    echo "将 $SUDO_USER 添加到 docker 组..."
    usermod -aG docker "$SUDO_USER"
    echo "注意: 需要重新登录才能生效"
fi

echo ""
echo "=== 安装完成 ==="
docker --version
docker compose version