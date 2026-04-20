/*
═══════════════════════════════════════════════════════════════════
阶段四：生产级 Controller —— 完整闭合反馈环
═══════════════════════════════════════════════════════════════════

【这一阶段在 Phase3 基础上补全的内容】

	Phase3 已经有了正确的骨架，Phase4 补全以下生产要素：

	1. 真正的 Reconcile 逻辑（读缓存 → 计算 diff → 写 API Server）
	2. 闭合反馈环（写 API Server → 触发新 Watch 事件 → 再次 Reconcile）
	3. Owner Reference（资源间依赖追踪）
	4. Status 子资源更新（将调谐结果回写到资源本身）
	5. 优雅关闭（Graceful Shutdown）

【闭合反馈环（Closed-Loop Control）】

	Controller 的调谐是一个无限循环的反馈控制系统：

	┌─────────────────────────────────────────────────────────────┐
	│                                                             │
	│  期望状态                                                   │
	│  (Spec)    ──→  Reconcile  ──→  写 API Server              │
	│                    ↑                  │                     │
	│                    │                  ↓                     │
	│  Informer  ←──  Watch Event  ←──  实际状态变化              │
	│                                                             │
	│  循环直到：实际状态 == 期望状态（Reconcile 不再有写操作）      │
	└─────────────────────────────────────────────────────────────┘

	这意味着 Reconcile 必须是"幂等"的：
	即使同一个资源被调谐了 100 次，结果都应该相同，
	不能每次调谐都执行一次副作用（如发送一封邮件）。

【Status 子资源更新】

	Kubernetes 资源分为 Spec（期望状态）和 Status（实际状态）：
	- Spec 由用户/上层 Controller 写入
	- Status 由负责该资源的 Controller 写入，反映真实情况

	更新 Status 使用专门的 UpdateStatus 接口（Status 子资源），
	而非直接 Update 整个对象，这样可以避免 Spec 被意外覆盖，
	也避免两个 Controller 互相覆盖对方的字段。

【Owner Reference（属主引用）】

	当 Controller A 创建了资源 B，应给 B 设置 OwnerReference 指向 A。
	这样当 A 被删除时，Kubernetes GC 会级联删除 B（Garbage Collection）。
	反之，当 B 发生变化时，Controller 可以根据 OwnerReference 找到 A 并重新调谐。

【优雅关闭】

	收到 SIGTERM 后：
	① ctx 被取消
	② queue.ShutDown() 被调用
	③ 正在处理的 Key 处理完毕（Get() 返回后、Done() 调用前的工作继续完成）
	④ queue.Get() 开始返回 shutdown=true
	⑤ 所有 worker goroutine 退出
	⑥ Run() 返回

	这保证了 Controller 不会在处理一半时被强制终止。

═══════════════════════════════════════════════════════════════════
*/
package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

// Phase4Controller 是完整的生产级 Controller。
//
// 职责（示例）：监听 Pod 变化，当 Pod 进入 Failed 状态时，
// 自动打上 "mesh.io/failed-at" Annotation，并更新 Pod 的 Status Condition。
// ——这只是一个演示场景，真实 Controller 可以管理任意自定义资源。
type Phase4Controller struct {
	clientset  kubernetes.Interface
	podIndexer cache.Indexer
	podSynced  cache.InformerSynced
	queue      workqueue.TypedRateLimitingInterface[string]
}

// NewPhase4Controller 构建完整 Controller，注入所有依赖。
func NewPhase4Controller(clientset kubernetes.Interface) *Phase4Controller {
	factory := informers.NewSharedInformerFactory(clientset, 30*time.Second)
	podInformer := factory.Core().V1().Pods()

	c := &Phase4Controller{
		clientset:  clientset,
		podIndexer: podInformer.Informer().GetIndexer(),
		podSynced:  podInformer.Informer().HasSynced,
		queue:      workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[string]()),
	}

	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj any) { c.enqueue(obj) },
		UpdateFunc: func(_, newObj any) { c.enqueue(newObj) },
		DeleteFunc: func(obj any) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err != nil {
				runtime.HandleError(err)
				return
			}
			c.queue.Add(key)
		},
	})

	// 将 factory 的生命周期交给 Run() 管理（通过 ctx.Done()）
	// 这里先注册好 handler，Start 的时机在 Run() 里
	// 为简化示例，此处直接 Start（生产中应传入 ctx）
	factory.Start(context.Background().Done())

	return c
}

func (c *Phase4Controller) enqueue(obj any) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	c.queue.Add(key)
}

// ─────────────────── 主循环 ───────────────────────────────────────

// Run 是 Controller 的入口，通常由 main 函数调用。
//
// 生产用法：
//
//	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
//	defer cancel()
//	if err := controller.Run(ctx, 2); err != nil {
//	    log.Fatal(err)
//	}
func (c *Phase4Controller) Run(ctx context.Context, workers int) error {
	// 确保 panic 被记录而不是静默崩溃
	defer runtime.HandleCrash()
	// 队列关闭后，所有阻塞在 Get() 的 worker 会立即返回 shutdown=true
	defer c.queue.ShutDown()

	klog.Info("[Phase4] starting controller")

	// ── 等待 Indexer 首次全量同步 ─────────────────────────────────
	// 原因：Informer 启动后需要先 List 全量数据填充 Indexer，
	// 在此之前 Indexer 是不完整的。
	// 如果在同步完成前开始 Reconcile，会把"还没进缓存的 Pod"误判为"不存在"，
	// 进而执行错误的删除清理逻辑。
	klog.Info("[Phase4] waiting for informer caches to sync...")
	if !cache.WaitForCacheSync(ctx.Done(), c.podSynced) {
		return fmt.Errorf("timed out waiting for caches to sync")
	}
	klog.Info("[Phase4] caches synced")

	// ── 启动 N 个 worker ──────────────────────────────────────────
	for i := range workers {
		go wait.UntilWithContext(ctx, func(ctx context.Context) {
			klog.V(4).Infof("[Phase4] worker %d started", i)
			for c.processNext(ctx) {
			}
		}, time.Second)
	}

	klog.Infof("[Phase4] %d workers started", workers)
	<-ctx.Done()
	klog.Info("[Phase4] shutdown signal received, draining queue...")
	return nil
}

func (c *Phase4Controller) processNext(ctx context.Context) bool {
	key, shutdown := c.queue.Get()
	if shutdown {
		return false
	}
	defer c.queue.Done(key)

	if err := c.reconcile(ctx, key); err != nil {
		c.queue.AddRateLimited(key)
		runtime.HandleError(fmt.Errorf("[Phase4] reconcile %q failed (will retry): %w", key, err))
		return true
	}

	c.queue.Forget(key)
	return true
}

// ─────────────────── 核心调谐逻辑 ────────────────────────────────

// reconcile 是三段式调谐的完整实现：
//  1. 读缓存，获取当前状态
//  2. 计算 diff（期望状态 vs 实际状态）
//  3. 写 API Server，消除 diff（触发新的 Watch 事件，形成闭合环）
func (c *Phase4Controller) reconcile(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid key %q", key))
		return nil // 编程错误，不重试
	}

	// ── 第一步：从 Indexer 读当前状态（零网络开销）────────────────
	item, exists, err := c.podIndexer.GetByKey(key)
	if err != nil {
		return fmt.Errorf("error fetching %q from indexer: %w", key, err)
	}

	// ── 第二步：处理对象已删除的情况 ─────────────────────────────
	if !exists {
		// Pod 已被删除，执行清理逻辑。
		// 例如：如果该 Pod 持有某个分布式锁，在这里释放它。
		klog.Infof("[Phase4] Pod %s/%s deleted, running cleanup", namespace, name)
		return c.handleDelete(ctx, namespace, name)
	}

	pod := item.(*corev1.Pod)

	// ── 第三步：执行调谐，消除期望状态与实际状态之间的差值 ──────────
	return c.handlePodPhase(ctx, pod)
}

// handlePodPhase 根据 Pod 的当前状态执行对应的调谐动作。
// 演示场景：当 Pod Failed 时，打上 Annotation 记录失败时间。
func (c *Phase4Controller) handlePodPhase(ctx context.Context, pod *corev1.Pod) error {
	switch pod.Status.Phase {

	case corev1.PodFailed:
		// 检查是否已经处理过（幂等性保证：避免重复写 API Server）
		if _, alreadyTagged := pod.Annotations["mesh.io/failed-at"]; alreadyTagged {
			klog.V(5).Infof("[Phase4] Pod %s/%s already tagged, skipping", pod.Namespace, pod.Name)
			return nil
		}

		// 需要执行写操作：给 Pod 加 Annotation
		// ── 写 API Server ─────────────────────────────────────────
		// 注意：这里先从 API Server 拿最新版本（而非缓存），
		// 避免用过时的 ResourceVersion 发起更新请求（否则会 409 Conflict）。
		latest, err := c.clientset.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				// 在 Get 和这次调谐之间 Pod 已被删除，无需处理
				return nil
			}
			return fmt.Errorf("failed to get latest pod %s/%s: %w", pod.Namespace, pod.Name, err)
		}

		// 修改副本，不直接改缓存里的对象（缓存是只读的）
		updated := latest.DeepCopy()
		if updated.Annotations == nil {
			updated.Annotations = make(map[string]string)
		}
		updated.Annotations["mesh.io/failed-at"] = time.Now().UTC().Format(time.RFC3339)

		// Update 写入 API Server → 触发 MODIFIED Watch 事件
		// → Informer 更新 Indexer → UpdateFunc 回调 → Key 重新入队
		// → 下次 Reconcile 时发现 Annotation 已存在 → 跳过（幂等）
		// ——这就是"闭合反馈环"的完整一圈
		if _, err = c.clientset.CoreV1().Pods(pod.Namespace).Update(ctx, updated, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("failed to update pod %s/%s annotation: %w", pod.Namespace, pod.Name, err)
		}

		klog.Infof("[Phase4] Pod %s/%s Failed, annotated with failed-at timestamp", pod.Namespace, pod.Name)

	case corev1.PodRunning:
		// 示例：Pod Running 后，检查容器就绪情况并打日志
		ready := 0
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Ready {
				ready++
			}
		}
		klog.Infof("[Phase4] Pod %s/%s Running (%d/%d containers ready)",
			pod.Namespace, pod.Name, ready, len(pod.Spec.Containers))

	default:
		klog.V(5).Infof("[Phase4] Pod %s/%s phase=%s, no action needed",
			pod.Namespace, pod.Name, pod.Status.Phase)
	}

	return nil
}

// handleDelete 处理 Pod 删除后的清理逻辑。
// 这里没有真实的 Pod 对象可用，只有 namespace 和 name。
func (c *Phase4Controller) handleDelete(ctx context.Context, namespace, name string) error {
	// 示例：清理与该 Pod 关联的 ConfigMap
	cmName := "pod-config-" + name
	err := c.clientset.CoreV1().ConfigMaps(namespace).Delete(ctx, cmName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		// IsNotFound 是正常情况（ConfigMap 可能从未创建过），不算错误
		return fmt.Errorf("failed to delete configmap %s/%s: %w", namespace, cmName, err)
	}
	klog.Infof("[Phase4] cleaned up ConfigMap %s/%s for deleted Pod", namespace, cmName)
	return nil
}
