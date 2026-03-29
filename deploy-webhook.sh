#!/bin/bash

# webhook 自动化部署脚本
# 使用方法:
#   ./deploy-webhook.sh [VERSION]
#   ./deploy-webhook.sh --dry-run [VERSION]
#   如果不指定 VERSION，将自动从现有版本号递增

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUILD_DIR="${SCRIPT_DIR}/build"
WEBHOOK_FILE="${BUILD_DIR}/webhook/webhook.yml"
DRY_RUN=false

CERT_DIR="${BUILD_DIR}/certs"
CERT_FILE="${CERT_DIR}/tls.crt"
KEY_FILE="${CERT_DIR}/tls.key"
WEBHOOK_NAMESPACE="default"
WEBHOOK_SERVICE_NAME="webhook"
WEBHOOK_SECRET="webhook"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

get_current_version() {
    local file="$1"
    if [ ! -f "$file" ]; then
        echo ""
        return
    fi

    local version
    version=$(sed -n 's/.*image:.*hjmasha\/mesh-webhook:v\([0-9.]*\).*/\1/p' "$file" | head -1)
    echo "$version"
}

auto_increment_version() {
    local current="$1"

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

update_image_version() {
    local file="$1"
    local new_version="$2"

    log_info "更新 $file 中的镜像版本 -> $new_version"

    if [[ "$OSTYPE" == "darwin"* ]] || sed --version 2>&1 | grep -q "BSD"; then
        sed -E -i '' "s#(hjmasha/mesh-webhook):[^[:space:]]+#\\1:${new_version}#g" "$file"
    else
        sed -E -i "s#(hjmasha/mesh-webhook):[^[:space:]]+#\\1:${new_version}#g" "$file"
    fi
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

show_help() {
    cat << EOF
webhook 自动化部署脚本

使用方法:
    $0 [OPTIONS] [VERSION]

选项:
    --dry-run       显示将要执行的步骤，但不实际执行
    -h, --help      显示此帮助信息

参数:
    VERSION         可选，指定版本号（例如 v0.1.54 或 0.1.54）
                    如果不指定，将自动从现有版本号递增

示例:
    $0
    $0 v0.1.54
    $0 0.1.54
    $0 --dry-run

功能:
    1. 在根目录执行 make push-webhook VERSION=vX.X.XX
    2. 更新 build/webhook/webhook.yml 中的镜像版本
    3. 自动生成 TLS 证书并更新 Secret
    4. 自动更新 build/webhook/webhook.yml 的 caBundle
    5. 在 build 目录执行 make webhook 并部署到 Kubernetes
EOF
}

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
                VERSION_ARG="$1"
                shift
                ;;
        esac
    done
}

main() {
    if [ "$DRY_RUN" = true ]; then
        log_info "开始 webhook 部署流程（DRY RUN 模式）..."
    else
        log_info "开始 webhook 部署流程..."
    fi

    local current_version
    current_version=$(get_current_version "$WEBHOOK_FILE")

    if [ -z "$current_version" ]; then
        log_warn "无法从 $WEBHOOK_FILE 获取当前版本号，使用默认版本 0.1.0"
        current_version="0.1.0"
    fi

    log_info "当前版本号: v${current_version}"

    local new_version
    if [ -n "$1" ]; then
        new_version="$1"
        new_version="${new_version#v}"
        new_version="v${new_version}"
    else
        new_version=$(auto_increment_version "$current_version")
    fi

    log_info "新版本号: ${new_version}"

    if [ "$DRY_RUN" = true ]; then
        echo ""
        echo "步骤1: 在根目录执行 make push-webhook VERSION=${new_version}"
        echo "步骤2: 更新 ${WEBHOOK_FILE} 中的镜像版本"
        echo "步骤3: 生成 TLS 证书并创建/更新 Secret ${WEBHOOK_NAMESPACE}/${WEBHOOK_SECRET}（服务 ${WEBHOOK_SERVICE_NAME}）"
        echo "步骤4: 更新 ${WEBHOOK_FILE} 中的 caBundle"
        echo "步骤5: 在 build 目录执行 make webhook"
        echo ""
        log_info "DRY RUN 完成。使用不带 --dry-run 参数执行实际部署"
        return 0
    fi

    log_info "步骤1: 编译并推送 webhook 镜像 (VERSION=${new_version})"
    cd "$SCRIPT_DIR"
    make push-webhook VERSION="${new_version}"

    log_info "步骤2: 更新 webhook YAML 中的镜像版本"
    update_image_version "$WEBHOOK_FILE" "$new_version"

    log_info "步骤3: 准备证书并更新 Secret"
    prepare_tls_cert
    upsert_tls_secret

    log_info "步骤4: 更新 webhook caBundle"
    update_ca_bundle "$WEBHOOK_FILE" "$CERT_FILE"

    log_info "步骤5: 部署 webhook 到 Kubernetes"
    cd "$BUILD_DIR"
    make webhook

    log_info "webhook 部署完成！版本: ${new_version}"
}

parse_args "$@"
main "$VERSION_ARG"
