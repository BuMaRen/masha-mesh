# Webhook 证书生成指南

此目录包含用于生成 mesh-ctrl webhook 所需的 TLS 证书的脚本和配置。

## 文件说明

- **san.conf** - OpenSSL 配置文件，定义了证书的主体备用名称 (SAN)
- **generate-certs.sh** - 自动生成证书的脚本
- **tls.key** - 生成的私钥（首次运行脚本后产生）
- **tls.crt** - 生成的证书（首次运行脚本后产生）
- **ca_bundle** - Base64 编码的 CA 证书，可直接复制到 YAML 文件中（首次运行脚本后产生）

## 快速开始

### 1. 生成证书

```bash
cd build/certs
chmod +x generate-certs.sh
./generate-certs.sh
```

脚本会自动生成以下文件：
- `tls.key` - TLS 私钥
- `tls.crt` - TLS 证书
- `ca_bundle` - Base64 编码的 CA 证书，可直接用于 YAML

### 2. 手动更新 Webhook 配置

### 2. 手动更新 Webhook 配置

编辑 `build/ctrl/webhook.yml`，将 `ca_bundle` 文件的内容复制到 `caBundle` 字段：

```bash
# 查看 ca_bundle 内容
cat ca_bundle
```

然后在 `webhook.yml` 中更新：

```yaml
webhooks:
  - name: webhook.example.com
    clientConfig:
      service:
        name: webhook-service
        namespace: default
        path: "/mutate"
      caBundle: LS0tLS1CRUdJTi... # 粘贴 ca_bundle 文件内容
```

或者使用 sed 命令自动更新：

```bash
CA_BUNDLE=$(cat ca_bundle)
sed -i "s@caBundle: .*@caBundle: $CA_BUNDLE@" ../../build/ctrl/webhook.yml
```

### 3. 创建 Kubernetes Secret

#### 方式一：直接创建

```bash
kubectl create secret tls mesh-webhook-certs \
  --cert=tls.crt \
  --key=tls.key \
  --namespace=default
```

#### 方式二：更新现有 Secret

```bash
kubectl create secret tls mesh-webhook-certs \
  --cert=tls.crt \
  --key=tls.key \
  --namespace=default \
  --dry-run=client -o yaml | kubectl apply -f -
```

#### 方式三：在部署配置中使用

在 `ctrl/deployment.yml` 中添加 volume 和 volume mount：

```yaml
spec:
  volumes:
  - name: webhook-certs
    secret:
      secretName: mesh-webhook-certs
      defaultMode: 0400
  containers:
  - name: mesh-ctrl
    volumeMounts:
    - name: webhook-certs
      mountPath: /etc/webhook/certs
      readOnly: true
```

## Webhook 配置详解

### webhook.yml 中的 caBundle 字段

`caBundle` 字段必须包含 **base64 编码**的 CA 证书，用于 Kubernetes 验证 webhook 服务的 TLS 证书。脚本生成的 `ca_bundle` 文件已经是 base64 编码格式，可以直接复制使用：

```yaml
webhooks:
  - name: webhook.example.com
    clientConfig:
      service:
        name: webhook-service
        namespace: default
        path: "/mutate"
      caBundle: LS0tLS1CRUdJTi... # 使用 ca_bundle 文件内容
```

**关键点：**
- 生成的 `ca_bundle` 文件已经是 base64 编码格式
- 直接复制 `ca_bundle` 文件内容到 `caBundle` 字段即可
- 无需额外的编码转换

## 证书配置详解

### san.conf 文件

```
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
CN = mesh-ctrl.default.svc

[v3_req]
subjectAltName = DNS:mesh-ctrl.default.svc,DNS:mesh-ctrl.default,DNS:localhost,IP:127.0.0.1
```

配置说明：
- **CN** - 证书的通用名称（Common Name）
- **subjectAltName** - 证书的主体备用名称，包含：
  - `mesh-ctrl.default.svc` - Kubernetes 服务的完全域名（FQDN）
  - `mesh-ctrl.default` - 服务的短域名
  - `localhost` - 本地环回域名
  - `127.0.0.1` - 本地环回 IP 地址

## 验证证书

### 查看证书信息

```bash
openssl x509 -in tls.crt -text -noout
```

### 验证证书有效期

```bash
openssl x509 -in tls.crt -noout -dates
```

### 验证证书和私钥匹配

```bash
# 证书的 modulus
openssl x509 -noout -modulus -in tls.crt | openssl md5

# 私钥的 modulus
openssl rsa -noout -modulus -in tls.key | openssl md5

# 两个 md5 值应该相同
```

## 手动生成证书（不使用脚本）

如果需要手动生成，可以执行以下命令：

```bash
# 1. 生成私钥
openssl genrsa -out tls.key 2048

# 2. 生成证书签名请求
openssl req -new -key tls.key -out tls.csr -config san.conf

# 3. 自签署证书
openssl x509 -req -days 365 -in tls.csr -signkey tls.key \
  -out tls.crt -extensions v3_req -extfile san.conf

# 4. 生成 CA Bundle（base64 编码）
base64 -w 0 tls.crt > ca_bundle

# 5. 清理临时文件
rm tls.csr
```

## 常见问题

### Q: 证书过期了怎么办？

A: 重新运行 `generate-certs.sh` 脚本，然后更新 Kubernetes Secret 和 webhook 配置：

```bash
./generate-certs.sh
kubectl create secret tls mesh-webhook-certs \
  --cert=tls.crt \
  --key=tls.key \
  --namespace=default \
  --dry-run=client -o yaml | kubectl apply -f -
# 然后使用新的 ca_bundle 内容更新 webhook.yml
```

### Q: 如何更新 webhook.yml 中的 caBundle？

A: 使用以下命令将 `ca_bundle` 内容自动复制到 webhook 配置：

```bash
CA_BUNDLE=$(cat ca_bundle)
sed -i "s@caBundle: .*@caBundle: $CA_BUNDLE@" ../../build/ctrl/webhook.yml
```

或者手动复制 `ca_bundle` 文件的内容到 webhook.yml 的 `caBundle` 字段。

### Q: 需要添加更多的 DNS 域名怎么办？

A: 编辑 `san.conf` 文件，在 `[v3_req]` 段的 `subjectAltName` 中添加更多的 DNS 条目，然后重新运行脚本。

示例：添加 `mesh-ctrl.example.com`

```
subjectAltName = DNS:mesh-ctrl.default.svc,DNS:mesh-ctrl.default,DNS:localhost,DNS:mesh-ctrl.example.com,IP:127.0.0.1
```

### Q: 如何验证 webhook 是否成功使用了证书？

A: 检查 webhook pod 的日志或验证 webhook 服务是否正确返回了证书：

```bash
kubectl logs -n default deployment/mesh-ctrl
```

## 参考资源

- [OpenSSL 文档](https://www.openssl.org/docs/)
- [Kubernetes Secret 文档](https://kubernetes.io/docs/concepts/configuration/secret/)
- [Kubernetes Webhook 文档](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/)
