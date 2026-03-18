#!/bin/bash

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}=== Kubernetes Sidecar Injector Webhook 部署脚本 ===${NC}\n"

# 检查必要工具
check_tools() {
    echo -e "${YELLOW}检查必要工具...${NC}"
    
    for tool in kubectl openssl docker; do
        if ! command -v $tool &> /dev/null; then
            echo -e "${RED}❌ 缺少 $tool 工具${NC}"
            exit 1
        fi
    done
    
    echo -e "${GREEN}✓ 所有工具都已安装${NC}\n"
}

# 生成自签名证书
generate_certs() {
    echo -e "${YELLOW}生成自签名证书...${NC}"
    
    CERT_DIR="./certs"
    mkdir -p "$CERT_DIR"
    
    # 生成私钥
    openssl genrsa -out "$CERT_DIR/tls.key" 2048
    
    # 创建一个 SAN 配置文件，用于生成包含 SAN 的证书
    SAN_CONFIG="$CERT_DIR/san.conf"
    cat > "$SAN_CONFIG" << EOF
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
CN = sidecar-injector.webhook-system.svc

[v3_req]
subjectAltName = DNS:sidecar-injector.webhook-system.svc,DNS:sidecar-injector.webhook-system,DNS:localhost,IP:127.0.0.1
EOF
    
    # 生成证书签名请求（带 SAN）
    openssl req -new \
        -key "$CERT_DIR/tls.key" \
        -out "$CERT_DIR/tls.csr" \
        -config "$SAN_CONFIG"
    
    # 生成自签名证书（带 SAN）
    openssl x509 -req \
        -days 365 \
        -in "$CERT_DIR/tls.csr" \
        -signkey "$CERT_DIR/tls.key" \
        -out "$CERT_DIR/tls.crt" \
        -extensions v3_req \
        -extfile "$SAN_CONFIG"
    
    echo -e "${GREEN}✓ 证书生成完成${NC}\n"
}

# 构建 Docker 镜像
build_image() {
    echo -e "${YELLOW}构建 Docker 镜像...${NC}"
    
    # 检查是否在 minikube 环境中
    if kubectl config current-context | grep -q minikube; then
        echo "检测到 minikube 环境，将镜像加载到 minikube..."
        eval $(minikube docker-env)
    else
        echo -e "${RED}❌ 当前 kubectl context 不是 minikube，请先执行: minikube start${NC}"
        exit 1
    fi
    
    docker build -t sidecar-injector:latest .
    
    echo -e "${GREEN}✓ Docker 镜像构建完成${NC}\n"
}

# 创建命名空间
create_namespace() {
    echo -e "${YELLOW}创建命名空间...${NC}"
    
    kubectl create namespace webhook-system || true
    
    echo -e "${GREEN}✓ 命名空间创建完成${NC}\n"
}

# 创建 ConfigMap 包含证书
create_configmap() {
    echo -e "${YELLOW}创建 ConfigMap 包含证书...${NC}"
    
    CERT_DIR="./certs"
    
    # 删除旧的 ConfigMap（如果存在）
    kubectl delete configmap webhook-certs -n webhook-system --ignore-not-found=true
    
    # 直接创建新的 ConfigMap，避免使用 --dry-run 导致数据丢失
    kubectl create configmap webhook-certs \
        --from-file=tls.crt="$CERT_DIR/tls.crt" \
        --from-file=tls.key="$CERT_DIR/tls.key" \
        -n webhook-system
    
    echo -e "${GREEN}✓ ConfigMap 创建完成${NC}\n"
}

# 部署 Webhook
deploy_webhook() {
    echo -e "${YELLOW}部署 Webhook...${NC}"
    
    CERT_DIR="./certs"
    
    # 兼容 macOS 和 Linux 的 base64 编码
    if [[ "$OSTYPE" == "darwin"* ]]; then
        CA_BUNDLE=$(cat "$CERT_DIR/tls.crt" | base64 | tr -d '\n')
    else
        CA_BUNDLE=$(cat "$CERT_DIR/tls.crt" | base64 -w 0)
    fi
    
    # 创建临时文件用于 sed 替换，排除 ConfigMap 部分
    TEMP_YAML=$(mktemp)
    trap "rm -f $TEMP_YAML" EXIT
    
    # 使用 awk 排除 ConfigMap 部分（从 kind: ConfigMap 到下一个 --- 之前）
    awk '
        /^kind: ConfigMap/ { skip=1; next }
        /^---$/ && skip { skip=0; next }
        !skip { print }
    ' k8s-webhook.yaml | \
    sed "s|caBundle: \"\"|caBundle: $CA_BUNDLE|g" > "$TEMP_YAML"
    
    kubectl apply -f "$TEMP_YAML"
    
    echo -e "${GREEN}✓ Webhook 部署完成${NC}\n"
}

# 等待 Deployment 就绪
wait_for_deployment() {
    echo -e "${YELLOW}等待 Deployment 就绪...${NC}"
    
    kubectl rollout status deployment/sidecar-injector -n webhook-system --timeout=60s
    
    echo -e "${GREEN}✓ Deployment 已就绪${NC}\n"
}

# 创建测试命名空间
create_test_namespace() {
    echo -e "${YELLOW}创建测试命名空间...${NC}"
    
    kubectl create namespace test-sidecar || true
    kubectl label namespace test-sidecar sidecar-injection=enabled --overwrite
    
    echo -e "${GREEN}✓ 测试命名空间创建完成${NC}\n"
}

# 测试 Webhook
test_webhook() {
    echo -e "${YELLOW}测试 Webhook...${NC}"
    
    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
  namespace: test-sidecar
spec:
  containers:
  - name: app
    image: nginx:latest
    ports:
    - containerPort: 80
EOF

    echo -e "${GREEN}✓ 测试 Pod 已创建${NC}\n"
    
    echo -e "${YELLOW}检查 Pod 中是否已注入 sidecar...${NC}"
    sleep 2
    
    kubectl get pod test-pod -n test-sidecar -o jsonpath='{.spec.containers[*].name}' | grep -q sidecar-injected
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ Sidecar 注入成功！${NC}\n"
    else
        echo -e "${RED}❌ Sidecar 注入失败${NC}\n"
    fi
}

# 显示日志
show_logs() {
    echo -e "${YELLOW}Webhook 服务器日志:${NC}"
    kubectl logs -n webhook-system deployment/sidecar-injector -f --tail=50
}

# 清理函数
cleanup() {
    echo -e "${YELLOW}清理资源...${NC}"
    
    read -p "是否删除所有部署的资源? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        kubectl delete namespace webhook-system test-sidecar || true
        rm -rf ./certs
        echo -e "${GREEN}✓ 清理完成${NC}\n"
    fi
}

# 显示使用帮助
show_help() {
    cat <<EOF
${GREEN}Kubernetes Sidecar Injector Webhook 部署脚本${NC}

前置要求:
    • kubectl 已安装
    • minikube 已安装并运行 (minikube start)
    • Docker 已安装
    • openssl 已安装（通常包含在 macOS 中）

使用方法:
    $0 [命令]

命令:
    deploy      - 完整部署（生成证书、构建镜像、部署到 K8s）
    certs       - 仅生成证书
    build       - 仅构建 Docker 镜像
    test        - 测试 Webhook 功能
    logs        - 显示 Webhook 日志
    cleanup     - 清理所有资源（仅删除 K8s 中的资源和本地 ./certs）
    help        - 显示帮助信息

注意:
    • 所有文件都保存在当前目录或其子目录中
    • 不会修改系统文件或安装全局软件
    • 生成的证书保存在 ./certs 目录

示例:
    $0 deploy   # 完整部署
    $0 logs     # 查看日志
    $0 cleanup  # 清理资源
EOF
}

# 主函数
main() {
    check_tools
    
    case "${1:-deploy}" in
        deploy)
            generate_certs
            build_image
            create_namespace
            create_configmap
            deploy_webhook
            wait_for_deployment
            create_test_namespace
            test_webhook
            echo -e "${GREEN}=== 部署完成！ ===${NC}\n"
            show_logs
            ;;
        certs)
            generate_certs
            ;;
        build)
            build_image
            ;;
        test)
            test_webhook
            ;;
        logs)
            show_logs
            ;;
        cleanup)
            cleanup
            ;;
        help)
            show_help
            ;;
        *)
            echo -e "${RED}未知命令: $1${NC}\n"
            show_help
            exit 1
            ;;
    esac
}

main "$@"