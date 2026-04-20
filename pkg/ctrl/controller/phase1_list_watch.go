/*
═══════════════════════════════════════════════════════════════════
阶段一：原始 List + Watch
═══════════════════════════════════════════════════════════════════

【这一阶段解决的问题】
如何让 Controller 感知到 Kubernetes 集群里资源的变化？

【核心原理：List-Watch 机制】
API Server 提供了两类 HTTP 接口，Controller 把它们配合使用：

 1. List：GET /api/v1/pods
    - 一次性返回当前所有 Pod 的快照
    - 响应里包含一个 resourceVersion（全局单调递增的时间戳）
    - Controller 用它来初始化本地状态

 2. Watch：GET /api/v1/pods?watch=true&resourceVersion=<rv>
    - 一条 HTTP/1.1 chunked 或 HTTP/2 的长连接
    - 服务端有新事件时主动往这条连接里写 JSON 块（chunk）
    - 每个 chunk 是一个 WatchEvent：{type: ADDED|MODIFIED|DELETED, object: Pod}
    - 客户端持续读取，实现"推送"而非"轮询"

【两者配合的时序】

	┌──────────────────────────────────────────────────────────────┐
	│  Controller                           API Server             │
	│      │                                     │                 │
	│      │──── List /api/v1/pods ─────────────→│                 │
	│      │←─── {items:[...], resourceVersion:X}│                 │
	│      │                                     │                 │
	│      │  （处理全量数据，记住 resourceVersion X）               │
	│      │                                     │                 │
	│      │──── Watch ?resourceVersion=X ──────→│（长连接建立）    │
	│      │←─── ADDED   pod/foo ────────────────│                 │
	│      │←─── MODIFIED pod/bar ───────────────│                 │
	│      │←─── DELETED  pod/baz ───────────────│（持续接收）     │
	└──────────────────────────────────────────────────────────────┘

	resourceVersion 的作用：
	告诉 API Server "从这个版本之后的事件开始推送给我"，
	保证 List 和 Watch 之间不遗漏任何事件，也不重复处理。

【这一阶段的局限性】
  - 没有本地缓存：每次需要读资源都要请求 API Server
  - 没有断线重连逻辑（Watch 连接断开后需要重新 List+Watch）
  - 没有事件处理的解耦（事件回调直接阻塞 Watch 循环）
    这些问题在 Phase 2（Informer）里全部解决。

═══════════════════════════════════════════════════════════════════
*/
package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// Phase1Controller 直接使用 List + Watch 原始接口感知 Pod 变化。
// 没有缓存、没有自动重连，是最底层的实现，用于理解 Informer 的基础。
type Phase1Controller struct {
	clientset kubernetes.Interface
}

func NewPhase1Controller(clientset kubernetes.Interface) *Phase1Controller {
	return &Phase1Controller{clientset: clientset}
}

// Run 是 Phase1 的主循环：先 List，再 Watch，循环处理事件。
func (c *Phase1Controller) Run(ctx context.Context) error {
	klog.Info("[Phase1] 开始 List 全量 Pod")

	// ── 第一步：List 全量资源 ──────────────────────────────────────
	// ListOptions{} 表示列出所有命名空间下的所有 Pod。
	// 返回的 PodList 里有一个 ResourceVersion 字段，这是当前集群状态的"时间戳"，
	// 后续 Watch 时需要带上它，告诉 API Server "从这里之后的变化推给我"。
	podList, err := c.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list pods failed: %w", err)
	}

	klog.Infof("[Phase1] List 完成，当前共 %d 个 Pod，resourceVersion=%s",
		len(podList.Items), podList.ResourceVersion)

	// 处理全量快照（模拟"初始化本地状态"）
	for i := range podList.Items {
		c.handleEvent(watch.Added, &podList.Items[i])
	}

	// ── 第二步：从上次 List 的 resourceVersion 开始 Watch ──────────
	// resourceVersion 保证连续性：List 返回 rv=X，Watch 从 rv=X 开始，
	// 中间没有任何事件被遗漏。
	klog.Infof("[Phase1] 开始 Watch，从 resourceVersion=%s 继续", podList.ResourceVersion)

	watcher, err := c.clientset.CoreV1().Pods("").Watch(ctx, metav1.ListOptions{
		// 关键：用 List 返回的 ResourceVersion，确保 List→Watch 无缝衔接
		ResourceVersion: podList.ResourceVersion,
	})
	if err != nil {
		return fmt.Errorf("watch pods failed: %w", err)
	}
	defer watcher.Stop()

	// ── 第三步：持续消费 Watch 事件流 ─────────────────────────────
	// watcher.ResultChan() 是一个 channel，API Server 每推送一个事件，
	// 就往这个 channel 里写一条 watch.Event。
	// 当连接断开（网络故障、API Server 重启）或 ctx 取消时，channel 被关闭。
	for {
		select {
		case event, ok := <-watcher.ResultChan():
			if !ok {
				// channel 关闭：Watch 连接已断开。
				// 生产场景需要在这里重新 List+Watch（Phase 2 的 Informer 自动处理这个）。
				klog.Warning("[Phase1] Watch channel closed，连接断开，需要重新 List+Watch")
				return nil
			}

			// 事件类型：watch.Added / watch.Modified / watch.Deleted / watch.Error
			if event.Type == watch.Error {
				// API Server 返回了错误事件（如 resourceVersion 过期，需要重新 List）
				klog.Errorf("[Phase1] Watch error event: %v", event.Object)
				return fmt.Errorf("watch error event received")
			}

			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				klog.Warningf("[Phase1] unexpected object type: %T", event.Object)
				continue
			}

			// 注意：这里直接在 Watch 循环里处理事件。
			// 如果 handleEvent 耗时很长，会阻塞整个 Watch 循环，
			// 导致后续事件积压在 channel 里，甚至被丢弃。
			// ——这是 Phase1 最大的问题，Phase3 用 WorkQueue 解决。
			c.handleEvent(event.Type, pod)

		case <-ctx.Done():
			klog.Info("[Phase1] context cancelled, stopping watch")
			return nil
		}
	}
}

// handleEvent 处理单个 Pod 事件（Phase1 直接在 Watch 循环里调用）。
func (c *Phase1Controller) handleEvent(eventType watch.EventType, pod *corev1.Pod) {
	klog.Infof("[Phase1] %s Pod: %s/%s (phase=%s)",
		eventType, pod.Namespace, pod.Name, pod.Status.Phase)
}
