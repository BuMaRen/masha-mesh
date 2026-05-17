# Prometheus 落地分阶段指导（含模板）

本文用于项目执行层面，回答两个问题：

1. 每个阶段要做什么
2. 每个阶段有哪些可直接复用的模板

适用对象：masha-mesh 控制面（mesh-ctrl）。

## 阶段总览

建议拆成 6 个阶段：

1. 指标规划
2. 代码埋点
3. 采集接入
4. 查询与看板
5. 告警与值班
6. 运营治理

每个阶段都给出：目标、动作清单、交付物、验收标准、模板。

---

## 阶段 1：指标规划

### 目标

明确监控对象与边界，避免后续重复返工。

### 要做什么

1. 识别核心用户路径和关键系统路径
2. 定义 SLI/SLO（可用性、延迟、错误率）
3. 列出首批业务指标，明确类型与单位
4. 定义 label 规则（禁止高基数字段）
5. 形成指标命名规范（namespace_subsystem_name）

### 交付物

- 指标字典 v1
- label 白名单
- SLO 草案

### 验收标准

- 每个指标都有 owner、含义、单位、聚合方式
- 所有 label 都是低基数且可枚举
- 关键路径至少覆盖 1 个计数指标 + 1 个时延指标

### 模板：指标字典


| 指标名 | 类型 | 含义 | 单位 | 标签 | 聚合方式 | 告警关联 | Owner |
|---|---|---|---|---|---|---|---|
| mesh_ctrl_sidecars_connected | Gauge | 当前连接 sidecar 数 | 个 | 无 | avg/max | 连接数突降 | @xxx |
| mesh_ctrl_subscription_events_total | Counter | 下发事件总量 | 次 | service,op_type | rate/sum by(service) | 事件吞吐异常 | @xxx |
| mesh_ctrl_reconcile_duration_seconds | Histogram | reconcile 延迟 | 秒 | reconciler,result | p95/p99 | 延迟过高 | @xxx |


### 模板：label 规则

```markdown
允许：service, op_type, result, reconciler
禁止：user_id, request_id, trace_id, pod_name(动态), sidecar_id
规则：
- label 值需可枚举或可归一化
- URL 必须模板化（/users/{id}）
```

---

## 阶段 2：代码埋点

### 目标

在不影响主流程的前提下，暴露稳定、可抓取的指标。

### 要做什么

1. 引入 prometheus client_golang 依赖
2. 新增 /metrics HTTP 端点（独立端口）
3. 定义并注册首批指标
4. 在 gRPC、reconciler、cache 关键路径打点
5. 增加基础测试与本地验证命令

### 交付物

- 指标代码（server.go + metrics.go）
- 关键路径打点 PR
- README 中的本地验证步骤

### 验收标准

- /metrics 可访问
- 关键指标可观测到非零变化
- 没有明显高基数 label

### 模板：指标定义（Go）

```go
var SubscriptionEventsTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "mesh",
		Subsystem: "ctrl",
		Name:      "subscription_events_total",
		Help:      "Total number of subscription events sent to sidecars.",
	},
	[]string{"service", "op_type"},
)
```

### 模板：耗时打点（Go）

```go
start := time.Now()
result := "success"
// do business logic
if err != nil {
	result = "failure"
}
metrics.ReconcileDuration.WithLabelValues("endpointslice", result).Observe(time.Since(start).Seconds())
```

### 模板：接入检查清单

```markdown
- [ ] /metrics 端点在 9091 暴露
- [ ] 指标已注册且命名规范
- [ ] Counter 只增不减
- [ ] Histogram 桶合理（覆盖 p50/p95/p99）
- [ ] 关键路径有 success/failure 标签
```

---

## 阶段 3：采集接入

### 目标

让 Prometheus 稳定抓到指标，且抓取配置可维护。

### 要做什么

1. Deployment 增加 metrics 端口
2. Service 暴露 metrics named port
3. 配置 ServiceMonitor 或 prometheus.yml scrape job
4. 校验目标发现与抓取成功率

### 交付物

- K8s YAML 变更（deployment/service/servicemonitor）
- 抓取目标状态截图或验证记录

### 验收标准

- Prometheus targets 页面状态为 UP
- 指标抓取间隔和超时符合预期
- 指标没有大面积 missing

### 模板：ServiceMonitor

```yaml
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
      scrapeTimeout: 10s
```

### 模板：原生 scrape job

```yaml
scrape_configs:
  - job_name: mesh-ctrl
    scrape_interval: 15s
    scrape_timeout: 10s
    metrics_path: /metrics
    static_configs:
      - targets:
          - mesh-ctrl.default.svc.cluster.local:9091
```

### 模板：验收命令

```bash
kubectl -n default port-forward svc/mesh-ctrl 9091:9091
curl -s localhost:9091/metrics | head
```

---

## 阶段 4：查询与看板

### 目标

让工程师能快速判断健康状态并定位问题。

### 要做什么

1. 设计总览看板（吞吐、错误、时延、资源）
2. 设计排障看板（按 service、op_type 细分）
3. 把高开销查询转 recording rules
4. 固化常用 PromQL 到文档

### 交付物

- Dashboard v1
- RecordingRules v1
- 查询手册

### 验收标准

- 总览看板能在 1 分钟内判断系统是否异常
- 常用查询延迟可接受
- 查询口径一致（同名指标解释一致）

### 模板：核心 PromQL

```promql
sum(rate(mesh_ctrl_subscription_events_total[5m]))

sum(rate(mesh_ctrl_subscription_events_total[5m])) by (service, op_type)

histogram_quantile(
  0.95,
  sum(rate(mesh_ctrl_reconcile_duration_seconds_bucket[5m])) by (le, reconciler)
)

increase(mesh_ctrl_subscription_send_failures_total[5m])
```

### 模板：Recording Rule

```yaml
groups:
  - name: mesh-ctrl-recording
    interval: 30s
    rules:
      - record: mesh_ctrl:subscription_events:rate5m
        expr: sum(rate(mesh_ctrl_subscription_events_total[5m])) by (service, op_type)
```

---

## 阶段 5：告警与值班

### 目标

把“看见异常”变成“及时通知并可执行处置”。

### 要做什么

1. 定义告警分级（P1/P2/P3）
2. 建立 Alertmanager 路由（团队、服务、级别）
3. 配置抑制与去重，控制噪音
4. 为每条告警补 runbook 链接

### 交付物

- AlertingRules v1
- Alertmanager 路由配置
- Runbook 文档

### 验收标准

- 告警误报率可接受
- 告警触发后有明确 owner 与处理路径
- 值班同学可在 runbook 内完成初步定位

### 模板：PrometheusRule

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: mesh-ctrl-alerts
  namespace: monitoring
spec:
  groups:
    - name: mesh-ctrl
      rules:
        - alert: MeshCtrlSubscriptionSendFailures
          expr: increase(mesh_ctrl_subscription_send_failures_total[5m]) > 0
          for: 5m
          labels:
            severity: warning
            team: mesh
          annotations:
            summary: "mesh-ctrl subscription send failures detected"
            description: "There are failed sends in the last 5 minutes."
            runbook: "https://internal/wiki/mesh-ctrl-send-failure"

        - alert: MeshCtrlReconcileP95TooHigh
          expr: histogram_quantile(0.95, sum(rate(mesh_ctrl_reconcile_duration_seconds_bucket[5m])) by (le, reconciler)) > 1
          for: 10m
          labels:
            severity: critical
            team: mesh
          annotations:
            summary: "mesh-ctrl reconcile p95 latency is high"
            description: "reconcile p95 > 1s for 10 minutes"
            runbook: "https://internal/wiki/mesh-ctrl-reconcile-latency"
```

### 模板：告警信息规范

```markdown
标题：[{{ severity }}][mesh-ctrl] {{ alertname }}
必含字段：service, environment, cluster, runbook
描述结构：现象 -> 影响 -> 建议操作
```

---

## 阶段 6：运营治理

### 目标

持续降低监控成本，提升指标与告警质量。

### 要做什么

1. 每月做指标瘦身（删除无用指标）
2. 每月做告警复盘（误报/漏报）
3. 调整抓取周期与保留时长
4. 定期评估高基数风险
5. 推动 SLO 与监控结果闭环

### 交付物

- 指标/告警月度治理报告
- 优化变更清单
- SLO 达成率报表

### 验收标准

- 无人使用指标逐步下降
- 告警噪音逐步下降
- 关键服务 SLO 可量化

### 模板：月度治理检查表

```markdown
- [ ] Top 20 高成本指标是否必要
- [ ] 新增 label 是否评审通过
- [ ] 告警误报率是否下降
- [ ] 是否有超过 30 天无人查看看板
- [ ] SLO 不达标项是否有行动计划
```

---

## 项目执行节奏（建议）

### 第 1 周

- 完成阶段 1 + 阶段 2
- 输出指标字典 v1
- 暴露 /metrics 并完成首批埋点

### 第 2 周

- 完成阶段 3 + 阶段 4
- 打通 Prometheus 抓取
- 发布 Dashboard v1

### 第 3 周

- 完成阶段 5
- 告警上线 + runbook

### 第 4 周及以后

- 按阶段 6 持续治理

---

## 一页式验收模板

```markdown
版本：v1.0
服务：mesh-ctrl
环境：dev/staging/prod

1. 指标规划
- 指标字典：已完成/未完成
- label 评审：已完成/未完成

2. 代码埋点
- /metrics：通过/失败
- 关键指标：通过/失败

3. 采集接入
- targets UP：通过/失败
- 抓取间隔符合预期：通过/失败

4. 查询看板
- 总览看板：通过/失败
- 排障看板：通过/失败

5. 告警值班
- 告警规则：通过/失败
- runbook 链接：通过/失败

6. 运营治理
- 月度机制：已建立/未建立

结论：可上线/需整改
整改项：
- xxx
- xxx
```

如果你需要，我可以继续把这份阶段文档中的模板拆成可直接 apply 的 YAML/Go 文件骨架（ServiceMonitor、PrometheusRule、metrics.go、server.go）。
