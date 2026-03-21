#!/bin/bash
# Cowork 一键部署脚本
# 用法: ./scripts/deploy.sh [选项]
#
# 选项:
#   --build    构建镜像
#   --dev      开发模式 (仅 coordinator + worker)
#   --full     完整模式 (包含测试 worker)
#   --prod     生产模式 (包含前端)
#   --stop     停止服务
#   --logs     查看日志
#   --clean    清理所有数据

set -e

cd "$(dirname "$0")/.."
PROJECT_DIR=$(pwd)

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# 检查 Docker
check_docker() {
    if ! command -v docker &> /dev/null; then
        log_error "Docker 未安装"
        echo "运行以下命令安装 Docker:"
        echo "  sudo ./scripts/install-docker.sh"
        exit 1
    fi
    
    if ! docker info &> /dev/null; then
        log_error "Docker 未运行或无权限"
        echo "尝试添加用户到 docker 组:"
        echo "  sudo usermod -aG docker \$USER"
        echo "然后重新登录"
        exit 1
    fi
}

# 构建前端
build_frontend() {
    log_info "构建前端..."
    cd "$PROJECT_DIR/web"
    npm install --silent
    npm run build
    cd "$PROJECT_DIR"
    log_info "前端构建完成"
}

# 构建镜像
build_images() {
    log_info "构建 Docker 镜像..."
    docker compose -f docker/docker-compose.yml build
    log_info "镜像构建完成"
}

# 启动服务
start_services() {
    local mode=${1:-dev}
    
    log_info "启动服务 (mode: $mode)..."
    
    case $mode in
        dev)
            docker compose -f docker/docker-compose.yml up -d coordinator worker-dev
            ;;
        full)
            docker compose -f docker/docker-compose.yml --profile full up -d
            ;;
        prod)
            build_frontend
            docker compose -f docker/docker-compose.yml --profile prod up -d
            ;;
    esac
    
    log_info "服务已启动"
    echo ""
    echo "访问地址:"
    echo "  - API: http://localhost:8080"
    [ "$mode" = "prod" ] && echo "  - 前端: http://localhost:3000"
    echo ""
    echo "查看日志: ./scripts/deploy.sh --logs"
}

# 停止服务
stop_services() {
    log_info "停止服务..."
    docker compose -f docker/docker-compose.yml down
    log_info "服务已停止"
}

# 查看日志
show_logs() {
    docker compose -f docker/docker-compose.yml logs -f
}

# 清理数据
clean_data() {
    log_warn "这将删除所有数据！"
    read -p "确认继续？ (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        docker compose -f docker/docker-compose.yml down -v
        docker volume rm cowork-data cowork-workdir 2>/dev/null || true
        log_info "数据已清理"
    fi
}

# 主入口
main() {
    cd "$PROJECT_DIR"
    
    case "${1:-}" in
        --build)
            check_docker
            build_images
            ;;
        --dev)
            check_docker
            start_services dev
            ;;
        --full)
            check_docker
            start_services full
            ;;
        --prod)
            check_docker
            start_services prod
            ;;
        --stop)
            stop_services
            ;;
        --logs)
            show_logs
            ;;
        --clean)
            clean_data
            ;;
        *)
            echo "Cowork 部署脚本"
            echo ""
            echo "用法: $0 [选项]"
            echo ""
            echo "选项:"
            echo "  --build    构建 Docker 镜像"
            echo "  --dev      启动开发模式 (coordinator + worker)"
            echo "  --full     启动完整模式 (包含测试 worker)"
            echo "  --prod     启动生产模式 (包含前端)"
            echo "  --stop     停止服务"
            echo "  --logs     查看日志"
            echo "  --clean    清理所有数据"
            echo ""
            echo "快速开始:"
            echo "  1. 安装 Docker: sudo ./scripts/install-docker.sh"
            echo "  2. 构建镜像:    ./scripts/deploy.sh --build"
            echo "  3. 启动服务:    ./scripts/deploy.sh --dev"
            ;;
    esac
}

main "$@"