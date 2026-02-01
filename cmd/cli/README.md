# Mesh CLI Client

命令行客户端，用于与 Mesh 控制平面进行交互。

## 编译

```bash
make cli
```

编译后的二进制文件位于 `_output/mesh-cli`

## 使用示例

### 1. 订阅服务更新（流式接收）

订阅某个服务的实时更新，持续接收来自控制平面的推送：

```bash
./mesh-cli subscribe -i this-is-my-instance-id -n mesh-ctrl -s mesh-ctrl:50051
```

参数说明：
- `-i, --instance`: 实例 ID（必填）
- `-n, --service`: 服务名称（必填）
- `-s, --server`: 控制平面服务器地址（默认: localhost:50051）

### 2. 查询服务端点列表

查询某个服务当前的端点状态：

```bash
./mesh-cli list -i my-instance-1 -n my-service -s localhost:50051
```

### 3. 取消订阅

取消对某个服务的订阅：

```bash
./mesh-cli unsubscribe -i my-instance-1 -n my-service -s localhost:50051
```

## 输出格式示例

```
[2026-02-01 10:30:45] Service Update:
  Operation Type: ADD
  Revision: 12345
  Endpoints (2):
    - UID: pod-abc-123
      IP: 10.0.1.100
      Port: 8080
    - UID: pod-def-456
      IP: 10.0.1.101
      Port: 8080
---
```

## 全局选项

- `-s, --server`: 指定控制平面服务器地址
- `-h, --help`: 显示帮助信息

## 命令列表

- `subscribe`: 订阅服务更新（流式）
- `list`: 查询服务端点列表
- `unsubscribe`: 取消订阅
- `help`: 显示帮助信息
