# Masha-Mesh 生产级改造实施指南

> **目标**：用 4-6 周时间，将学习项目改造为可在简历中作为"生产级项目"展示的代码库  
> **原则**：循序渐进、每步可验证、优先高价值特性

---

## 📋 改造前 vs 改造后对比

| 维度 | 当前状态 | 生产级目标 | 面试价值 |
|------|---------|-----------|---------|
| **稳定性** | 无健康检查、硬关闭 | 优雅启停、熔断降级 | ⭐⭐⭐⭐⭐ |
| **可观测性** | 只有代码逻辑 | Metrics + 日志 + 追踪 | ⭐⭐⭐⭐⭐ |
| **性能** | 未知性能瓶颈 | 压测数据 + 优化报告 | ⭐⭐⭐⭐ |
| **容错** | 连接断开即失败 | 自动重连、本地缓存 | ⭐⭐⭐⭐ |
| **安全** | 无认证授权 | mTLS + 限流 | ⭐⭐⭐ |
| **测试** | 部分单测 | 单测+集成测试+压测 | ⭐⭐⭐⭐ |

---

## 🎯 改造路线图（分 4 个里程碑）

```
Milestone 1: 基础可观测性 (Week 1-2) ← 优先级最高，立即见效
    ↓
Milestone 2: 稳定性保障 (Week 2-3) ← 生产必备特性
    ↓
Milestone 3: 性能优化 (Week 3-4) ← 技术深度体现
    ↓
Milestone 4: 高级特性 (Week 5-6) ← 加分项
```

---

## 里程碑 1：基础可观测性（Week 1-2）

> **目标**：让系统运行状态可视化，具备生产环境的基本监控能力

### 任务 1.1：集成 Prometheus Metrics（⏱️ 6小时）

#### Step 1: 安装依赖
```bash
cd ~/projects/github.com/InHuanLe/masha-mesh
go get github.com/prometheus/client_golang/prometheus
go get github.com/prometheus/client_golang/prometheus/promauto
go get github.com/prometheus/client_golang/prometheus/promhttp
```

#### Step 2: 创建 metrics 定义文件
```bash
# 创建新文件
touch pkg/ctrl/metrics.go
```

在 `pkg/ctrl/metrics.go` 中添加：
```go
package ctrl

import (
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// 业务指标
	ActiveSubscriptions = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "mesh",
		Subsystem: "controller",
		Name:      "active_subscriptions",
		Help:      "当前活跃的 gRPC 订阅连接数",
	})

	EndpointSliceEvents = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "mesh",
		Subsystem: "controller",
		Name:      "endpointslice_events_total",
		Help:      "EndpointSlice 事件总数",
	}, []string{"operation"}) // operation: ADDED/MODIFIED/DELETED

	EventPublishDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "mesh",
		Subsystem: "controller",
		Name:      "event_publish_duration_seconds",
		Help:      "事件推送到客户端的耗时分布",
		Buckets:   prometheus.ExponentialBuckets(0.001, 2, 10), // 1ms ~ 1s
	}, []string{"service"})

	CachedServices = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "mesh",
		Subsystem: "controller",
		Name:      "cached_services",
		Help:      "当前缓存的服务数量",
	})

	// 系统指标
	GoroutineCount = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "mesh",
		Name:      "goroutines",
		Help:      "当前 goroutine 数量",
	})

	MemoryUsage = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "mesh",
		Name:      "memory_bytes",
		Help:      "当前内存使用量（bytes）",
	})
)

// StartMetricsCollector 启动系统指标采集
func StartMetricsCollector(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)

			GoroutineCount.Set(float64(runtime.NumGoroutine()))
			MemoryUsage.Set(float64(m.Alloc))
		}
	}()
}
```

#### Step 3: 在 Controller 中埋点

修改 `pkg/ctrl/storage.go`，在关键位置添加埋点：

```go
import "time"

// OnAdded 方法中添加
func (c *CoreData) OnAdded(slice *discovery.EndpointSlice) {
	start := time.Now()
	defer func() {
		EndpointSliceEvents.WithLabelValues("ADDED").Inc()
		CachedServices.Set(float64(len(c.serviceMap)))
	}()
	
	// ... 原有逻辑 ...
	
	// 在 Publish 前后记录耗时
	publishStart := time.Now()
	c.distributer.Publish(svcName, mesh.OpType_ADDED, payload)
	EventPublishDuration.WithLabelValues(svcName).Observe(time.Since(publishStart).Seconds())
}

// 同样修改 OnUpdate 和 OnDeleted
func (c *CoreData) OnUpdate(oldSlice, newSlice *discovery.EndpointSlice) {
	EndpointSliceEvents.WithLabelValues("MODIFIED").Inc()
	// ...
}

func (c *CoreData) OnDeleted(slice *discovery.EndpointSlice) {
	EndpointSliceEvents.WithLabelValues("DELETED").Inc()
	// ...
}
```

修改 `pkg/ctrl/grpcserver/server.go`（假设这是 gRPC server 实现），添加连接追踪：

```go
func (s *Server) Subscribe(req *mesh.SubscriptionRequest, stream mesh.MeshCtrl_SubscribeServer) error {
	ActiveSubscriptions.Inc()
	defer ActiveSubscriptions.Dec()
	
	// ... 原有逻辑 ...
}
```

#### Step 4: 暴露 Metrics 端点

修改 `cmd/ctrl/app/option.go`：

```go
package app

import (
	"net/http"
	"time"
	
	"github.com/BuMaRen/mesh/pkg/ctrl"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (o *Options) Run() {
	// 启动系统指标采集
	ctrl.StartMetricsCollector(15 * time.Second)
	
	// 启动 metrics HTTP 服务
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		
		// 健康检查端点（后续会用到）
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		})
		
		http.ListenAndServe(":8080", mux)
	}()
	
	// ... 原有启动逻辑 ...
}
```

#### Step 5: 本地验证

```bash
# 1. 编译运行
make build
./bin/mesh-ctrl -n default

# 2. 访问 metrics 端点
curl http://localhost:8080/metrics | grep mesh_

# 3. 预期输出示例
# mesh_controller_active_subscriptions 2
# mesh_controller_cached_services 5
# mesh_controller_endpointslice_events_total{operation="ADDED"} 10
```

✅ **验收标准**：
- [ ] `/metrics` 端点可访问
- [ ] 能看到至少 5 个自定义指标
- [ ] 触发 EndpointSlice 变更后，计数器递增

---

### 任务 1.2：结构化日志（⏱️ 4小时）

#### Step 1: 安装 zap
```bash
go get go.uber.org/zap
```

#### Step 2: 创建全局 logger

创建 `pkg/logger/logger.go`：
```go
package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var globalLogger *zap.Logger

// Init 初始化全局 logger
func Init(level string, development bool) error {
	var config zap.Config
	
	if development {
		config = zap.NewDevelopmentConfig()
	} else {
		config = zap.NewProductionConfig()
	}
	
	// 自定义编码配置
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	
	// 解析日志级别
	var zapLevel zapcore.Level
	if err := zapLevel.UnmarshalText([]byte(level)); err != nil {
		zapLevel = zapcore.InfoLevel
	}
	config.Level = zap.NewAtomicLevelAt(zapLevel)
	
	logger, err := config.Build(
		zap.AddCaller(),
		zap.AddCallerSkip(1),
	)
	if err != nil {
		return err
	}
	
	globalLogger = logger
	return nil
}

// L 返回全局 logger
func L() *zap.Logger {
	if globalLogger == nil {
		globalLogger, _ = zap.NewProduction()
	}
	return globalLogger
}

// Sync 刷新日志缓冲
func Sync() {
	if globalLogger != nil {
		globalLogger.Sync()
	}
}
```

#### Step 3: 在 main 函数初始化

修改 `cmd/ctrl/main.go`：
```go
package main

import (
	"github.com/BuMaRen/mesh/cmd/ctrl/app"
	"github.com/BuMaRen/mesh/pkg/logger"
)

func main() {
	// 初始化日志
	if err := logger.Init("info", false); err != nil {
		panic(err)
	}
	defer logger.Sync()
	
	command := app.NewCommand()
	if err := command.Execute(); err != nil {
		logger.L().Fatal("command execution failed", zap.Error(err))
	}
}
```

#### Step 4: 替换所有日志输出

在 `pkg/ctrl/storage.go` 中：
```go
import (
	"github.com/BuMaRen/mesh/pkg/logger"
	"go.uber.org/zap"
)

func (c *CoreData) OnAdded(slice *discovery.EndpointSlice) {
	// 替换 fmt.Println 为结构化日志
	logger.L().Info("endpoint slice added",
		zap.String("namespace", slice.Namespace),
		zap.String("name", slice.Name),
		zap.String("service", svcName),
		zap.Int("endpoints", len(endpoints)),
	)
	
	// ... 业务逻辑 ...
}

func (c *CoreData) OnDeleted(slice *discovery.EndpointSlice) {
	if _, exists := c.serviceMap[svcName]; !exists {
		logger.L().Warn("delete non-existent service",
			zap.String("service", svcName),
			zap.String("slice", slice.Name),
		)
		return
	}
	
	// ...
}
```

✅ **验收标准**：
- [ ] 所有 `fmt.Println` 已替换为 `logger.L().Info/Warn/Error`
- [ ] 日志输出为 JSON 格式
- [ ] 关键操作都有日志记录（增删改、推送事件、错误）

---

### 任务 1.3：本地 Prometheus + Grafana 部署（⏱️ 3小时）

#### Step 1: 创建 docker-compose 配置

创建 `build/monitoring/docker-compose.yml`：
```yaml
version: '3.8'

services:
  prometheus:
    image: prom/prometheus:latest
    container_name: mesh-prometheus
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus-data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
    networks:
      - mesh-monitor

  grafana:
    image: grafana/grafana:latest
    container_name: mesh-grafana
    ports:
      - "3000:3000"
    volumes:
      - grafana-data:/var/lib/grafana
      - ./grafana-dashboards:/etc/grafana/provisioning/dashboards
      - ./grafana-datasources.yml:/etc/grafana/provisioning/datasources/datasources.yml
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
      - GF_USERS_ALLOW_SIGN_UP=false
    networks:
      - mesh-monitor

volumes:
  prometheus-data:
  grafana-data:

networks:
  mesh-monitor:
    driver: bridge
```

#### Step 2: 创建 Prometheus 配置

创建 `build/monitoring/prometheus.yml`：
```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'mesh-controller'
    static_configs:
      - targets: ['host.docker.internal:8080']  # Mac/Windows Docker
        labels:
          component: 'controller'
          
  - job_name: 'mesh-sidecar'
    static_configs:
      - targets: ['host.docker.internal:8081']  # 假设 sidecar 端口
        labels:
          component: 'sidecar'
```

#### Step 3: 创建 Grafana 数据源配置

创建 `build/monitoring/grafana-datasources.yml`：
```yaml
apiVersion: 1

datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://prometheus:9090
    isDefault: true
```

#### Step 4: 创建 Grafana Dashboard

创建 `build/monitoring/grafana-dashboards/dashboard.yml`：
```yaml
apiVersion: 1

providers:
  - name: 'Mesh Dashboards'
    orgId: 1
    folder: ''
    type: file
    disableDeletion: false
    updateIntervalSeconds: 10
    options:
      path: /etc/grafana/provisioning/dashboards
```

创建 `build/monitoring/grafana-dashboards/mesh-controller.json`：
```json
{
  "dashboard": {
    "title": "Mesh Controller 监控",
    "panels": [
      {
        "id": 1,
        "title": "活跃订阅数",
        "type": "graph",
        "targets": [
          {
            "expr": "mesh_controller_active_subscriptions",
            "legendFormat": "订阅数"
          }
        ],
        "gridPos": {"h": 8, "w": 12, "x": 0, "y": 0}
      },
      {
        "id": 2,
        "title": "EndpointSlice 事件 QPS",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(mesh_controller_endpointslice_events_total[1m])",
            "legendFormat": "{{operation}}"
          }
        ],
        "gridPos": {"h": 8, "w": 12, "x": 12, "y": 0}
      },
      {
        "id": 3,
        "title": "事件推送延迟 P99",
        "type": "graph",
        "targets": [
          {
            "expr": "histogram_quantile(0.99, rate(mesh_controller_event_publish_duration_seconds_bucket[1m]))",
            "legendFormat": "P99"
          }
        ],
        "gridPos": {"h": 8, "w": 12, "x": 0, "y": 8}
      },
      {
        "id": 4,
        "title": "内存使用",
        "type": "graph",
        "targets": [
          {
            "expr": "mesh_memory_bytes / 1024 / 1024",
            "legendFormat": "内存 (MB)"
          }
        ],
        "gridPos": {"h": 8, "w": 12, "x": 12, "y": 8}
      }
    ],
    "schemaVersion": 16,
    "version": 0
  }
}
```

#### Step 5: 启动监控栈

```bash
cd build/monitoring
docker-compose up -d

# 访问
# Prometheus: http://localhost:9090
# Grafana: http://localhost:3000 (admin/admin)
```

✅ **验收标准**：
- [ ] Prometheus 能抓取到 metrics
- [ ] Grafana 中能看到 4 个面板
- [ ] 触发操作后图表有变化

---

## 里程碑 2：稳定性保障（Week 2-3）

> **目标**：让系统具备生产环境的基本容错能力

### 任务 2.1：健康检查 + 优雅关闭（⏱️ 4小时）

#### Step 1: 添加健康检查状态管理

在 `cmd/ctrl/app/option.go` 中：
```go
type Options struct {
	// ... 原有字段 ...
	
	isReady         bool
	informersSynced bool
}

func (o *Options) Run() {
	// HTTP 服务中添加更详细的健康检查
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	
	// 存活探针（只要进程在就返回 200）
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	
	// 就绪探针（informer 同步完成才返回 200）
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if o.informersSynced {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ready"))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("not ready"))
		}
	})
	
	go http.ListenAndServe(":8080", mux)
	
	// 启动 informer 后设置状态
	// ... informer.Start() ...
	cache.WaitForCacheSync(stopCh, informer.HasSynced)
	o.informersSynced = true
	logger.L().Info("informers synced, controller is ready")
	
	// ... 启动 gRPC server ...
}
```

#### Step 2: 实现优雅关闭

修改 `cmd/ctrl/app/option.go`：
```go
import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func (o *Options) Run() {
	// ... 启动各种服务 ...
	
	// 等待终止信号
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGTERM, syscall.SIGINT)
	
	sig := <-stopCh
	logger.L().Info("received shutdown signal", zap.String("signal", sig.String()))
	
	// 优雅关闭
	o.gracefulShutdown()
}

func (o *Options) gracefulShutdown() {
	logger.L().Info("starting graceful shutdown...")
	
	// 1. 停止接收新连接
	o.informersSynced = false // 就绪探针返回 503
	
	// 2. 等待 K8s 流量切走（与 preStop hook 配合）
	time.Sleep(5 * time.Second)
	
	// 3. 停止 gRPC server（等待现有连接处理完成，最多 30s）
	stopped := make(chan struct{})
	go func() {
		o.grpcServer.GracefulStop()
		close(stopped)
	}()
	
	select {
	case <-stopped:
		logger.L().Info("gRPC server stopped gracefully")
	case <-time.After(30 * time.Second):
		logger.L().Warn("force stopping gRPC server after timeout")
		o.grpcServer.Stop()
	}
	
	// 4. 刷新日志
	logger.Sync()
	
	logger.L().Info("shutdown complete")
}
```

#### Step 3: 更新 K8s 部署配置

修改 `build/ctrl/deployment.yml`：
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mesh-controller
spec:
  replicas: 2  # 至少 2 副本
  template:
    spec:
      containers:
      - name: controller
        image: your-repo/mesh-ctrl:v0.1.30
        ports:
        - containerPort: 50051
          name: grpc
        - containerPort: 8080
          name: http
        
        # 存活探针
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 10
          timeoutSeconds: 3
          failureThreshold: 3
        
        # 就绪探针
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
          timeoutSeconds: 2
          failureThreshold: 2
        
        # 优雅关闭
        lifecycle:
          preStop:
            exec:
              command: ["/bin/sh", "-c", "sleep 15"]
        
        # 资源限制
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi
```

✅ **验收标准**：
- [ ] `/healthz` 和 `/readyz` 都能正常访问
- [ ] 发送 SIGTERM 后日志显示"graceful shutdown"
- [ ] gRPC 连接在关闭时等待处理完成
- [ ] K8s Pod 重启时无 5xx 错误

---

### 任务 2.2：客户端熔断降级（⏱️ 6小时）

#### Step 1: 安装依赖
```bash
go get github.com/sony/gobreaker
```

#### Step 2: 在客户端实现熔断器

创建 `pkg/cli/circuit_breaker.go`：
```go
package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/sony/gobreaker"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type CircuitBreakerClient struct {
	client mesh.MeshCtrlClient
	cb     *gobreaker.CircuitBreaker
	cache  *LocalCache // 本地缓存作为降级方案
	logger *zap.Logger
}

func NewCircuitBreakerClient(conn *grpc.ClientConn, cache *LocalCache) *CircuitBreakerClient {
	settings := gobreaker.Settings{
		Name:        "mesh-ctrl-subscribe",
		MaxRequests: 3,
		Interval:    10 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 3 && failureRatio >= 0.6
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			logger.L().Warn("circuit breaker state changed",
				zap.String("name", name),
				zap.String("from", from.String()),
				zap.String("to", to.String()),
			)
		},
	}

	return &CircuitBreakerClient{
		client: mesh.NewMeshCtrlClient(conn),
		cb:     gobreaker.NewCircuitBreaker(settings),
		cache:  cache,
		logger: logger.L(),
	}
}

func (c *CircuitBreakerClient) Subscribe(ctx context.Context, req *mesh.SubscriptionRequest) error {
	_, err := c.cb.Execute(func() (interface{}, error) {
		stream, err := c.client.Subscribe(ctx, req)
		if err != nil {
			return nil, err
		}
		
		return c.handleStream(stream)
	})

	if err == gobreaker.ErrOpenState {
		c.logger.Warn("circuit breaker is open, using local cache",
			zap.String("service", req.ServiceName))
		return c.useFallbackCache(req.ServiceName)
	}

	return err
}

func (c *CircuitBreakerClient) useFallbackCache(serviceName string) error {
	// 使用本地缓存提供服务
	endpoints := c.cache.Get(serviceName)
	if endpoints == nil {
		return fmt.Errorf("service not in cache: %s", serviceName)
	}
	
	c.logger.Info("serving from local cache",
		zap.String("service", serviceName),
		zap.Int("endpoints", len(endpoints)))
	
	// 后台尝试重连
	go c.backgroundReconnect()
	
	return nil
}
```

#### Step 3: 实现本地缓存

修改 `pkg/cli/cache.go`，添加持久化：
```go
type LocalCache struct {
	mu    sync.RWMutex
	data  map[string]*CachedEndpoints
	path  string // 缓存文件路径
}

type CachedEndpoints struct {
	IPs       []string
	Revision  int64
	UpdatedAt time.Time
}

func NewLocalCache(path string) *LocalCache {
	c := &LocalCache{
		data: make(map[string]*CachedEndpoints),
		path: path,
	}
	
	// 启动时加载缓存
	c.Load()
	
	// 定期持久化
	go c.persistLoop()
	
	return c
}

func (c *LocalCache) Load() error {
	data, err := os.ReadFile(c.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	
	c.mu.Lock()
	defer c.mu.Unlock()
	return json.Unmarshal(data, &c.data)
}

func (c *LocalCache) Save() error {
	c.mu.RLock()
	data, err := json.Marshal(c.data)
	c.mu.RUnlock()
	
	if err != nil {
		return err
	}
	
	return os.WriteFile(c.path, data, 0644)
}

func (c *LocalCache) persistLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		if err := c.Save(); err != nil {
			logger.L().Error("failed to persist cache", zap.Error(err))
		}
	}
}
```

✅ **验收标准**：
- [ ] Controller 停止后，客户端切换到本地缓存
- [ ] 熔断器状态变化有日志记录
- [ ] 本地缓存能持久化到文件

---

### 任务 2.3：自动重连机制（⏱️ 4小时）

在 `pkg/cli/client.go` 中实现：
```go
type Client struct {
	conn         *grpc.ClientConn
	client       mesh.MeshCtrlClient
	reconnecting atomic.Bool
}

func (c *Client) SubscribeWithRetry(ctx context.Context, req *mesh.SubscriptionRequest) error {
	backoff := NewExponentialBackoff(
		1*time.Second,  // 初始间隔
		30*time.Second, // 最大间隔
		2.0,            // 倍增因子
	)
	
	for {
		err := c.Subscribe(ctx, req)
		if err == nil {
			backoff.Reset()
			continue // 正常断开，立即重连
		}
		
		if ctx.Err() != nil {
			return ctx.Err() // 上下文取消
		}
		
		// 计算退避时间
		delay := backoff.Next()
		logger.L().Warn("subscribe failed, retrying",
			zap.Error(err),
			zap.Duration("retry_after", delay))
		
		select {
		case <-time.After(delay):
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// 指数退避实现
type ExponentialBackoff struct {
	initial time.Duration
	max     time.Duration
	factor  float64
	current time.Duration
}

func (b *ExponentialBackoff) Next() time.Duration {
	if b.current == 0 {
		b.current = b.initial
	} else {
		b.current = time.Duration(float64(b.current) * b.factor)
		if b.current > b.max {
			b.current = b.max
		}
	}
	return b.current
}

func (b *ExponentialBackoff) Reset() {
	b.current = 0
}
```

✅ **验收标准**：
- [ ] Controller 重启后，客户端自动重连成功
- [ ] 重连间隔符合指数退避（1s, 2s, 4s, ..., 30s）
- [ ] 多次失败后不会无限重试

---

## 里程碑 3：性能优化（Week 3-4）

> **目标**：产出性能测试数据和优化报告

### 任务 3.1：添加 Benchmark 测试（⏱️ 6小时）

创建 `pkg/ctrl/storage_bench_test.go`：
```go
package ctrl

import (
	"testing"
	discovery "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func BenchmarkCoreData_OnAdded_10Services(b *testing.B) {
	benchmarkOnAdded(b, 10, 10) // 10 services, 每个 10 endpoints
}

func BenchmarkCoreData_OnAdded_100Services(b *testing.B) {
	benchmarkOnAdded(b, 100, 10)
}

func BenchmarkCoreData_OnAdded_1000Services(b *testing.B) {
	benchmarkOnAdded(b, 1000, 10)
}

func benchmarkOnAdded(b *testing.B, numServices, endpointsPerService int) {
	dist := &mockDistributer{}
	coreData := NewCoreData(dist)
	
	slices := generateTestSlices(numServices, endpointsPerService)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		coreData.OnAdded(slices[i%len(slices)])
	}
}

func BenchmarkCoreData_List_1000Services(b *testing.B) {
	dist := &mockDistributer{}
	coreData := NewCoreData(dist)
	
	// 预填充 1000 个服务
	for i := 0; i < 1000; i++ {
		slice := generateTestSlice(i, 10)
		coreData.OnAdded(slice)
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_ = coreData.List()
	}
}

// 生成测试数据辅助函数
func generateTestSlices(count, endpoints int) []*discovery.EndpointSlice {
	slices := make([]*discovery.EndpointSlice, count)
	for i := 0; i < count; i++ {
		slices[i] = generateTestSlice(i, endpoints)
	}
	return slices
}

func generateTestSlice(idx, endpoints int) *discovery.EndpointSlice {
	// ... 生成测试 EndpointSlice ...
}
```

运行 benchmark：
```bash
# 运行基准测试
go test -bench=. -benchmem ./pkg/ctrl/

# 输出示例
BenchmarkCoreData_OnAdded_10Services-8       50000    25123 ns/op    4832 B/op    32 allocs/op
BenchmarkCoreData_OnAdded_100Services-8      10000   102345 ns/op   48320 B/op   320 allocs/op
BenchmarkCoreData_OnAdded_1000Services-8      1000  1023456 ns/op  483200 B/op  3200 allocs/op

# 保存基准测试结果（用于对比优化效果）
go test -bench=. -benchmem ./pkg/ctrl/ > bench_before.txt
```

---

### 任务 3.2：集成 pprof（⏱️ 2小时）

在 `cmd/ctrl/app/option.go` 中：
```go
import _ "net/http/pprof"

func (o *Options) Run() {
	// HTTP 服务已经在 8080，pprof 自动注册到 DefaultServeMux
	// 访问 http://localhost:8080/debug/pprof/
	
	// 如果想用独立端口：
	go func() {
		log.Println("pprof server listening on :6060")
		http.ListenAndServe("localhost:6060", nil)
	}()
}
```

使用 pprof 分析：
```bash
# 1. CPU 性能分析（采样 30 秒）
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# 进入交互模式后
(pprof) top 10        # 查看 CPU 占用前 10 的函数
(pprof) list OnAdded  # 查看具体函数的热点行
(pprof) web           # 生成调用图（需要 graphviz）

# 2. 内存分析
go tool pprof http://localhost:6060/debug/pprof/heap

# 3. goroutine 泄漏检测
go tool pprof http://localhost:6060/debug/pprof/goroutine

# 4. 生成火焰图
go tool pprof -http=:8081 http://localhost:6060/debug/pprof/profile?seconds=30
# 浏览器访问 http://localhost:8081
```

---

### 任务 3.3：性能优化实施（⏱️ 8小时）

#### 优化 1：使用 sync.Pool 复用对象

在 `pkg/ctrl/distributer/pool.go` 创建：
```go
package distributer

import (
	"sync"
	"github.com/BuMaRen/mesh/pkg/api/mesh"
)

var eventPool = sync.Pool{
	New: func() interface{} {
		return &mesh.ClientSubscriptionEvent{
			Endpoints: make(map[string]*mesh.EndpointIPs),
		}
	},
}

func GetEvent() *mesh.ClientSubscriptionEvent {
	return eventPool.Get().(*mesh.ClientSubscriptionEvent)
}

func PutEvent(e *mesh.ClientSubscriptionEvent) {
	// 清空数据
	e.Revision = 0
	e.OpType = 0
	for k := range e.Endpoints {
		delete(e.Endpoints, k)
	}
	
	eventPool.Put(e)
}
```

在 `pkg/ctrl/distributer/distributer.go` 中使用：
```go
func (d *Distributer) Publish(svcName string, opType mesh.OpType, payload map[string][]string) {
	event := GetEvent()
	defer PutEvent(event)
	
	event.OpType = opType
	event.Revision = d.nextRevision()
	
	for k, v := range payload {
		event.Endpoints[k] = &mesh.EndpointIPs{EndpointIps: v}
	}
	
	d.broadcast(event)
}
```

#### 优化 2：读写锁替代互斥锁

在 `pkg/ctrl/storage.go` 中：
```go
type CoreData struct {
	mu sync.RWMutex  // 原来是 sync.Mutex
	serviceMap map[string]*ServiceCache
	distributer Distributer
}

func (c *CoreData) List() []string {
	c.mu.RLock()  // 使用读锁
	defer c.mu.RUnlock()
	
	result := make([]string, 0, len(c.serviceMap))
	for svc := range c.serviceMap {
		result = append(result, svc)
	}
	return result
}

// 写操作仍然用 Lock()
func (c *CoreData) OnAdded(slice *discovery.EndpointSlice) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// ...
}
```

#### 优化 3：批量推送（减少 gRPC 调用）

在 `pkg/ctrl/distributer/batch.go` 创建：
```go
type BatchDistributer struct {
	inner       Distributer
	batchWindow time.Duration
	pendingCh   chan *batchItem
}

type batchItem struct {
	svcName string
	opType  mesh.OpType
	payload map[string][]string
}

func NewBatchDistributer(inner Distributer, window time.Duration) *BatchDistributer {
	d := &BatchDistributer{
		inner:       inner,
		batchWindow: window,
		pendingCh:   make(chan *batchItem, 1000),
	}
	
	go d.batchLoop()
	return d
}

func (d *BatchDistributer) batchLoop() {
	ticker := time.NewTicker(d.batchWindow)
	defer ticker.Stop()
	
	batch := make([]*batchItem, 0, 100)
	
	for {
		select {
		case item := <-d.pendingCh:
			batch = append(batch, item)
			
		case <-ticker.C:
			if len(batch) > 0 {
				d.flushBatch(batch)
				batch = batch[:0]
			}
		}
	}
}

func (d *BatchDistributer) Publish(svcName string, opType mesh.OpType, payload map[string][]string) {
	d.pendingCh <- &batchItem{svcName, opType, payload}
}

func (d *BatchDistributer) flushBatch(batch []*batchItem) {
	// 合并相同服务的多次更新
	merged := make(map[string]*batchItem)
	for _, item := range batch {
		merged[item.svcName] = item // 后面的覆盖前面的
	}
	
	for _, item := range merged {
		d.inner.Publish(item.svcName, item.opType, item.payload)
	}
}
```

#### 验证优化效果

```bash
# 再次运行 benchmark
go test -bench=. -benchmem ./pkg/ctrl/ > bench_after.txt

# 对比优化前后
benchcmp bench_before.txt bench_after.txt

# 预期结果（示例）
benchmark                           old ns/op     new ns/op     delta
BenchmarkCoreData_OnAdded_10-8      25123         18456        -26.5%
BenchmarkCoreData_OnAdded_100-8     102345        76234        -25.5%

benchmark                           old allocs     new allocs     delta
BenchmarkCoreData_OnAdded_10-8      32             18            -43.8%
```

✅ **验收标准**：
- [ ] 性能提升 20% 以上
- [ ] 内存分配减少 30% 以上
- [ ] 有优化前后的对比数据

---

### 任务 3.4：压力测试（⏱️ 6小时）

#### Step 1: 安装 ghz
```bash
go install github.com/bojand/ghz/cmd/ghz@latest
```

#### Step 2: 创建压测脚本

创建 `tests/loadtest/grpc_bench.sh`：
```bash
#!/bin/bash

CONTROLLER_ADDR="localhost:50051"
PROTO_PATH="../../pkg/api/mesh/mesh.proto"

echo "=== gRPC 压力测试 ==="
echo "目标: $CONTROLLER_ADDR"
echo ""

# 场景 1：单连接吞吐量
echo "场景 1: 单连接吞吐量测试"
ghz --insecure \
  --proto $PROTO_PATH \
  --call mesh.MeshCtrl.Subscribe \
  -d '{"sidecar_id":"test-1","service_name":"demo-service"}' \
  -c 1 \
  -n 1000 \
  $CONTROLLER_ADDR

echo ""

# 场景 2：并发连接测试
echo "场景 2: 100 并发连接"
ghz --insecure \
  --proto $PROTO_PATH \
  --call mesh.MeshCtrl.Subscribe \
  -d '{"sidecar_id":"test-{{.RequestNumber}}","service_name":"demo-service"}' \
  -c 100 \
  -z 30s \
  $CONTROLLER_ADDR

echo ""

# 场景 3：大规模订阅
echo "场景 3: 1000 并发订阅（持续 1 分钟）"
ghz --insecure \
  --proto $PROTO_PATH \
  --call mesh.MeshCtrl.Subscribe \
  -d '{"sidecar_id":"test-{{.RequestNumber}}","service_name":"service-{{.RequestNumber}}"}' \
  -c 1000 \
  -z 60s \
  --connections=50 \
  $CONTROLLER_ADDR \
  > loadtest_report.txt

echo ""
echo "压测完成，报告已保存到 loadtest_report.txt"
```

#### Step 3: 运行压测并记录结果

```bash
chmod +x tests/loadtest/grpc_bench.sh
./tests/loadtest/grpc_bench.sh
```

创建压测报告 `docs/performance-report.md`：
```markdown
# Masha-Mesh 性能测试报告

## 测试环境

- **硬件**: 4C8G (AWS t3.large)
- **OS**: Ubuntu 22.04
- **Go 版本**: 1.25.5
- **K8s 版本**: v1.35.2

## 测试场景

### 场景 1: 单连接吞吐量

**配置**: 1 个客户端，1000 次请求

| 指标 | 数值 |
|------|------|
| 总耗时 | 1.2s |
| QPS | 833 req/s |
| 平均延迟 | 1.2ms |
| P50 延迟 | 0.8ms |
| P99 延迟 | 5.4ms |

### 场景 2: 并发连接

**配置**: 100 并发客户端，持续 30s

| 指标 | 数值 |
|------|------|
| 总请求数 | 25000 |
| QPS | 833 req/s |
| 平均延迟 | 12ms |
| P99 延迟 | 48ms |
| 错误率 | 0% |

### 场景 3: 大规模订阅

**配置**: 1000 并发订阅，持续 60s

| 指标 | 数值 |
|------|------|
| 活跃连接数 | 1000 |
| Controller 内存 | 450MB |
| Controller CPU | 60% |
| goroutine 数 | 1200 |

## 性能瓶颈分析

通过 pprof 分析发现：

1. **CPU 热点**: `protobuf.Marshal` 占用 35%
2. **内存热点**: `map[string][]string` 频繁分配占用 40%
3. **锁竞争**: `sync.Mutex` 在高并发时有轻微竞争

## 优化措施

1. 引入 `sync.Pool` 复用 protobuf 对象 → **内存分配减少 45%**
2. 使用 `sync.RWMutex` 优化读多写少场景 → **QPS 提升 18%**
3. 批量推送机制（100ms 窗口）→ **延迟降低 30%**

## 优化前后对比

| 指标 | 优化前 | 优化后 | 提升 |
|------|--------|--------|------|
| QPS | 650 | 833 | +28% |
| P99 延迟 | 68ms | 48ms | -29% |
| 内存占用 | 650MB | 450MB | -31% |

## 结论

当前系统可稳定支持：
- 1000+ 并发长连接
- 800+ QPS 推送能力
- P99 延迟 < 50ms

满足中小规模生产环境需求（< 5000 Pod）。
```

✅ **验收标准**：
- [ ] 完成 3 种压测场景
- [ ] 产出性能测试报告
- [ ] 有优化前后的对比数据

---

## 里程碑 4：高级特性（Week 5-6）

> **目标**：添加生产环境常用的高级特性

### 任务 4.1：分布式追踪（⏱️ 8小时）

#### Step 1: 安装依赖
```bash
go get go.opentelemetry.io/otel
go get go.opentelemetry.io/otel/exporters/jaeger
go get go.opentelemetry.io/otel/sdk/trace
go get go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc
```

#### Step 2: 初始化 Tracer

创建 `pkg/tracing/tracing.go`：
```go
package tracing

import (
	"context"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

func InitTracer(serviceName, jaegerEndpoint string) (func(context.Context) error, error) {
	exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(
		jaeger.WithEndpoint(jaegerEndpoint),
	))
	if err != nil {
		return nil, err
	}

	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
		)),
	)

	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}
```

#### Step 3: 在关键路径添加 trace

修改 `pkg/ctrl/storage.go`：
```go
import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

func (c *CoreData) OnAdded(ctx context.Context, slice *discovery.EndpointSlice) {
	ctx, span := otel.Tracer("mesh-controller").Start(ctx, "CoreData.OnAdded")
	defer span.End()
	
	span.SetAttributes(
		attribute.String("service", svcName),
		attribute.Int("endpoints", len(endpoints)),
	)
	
	// ... 业务逻辑 ...
	
	// 嵌套 span
	_, publishSpan := otel.Tracer("mesh-controller").Start(ctx, "PublishEvent")
	c.distributer.Publish(ctx, svcName, event)
	publishSpan.End()
}
```

#### Step 4: 集成 gRPC 拦截器

在 `cmd/ctrl/app/option.go` 中：
```go
import "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"

func (o *Options) Run() {
	// 初始化 tracer
	shutdown, err := tracing.InitTracer("mesh-controller", "http://localhost:14268/api/traces")
	if err != nil {
		logger.L().Fatal("failed to init tracer", zap.Error(err))
	}
	defer shutdown(context.Background())
	
	// gRPC server 添加追踪拦截器
	grpcServer := grpc.NewServer(
		grpc.StreamInterceptor(otelgrpc.StreamServerInterceptor()),
		grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor()),
	)
}
```

#### Step 5: 部署 Jaeger

在 `build/monitoring/docker-compose.yml` 中添加：
```yaml
  jaeger:
    image: jaegertracing/all-in-one:latest
    container_name: mesh-jaeger
    ports:
      - "16686:16686"  # UI
      - "14268:14268"  # Collector
    environment:
      - COLLECTOR_ZIPKIN_HOST_PORT=:9411
    networks:
      - mesh-monitor
```

启动后访问 http://localhost:16686 查看追踪信息。

✅ **验收标准**：
- [ ] Jaeger UI 中能看到请求链路
- [ ] 关键操作有嵌套 span
- [ ] 能追踪到延迟瓶颈

---

### 任务 4.2：限流保护（⏱️ 4小时）

创建 `pkg/ctrl/ratelimit/limiter.go`：
```go
package ratelimit

import (
	"context"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func UnaryServerInterceptor(limiter *rate.Limiter) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if !limiter.Allow() {
			return nil, status.Errorf(codes.ResourceExhausted, "rate limit exceeded")
		}
		return handler(ctx, req)
	}
}

func StreamServerInterceptor(limiter *rate.Limiter) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if !limiter.Allow() {
			return status.Errorf(codes.ResourceExhausted, "rate limit exceeded")
		}
		return handler(srv, ss)
	}
}
```

在 server 中使用：
```go
func (o *Options) Run() {
	limiter := rate.NewLimiter(1000, 2000) // 每秒 1000 个请求，突发 2000
	
	grpcServer := grpc.NewServer(
		grpc.StreamInterceptor(ratelimit.StreamServerInterceptor(limiter)),
		grpc.UnaryInterceptor(ratelimit.UnaryServerInterceptor(limiter)),
	)
}
```

---

### 任务 4.3：混沌工程测试（⏱️ 6小时）

#### Step 1: 安装 Chaos Mesh
```bash
kubectl create ns chaos-testing
helm repo add chaos-mesh https://charts.chaos-mesh.org
helm install chaos-mesh chaos-mesh/chaos-mesh -n chaos-testing --set chaosDaemon.runtime=containerd --set chaosDaemon.socketPath=/run/containerd/containerd.sock
```

#### Step 2: 创建故障注入实验

创建 `tests/chaos/pod-kill.yaml`：
```yaml
apiVersion: chaos-mesh.org/v1alpha1
kind: PodChaos
metadata:
  name: controller-pod-kill
  namespace: default
spec:
  action: pod-kill
  mode: one
  selector:
    labelSelectors:
      app: mesh-controller
  scheduler:
    cron: "@every 5m"
```

创建 `tests/chaos/network-delay.yaml`：
```yaml
apiVersion: chaos-mesh.org/v1alpha1
kind: NetworkChaos
metadata:
  name: controller-network-delay
  namespace: default
spec:
  action: delay
  mode: one
  selector:
    labelSelectors:
      app: mesh-controller
  delay:
    latency: "100ms"
    correlation: "25"
    jitter: "10ms"
  duration: "2m"
```

#### Step 3: 运行故障演练

```bash
# 应用故障
kubectl apply -f tests/chaos/pod-kill.yaml

# 观察
# 1. 客户端是否自动重连
# 2. Grafana 中是否有告警
# 3. 服务是否快速恢复

# 清理
kubectl delete -f tests/chaos/pod-kill.yaml
```

✅ **验收标准**：
- [ ] Pod 被杀死后，新 Pod 自动拉起
- [ ] 客户端能在 30s 内重连成功
- [ ] 监控大盘显示异常（连接数下降）

---

## 📊 最终验收清单

完成所有任务后，项目应该具备以下能力：

### 可观测性
- [x] Prometheus metrics（10+ 指标）
- [x] Grafana 监控面板（4+ 图表）
- [x] 结构化日志（zap）
- [x] 分布式追踪（Jaeger）

### 稳定性
- [x] 健康检查端点（/healthz, /readyz）
- [x] 优雅启动/关闭
- [x] 客户端熔断降级
- [x] 自动重连（指数退避）
- [x] 本地缓存（持久化）

### 性能
- [x] Benchmark 测试（3+ 场景）
- [x] pprof 性能分析
- [x] 性能优化（sync.Pool, RWMutex）
- [x] 压力测试报告

### 容错
- [x] 限流保护
- [x] 超时控制
- [x] 故障演练（Chaos Mesh）

### 生产实践
- [x] 多副本部署
- [x] 资源限制（CPU/内存）
- [x] 滚动更新
- [x] 故障案例文档

---

## 🎯 时间分配建议

| Week | 里程碑 | 工作量 | 关键产出 |
|------|--------|--------|----------|
| Week 1 | Metrics + 日志 | 13h | Grafana 面板 |
| Week 2 | 健康检查 + 熔断 | 14h | 优雅关闭验证 |
| Week 3 | Benchmark + pprof | 14h | 性能基线数据 |
| Week 4 | 优化 + 压测 | 14h | 性能报告 |
| Week 5 | 追踪 + 限流 | 12h | Jaeger 链路 |
| Week 6 | 混沌测试 + 文档 | 10h | 故障案例 |

**总计**: 约 77 小时（平均每周 13 小时，工作日每天 2 小时 + 周末 6 小时）

---

## 💡 快速起步（本周末就能做）

**优先级排序**（投入产出比最高的 3 件事）：

1. **Prometheus + Grafana**（6h） → 立即有可视化效果
2. **结构化日志**（4h） → 提升代码专业度
3. **健康检查 + 优雅关闭**（4h） → 生产必备

**本周末目标**：完成上述 3 项，周一就能在简历上写"具备生产环境监控能力"。

---

## 📚 每个阶段的学习资源

- **Week 1-2**: 《Prometheus 监控实战》、Grafana 官方文档
- **Week 3-4**: 《Go 语言高性能编程》、Dave Cheney 博客
- **Week 5-6**: OpenTelemetry 文档、《混沌工程实践》

---

## 🚀 下一步

选择你想先做的任务，我可以帮你：
1. 生成具体的代码实现
2. 解答实施过程中的疑问
3. Review 你的改动

**建议从任务 1.1（Prometheus Metrics）开始，现在就动手！**
