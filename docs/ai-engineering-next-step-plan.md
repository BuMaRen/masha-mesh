# masha-mesh AI工程化下一步计划（定制版）

## 1. 目标定位

你当前项目已经具备 AI 工程化最关键的底座：
- K8s 控制面事件驱动（reconciler + informer）
- webhook 准入与 sidecar 注入
- gRPC 分发通道
- metrics 观测基础

下一步不是“做一个聊天功能”，而是把 masha-mesh 演进成：
- 面向 AI 工作负载的控制平面（AI-aware Control Plane）
- 能做可用性判定、成本治理、降级兜底的工程平台

## 2. 现状与机会点（映射当前代码）

### 2.1 已有可复用能力
- 启动编排：`internal/ctrl/logic.go`
- 准入注入：`internal/ctrl/webhook/server.go` + `internal/ctrl/webhook/aggregation.go`
- CRD 变更收敛：`internal/ctrl/reconciler/container_reconciler.go`
- 事件广播：`internal/ctrl/reconciler/endpointslice_reconciler.go`
- 观测基础：`pkg/metrics/metric.go`

### 2.2 当前短板（正好是 AI 工程化切入点）
- webhook 只做注入，不做 AI 资源与成本准入策略
- controller 处理了容器更新，但没有 AI 负载健康语义（模型是否就绪）
- metrics 里没有 AI 维度（token、推理延迟、降级次数）
- 缺少“失败自动降级”路径（模型不可用时走规则引擎/静态策略）

## 3. 先做什么（第一优先级：2周）

### Sprint A（第1周）：把“AI可观测性”接到现有骨架

#### 任务 A1：新增 AI 指标（最小闭环）
- 文件：`pkg/metrics/metric.go`
- 增加：
  - `ai_inference_requests_total{provider,model,result}`
  - `ai_inference_duration_seconds{provider,model}`
  - `ai_fallback_total{reason}`
  - `ai_token_estimate_total{provider,model,direction}`
- 验收：能在 /metrics 中看到新增指标，并由最小 demo 触发增长

#### 任务 A2：在 webhook 注入链路补“AI 标签与策略开关”
- 文件：`internal/ctrl/webhook/aggregation.go`
- 逻辑：对带 `masha.io/ai-enabled=true` 的 Pod 执行额外准入检查（初期只做结构检查，不做拒绝）
- 验收：AdmissionReview 日志能区分普通 workload 与 AI workload

#### 任务 A3：补结构化日志字段
- 文件：`internal/ctrl/webhook/aggregation.go`、`internal/ctrl/reconciler/container_reconciler.go`
- 统一字段：`namespace`、`workload`、`ai_enabled`、`policy_result`、`reconcile_result`
- 验收：排障时可按字段检索，不再依赖模糊文本 grep

### Sprint B（第2周）：把“AI准入治理”做成真实能力

#### 任务 B1：实现 AI workload 最小准入策略
- 文件：`internal/ctrl/webhook/aggregation.go`
- 策略建议（先做 2 条硬规则）：
  1. AI workload 必须声明资源 requests/limits（CPU/Memory）
  2. 必须带业务归属标签（如 `masha.io/owner`）
- 验收：不合规 Pod 被拒绝，并返回明确 AdmissionResponse message

#### 任务 B2：为策略新增配置项
- 文件：`internal/ctrl/webhook/options.go`、`internal/ctrl/options.go`
- 增加开关：
  - `--ai-policy-enforce`（默认 false）
  - `--ai-required-owner-label`（默认 `masha.io/owner`）
- 验收：开关可控，灰度启用

#### 任务 B3：补单测
- 文件：新增 `internal/ctrl/webhook/aggregation_test.go`
- 至少覆盖：
  - AI 标签缺失资源 -> reject
  - owner 标签缺失 -> reject
  - 非 AI workload -> allow
- 验收：`go test ./...` 通过且新增用例可复现策略

## 4. 第二阶段（3-6周）：从“准入”升级到“AI工程化控制面”

### 阶段目标
把 masha-mesh 从注入控制器升级为“AI流量治理 + 失败兜底”平台。

### 核心工作包

#### WP1：AI 健康语义（Model Readiness）
- 在 reconciler 侧补“健康状态投影”，不只看 Pod Running，而看推理服务可用
- 初期可通过 sidecar 上报心跳状态模拟模型就绪
- 产出：`AIReady` 状态维度 + 指标

#### WP2：失败降级通道（Fallback）
- 在 gRPC 分发层引入 result 状态：`PRIMARY_OK` / `FALLBACK_USED` / `FAILED`
- 失败原因分类：超时、限流、上游不可达
- 产出：可观测到降级发生率

#### WP3：成本治理（FinOps Lite）
- 先做估算，不依赖具体模型厂商：
  - 入参长度估算 input token
  - 出参长度估算 output token
- 结合 owner 标签输出按业务维度聚合统计
- 产出：每个 owner 的 token 估算看板基础数据

## 5. 第三阶段（7-10周）：形成“可讲可投递”的 AI工程化作品

### 交付目标
- 一个可运行的 demo 路径：
  1. 提交 AI workload
  2. webhook 执行策略准入
  3. controller 收敛与分发
  4. metrics 展示请求、延迟、降级、成本估算
- 一份架构文档（非模板）说明你的取舍
- 一份故障演练记录（至少 2 个场景）

### 故障演练建议
1. 上游超时：验证 fallback 与告警链路
2. 配置错误：验证 webhook 拒绝与可读错误信息

## 6. 技能成长路径（与你的职业目标直接对应）

### 你正在形成的能力标签
- K8s 控制面开发（Controller + Admission）
- AI workload governance（准入、资源、成本）
- 可观测性工程（metrics + structured logging）
- 韧性工程（fallback、故障分类、SLO）

### 这比“只会调 Prompt”更有竞争力的原因
你做的是“让 AI 在生产环境可控运行”的平台能力，而不是一次性 API 调用。

## 7. 本周就能执行的 TODO（直接开工清单）

1. 在 `pkg/metrics/metric.go` 增加 4 个 AI 指标并注册
2. 在 `internal/ctrl/webhook/aggregation.go` 增加 `masha.io/ai-enabled` 分支逻辑（先只记录，不拒绝）
3. 新增 `internal/ctrl/webhook/aggregation_test.go` 验证 AdmissionReview 分支
4. 在 `internal/ctrl/webhook/options.go` 与 `internal/ctrl/options.go` 增加 AI 策略开关
5. 写一页 `docs/architecture-and-request-flow.md` 的真实版（替换模板内容），补上 AI 准入与降级流程图

---

## 8. 里程碑验收标准

### M1（2周）
- AI workload 能被识别、审计、可观测
- 策略可灰度启停
- 单元测试覆盖核心准入分支

### M2（6周）
- 出现上游故障时可自动降级
- 有按 owner 的 token 成本估算
- 能输出基础 SLO 指标（成功率、P95、降级率）

### M3（10周）
- 形成完整项目叙事：问题 -> 设计 -> 权衡 -> 故障 -> 数据结果
- 可直接作为“AI工程化平台”方向的面试/简历核心项目

---

如果你只做一件事，请先做：

把 webhook 从“注入器”升级为“AI workload 准入与治理入口”。

这是你当前仓库里投入产出比最高、且最能体现 AI工程化价值的一步。