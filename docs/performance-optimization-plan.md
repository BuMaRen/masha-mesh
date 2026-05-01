# Service Mesh 性能优化实践计划

## 一、性能测试体系建设

### 1.1 基准测试（Benchmark）
```bash
# 在 pkg/ctrl 和 pkg/cli 中添加 benchmark
go test -bench=. -benchmem ./pkg/...
```

**重点测试场景：**
- EndpointSlice 合并性能（不同规模：100/1000/10000 endpoints）
- gRPC 流式推送吞吐量
- 缓存查询性能（不同 service 数量）
- protobuf 序列化/反序列化性能

**示例代码结构：**
```go
// pkg/ctrl/storage_bench_test.go
func BenchmarkCoreData_OnAdded_SmallScale(b *testing.B)  // 10 services
func BenchmarkCoreData_OnAdded_MediumScale(b *testing.B) // 100 services  
func BenchmarkCoreData_OnAdded_LargeScale(b *testing.B)  // 1000 services
```

### 1.2 压力测试（Load Testing）

**工具选择：**
- `ghz`：gRPC 压测工具
- `hey`/`wrk`：HTTP 压测工具
- `k6`：现代化压测工具

**测试场景设计：**
```bash
# 场景1：单 sidecar 订阅压测
ghz --insecure \
  --proto ./pkg/api/mesh/mesh.proto \
  --call mesh.MeshCtrl.Subscribe \
  -d '{"sidecar_id":"test-1","service_name":"demo-service"}' \
  -c 100 -n 10000 \
  localhost:50051

# 场景2：多 sidecar 并发订阅
# 测试 controller 处理 1000+ 并发订阅的能力

# 场景3：高频变更压测
# 模拟 K8s 频繁更新 EndpointSlice 场景
```

**压测报告模板：**
```
测试环境：4C8G K8s 集群
并发连接数：100/500/1000
持续时间：5 分钟
关键指标：
- QPS: 5000+
- P99 延迟: < 50ms
- 内存占用: < 500MB
- CPU 使用率: < 60%
```

### 1.3 性能分析工具实践

**Go pprof 集成：**
```go
// cmd/ctrl/app/option.go
import _ "net/http/pprof"

func (o *Options) Run() {
    // 添加 pprof 端点
    go func() {
        http.ListenAndServe("localhost:6060", nil)
    }()
}
```

**分析流程：**
```bash
# CPU 分析
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
# 内存分析
go tool pprof http://localhost:6060/debug/pprof/heap
# 协程分析
go tool pprof http://localhost:6060/debug/pprof/goroutine

# 生成火焰图
go tool pprof -http=:8080 profile.pb.gz
```

## 二、具体优化方向

### 2.1 高频优化点

**a) 减少内存分配**
```go
// 优化前：每次都创建新 slice
func (c *CoreData) List() []string {
    result := []string{}
    for svc := range c.serviceMap {
        result = append(result, svc)
    }
    return result
}

// 优化后：复用 slice
func (c *CoreData) List() []string {
    if c.cachedList != nil && !c.dirty {
        return c.cachedList
    }
    result := make([]string, 0, len(c.serviceMap))
    for svc := range c.serviceMap {
        result = append(result, svc)
    }
    c.cachedList = result
    c.dirty = false
    return result
}
```

**b) gRPC 连接池优化**
```go
// 客户端连接复用
var (
    connPool sync.Map // key: address, value: *grpc.ClientConn
)
```

**c) protobuf 对象复用**
```go
// 使用 sync.Pool 复用 protobuf 消息
var eventPool = sync.Pool{
    New: func() interface{} {
        return &mesh.ClientSubscriptionEvent{}
    },
}
```

**d) 并发控制优化**
```go
// 使用 sync.RWMutex 替代 sync.Mutex（读多写少场景）
type CoreData struct {
    mu sync.RWMutex // 原来是 sync.Mutex
    serviceMap map[string]*ServiceCache
}

func (c *CoreData) List() []string {
    c.mu.RLock() // 读锁
    defer c.mu.RUnlock()
    // ...
}
```

### 2.2 架构层面优化

**a) 批量推送机制**
```go
// 当前：每次变更立即推送
// 优化：合并 100ms 内的变更，批量推送
type Distributer struct {
    batchWindow time.Duration
    pendingEvents chan *Event
}
```

**b) 增量更新**
```go
// 当前：全量推送 endpoints
// 优化：只推送变更的 endpoints（delta）
message DeltaUpdate {
    repeated string added_ips = 1;
    repeated string removed_ips = 2;
}
```

**c) 多级缓存**
```go
// L1: 本地内存缓存（当前已有）
// L2: 添加 LRU 缓存淘汰策略
import "github.com/hashicorp/golang-lru"

cache, _ := lru.New(1000) // 最多缓存 1000 个 service
```

## 三、可观测性（生产必备）

### 3.1 集成 Prometheus metrics

```go
import "github.com/prometheus/client_golang/prometheus"

var (
    subscriptionCount = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "mesh_active_subscriptions",
        Help: "Number of active gRPC subscriptions",
    })
    
    endpointUpdateDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
        Name: "mesh_endpoint_update_duration_seconds",
        Help: "Endpoint update processing duration",
    })
)
```

### 3.2 分布式追踪（OpenTelemetry）

```go
import "go.opentelemetry.io/otel"

// 在关键路径添加 trace
ctx, span := tracer.Start(ctx, "ProcessEndpointUpdate")
defer span.End()
```

### 3.3 结构化日志

```go
// 替换 fmt.Println 为结构化日志
import "go.uber.org/zap"

logger.Info("endpoint updated",
    zap.String("service", svcName),
    zap.Int64("revision", revision),
    zap.Duration("latency", time.Since(start)),
)
```

## 四、成果展示（简历/面试）

### 简历中可以这样写：

**项目：Service Mesh 性能优化实践**
- 通过 pprof 分析发现热点函数，优化内存分配减少 40% GC 压力
- 引入 sync.Pool 复用 protobuf 对象，单机 QPS 从 3000 提升至 8000+
- 实现批量推送机制，P99 延迟从 200ms 降低至 50ms 以内
- 集成 Prometheus metrics，建立性能监控体系（QPS/延迟/内存）
- 压测验证：单 Controller 支持 1000+ 并发 gRPC 长连接

### 面试常见问题准备：

Q: 你做过哪些性能优化？
A: 举例说明上述 3-4 个具体优化点，用数据说话

Q: 如何定位性能瓶颈？
A: pprof + 压测 + 监控 + 火焰图分析

Q: Go 程序常见的性能问题？
A: 过度内存分配、goroutine 泄漏、锁竞争、GC 压力

## 五、实施计划（2-3 周完成）

**Week 1：**
- [ ] 添加 benchmark 测试（5 个核心函数）
- [ ] 集成 pprof，分析热点代码
- [ ] 实现 1-2 个优化点（内存复用、读写锁）

**Week 2：**
- [ ] 搭建压测环境（ghz + k6）
- [ ] 完成 3 种压测场景，记录基线数据
- [ ] 优化后对比测试，产出性能报告

**Week 3：**
- [ ] 集成 Prometheus metrics（5-10 个关键指标）
- [ ] 添加 Grafana 监控面板
- [ ] 撰写优化总结文档

## 六、参考资料

- 《Go 语言高性能编程》
- Uber Go 性能优化实践：https://github.com/uber-go/guide
- Istio 性能基准测试：https://istio.io/latest/docs/ops/deployment/performance-and-scalability/
- gRPC 性能最佳实践：https://grpc.io/docs/guides/performance/
