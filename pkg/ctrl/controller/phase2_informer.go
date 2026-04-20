/*
═══════════════════════════════════════════════════════════════════
阶段二：SharedIndexInformer —— 本地缓存 + 自动重连
═══════════════════════════════════════════════════════════════════

【这一阶段解决的问题】
Phase1 有三个核心缺陷，Informer 全部解决：

 1. 无本地缓存
    - Phase1：每次查 Pod 都要请求 API Server（高延迟、高压力）
    - Informer：在内存里维护一份 Indexer（本地缓存），读操作直接走缓存

 2. 无断线重连
    - Phase1：Watch 断开后必须手动重新 List+Watch
    - Informer：内部有 Reflector 组件，自动完成重连和 ResourceVersion 管理

 3. 无 List→Watch 保证
    - Phase1：开发者需要自己正确传递 ResourceVersion
    - Informer：Reflector 内部保证 List 和 Watch 之间零间隔、不丢事件

【Informer 内部结构】

	┌─────────────────────────────────────────────────────────────┐
	│                    SharedIndexInformer                      │
	│                                                             │
	│  ┌──────────────┐     ┌──────────────┐     ┌───────────┐  │
	│  │   Reflector  │────→│  DeltaFIFO   │────→│  Indexer  │  │
	│  │              │     │  (事件队列)   │     │ (本地缓存) │  │
	│  │ List + Watch │     └──────┬───────┘     └───────────┘  │
	│  │ 自动重连      │            │                             │
	│  └──────────────┘            │ processLoop                 │
	│                              ↓                             │
	│                    ResourceEventHandler                     │
	│                    (AddFunc/UpdateFunc/DeleteFunc)          │
	└─────────────────────────────────────────────────────────────┘

	组件职责：
	- Reflector：执行 List+Watch，将收到的事件写入 DeltaFIFO 队列
	- DeltaFIFO：有序事件队列，同一对象的多次变化按顺序排列
	- Indexer：线程安全的本地缓存，支持按字段/标签索引快速查找
	- processLoop：从 DeltaFIFO 取事件 → 更新 Indexer → 触发 EventHandler

【首次同步（Full Resync / List Sync）】

	Informer 启动时必须先完成一次全量同步，之后缓存才是完整的：

	① Reflector.List() → 拉取所有 Pod → 写入 DeltaFIFO（Replace 操作）
	② processLoop 消费 DeltaFIFO，依次触发 AddFunc 回调 + 填充 Indexer
	③ HasSynced() 返回 true
	④ 之后 Reflector.Watch() 开始，只处理增量事件

	在 ③ 之前不能对缓存做任何查询，否则会因缓存不完整产生误判。
	（Phase4 用 cache.WaitForCacheSync 等待这一步完成）

【这一阶段的局限性】

	EventHandler 回调（AddFunc 等）是在 Informer 内部的 processLoop goroutine
	里同步调用的。如果回调耗时过长，会阻塞 processLoop，导致：
	- 后续事件的 Indexer 更新被延迟
	- 新事件的回调被积压
	这就是 Phase3 引入 WorkQueue 的动机。

═══════════════════════════════════════════════════════════════════
*/
package controller

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// Phase2Controller 使用 SharedIndexInformer 感知 Pod 变化。
// 相比 Phase1，获得了：本地缓存、自动重连、List→Watch 无缝衔接。
// 但事件仍然在 Informer 的 processLoop goroutine 里同步处理。
type Phase2Controller struct {
	clientset kubernetes.Interface
}

func NewPhase2Controller(clientset kubernetes.Interface) *Phase2Controller {
	return &Phase2Controller{clientset: clientset}
}

// Run 启动 SharedInformerFactory 并注册事件处理器。
func (c *Phase2Controller) Run(ctx context.Context) {
	// SharedInformerFactory 是 Informer 的工厂。
	// 第二个参数 resyncPeriod：定期对所有对象强制触发一次 UpdateFunc 回调（即使没有实际变化），
	// 用于处理因网络异常等原因导致的事件丢失。
	// 设置为 0 表示禁用定期全量 Resync（Watch 足够可靠时可以关闭）。
	factory := informers.NewSharedInformerFactory(c.clientset, 30*time.Second)

	// 从 factory 获取 Pod 的 SharedIndexInformer。
	// factory 保证：同一进程内对 Pod 资源只建立一条 Watch 连接，
	// 即使多处代码都调用 factory.Core().V1().Pods()，底层共享同一个 Informer。
	podInformer := factory.Core().V1().Pods().Informer()

	// 注册事件处理器。
	// 同一个 Informer 可以注册多个 EventHandler，它们都会收到同一份事件通知。
	// 这也是 "Shared"InformerFactory 名字的含义：多个 handler 共享一个 Watch 连接。
	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		// AddFunc：Pod 被新建，或 Informer 首次 List 时触发（每个存量 Pod 都触发一次）
		AddFunc: func(obj any) {
			pod := obj.(*corev1.Pod)
			klog.Infof("[Phase2][AddFunc] Pod added: %s/%s", pod.Namespace, pod.Name)

			// ⚠️ 问题所在：
			// 这里如果做耗时操作（如调用外部 API、等待 IO），
			// 会阻塞 processLoop，导致 Indexer 更新和其他回调被延迟。
			// Phase3 的解决方案：回调里只入队 Key，不做任何处理。
			c.processDirectly(pod)
		},

		// UpdateFunc：Pod 的任何字段发生变化时触发（包括 Status、Annotations 等）
		// oldObj 是变化前的对象（从 Indexer 快照复制），newObj 是变化后的对象。
		UpdateFunc: func(oldObj, newObj any) {
			oldPod := oldObj.(*corev1.Pod)
			newPod := newObj.(*corev1.Pod)

			// 最佳实践：用 ResourceVersion 判断对象是否真的有变化。
			// Resync 会触发 UpdateFunc，但 oldObj 和 newObj 的 ResourceVersion 相同，
			// 说明是周期性 Resync 而非真实变更，可以跳过处理节省资源。
			if oldPod.ResourceVersion == newPod.ResourceVersion {
				klog.V(5).Infof("[Phase2][UpdateFunc] Pod %s/%s resync, skipping",
					newPod.Namespace, newPod.Name)
				return
			}

			klog.Infof("[Phase2][UpdateFunc] Pod updated: %s/%s (%s → %s)",
				newPod.Namespace, newPod.Name,
				oldPod.Status.Phase, newPod.Status.Phase)
			c.processDirectly(newPod)
		},

		// DeleteFunc：Pod 被删除时触发。
		// ⚠️ 特殊情况：obj 可能是 cache.DeletedFinalStateUnknown 类型！
		// 当 Informer 因重启或网络中断错过删除事件时，API Server 会发送
		// DeletedFinalStateUnknown 来告知"这个对象可能已被删除，但我不确定"。
		// 必须用类型断言正确处理，否则会 panic。
		DeleteFunc: func(obj any) {
			// 先尝试直接断言为 Pod
			pod, ok := obj.(*corev1.Pod)
			if !ok {
				// 不是 Pod，尝试从 DeletedFinalStateUnknown 中取出
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					runtime.HandleError(nil) // 记录意外类型，继续运行
					return
				}
				pod, ok = tombstone.Obj.(*corev1.Pod)
				if !ok {
					runtime.HandleError(nil)
					return
				}
				klog.Warningf("[Phase2][DeleteFunc] tombstone: Pod %s/%s (informer missed delete event)",
					pod.Namespace, pod.Name)
			}

			klog.Infof("[Phase2][DeleteFunc] Pod deleted: %s/%s", pod.Namespace, pod.Name)
			c.processDirectly(pod)
		},
	})

	// 演示如何从本地缓存（Indexer）查询 Pod，不经过 API Server
	go c.demoReadFromCache(ctx, podInformer)

	// 启动所有通过 factory 创建的 Informer（开始 List+Watch）
	factory.Start(ctx.Done())

	// 阻塞等待至少一次全量同步完成（Indexer 初始化完毕后才安全查询）
	// 这里仅打日志演示，Phase4 里会用 cache.WaitForCacheSync 在 Run 入口强制等待
	factory.WaitForCacheSync(ctx.Done())
	klog.Info("[Phase2] informer cache synced, ready to serve")

	<-ctx.Done()
	klog.Info("[Phase2] shutting down")
}

// processDirectly 是 Phase2 里直接在回调里调用的处理函数。
// 在真实场景里，如果这里有任何 IO 或慢操作，整个 Informer 都会被拖慢。
func (c *Phase2Controller) processDirectly(pod *corev1.Pod) {
	// 模拟处理逻辑（仅打印，无副作用）
	klog.V(4).Infof("[Phase2] processing pod %s/%s phase=%s",
		pod.Namespace, pod.Name, pod.Status.Phase)
}

// demoReadFromCache 演示在缓存 sync 完成后如何从 Indexer 读数据，
// 而无需请求 API Server。
func (c *Phase2Controller) demoReadFromCache(ctx context.Context, informer cache.SharedIndexInformer) {
	// 等待缓存初始化完毕
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		return
	}

	// 从 Indexer（本地缓存）列出所有 Pod，O(1) 内存操作，零网络开销
	for _, obj := range informer.GetStore().List() {
		pod := obj.(*corev1.Pod)
		klog.V(4).Infof("[Phase2][Cache] pod: %s/%s", pod.Namespace, pod.Name)
	}
}
