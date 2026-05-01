#!/bin/bash

# 部署自动化脚本
# 使用方法: 
#   ./deploy.sh [VERSION]
#   ./deploy.sh --dry-run [VERSION]
#   如果不指定VERSION，将自动从现有版本号递增

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUILD_DIR="${SCRIPT_DIR}/build"
DRY_RUN=false

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
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

# 从deployment.yml文件中提取当前版本号
get_current_version() {
    local file="$1"
    if [ ! -f "$file" ]; then
        echo ""
        return
    fi
    
    # 提取image行中的版本号（使用sed更便携）
    # 使用 [^:]* 匹配 ctrl 或 cli，避免 BSD sed 不支持 \| 的问题
    local version=$(sed -n 's/.*image:.*hjmasha\/mesh-[^:]*:v\([0-9.]*\).*/\1/p' "$file" | head -1)
    echo "$version"
}

# 自动递增版本号
auto_increment_version() {
    local current="$1"
    
    # 提取v0.1.xx中的xx部分
    if [[ $current =~ ^([0-9]+\.[0-9]+\.)([0-9]+)$ ]]; then
        local prefix="${BASH_REMATCH[1]}"
        local number="${BASH_REMATCH[2]}"
        local new_number=$((number + 1))
        echo "v${prefix}${new_number}"
    else
        log_error "无法解析版本号格式: $current"
        exit 1
    fi
}

# 更新deployment.yml文件中的版本号
update_deployment_file() {
    local file="$1"
    local new_version="$2"

    log_info "更新 $file 中的镜像版本 -> $new_version"

    # 直接按镜像名替换tag，不依赖旧版本号，避免ctrl/cli版本不一致时替换失败
    # 兼容macOS和Linux的sed -i语法
    if [[ "$OSTYPE" == "darwin"* ]] || sed --version 2>&1 | grep -q "BSD"; then
        # macOS/BSD sed需要提供备份扩展名参数，使用空字符串表示不备份
        sed -E -i '' "s#(hjmasha/mesh-(ctrl|cli)):[^[:space:]]+#\\1:${new_version}#g" "$file"
    else
        # GNU sed
        sed -E -i "s#(hjmasha/mesh-(ctrl|cli)):[^[:space:]]+#\\1:${new_version}#g" "$file"
    fi
}

# 主函数
main() {
    if [ "$DRY_RUN" = true ]; then
        log_info "开始部署流程（DRY RUN 模式）..."
    else
        log_info "开始部署流程..."
    fi
    
    # 获取当前版本号
    local ctrl_file="${BUILD_DIR}/ctrl/deployment.yml"
    local cli_file="${BUILD_DIR}/cli/deployment.yml"
    local test_client_file="${SCRIPT_DIR}/tests/client/deployment.yml"
    local test_server_file="${SCRIPT_DIR}/tests/server/deployment.yml"
    
    local current_version=$(get_current_version "$ctrl_file")
    
    if [ -z "$current_version" ]; then
        log_warn "无法从 $ctrl_file 获取当前版本号"
        current_version="0.1.0"
    fi
    
    log_info "当前版本号: v${current_version}"
    
    # 确定新版本号
    local new_version
    if [ -n "$1" ]; then
        # 使用用户指定的版本号
        new_version="$1"
        # 移除开头的v（如果有的话）
        new_version="${new_version#v}"
        new_version="v${new_version}"
    else
        # 自动递增版本号
        new_version=$(auto_increment_version "$current_version")
    fi
    
    log_info "新版本号: ${new_version}"
    
    if [ "$DRY_RUN" = true ]; then
        log_info "DRY RUN: 将执行以下步骤"
        echo ""
        echo "步骤1: 在根目录执行 make push VERSION=${new_version}"
        echo "步骤2: 更新以下文件中的版本号："
        echo "  - ${ctrl_file}"
        echo "  - ${cli_file}"
        echo "  - ${test_client_file} (仅 mesh-cli 镜像)"
        echo "  - ${test_server_file} (仅 mesh-cli 镜像)"
        echo "步骤3: 在 build 目录执行 make all"
        echo "步骤4: 执行 kubectl apply"
        echo "  - kubectl apply -f ${test_client_file}"
        echo "  - kubectl apply -f ${test_server_file}"
        echo ""
        log_info "DRY RUN 完成。使用不带 --dry-run 参数执行实际部署"
        return 0
    fi
    
    # 步骤1: 在根目录执行 make push
    log_info "步骤1: 编译并推送镜像到DockerHub (VERSION=${new_version})"
    cd "$SCRIPT_DIR"
    make push VERSION="${new_version}"
    
    # 步骤2: 更新deployment.yml文件
    log_info "步骤2: 更新deployment.yml文件中的版本号"
    
    update_deployment_file "$ctrl_file" "$new_version"
    update_deployment_file "$cli_file" "$new_version"
    update_deployment_file "$test_client_file" "$new_version"
    update_deployment_file "$test_server_file" "$new_version"
    
    # 步骤3: 部署到本地Kubernetes集群
    log_info "步骤3: 部署到本地Kubernetes集群"
    cd "$BUILD_DIR"
    make all

    # 步骤4: 更新测试环境deployment
    log_info "步骤4: 应用 tests deployment 配置"
    cd "$SCRIPT_DIR"
    kubectl apply -f "$test_client_file"
    kubectl apply -f "$test_server_file"
    
    log_info "部署完成！版本: ${new_version}"
    log_info "已更新的文件:"
    log_info "  - ${ctrl_file}"
    log_info "  - ${cli_file}"
    log_info "  - ${test_client_file}"
    log_info "  - ${test_server_file}"
}

# 显示帮助信息
show_help() {
    cat << EOF
部署自动化脚本

使用方法:
    $0 [OPTIONS] [VERSION]

选项:
    --dry-run       显示将要执行的步骤，但不实际执行
    -h, --help      显示此帮助信息

参数:
    VERSION         可选，指定版本号（例如 v0.1.28 或 0.1.28）
                    如果不指定，将自动从现有版本号递增

示例:
    $0                      # 自动递增版本号并部署
    $0 v0.1.30              # 使用指定版本号 v0.1.30 部署
    $0 0.1.31               # 使用指定版本号 v0.1.31 部署
    $0 --dry-run            # 查看将要执行的步骤（自动递增版本）
    $0 --dry-run v0.1.30    # 查看将要执行的步骤（使用 v0.1.30）

功能:
    1. 在根目录执行 make push VERSION=vX.X.XX
    2. 更新 build/ctrl/deployment.yml 和 build/cli/deployment.yml 中的镜像版本
    3. 更新 tests/client/deployment.yml 和 tests/server/deployment.yml 中 mesh-cli 镜像版本
    4. 在 build 目录执行 make all 部署到本地Kubernetes集群
    5. 自动 kubectl apply tests/client/deployment.yml 与 tests/server/deployment.yml
EOF
}

# 解析命令行参数
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_help
                exit 0
                ;;
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            *)
                # 剩余参数作为版本号
                VERSION_ARG="$1"
                shift
                ;;
        esac
    done
}

# 解析命令行参数并执行
parse_args "$@"
main "$VERSION_ARG"
