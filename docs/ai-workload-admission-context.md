# AI 工作负载准入体系（AI 上下文版）

## 1. 文档目的

本文件定义 masha-mesh 的 AI 工作负载准入需求，目标是把准入从“是否注入 sidecar”升级为“策略化治理入口”。

使用方式：
- 作为产品需求说明（PRD-lite）
- 作为实现前的策略契约
- 作为 AI 代码助手的上下文输入

## 2. 准入范围与边界

### 2.1 准入范围

- Pod/Job 的创建与更新请求
- 与 AI 推理/训练相关的 workload
- 标签、资源、队列、租户、合规约束

### 2.2 非范围

- 具体任务调度算法求解
- 模型推理业务逻辑
- GPU 驱动与 CUDA 底层实现

## 3. 准入对象识别

### 3.1 AI workload 判定规则（建议）

满足任一条件即判定为 AI workload：
1. label: masha.io/ai-enabled=true
2. 资源声明包含 nvidia.com/gpu
3. label: workload-type in [inference, training]

### 3.2 元数据最小集合

必填标签：
- masha.io/owner
- masha.io/workload-type
- masha.io/queue

推荐标签：
- masha.io/cost-center
- masha.io/model-name
- masha.io/environment

## 4. 准入策略目录（Policy Catalog）

### P1. 资源声明策略

- 规则：AI workload 必须声明 requests/limits。
- 目的：避免超卖与资源争抢。
- 失败返回：missing requests/limits for AI workload。

### P2. GPU 资源策略

- 规则：需要 GPU 的 workload 必须声明 nvidia.com/gpu > 0。
- 目的：避免“隐式 GPU 需求”导致调度失败。
- 失败返回：gpu workload missing nvidia.com/gpu limit。

### P3. 租户归属策略

- 规则：必须包含 owner 标签，且在白名单租户中。
- 目的：支持审计和成本归因。
- 失败返回：invalid or missing owner label。

### P4. 队列策略

- 规则：queue 必须是允许值（realtime/train/batch 等）。
- 目的：保证调度和优先级策略可执行。
- 失败返回：invalid queue label。

### P5. 镜像合规策略

- 规则：镜像仓库必须在信任列表。
- 目的：供应链安全。
- 失败返回：image registry is not allowed。

### P6. 风险模式策略

- 规则：高风险环境可启用 stricter policy（例如必须启用只读根文件系统）。
- 目的：生产环境加固。
- 失败返回：security policy not satisfied。

## 5. 准入决策流程

1. 解析 AdmissionReview。
2. 判断是否 AI workload。
3. 非 AI workload：走兼容路径（allow + audit）。
4. AI workload：按策略顺序逐条评估。
5. 第一条失败即 reject（或按配置 accumulative errors）。
6. 输出 AdmissionResponse（allow/reject + reason + policy id）。
7. 写入审计日志和指标。

## 6. 策略执行模式

### 6.1 模式定义

- audit：仅记录违规，不拒绝。
- enforce：违规即拒绝。
- dry-run：模拟执行，不影响请求。

### 6.2 灰度策略

- 先 namespace 灰度
- 再 owner 灰度
- 最后全局 enforce

## 7. Admission 输入输出契约

### 7.1 输入契约

- AdmissionReview.request.uid
- AdmissionReview.request.object.raw
- operation: CREATE/UPDATE

### 7.2 输出契约

- allowed: true/false
- status.message: 可读错误信息
- warnings: 非阻断风险提示
- patch/patchType: 仅在需要 mutation 时返回

### 7.3 错误信息规范

建议格式：
- [PolicyID] human-readable reason; remediation hint

示例：
- [P3] missing label masha.io/owner; set owner for cost attribution

## 8. 观测与审计要求

### 8.1 指标

- ai_admission_requests_total{result,policy,mode}
- ai_admission_latency_seconds{mode}
- ai_admission_rejections_total{policy,reason}

### 8.2 日志字段标准

- request_uid
- namespace
- name
- owner
- workload_type
- queue
- policy_id
- decision
- reason
- mode
- latency_ms

### 8.3 审计追踪

- 每次拒绝必须可追溯到 policy_id 和版本号
- 每次策略变更必须有发布记录

## 9. 失败与降级策略

### 9.1 Webhook 不可用

- 选项 A：fail-close（更安全）
- 选项 B：fail-open（更可用）

建议：
- 生产默认 fail-close
- 预发默认 fail-open

### 9.2 策略存储不可读

- 回退到上一个已加载稳定版本
- 打告警并记录 policy_load_failure

### 9.3 高峰期延迟过高

- 降低非关键策略复杂度
- 开启缓存（例如租户白名单缓存）

## 10. 验收标准（Definition of Done）

### 10.1 功能验收

1. AI workload 可被识别。
2. 核心策略可阻断不合规请求。
3. 支持 audit/enforce 两模式。

### 10.2 稳定性验收

1. Admission P95 延迟在目标阈值内。
2. Webhook 重启后策略可自动恢复。

### 10.3 可运维验收

1. 任一拒绝都可通过日志定位原因。
2. 可按 owner 输出拒绝排行。

## 11. 里程碑建议

### M1（1-2 周）

- AI workload 识别
- P1/P3/P4 三条策略
- audit 模式 + 指标日志

### M2（3-4 周）

- enforce 模式
- 策略灰度发布
- 错误信息标准化

### M3（5-6 周）

- 策略版本化
- 失败回退机制
- 准入报表（拒绝趋势、租户分布）

## 12. 可直接喂给 AI 的上下文片段（示例）

```yaml
admission_context:
  project: masha-mesh
  objective: enforce AI workload governance at admission phase
  ai_workload_detection:
    - label masha.io/ai-enabled=true
    - gpu resource nvidia.com/gpu > 0
    - label workload-type in [inference, training]
  required_labels:
    - masha.io/owner
    - masha.io/workload-type
    - masha.io/queue
  core_policies:
    - P1 resource declaration required
    - P3 owner required and valid
    - P4 queue must be allowed
  modes:
    - audit
    - enforce
  observability:
    - ai_admission_requests_total
    - ai_admission_rejections_total
    - ai_admission_latency_seconds
  failure_strategy:
    - production fail-close
    - staging fail-open
```

## 13. 实施前澄清问题

1. AI workload 的唯一判定口径是什么。
2. owner 白名单来源是静态配置还是 CRD。
3. 队列集合由谁维护。
4. 生产默认是否接受 fail-close。
5. 拒绝提示是否对开发者可见。
