# masha-mesh 控制面 Prometheus 指标监控落地指南

本文给出一套可直接落地的方案，目标是让 mesh-ctrl 具备可观测性：

- 可被 Prometheus 定时抓取
- 同时暴露运行时指标与业务指标
- 能支撑排障、容量判断和基本告警

适用范围：当前仓库中的控制面进程（cmd/ctrl + pkg/ctrl）。

## 1. 目标与原则

建议先落地最小闭环，再逐步扩展：

1. 暴露 /metrics HTTP 端点
2. 接入基础指标（Go runtime + process）
3. 增加核心业务指标（订阅数、事件推送、缓存规模、耗时）
4. 将指标接入 Prometheus（ServiceMonitor 或 annotations）
5. 配置基础告警规则并验证

指标设计原则：

- 稳定：指标名和标签一旦上线，避免频繁变更
- 低基数：避免把 podName、sidecarId 这类高基数字段做 label
- 可聚合：优先使用 service、opType、result 等可聚合维度
- 可解释：每个指标要有明确业务含义和单位

## 2. 代码接入方案

### 2.1 新增依赖

在仓库根目录执行：

		go get github.com/prometheus/client_golang/prometheus
		go get github.com/prometheus/client_golang/prometheus/promhttp
		go mod tidy

### 2.2 增加指标 HTTP Server

建议新建文件：

- pkg/ctrl/metrics/server.go

核心逻辑：单独监听一个 metrics 端口（例如 :9091），只提供 /metrics。

示例实现：

		package metrics

		import (
				"context"
				"net"
				"net/http"

				"github.com/prometheus/client_golang/prometheus/promhttp"
				"k8s.io/klog/v2"
		)

		func StartMetricsServer(ctx context.Context, addr string) error {
				mux := http.NewServeMux()
				mux.Handle("/metrics", promhttp.Handler())

				ln, err := net.Listen("tcp", addr)
				if err != nil {
						return err
				}

				srv := &http.Server{Handler: mux}

				go func() {
						<-ctx.Done()
						if err := srv.Close(); err != nil {
								klog.Warningf("metrics server close failed: %v", err)
						}
				}()

				return srv.Serve(ln)
		}

### 2.3 定义业务指标

建议新建文件：

- pkg/ctrl/metrics/metrics.go

建议先落地以下指标：

- mesh_ctrl_sidecars_connected（Gauge）
	- 当前已连接 sidecar 数
- mesh_ctrl_subscription_events_total（CounterVec: service, op_type）
	- 推送给 sidecar 的事件总量
- mesh_ctrl_subscription_send_failures_total（CounterVec: service）
	- 推送失败次数
- mesh_ctrl_reconcile_duration_seconds（HistogramVec: reconciler, result）
	- 核心 reconcile 延迟，单位秒
- mesh_ctrl_cache_objects（GaugeVec: cache）
	- cache 对象数量，比如 endpointslice、container

示例（节选）：

		package metrics

		import "github.com/prometheus/client_golang/prometheus"

		var (
				SidecarsConnected = prometheus.NewGauge(prometheus.GaugeOpts{
						Namespace: "mesh",
						Subsystem: "ctrl",
						Name:      "sidecars_connected",
						Help:      "Current number of connected sidecars.",
				})

				SubscriptionEventsTotal = prometheus.NewCounterVec(
						prometheus.CounterOpts{
								Namespace: "mesh",
								Subsystem: "ctrl",
								Name:      "subscription_events_total",
								Help:      "Total number of subscription events sent to sidecars.",
						},
						[]string{"service", "op_type"},
				)
		)

		func MustRegister() {
				prometheus.MustRegister(
						SidecarsConnected,
						SubscriptionEventsTotal,
				)
		}

### 2.4 在启动流程挂载 metrics

接入点建议：

- pkg/ctrl/logic.go

在 StartUp 初始化阶段做两件事：

1. 调用 metrics.MustRegister()
2. 起一个 goroutine 运行 metrics.StartMetricsServer(ctx, ":9091")

建议改造 StartUp 的方式：

- 将 metrics server 也纳入 WaitGroup
- 退出时和 grpc/informer 一样由 context 控制

### 2.5 在关键路径打点

优先打点位置：

1. gRPC 订阅生命周期
	 - 文件：pkg/ctrl/grpcserver/server.go
	 - Subscribe 成功建立连接后：SidecarsConnected.Inc()
	 - defer 清理时：SidecarsConnected.Dec()
	 - 每次 sss.Send 成功：SubscriptionEventsTotal.WithLabelValues(serviceName, opType).Inc()
	 - Send 失败：SubscriptionSendFailuresTotal.WithLabelValues(serviceName).Inc()

2. Reconciler
	 - 文件：pkg/ctrl/reconciler/endpointslice-reconciler.go
	 - 文件：pkg/ctrl/reconciler/container-reconciler.go
	 - 在 OnAdded/OnUpdated/OnDeleted 外层记录耗时
	 - 成功/失败写入 result 标签（success, failure）

3. Cache
	 - 文件：pkg/ctrl/data/endpointslice_cache.go
	 - 文件：pkg/ctrl/data/containers_cache.go
	 - 在增删后更新 GaugeVec(cache=endpointslice|container)

## 3. Kubernetes 暴露方式

当前部署文件位于：

- build/ctrl/deployment.yml
- build/ctrl/service.yml

### 3.1 Deployment 增加 metrics 端口

在容器 ports 增加一项：

		- containerPort: 9091
			name: metrics

如使用参数化地址，也可在 args 增加：

		--metrics-address=:9091

注意：当前仓库 deployment 参数仍在使用短参数形式（-p, -n）。如果后续统一为长参数，请同步更新启动参数定义，避免参数漂移。

### 3.2 Service 暴露 metrics 端口

在 build/ctrl/service.yml 的 ports 下增加：

		- port: 9091
			targetPort: metrics
			protocol: TCP
			name: metrics

### 3.3 使用 annotations 让 Prometheus 抓取（非 Operator 场景）

给 Service metadata.annotations 增加：

		prometheus.io/scrape: "true"
		prometheus.io/path: "/metrics"
		prometheus.io/port: "9091"

## 4. Prometheus 抓取配置

### 4.1 使用 Prometheus Operator（推荐）

可新增 ServiceMonitor（示例）：

		apiVersion: monitoring.coreos.com/v1
		kind: ServiceMonitor
		metadata:
			name: mesh-ctrl
			namespace: monitoring
			labels:
				release: prometheus
		spec:
			namespaceSelector:
				matchNames:
					- default
			selector:
				matchLabels:
					app: mesh-ctrl
			endpoints:
				- port: metrics
					path: /metrics
					interval: 15s

### 4.2 原生 Prometheus（静态抓取）

在 prometheus.yml 增加 job（示例）：

		scrape_configs:
			- job_name: mesh-ctrl
				scrape_interval: 15s
				metrics_path: /metrics
				static_configs:
					- targets:
							- mesh-ctrl.default.svc.cluster.local:9091

## 5. 验证清单

按以下顺序验收：

1. 本地编译

			 go test ./...
			 make ctrl

2. 部署

			 cd build
			 make all

3. 端点可达

			 kubectl -n default port-forward svc/mesh-ctrl 9091:9091
			 curl -s localhost:9091/metrics | head

4. 查询关键指标

			 curl -s localhost:9091/metrics | grep mesh_ctrl_sidecars_connected
			 curl -s localhost:9091/metrics | grep mesh_ctrl_subscription_events_total

5. Prometheus UI 验证
	 - mesh_ctrl_sidecars_connected
	 - rate(mesh_ctrl_subscription_events_total[5m])
	 - histogram_quantile(0.95, sum(rate(mesh_ctrl_reconcile_duration_seconds_bucket[5m])) by (le, reconciler))

## 6. 告警建议（第一版）

建议先上 3 条实用告警：

1. 推送失败增长
	 - 表达式：increase(mesh_ctrl_subscription_send_failures_total[5m]) > 0
	 - 含义：控制面到 sidecar 推送出现失败

2. Reconcile P95 过高
	 - 表达式：
		 histogram_quantile(0.95, sum(rate(mesh_ctrl_reconcile_duration_seconds_bucket[5m])) by (le, reconciler)) > 1
	 - 含义：核心控制回路出现延迟抖动

3. Sidecar 连接数突降
	 - 表达式：mesh_ctrl_sidecars_connected < 1
	 - 含义：可能出现控制面不可达、网络策略变更或 sidecar 批量重启

## 7. 演进路线（建议）

第一阶段（本周）：

- 暴露 /metrics
- 落地 3 到 5 个核心业务指标
- 打通 Prometheus 抓取

第二阶段（下周）：

- 增加 Dashboard（连接数、事件速率、reconcile 延迟、失败率）
- 增加告警收敛规则（分级告警、抑制策略）

第三阶段：

- 引入 Trace 与日志关联（通过 request id 或 service 标签联动）
- 按 SLO 定义可观测性目标（可用性、时延、错误率）

---

如果你希望，我可以在下一步直接按本文档把第一阶段代码和 YAML 改动一起落地（包含 metrics server、指标定义、deployment/service patch、ServiceMonitor 示例文件）。
