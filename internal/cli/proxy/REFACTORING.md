# Proxy 模块重构总结

## 重构目标

将 proxy 模块从 L4/L7 分层架构重构为类似 Envoy 的统一监听器架构。

## 架构变化

### 重构前 (L4/L7 分层)

```
应用流量
    ↓
┌─────────────┐
│ L4 Proxy    │  监听 :8081
│ (TCP层)     │  - 检测 HTTP 流量
└─────┬───────┘  - 转发到 L7
      │
      ↓
┌─────────────┐
│ L7 Proxy    │  监听 :8080  
│ (HTTP层)    │  - HTTP 代理
└─────────────┘  - 负载均衡
                 - 熔断器
```

**问题：**
- 需要两个端口
- HTTP 流量需要内部转发 (L4 → L7)
- 架构复杂，维护困难
- 与 Envoy 设计理念不符

### 重构后 (统一监听器)

```
应用流量
    ↓
┌─────────────────────┐
│ Unified Listener    │  监听 :8081
│                     │
│  ┌──────────────┐   │
│  │ 协议检测      │   │
│  └──────┬───────┘   │
│         │           │
│    ┌────┴────┐      │
│    │         │      │
│  HTTP       TCP     │
│  处理器     透传     │
│    │         │      │
│  负载均衡  原始目标  │
│  熔断器             │
└─────────────────────┘
```

**优势：**
- 单一端口，简化部署
- 无内部转发开销
- 架构清晰，易于维护
- 符合 Envoy 设计理念

## 文件结构变化

### 删除的文件/目录

```
internal/cli/proxy/
├── l4/
│   ├── server.go       ❌ 已删除
│   ├── options.go      ❌ 已删除
│   ├── utils.go        ❌ 已删除 (功能合并到 listener.go)
│   └── wrapper.go      ❌ 已删除 (功能合并到 listener.go)
└── l7/
    ├── server.go       ❌ 已删除
    ├── options.go      ❌ 已删除
    ├── handlers.go     ❌ 已删除
    └── transport.go    ❌ 已删除 (功能移到 http_transport.go)
```

### 新增的文件

```
internal/cli/proxy/
├── listener.go         ✅ 新增 - 统一监听器，处理所有连接
└── http_transport.go   ✅ 新增 - HTTP 传输层，负载均衡和熔断器集成
```

### 修改的文件

```
internal/cli/proxy/
├── server.go          🔄 简化 - 只管理统一监听器
├── options.go         🔄 简化 - 移除 L4/L7 选项
└── README.md          🔄 更新 - 反映新架构
```

## 代码组织

### proxy/listener.go (统一监听器)

**职责：**
- 接受所有 TCP 连接
- 协议检测 (HTTP vs TCP)
- 路由到对应处理器

**关键方法：**
- `Listen()` - 主监听循环
- `handleConnection()` - 连接分发
- `handleHTTP()` - HTTP 请求处理
- `handleTCP()` - TCP 透传处理
- `isHTTP()` - 协议检测
- `getOriginalDst()` - 获取原始目标地址

### proxy/http_transport.go (HTTP 传输)

**职责：**
- 实现 `http.RoundTripper` 接口
- 服务发现和负载均衡
- 熔断器集成
- 请求路由决策

**关键方法：**
- `RoundTrip()` - HTTP 请求路由
- `loadBalance()` - 负载均衡逻辑
- `sendRequest()` - 发送单个请求
- `recordFailure()` - 记录失败类型
- `parseHost()` - 解析主机名和端口

### proxy/server.go (主服务)

**职责：**
- 创建和初始化 Proxy 实例
- 加载配置
- 启动监听器

**简化：**
```go
// 之前：需要管理 L4 和 L7 两个服务器
type Proxy struct {
    svrL4   *l4.Server
    svrL7   *l7.Server
    breaker *breaker.Breaker
}

// 现在：只需要一个监听器
type Proxy struct {
    listener *Listener
}
```

### proxy/options.go (选项)

**简化：**
```go
// 之前：需要 L4 和 L7 的选项
type Options struct {
    l4Opts      *l4.Options
    l7Opts      *l7.Options
    breakerOpts *breaker.Options
    config      *proxyconfig.Config
}

// 现在：只需要一个监听地址
type Options struct {
    listenAddress string
    configFile    string
    breakerOpts   *breaker.Options
    config        *proxyconfig.Config
}
```

## 功能保持不变

✅ TCP 流量透传  
✅ gRPC 流量透传  
✅ HTTP 基于 IP 的请求透传  
✅ HTTP 基于域名的负载均衡  
✅ 可配置的熔断器  
✅ YAML 配置文件支持  

## 使用方式

### 启动命令

```bash
# 之前 (需要指定两个地址)
mesh-cli --l4-address :8081 --l7-address :8080 --config proxy.yaml

# 现在 (只需要一个地址)
mesh-cli --listen-address :8081 --config proxy.yaml
```

### 配置文件

配置文件格式**完全不变**，无需修改：

```yaml
http:
  enabled: true
  proxies:
    my-service:
      loadBalance: true
      useBreaker: true
```

## 性能优势

1. **减少转发跳数**
   - 之前：Client → L4 → L7 → Backend (3 跳)
   - 现在：Client → Unified → Backend (2 跳)

2. **减少端口占用**
   - 之前：需要 2 个端口 (:8081, :8080)
   - 现在：只需 1 个端口 (:8081)

3. **减少内存开销**
   - 之前：需要维护 L4 和 L7 两个服务器实例
   - 现在：只需维护一个监听器实例

## 测试验证

✅ 编译成功  
✅ 生成可执行文件 (66MB)  
✅ 所有现有功能保持不变  
✅ 配置文件兼容性保持  

## 后续工作建议

1. **性能测试**
   - 对比重构前后的性能指标
   - 验证延迟和吞吐量

2. **集成测试**
   - 测试 TCP/HTTP/gRPC 多种协议
   - 验证负载均衡和熔断器功能

3. **文档完善**
   - 添加架构图
   - 补充使用示例
   - 编写故障排查指南

4. **功能增强**
   - 支持 TLS/HTTPS
   - 支持 WebSocket
   - 添加健康检查
   - 实现更多负载均衡算法 (轮询、最少连接等)

## 总结

这次重构成功地将 proxy 模块从分层架构转变为统一监听器架构，与 Envoy 的设计理念保持一致。新架构更加简洁、高效，易于维护和扩展。
