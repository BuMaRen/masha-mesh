#!/bin/bash

# 生成 Webhook 证书脚本
# 此脚本为 mesh-ctrl webhook 生成自签署证书

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 证书参数
KEY_FILE="tls.key"
CSR_FILE="tls.csr"
CRT_FILE="tls.crt"
CA_BUNDLE_FILE="ca_bundle"
CONFIG_FILE="san.conf"
DAYS=365

echo -e "${YELLOW}开始生成 Webhook 证书...${NC}"

# 检查配置文件是否存在
if [ ! -f "$CONFIG_FILE" ]; then
    echo -e "${RED}错误: san.conf 配置文件不存在${NC}"
    exit 1
fi

# 1. 生成私钥
echo -e "${GREEN}[1/4]${NC} 生成私钥 ($KEY_FILE)..."
openssl genrsa -out "$KEY_FILE" 2048 2>/dev/null

# 2. 生成证书签名请求 (CSR)
echo -e "${GREEN}[2/4]${NC} 生成证书签名请求 ($CSR_FILE)..."
openssl req -new -key "$KEY_FILE" -out "$CSR_FILE" -config "$CONFIG_FILE" 2>/dev/null

# 3. 自签署生成证书
echo -e "${GREEN}[3/4]${NC} 生成自签署证书 ($CRT_FILE)..."
openssl x509 -req -days "$DAYS" -in "$CSR_FILE" -signkey "$KEY_FILE" \
    -out "$CRT_FILE" -extensions v3_req -extfile "$CONFIG_FILE" 2>/dev/null

# 4. 生成 CA Bundle（base64 编码，直接用于 YAML）
echo -e "${GREEN}[4/4]${NC} 生成 CA Bundle (base64 编码)..."
base64 -w 0 "$CRT_FILE" > "$CA_BUNDLE_FILE"

# 清理 CSR 文件（已不再需要）
rm -f "$CSR_FILE"

# 验证证书信息
echo ""
echo -e "${YELLOW}证书信息：${NC}"
echo "---"
openssl x509 -in "$CRT_FILE" -text -noout | grep -A 1 "Subject:\|CN =\|DNS:"
echo "---"

# 显示生成的文件
echo ""
echo -e "${GREEN}✓ 证书生成成功！${NC}"
echo "生成的文件："
ls -lh "$KEY_FILE" "$CRT_FILE" "$CA_BUNDLE_FILE"

echo ""
echo -e "${YELLOW}下一步：${NC}"
echo "1. 创建 Kubernetes Secret:"
echo "   kubectl create secret tls mesh-webhook-certs \\"
echo "     --cert=$CRT_FILE \\"
echo "     --key=$KEY_FILE \\"
echo "     --namespace=default"
echo ""
echo "2. 更新 webhook.yml 中的 caBundle 字段："
echo "   caBundle: \$(cat $CA_BUNDLE_FILE)"
echo ""
echo "3. 应用 Webhook 配置："
echo "   kubectl apply -f ../../build/ctrl/webhook.yml"
