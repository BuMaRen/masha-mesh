#!/bin/bash

# ctrl 调试部署脚本
# 用于单独部署和调试 mesh-ctrl 组件
# 使用方法: 
#   ./deploy-ctrl.sh [--skip-push] [--dry-run]
#
# 选项:
#   --skip-push    跳过镜像推送，仅构建和部署
#   --dry-run      仅显示将要执行的操作，不实际执行

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUILD_DIR="${SCRIPT_DIR}/build"
IMG_CTRL="hjmasha/mesh-ctrl"
SKIP_PUSH=false
DRY_RUN=false

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 打印信息函数
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_debug() {
    echo -e "${BLUE}[DEBUG]${NC} $1"
}

# 解析命令行参数
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --skip-push)
                SKIP_PUSH=true
                shift
                ;;
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            *)
                log_error "未知参数: $1"
                exit 1
                ;;
        esac
    done
}

# 从deployment.yml文件中提取当前版本号
get_current_version() {
    local file="$1"
    if [ ! -f "$file" ]; then
        echo ""
        return
    fi
    
    # 提取 image 行中的版本号（形式：hjmasha/mesh-ctrl:tX.Y.Z）
    local version=$(sed -n 's/.*image:.*hjmasha\/mesh-ctrl:\(t[0-9.]*\).*/\1/p' "$file" | head -1)
    echo "$version"
}

# 自动递增版本号
auto_increment_version() {
    local current="$1"
    
    # 检查是否为 t 开头的版本号格式（如 t0.1.5）
    if [[ $current =~ ^t([0-9]+\.[0-9]+\.)([0-9]+)$ ]]; then
        local prefix="${BASH_REMATCH[1]}"
        local number="${BASH_REMATCH[2]}"
        local new_number=$((number + 1))
        echo "t${prefix}${new_number}"
    else
        log_error "无法解析版本号格式: $current （期望格式：t0.1.0）"
        exit 1
    fi
}

# 更新deployment.yml文件中的版本号
update_deployment_file() {
    local file="$1"
    local new_version="$2"

    log_info "更新 $file 中的镜像版本 -> $new_version"

    if [ "$DRY_RUN" = true ]; then
        log_debug "（DRY RUN）将执行: sed 替换 $file"
        return
    fi

    # 兼容macOS和Linux的sed -i语法
    if [[ "$OSTYPE" == "darwin"* ]] || sed --version 2>&1 | grep -q "BSD"; then
        # macOS/BSD sed
        sed -E -i '' "s#(hjmasha/mesh-ctrl):[^[:space:]]+#\1:${new_version}#g" "$file"
    else
        # GNU sed
        sed -E -i "s#(hjmasha/mesh-ctrl):[^[:space:]]+#\1:${new_version}#g" "$file"
    fi
}

# 编译 ctrl
build_ctrl() {
    log_info "编译 mesh-ctrl..."
    
    if [ "$DRY_RUN" = true ]; then
        log_debug "（DRY RUN）将执行: CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o _output/mesh-ctrl cmd/ctrl/main.go"
        return
    fi

    cd "$SCRIPT_DIR"
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o _output/mesh-ctrl cmd/ctrl/main.go
    log_info "mesh-ctrl 编译完成"
}

# 构建 docker 镜像
build_image() {
    local version="$1"
    
    log_info "构建 Docker 镜像: ${IMG_CTRL}:${version}"
    
    if [ "$DRY_RUN" = true ]; then
        log_debug "（DRY RUN）将执行: docker build -f ${BUILD_DIR}/ctrl/Dockerfile . -t ${IMG_CTRL}:${version}"
        return
    fi

    cd "$SCRIPT_DIR"
    docker build -f "${BUILD_DIR}/ctrl/Dockerfile" . -t "${IMG_CTRL}:${version}" -t "${IMG_CTRL}:latest"
    log_info "Docker 镜像构建完成"
}

# 推送镜像
push_image() {
    local version="$1"
    
    log_info "推送镜像: ${IMG_CTRL}:${version}"
    
    if [ "$DRY_RUN" = true ]; then
        log_debug "（DRY RUN）将执行: docker push ${IMG_CTRL}:${version} && docker push ${IMG_CTRL}:latest"
        return
    fi

    docker push "${IMG_CTRL}:${version}"
    docker push "${IMG_CTRL}:latest"
    log_info "镜像推送完成"
}

# 部署到kubernetes
deploy_to_k8s() {
    log_info "部署到 Kubernetes..."
    
    if [ "$DRY_RUN" = true ]; then
        log_debug "（DRY RUN）将执行: kubectl apply -f ${BUILD_DIR}/ctrl/deployment.yml"
        return
    fi

    kubectl apply -f "${BUILD_DIR}/ctrl/deployment.yml"
    log_info "Kubernetes 部署完成"
}

# 检查所有必需的命令
check_requirements() {
    local missing_cmd=false
    
    for cmd in go docker kubectl sed; do
        if ! command -v "$cmd" &> /dev/null; then
            log_error "缺少必需的命令: $cmd"
            missing_cmd=true
        fi
    done
    
    if [ "$missing_cmd" = true ]; then
        exit 1
    fi
}

# 主函数
main() {
    parse_args "$@"
    
    log_info "========== mesh-ctrl 调试部署脚本 =========="
    
    if [ "$DRY_RUN" = true ]; then
        log_warn "运行模式: DRY RUN（仅显示操作，不实际执行）"
    fi
    
    check_requirements
    
    local ctrl_file="${BUILD_DIR}/ctrl/deployment.yml"
    local current_version=$(get_current_version "$ctrl_file")
    
    if [ -z "$current_version" ]; then
        log_warn "无法从 deployment.yml 获取当前版本号，使用初始版本: t0.1.0"
        current_version="t0.1.0"
    else
        log_info "当前版本: $current_version"
    fi
    
    # 递增版本号
    local new_version=$(auto_increment_version "$current_version")
    log_info "新版本号: $new_version"
    
    # 执行编译、构建、推送、部署流程
    build_ctrl
    update_deployment_file "$ctrl_file" "$new_version"
    build_image "$new_version"
    
    if [ "$SKIP_PUSH" = false ]; then
        push_image "$new_version"
    else
        log_info "跳过镜像推送（--skip-push 选项）"
    fi
    
    deploy_to_k8s
    
    log_info "========== 部署流程完成 =========="
    log_info "镜像版本: $new_version"
    log_info "可使用以下命令查看部署状态:"
    log_info "  kubectl get deployment mesh-ctrl"
    log_info "  kubectl describe deployment mesh-ctrl"
    log_info "  kubectl logs -f deployment/mesh-ctrl"
}

main "$@"
