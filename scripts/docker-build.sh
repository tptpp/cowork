#!/bin/bash
# Docker 构建和部署脚本

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_DIR"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# 构建前端
build_frontend() {
    log_info "Building frontend..."
    cd web
    npm install
    npm run build
    cd ..
    log_info "Frontend build completed"
}

# 构建 Docker 镜像
build_images() {
    log_info "Building Docker images..."
    docker-compose -f docker/docker-compose.yml build
    log_info "Docker images built"
}

# 启动服务
start_services() {
    log_info "Starting services..."
    docker-compose -f docker/docker-compose.yml up -d
    log_info "Services started"
    
    echo ""
    log_info "Gateway: http://localhost:8080"
    log_info "Health check: http://localhost:8080/health"
    log_info "API docs: http://localhost:8080/api/system/stats"
}

# 停止服务
stop_services() {
    log_info "Stopping services..."
    docker-compose -f docker/docker-compose.yml down
    log_info "Services stopped"
}

# 查看日志
view_logs() {
    docker-compose -f docker/docker-compose.yml logs -f "$@"
}

# 清理
cleanup() {
    log_warn "This will remove all containers, volumes, and images"
    read -p "Are you sure? (y/N) " confirm
    if [[ "$confirm" =~ ^[Yy]$ ]]; then
        docker-compose -f docker/docker-compose.yml down -v --rmi local
        log_info "Cleanup completed"
    fi
}

# 使用帮助
usage() {
    echo "Usage: $0 <command>"
    echo ""
    echo "Commands:"
    echo "  build       Build frontend and Docker images"
    echo "  frontend    Build frontend only"
    echo "  images      Build Docker images only"
    echo "  start       Start services"
    echo "  stop        Stop services"
    echo "  restart     Restart services"
    echo "  logs        View logs (optional: service name)"
    echo "  status      Show service status"
    echo "  cleanup     Remove all containers, volumes, and images"
    echo ""
    echo "Examples:"
    echo "  $0 build"
    echo "  $0 start"
    echo "  $0 logs gateway"
}

# 状态
show_status() {
    docker-compose -f docker/docker-compose.yml ps
}

# 主入口
case "${1:-}" in
    build)
        build_frontend
        build_images
        ;;
    frontend)
        build_frontend
        ;;
    images)
        build_images
        ;;
    start)
        start_services
        ;;
    stop)
        stop_services
        ;;
    restart)
        stop_services
        start_services
        ;;
    logs)
        shift
        view_logs "$@"
        ;;
    status)
        show_status
        ;;
    cleanup)
        cleanup
        ;;
    *)
        usage
        exit 1
        ;;
esac