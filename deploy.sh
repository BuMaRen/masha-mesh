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
NO_PUSH=false

CERT_DIR="${BUILD_DIR}/certs"
CERT_FILE="${CERT_DIR}/tls.crt"
KEY_FILE="${CERT_DIR}/tls.key"
WEBHOOK_NAMESPACE="default"
WEBHOOK_SECRET="webhook"

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

prepare_tls_cert() {
    log_info "生成 webhook TLS 证书"
    mkdir -p "$CERT_DIR"

    cd "$SCRIPT_DIR"
    ./build/certs/self-signed.sh

    if [ ! -f "$CERT_FILE" ] || [ ! -f "$KEY_FILE" ]; then
        log_error "证书生成失败: 未找到 $CERT_FILE 或 $KEY_FILE"
        exit 1
    fi
}

upsert_tls_secret() {
    log_info "创建/更新 TLS Secret: ${WEBHOOK_NAMESPACE}/${WEBHOOK_SECRET}"
    if [ "$WEBHOOK_NAMESPACE" != "default" ]; then
        kubectl create namespace "$WEBHOOK_NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -
    fi
    kubectl -n "$WEBHOOK_NAMESPACE" create secret tls "$WEBHOOK_SECRET" \
        --cert="$CERT_FILE" \
        --key="$KEY_FILE" \
        --dry-run=client -o yaml | kubectl apply -f -
}

update_ca_bundle() {
    local file="$1"
    local cert_file="$2"

    local ca_bundle
    ca_bundle=$(base64 < "$cert_file" | tr -d '\n')

    log_info "更新 $file 中的 webhook caBundle"
    if [[ "$OSTYPE" == "darwin"* ]] || sed --version 2>&1 | grep -q "BSD"; then
        sed -E -i '' "s#(^[[:space:]]*caBundle:).*#\\1 ${ca_bundle}#g" "$file"
    else
        sed -E -i "s#(^[[:space:]]*caBundle:).*#\\1 ${ca_bundle}#g" "$file"
    fi
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

    # 直接按镜像名替换tag中的版本号（vX.Y.Z），避免ctrl/cli版本不一致时替换失败
    # 该规则同样会更新 --injection-image-tag 中的 hjmasha/mesh-cli:vX.Y.Z
    # 兼容macOS和Linux的sed -i语法
    if [[ "$OSTYPE" == "darwin"* ]] || sed --version 2>&1 | grep -q "BSD"; then
        # macOS/BSD sed需要提供备份扩展名参数，使用空字符串表示不备份
        sed -E -i '' "s#(hjmasha/mesh-(ctrl|cli)):v?[0-9]+(\.[0-9]+){2}#\\1:${new_version}#g" "$file"
    else
        # GNU sed
        sed -E -i "s#(hjmasha/mesh-(ctrl|cli)):v?[0-9]+(\.[0-9]+){2}#\\1:${new_version}#g" "$file"
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
    local ctrl_file="${BUILD_DIR}/ctrl/mesh-ctrl-pod.yaml"
    local cli_file="${BUILD_DIR}/cli/deployment.yml"
    
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
        log_info "DRY RUN: 将会运行以下命令（按顺序）"
        echo ""
        echo "# 步骤1: 编译镜像（默认推送，可通过 --no-push 关闭）"
        echo "cd ${SCRIPT_DIR}"
        if [ "$NO_PUSH" = true ]; then
            echo "make build VERSION=${new_version}"
        else
            echo "make push VERSION=${new_version}"
        fi
        echo ""

        echo "# 步骤2: 更新 deployment 文件中的镜像版本"
        echo "# Linux(GNU sed):"
        echo "sed -E -i \"s#(hjmasha/mesh-(ctrl|cli)):v?[0-9]+(\\.[0-9]+){2}#\\1:${new_version}#g\" ${ctrl_file}"
        echo "sed -E -i \"s#(hjmasha/mesh-(ctrl|cli)):v?[0-9]+(\\.[0-9]+){2}#\\1:${new_version}#g\" ${cli_file}"
        echo "# macOS/BSD sed 时会使用: sed -E -i '' ..."
        echo ""

        echo "# 步骤3: 生成证书并更新 Secret"
        echo "mkdir -p ${CERT_DIR}"
        echo "cd ${SCRIPT_DIR}"
        echo "./build/certs/self-signed.sh"
        if [ "$WEBHOOK_NAMESPACE" != "default" ]; then
            echo "kubectl create namespace ${WEBHOOK_NAMESPACE} --dry-run=client -o yaml | kubectl apply -f -"
        fi
        echo "kubectl -n ${WEBHOOK_NAMESPACE} create secret tls ${WEBHOOK_SECRET} --cert=${CERT_FILE} --key=${KEY_FILE} --dry-run=client -o yaml | kubectl apply -f -"
        echo ""

        echo "# 步骤4: 更新 webhook caBundle"
        echo "ca_bundle=\$(base64 < ${CERT_FILE} | tr -d '\\n')"
        echo "# Linux(GNU sed):"
        echo "sed -E -i \"s#(^[[:space:]]*caBundle:).*#\\1 \${ca_bundle}#g\" ${ctrl_file}"
        echo "# macOS/BSD sed 时会使用: sed -E -i '' ..."
        echo ""

        echo "# 步骤5: 部署到本地 Kubernetes 集群"
        echo "cd ${BUILD_DIR}"
        echo "make all"
        echo ""
        log_info "DRY RUN 完成。使用不带 --dry-run 参数执行实际部署"
        return 0
    fi
    
    # 步骤1: 在根目录编译镜像（默认推送，可通过参数关闭）
    cd "$SCRIPT_DIR"
    if [ "$NO_PUSH" = true ]; then
        log_info "步骤1: 编译镜像但不推送到远端 (VERSION=${new_version})"
        make build VERSION="${new_version}"
    else
        log_info "步骤1: 编译并推送镜像到DockerHub (VERSION=${new_version})"
        make push VERSION="${new_version}"
    fi
    
    # 步骤2: 更新deployment.yml文件
    log_info "步骤2: 更新deployment.yml文件中的版本号"
    
    update_deployment_file "$ctrl_file" "$new_version"
    update_deployment_file "$cli_file" "$new_version"
    
    # 步骤3: 准备 webhook 证书并更新 Secret
    log_info "步骤3: 准备 webhook 证书并更新 Secret"
    prepare_tls_cert
    upsert_tls_secret

    # 步骤4: 更新 webhook caBundle
    log_info "步骤4: 更新 webhook caBundle"
    update_ca_bundle "$ctrl_file" "$CERT_FILE"

    # 步骤5: 部署到本地Kubernetes集群
    log_info "步骤5: 部署到本地Kubernetes集群"
    cd "$BUILD_DIR"
    make all

    log_info "部署完成！版本: ${new_version}"
    log_info "已更新的文件:"
    log_info "  - ${ctrl_file}"
    log_info "  - ${cli_file}"
}

# 显示帮助信息
show_help() {
    cat << EOF
部署自动化脚本

使用方法:
    $0 [OPTIONS] [VERSION]

选项:
    --dry-run       显示将要执行的步骤，但不实际执行
    --no-push       仅编译镜像，不推送到远端仓库（默认会推送）
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
    $0 --no-push            # 编译镜像但不推送，随后继续部署

功能:
    1. 在根目录执行 make push VERSION=vX.X.XX（或使用 --no-push 时执行 make build）
    2. 更新 build/ctrl/mesh-ctrl-pod.yaml 和 build/cli/deployment.yml 中的镜像版本
    3. 自动生成 TLS 证书并创建/更新 default/webhook Secret
    4. 自动更新 build/ctrl/mesh-ctrl-pod.yaml 的 caBundle
    5. 在 build 目录执行 make all 部署到本地Kubernetes集群
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
            --no-push)
                NO_PUSH=true
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
