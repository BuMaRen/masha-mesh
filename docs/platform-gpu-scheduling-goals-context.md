# 平台层 GPU 调度目标细化（AI 上下文版）

## 1. 文档目的

本文件把“平台层 GPU 调度可实现功能”细化为可执行需求卡片，粒度达到可直接作为 AI 需求上下文输入。

适用范围：
- Kubernetes 平台调度与治理层
- Go 控制面（Webhook、Controller、Policy、Metrics）
- 不含 CUDA Kernel、NCCL 内核、驱动开发

## 2. 统一需求卡片模板（AI 可解析）

每个目标均按以下结构描述：

- GoalID：唯一标识
- GoalName：目标名称
- Problem：要解决的问题
- BusinessValue：业务价值
- Actors：参与角色
- Inputs：输入信号（对象/标签/指标）
- DecisionLogic：决策逻辑
- Outputs：输出动作
- Observability：观测指标与日志
- FailureModes：失败模式
- Guardrails：安全护栏
- Acceptance：验收标准
- NonGoals：非目标

---

## 3. 目标细化清单

### GoalID: G1
### GoalName: 任务准入控制（GPU 作业门禁）

- Problem：任务未声明 GPU 资源、租户归属、优先级时直接入场，导致集群不可控。
- BusinessValue：阻断低质量任务，降低事故率。
- Actors：平台管理员、业务团队、SRE。
- Inputs：
  - Pod/Job AdmissionReview
  - labels: owner, workload-type, queue
  - resources.limits["nvidia.com/gpu"]
- DecisionLogic：
  1. 是否声明 GPU 资源。
  2. 是否带 owner 标签。
  3. 是否在允许队列集合中。
  4. 是否命中黑名单镜像策略。
- Outputs：allow/reject + rejection reason。
- Observability：
  - gpu_admission_requests_total{result,reason}
  - 审计日志字段: request_uid, owner, queue, reject_reason
- FailureModes：Webhook 超时、策略加载失败。
- Guardrails：策略失败默认 fail-close 或 fail-open 需可配置。
- Acceptance：不合规任务 100% 被拦截；误杀率低于设定阈值。
- NonGoals：不负责具体节点绑定。

### GoalID: G2
### GoalName: 队列与优先级调度

- Problem：关键任务和实验任务争抢资源，导致关键路径延迟。
- BusinessValue：保障高优任务 SLA。
- Actors：平台管理员、业务 owner。
- Inputs：
  - queue: realtime/train/batch
  - priorityClass
  - 当前排队长度和等待时长
- DecisionLogic：
  1. 按队列优先级排序。
  2. 同队列内按提交时间或公平份额排序。
  3. 可配置抢占策略。
- Outputs：调度顺序、抢占动作建议。
- Observability：
  - gpu_queue_wait_seconds{queue,p50,p95}
  - gpu_queue_depth{queue}
- FailureModes：优先级配置冲突。
- Guardrails：关键队列必须保底容量。
- Acceptance：关键队列 P95 等待时长明显下降。
- NonGoals：不实现底层 scheduler 重写。

### GoalID: G3
### GoalName: 配额与多租户隔离

- Problem：单一租户占满 GPU，其他团队饥饿。
- BusinessValue：资源公平和成本可控。
- Actors：平台管理员、财务、团队负责人。
- Inputs：
  - tenant quota CRD
  - namespace/owner 资源使用量
- DecisionLogic：
  1. 校验租户 hard quota。
  2. 超限任务拒绝或排队。
  3. 支持 burst quota 短时放量。
- Outputs：准入结果、配额告警。
- Observability：
  - gpu_quota_usage_ratio{tenant}
  - gpu_quota_rejections_total{tenant}
- FailureModes：计量延迟导致误判。
- Guardrails：设置采样窗口，避免瞬时抖动误拒绝。
- Acceptance：无租户可长期独占全部资源。
- NonGoals：不处理财务结算系统对账。

### GoalID: G4
### GoalName: 公平共享与防饥饿

- Problem：小团队任务长期排队。
- BusinessValue：提升组织整体吞吐和满意度。
- Inputs：历史使用量、当前配额、等待时长。
- DecisionLogic：fair-share 评分 = 历史占用惩罚 + 等待时长补偿。
- Outputs：公平调度权重。
- Observability：
  - gpu_fairshare_score{tenant}
  - gpu_starvation_events_total{tenant}
- FailureModes：评分算法不稳定。
- Guardrails：评分变化速率限制。
- Acceptance：长尾等待任务比例下降。
- NonGoals：不做学术最优算法求解。

### GoalID: G5
### GoalName: 拓扑感知调度（平台层）

- Problem：跨 NUMA、跨 NVLink 拓扑导致性能抖动。
- BusinessValue：提高训练吞吐与稳定性。
- Inputs：
  - 节点 GPU 拓扑标签（由 exporter 或 node-feature-discovery 提供）
  - 任务对卡数和拓扑偏好
- DecisionLogic：优先选择拓扑评分高的节点。
- Outputs：调度建议标签或约束。
- Observability：
  - gpu_topology_match_ratio
  - training_step_time_seconds
- FailureModes：拓扑数据过期。
- Guardrails：无拓扑数据时回退普通策略。
- Acceptance：同类任务吞吐提升可观测。
- NonGoals：不实现底层硬件采集驱动。

### GoalID: G6
### GoalName: 资源碎片治理

- Problem：碎片化导致大任务无法调度。
- BusinessValue：提升有效利用率。
- Inputs：节点卡位分布、待调度任务规格。
- DecisionLogic：
  1. 装箱优先/打散优先策略可切换。
  2. 对大任务保留连续资源窗口。
- Outputs：资源重排建议、调度偏好。
- Observability：
  - gpu_fragmentation_index
  - large_job_pending_seconds
- FailureModes：过度重排影响短作业。
- Guardrails：重排频率和影响面限制。
- Acceptance：大任务排队时长下降。
- NonGoals：不进行在线迁移 GPU 进程。

### GoalID: G7
### GoalName: GPU 健康感知与故障绕障

- Problem：坏卡或异常节点反复接单。
- BusinessValue：降低任务失败率。
- Inputs：GPU 健康指标（ECC/XID/温度/利用率异常）。
- DecisionLogic：
  1. 异常评分超过阈值则标记节点不可调度。
  2. 自动打 taint 或下调权重。
- Outputs：节点隔离动作。
- Observability：
  - gpu_node_unhealthy_total{reason}
  - gpu_task_failures_total{node,reason}
- FailureModes：误判导致容量下降。
- Guardrails：多信号确认后再隔离。
- Acceptance：健康异常节点上的失败任务占比下降。
- NonGoals：不修复硬件本身。

### GoalID: G8
### GoalName: 弹性扩缩与容量联动

- Problem：排队过长但集群未扩容，或低负载时成本过高。
- BusinessValue：成本与时延平衡。
- Inputs：队列长度、等待时长、GPU 利用率。
- DecisionLogic：
  - 扩容触发: queue_wait_p95 > T1 且 utilization > T2
  - 缩容触发: queue_depth 低且 idle 时长 > T3
- Outputs：扩缩容建议或事件。
- Observability：
  - gpu_autoscale_actions_total{type}
  - gpu_cluster_utilization
- FailureModes：抖动扩缩容。
- Guardrails：冷却时间、最小/最大节点边界。
- Acceptance：单位任务成本下降且 SLA 不恶化。
- NonGoals：不绑定某云厂商 autoscaler 实现。

### GoalID: G9
### GoalName: 抢占与回收

- Problem：紧急任务无法及时获得资源。
- BusinessValue：关键业务优先保障。
- Inputs：任务优先级、可中断标记、检查点能力。
- DecisionLogic：
  1. 仅可中断任务可被抢占。
  2. 抢占前校验业务白名单。
- Outputs：驱逐事件、重排队事件。
- Observability：
  - gpu_preemptions_total{queue,reason}
  - preempted_job_recovery_seconds
- FailureModes：频繁抢占导致吞吐下降。
- Guardrails：每租户抢占上限。
- Acceptance：紧急任务启动时延显著降低。
- NonGoals：不保证无损中断恢复（取决于业务应用）。

### GoalID: G10
### GoalName: 训练与推理混部治理

- Problem：训练任务挤压在线推理，导致延迟超标。
- BusinessValue：在线服务稳定。
- Inputs：workload-type、QoS class、实时延迟指标。
- DecisionLogic：
  1. 推理服务设保底资源。
  2. 训练任务可被限流或降权。
- Outputs：队列限流、优先级调整。
- Observability：
  - inference_latency_seconds{p95,p99}
  - mixed_cluster_contention_events_total
- FailureModes：策略冲突。
- Guardrails：推理 SLA 优先级高于离线训练。
- Acceptance：推理 P99 稳定在目标阈值内。
- NonGoals：不做模型服务框架实现。

### GoalID: G11
### GoalName: 可观测与审计闭环

- Problem：无法解释“为什么这个任务被拒绝或延迟”。
- BusinessValue：提升排障效率与合规性。
- Inputs：Admission、Reconcile、Queue、Node 事件。
- DecisionLogic：统一事件模型与 trace id 关联。
- Outputs：审计日志、可追踪链路。
- Observability：
  - scheduling_decision_logs_total
  - decision_explainability_ratio
- FailureModes：日志字段不统一。
- Guardrails：强制公共字段 schema。
- Acceptance：任一故障可在 10 分钟内定位原因。
- NonGoals：不替代外部 SIEM。

### GoalID: G12
### GoalName: 成本归因与预算控制

- Problem：GPU 成本归因不清，预算失控。
- BusinessValue：支持经营决策。
- Inputs：GPU 时长、租户标签、任务类型。
- DecisionLogic：按 tenant/project/model 聚合成本。
- Outputs：预算告警、超额阻断策略。
- Observability：
  - gpu_cost_estimate_total{tenant,project}
  - budget_alerts_total{tenant}
- FailureModes：标签缺失导致归因偏差。
- Guardrails：缺标签任务拒绝或进入隔离队列。
- Acceptance：能输出周/月成本 TopN 归因。
- NonGoals：不做财务系统税费逻辑。

### GoalID: G13
### GoalName: 策略即配置（Policy as Code）

- Problem：策略散落在代码中，难灰度和回滚。
- BusinessValue：提升变更安全性。
- Inputs：Policy CRD、版本号、生效范围。
- DecisionLogic：
  1. 策略版本化。
  2. 支持 dry-run/audit/enforce。
- Outputs：策略评估结果、命中详情。
- Observability：
  - policy_evaluations_total{policy,result}
  - policy_rollbacks_total
- FailureModes：策略发布后大面积拒绝。
- Guardrails：分阶段灰度 + 自动回滚阈值。
- Acceptance：策略发布可回放、可追溯。
- NonGoals：不实现通用策略语言编译器。

### GoalID: G14
### GoalName: 生态组件集成（Volcano/Kueue/Autoscaler）

- Problem：平台能力孤立，无法形成端到端闭环。
- BusinessValue：复用社区生态，降低维护成本。
- Inputs：队列状态、工作负载状态、扩缩容信号。
- DecisionLogic：通过标准 CRD/注解与外部组件协作。
- Outputs：联动调度与扩缩容。
- Observability：
  - integration_errors_total{component}
  - integration_latency_seconds{component}
- FailureModes：外部组件版本不兼容。
- Guardrails：适配层版本探测与回退。
- Acceptance：在不改核心调度器前提下完成能力联动。
- NonGoals：不 fork 外部项目主线代码。

---

## 4. 推荐 MVP 子集（先做 5 项）

优先顺序：G1 -> G3 -> G7 -> G11 -> G12

- 第一阶段：可控（准入、配额）
- 第二阶段：可用（健康绕障）
- 第三阶段：可解释（审计、成本）

## 5. 可直接喂给 AI 的上下文片段（示例）

```yaml
system_context:
  project: masha-mesh
  objective: build platform-layer GPU scheduling governance
  constraints:
    - no CUDA kernel development
    - control-plane in Go
  priorities:
    - admission control
    - multi-tenant quota
    - gpu health aware scheduling
    - observability and audit
    - cost attribution

current_capabilities:
  - webhook mutation/admission skeleton
  - reconciler event-driven loop
  - metrics endpoint

gap_to_fill:
  - gpu policy enforcement
  - quota CRD and evaluator
  - health signal ingestion
  - unified decision logs
  - tenant-level cost metrics
```

## 6. 需求澄清问题（实施前）

1. 目标场景以训练为主还是推理为主。
2. 多租户隔离以 namespace 还是 owner label 为主键。
3. 拒绝策略默认 fail-close 还是 fail-open。
4. 是否允许任务抢占。
5. 成本统计需要估算还是账单对齐。
