/*
═══════════════════════════════════════════════════════════════════
阶段三：Informer + WorkQueue —— 解耦感知与处理
═══════════════════════════════════════════════════════════════════

【这一阶段解决的问题】
Phase2 的 EventHandler 回调在 Informer 的 processLoop goroutine 里同步执行。
如果回调耗时，processLoop 被阻塞，Indexer 更新延迟，整个 Informer 卡死。

解决方案：回调只做一件事——把 Key（"namespace/name"）放入 WorkQueue，
让独立的 worker goroutine 异步消费队列，处理真正的业务逻辑。

【WorkQueue 的数据流】

	Informer processLoop                 WorkQueue                Worker goroutine
	     │                                   │                          │
	     │─ AddFunc("default/foo") ─────────→│ enqueue "default/foo"   │
	     │─ UpdateFunc("default/foo") ───────→│ (已在队列里，去重，忽略) │
	     │─ UpdateFunc("default/bar") ───────→│ enqueue "default/bar"   │
	     │                                   │                          │
	     │  （processLoop 继续处理下一个事件）  │←── Get() 取出 foo ───────│
	     │                                   │                          │
	     │                                   │   Reconcile("default/foo")
	     │                                   │   从 Indexer 读最新状态  │
	     │                                   │   执行业务逻辑           │
	     │                                   │   Done("default/foo") ──→│

【WorkQueue 的三个关键特性】

 1. 去重（Deduplication）
    同一个 Key 如果已在队列里（还未被取出处理），再次入队会被忽略。
    意义：Pod 在很短时间内连续变化 100 次，最终只处理一次（最新状态）。

 2. 并发安全（Processing 锁）
    同一个 Key 在被 worker 取出后（调用 Get() 到调用 Done() 之间），
    即使再次入队，新的入队请求会被暂存（不会立刻重复处理）。
    等 Done() 调用后，暂存的入队请求才会生效。

 3. 限速（Rate Limiting）
    RateLimitingQueue 在失败重试时使用指数退避（ItemExponentialFailureRateLimiter）：
    第 1 次失败后等 5ms，第 2 次 10ms，第 3 次 20ms ... 上限 1000s。
    防止一个持续失败的 Key 把 worker 全部占满（热循环）。

【队列里存 Key 而不是对象的原因】

	如果存对象（整个 Pod struct），从入队到被处理之间，Pod 可能又变化了，
	worker 处理的是"过时的对象"，违背了 Level-Triggered 语义。
	存 Key 则每次 Reconcile 时都从 Indexer 读最新状态，永远处理当前状态。

【Level-Triggered vs Edge-Triggered】

	Edge-Triggered（边沿触发）：感知到"有变化"这个事件，处理该变化的 diff
	  → 如果事件丢失或处理失败，就永远不会再处理
	  → 需要精确的 diff 计算

	Level-Triggered（电平触发）：感知到"有变化"，但处理时读最新状态
	  → 无论触发了多少次，调谐结果只取决于当前状态
	  → 天然幂等：重试多次结果相同；处理完后状态已达期望，就不需要再处理
	  → Kubernetes 所有 Controller 都采用这种模式

═══════════════════════════════════════════════════════════════════
*/
package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

// Phase3Controller 使用 Informer + WorkQueue，实现感知与处理的完全解耦。
type Phase3Controller struct {
	clientset  kubernetes.Interface
	podIndexer cache.Indexer // 本地缓存，直接读，不请求 API Server
	podSynced  cache.InformerSynced
	// TypedRateLimitingInterface[string]：泛型版本，Key 的类型安全为 string
	queue workqueue.TypedRateLimitingInterface[string]
}

func NewPhase3Controller(clientset kubernetes.Interface) *Phase3Controller {
	factory := informers.NewSharedInformerFactory(clientset, 30*time.Second)
	podInformer := factory.Core().V1().Pods()

	c := &Phase3Controller{
		clientset:  clientset,
		podIndexer: podInformer.Informer().GetIndexer(),
		podSynced:  podInformer.Informer().HasSynced,
		// DefaultTypedControllerRateLimiter 组合了两种限速器：
		//   - ItemExponentialFailureRateLimiter：单个 Key 失败时指数退避
		//   - BucketRateLimiter：全局令牌桶，限制整体入队速率（10 qps，100 burst）
		queue: workqueue.NewTypedRateLimitingQueue(
			workqueue.DefaultTypedControllerRateLimiter[string](),
		),
	}

	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		// ── 回调只做一件事：入队 Key ──────────────────────────────────
		// 不做任何业务逻辑，保证 processLoop 不被阻塞。
		AddFunc:    func(obj any) { c.enqueue(obj) },
		UpdateFunc: func(_, newObj any) { c.enqueue(newObj) },
		DeleteFunc: func(obj any) { c.enqueueForDelete(obj) },
	})

	// factory.Start 在后台启动 Informer goroutine（List+Watch 开始工作）
	factory.Start(context.Background().Done()) // 生产中应传入受控的 ctx.Done()

	return c
}

// enqueue 从对象提取 Key 并加入队列。
func (c *Phase3Controller) enqueue(obj any) {
	// MetaNamespaceKeyFunc 生成 "namespace/name" 格式的 Key。
	// 对于集群级资源（如 Node），只有 name，没有 namespace 前缀。
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(fmt.Errorf("enqueue: failed to get key: %w", err))
		return
	}
	// Add 是幂等的：如果 Key 已在队列里，调用 Add 不会产生重复条目。
	c.queue.Add(key)
	klog.V(5).Infof("[Phase3] enqueued: %s", key)
}

// enqueueForDelete 专门处理删除事件，正确应对 DeletedFinalStateUnknown。
func (c *Phase3Controller) enqueueForDelete(obj any) {
	// DeletionHandlingMetaNamespaceKeyFunc 内部自动处理 DeletedFinalStateUnknown：
	// 如果 obj 是该包装类型，会从中提取真实对象再生成 Key。
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(fmt.Errorf("enqueueForDelete: failed to get key: %w", err))
		return
	}
	c.queue.Add(key)
	klog.V(5).Infof("[Phase3] enqueued for delete: %s", key)
}

// Run 等待缓存同步后启动 workers 消费 WorkQueue。
func (c *Phase3Controller) Run(ctx context.Context, workers int) error {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown() // 通知所有 worker 在处理完当前 Key 后退出

	// ── 等待 Indexer 完成首次全量同步 ─────────────────────────────
	// 在此之前 Indexer 是不完整的，不能开始处理（否则会把"缓存里没有"误判为"已被删除"）
	klog.Info("[Phase3] waiting for cache sync...")
	if !cache.WaitForCacheSync(ctx.Done(), c.podSynced) {
		return fmt.Errorf("timed out waiting for cache sync")
	}
	klog.Info("[Phase3] cache synced, starting workers")

	// ── 启动多个 worker goroutine ─────────────────────────────────
	// wait.UntilWithContext：每隔 1s 重启 worker（防止 worker 因 panic 退出后消失）
	for i := range workers {
		go wait.UntilWithContext(ctx, func(ctx context.Context) {
			klog.V(4).Infof("[Phase3] worker %d started", i)
			// processLoop：不断从队列取 Key 处理，直到队列关闭
			for c.processNext(ctx) {
			}
		}, time.Second)
	}

	<-ctx.Done()
	klog.Info("[Phase3] shutting down")
	return nil
}

// processNext 从队列取出一个 Key，调用 reconcile，处理成功/失败的后续操作。
// 返回 false 表示队列已关闭，worker 应该退出。
func (c *Phase3Controller) processNext(ctx context.Context) bool {
	// Get 会阻塞，直到队列里有新 Key 或队列被 ShutDown()。
	key, shutdown := c.queue.Get()
	if shutdown {
		return false
	}
	// Done 必须调用：通知队列"这个 Key 我处理完了"。
	// 若不调用 Done，这个 Key 在 Processing 集合里永远不会被清除，
	// 导致后续同一 Key 的入队请求一直被暂存，永远不会被处理。
	defer c.queue.Done(key)

	err := c.reconcile(ctx, key)
	if err == nil {
		// 成功：清除该 Key 在限速器里的失败计数，
		// 下次这个 Key 再入队时不受退避延迟影响。
		c.queue.Forget(key)
		return true
	}

	// 失败：AddRateLimited 将 Key 重新加入队列，并根据失败次数计算等待时间。
	// 第 N 次失败后的等待时间 = min(baseDelay * 2^(N-1), maxDelay)
	// 默认 baseDelay=5ms，maxDelay=1000s。
	c.queue.AddRateLimited(key)
	runtime.HandleError(fmt.Errorf("[Phase3] reconcile failed for %q, requeued: %w", key, err))
	return true
}

// reconcile 是业务核心：从缓存读最新状态，执行调谐逻辑。
// 这里始终处理的是"当前状态"，而非事件触发时的"快照状态"。
func (c *Phase3Controller) reconcile(_ context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		// key 格式错误，属于编程 bug，不重试，直接丢弃
		runtime.HandleError(fmt.Errorf("invalid key %q: %w", key, err))
		return nil
	}

	// 从 Indexer（内存缓存）读对象，不请求 API Server
	item, exists, err := c.podIndexer.GetByKey(key)
	if err != nil {
		return fmt.Errorf("error fetching %q from indexer: %w", key, err)
	}

	if !exists {
		// 对象不在缓存里 = 已被删除（List 同步完成后缓存是完整的）
		klog.Infof("[Phase3] Pod %s/%s deleted, skipping", namespace, name)
		return nil
	}

	pod := item.(*corev1.Pod)

	// 真正的业务逻辑放在这里
	// 这个函数运行在独立的 worker goroutine 里，
	// 无论执行多久，都不会影响 Informer 的 processLoop。
	klog.Infof("[Phase3] reconcile Pod %s/%s phase=%s",
		pod.Namespace, pod.Name, pod.Status.Phase)

	return nil
}

// ─── 演示：队列的去重特性 ─────────────────────────────────────────

// DemoDedup 演示 WorkQueue 的去重行为（仅用于测试和理解，非生产代码）。
//
// 假设 Pod "default/foo" 在 1ms 内连续变化了 3 次，
// 三个 UpdateFunc 回调分别调用 queue.Add("default/foo")。
// WorkQueue 的行为：
//   - 第一次 Add：Key 入队，队列：["default/foo"]
//   - 第二次 Add：Key 已在队列里，忽略
//   - 第三次 Add：同上，忽略
//   - worker Get()：取出 "default/foo"，此时 Indexer 里是最新的第三次状态
//   - worker 只调谐一次，处理的是最终状态，三次变化合并为一次处理
func (c *Phase3Controller) DemoDedup() {
	// 模拟 3 次快速入队同一 Key
	c.queue.Add("default/foo")
	c.queue.Add("default/foo") // 去重，忽略
	c.queue.Add("default/foo") // 去重，忽略

	klog.Infof("[Phase3][DemoDedup] queue length after 3 adds: %d (expected: 1)", c.queue.Len())
}

// DemoRateLimit 演示 WorkQueue 的限速/重试行为（仅用于理解，非生产代码）。
//
//	reconcile 失败 → AddRateLimited → 等待 5ms → 重新出队 → reconcile
//	reconcile 再次失败 → AddRateLimited → 等待 10ms → 重新出队 → reconcile
//	...（指数退避，上限 1000s）
func (c *Phase3Controller) DemoRateLimit() {
	// 第一次失败：下次出队需等 5ms
	c.queue.AddRateLimited("default/bar")
	// 第二次失败：下次出队需等 10ms
	c.queue.AddRateLimited("default/bar")
	// 调谐成功：清除失败计数
	c.queue.Forget("default/bar")
	klog.Info("[Phase3][DemoRateLimit] after Forget, default/bar rate limit reset")
}
