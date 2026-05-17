package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	registerMetricsOnce sync.Once

	// SideCarsConnected 表示当前时刻已建立连接的 sidecar 数量。
	//
	// 业务里推荐这样打点：
	// 1. 连接建立成功后调用 Inc()。
	// 2. 连接关闭、订阅结束、协程退出前在 defer 里调用 Dec()。
	//
	// 适用场景：gRPC Subscribe 建立连接成功后 +1，连接断开时 -1。
	// 这个指标反映的是“当前有多少个在线连接”，因此要用 Gauge，
	// 而不是 Counter。
	//
	// 最小示例：
	//
	//  func (s *GrpcServer) Subscribe(...) error {
	//      metrics.SideCarsConnected.Inc()
	//      defer metrics.SideCarsConnected.Dec()
	//
	//      // 后面执行业务逻辑
	//      return nil
	//  }
	//
	// 你可以把它理解成“当前在线人数”，有人进来就 +1，离开就 -1。
	SideCarsConnected = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "default",
		Subsystem: "mesh_ctrl",
		Name:      "sidecars_connected",
		Help:      "Current number of connected sidecars.",
	})

	// SubscriptionEventsTotal 表示事件发送总次数。
	//
	// 业务里推荐这样打点：
	// 1. 每当一条事件真正发送成功后，调用
	//    WithLabelValues(eventType, opType).Inc()。
	// 2. 如果一次业务动作里发送了多条事件，就按实际发送条数累加。
	// 3. 不要在“准备发送”时打点，而要在“发送成功”后打点，
	//    否则统计值会偏大。
	//
	// label 建议含义：
	// - event_type: 事件类别，例如 endpointslice、container。
	// - op_type: 操作类型，例如 add、update、delete。
	//
	// 注意：这两个 label 都应该是低基数、可枚举值，
	// 不要放 sidecar_id、request_id 这类高基数字段。
	//
	// 最小示例：
	//
	//  func sendEvent(eventType, opType string) error {
	//      err := doSend()
	//      if err != nil {
	//          return err
	//      }
	//
	//      metrics.SubscriptionEventsTotal.
	//          WithLabelValues(eventType, opType).
	//          Inc()
	//      return nil
	//  }
	//
	// 如果发送了 3 条事件，就成功一次加一次；
	// 不要在“准备发送”时加，而要在“确认成功发送”后再加。
	SubscriptionEventsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "default",
		Subsystem: "mesh_ctrl",
		Name:      "subscription_events_total",
		Help:      "Total number of subscription events.",
	}, []string{"event_type", "op_type"})

	// ReconcileDurationSeconds 表示一次 reconcile 处理耗时。
	//
	// 业务里推荐这样打点：
	// 1. 在函数开始时记录 start := time.Now()。
	// 2. 在函数返回前调用 Observe(time.Since(start).Seconds())。
	// 3. 建议把整个一次完整处理流程包起来，而不是只统计其中一小段。
	//
	// 适用场景：OnAdded、OnUpdated、OnDeleted 这类调谐入口。
	// 该指标用于后续计算 P50/P95/P99 延迟。
	//
	// 如果后续要区分不同 reconciler 或 success/failure，
	// 更适合改成 HistogramVec，并增加 reconciler、result 标签。
	//
	// 最小示例：
	//
	//  func (r *Reconciler) OnAdded(obj any) {
	//      start := time.Now()
	//      defer func() {
	//          metrics.ReconcileDurationSeconds.Observe(
	//              time.Since(start).Seconds(),
	//          )
	//      }()
	//
	//      // 后面执行业务逻辑
	//  }
	//
	// 你可以把它理解成“给这次处理过程掐表计时”，
	// 函数开始时记下时间，结束时把耗时上报出去。
	ReconcileDurationSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "default",
		Subsystem: "mesh_ctrl",
		Name:      "reconcile_duration_seconds",
		Help:      "Duration of reconcile operations in seconds.",
	})
)

// MustRegister 在进程启动阶段调用一次即可，用于把所有指标注册到默认 registry。
//
// 一般做法：
// 1. 在 main 或 StartUp 初始化时调用一次。
// 2. 不要在业务热路径里重复调用，否则可能触发重复注册 panic。
//
// 最小示例：
//
//	func main() {
//	    metrics.MustRegister()
//	    // 再启动 grpc/http/server 等其他组件
//	}
func MustRegister() {
	registerMetricsOnce.Do(func() {
		prometheus.MustRegister(
			SideCarsConnected,
			SubscriptionEventsTotal,
			ReconcileDurationSeconds,
		)
	})
}
