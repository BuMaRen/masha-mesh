# 熔断器性能与锁问题分析报告

## 范围

- 分析对象：`internal/cli/breaker/breaker.go`、`internal/cli/breaker/subbreaker.go`、`internal/cli/breaker/counter.go`
- 关注点：锁粒度、锁顺序、吞吐性能、窗口推进复杂度、统计正确性风险
- 说明：本报告基于代码审查、并发行为测试与 race 检测结果

## 已确认项

1. 锁顺序目前一致
- `subBreaker` 路径中为先拿 `sb.mtx`，再调用 `counter` 方法拿 `c.mtx`。
- 未发现 `counter` 持锁后反向请求 `sb.mtx` 的代码路径。
- 结论：当前代码未见明显锁顺序反转死锁点。

2. 半开探测配额具备串行保护
- `probeUsed` 在 `allowed` 内持有 `sb.mtx` 递增。
- 并发 `allowed` 场景下配额不会超发。

3. Breaker 按 key 初始化已原子化
- `Allowed` 与 `record` 都改为在同一写锁临界区内完成“检查 + 创建 + 使用”。
- 首次 key 调用 `Allowed` 的 nil pointer panic 已消除。
- 原先 TOCTOU（先检查后使用）导致的实例覆盖风险已消除。

4. 统计可见性已修复
- `counter.Summary` 已包含 `current` 桶。
- 记录后立即判定可见，不再依赖下一次 `Flush` 才生效。

## 已修复问题

### P0（已修复）

- [x] 首次 `Allowed` 可能 nil pointer panic
  - 现状：已修复。
  - 方式：写锁内初始化并复用同一 `sb` 引用。

### P1（已修复）

- [x] `Breaker` 级 TOCTOU 竞态（检查与使用分离）
  - 现状：已修复。
  - 方式：合并为同一临界区内的 get-or-create。

- [x] 统计可见性延迟（`Summary` 忽略 `current`）
  - 现状：已修复。
  - 方式：`Summary` 汇总历史桶与 `current` 桶。

## 待优化问题（按严重级别）

### P1（高优先级）

- [ ] 窗口推进后可能丢桶，统计被静默丢弃
  - 位置：`counter.Flush`、`counter.PushEmpty` 调用 `units.Enqueue(...)` 后未检查返回值。
  - 影响：
    - 当环形队列满时，`Enqueue` 返回 `false`，但调用方忽略，导致当前桶或空桶推进被丢弃。
    - 统计窗口可能不再代表真实最近时间段，造成熔断判定偏差。
  - 建议：
    - 为窗口容器提供“覆盖最旧桶”的入队语义，或在 `Enqueue=false` 时显式淘汰最旧桶再入队。
    - 至少在失败时计数或日志告警，避免静默失真。

- [ ] `advance` 在大时间跨度下持锁循环，尾延迟可能放大
  - 位置：`subbreaker.advance`
  - 影响：
    - `advance` 持有 `sb.mtx`，并在循环中多次调用 `PushEmpty`（每次都要获取 `c.mtx`）。
    - 时间跨度大时循环次数线性增长，单次请求可能阻塞后续所有请求（同 key）。
  - 建议：
    - 改为批量推进：一次性计算需要推进的桶数，由 `counter` 提供批量 API 完成。
    - 在跨度 >= capacity 时直接 reset，避免 O(delta) 循环。

### P2（中优先级）

- [ ] Closed 状态每次 `allowed` 都做全窗口汇总，热路径开销偏高
  - 位置：`allowed` -> `windowCounter.Summary`
  - 影响：
    - `Summary` 每次遍历队列并构建汇总对象，为 O(window_size)。
    - 高频流量下 CPU 开销明显，且在 `sb.mtx` 持有期间发生，放大串行化影响。
  - 建议：
    - 维护 rolling summary（增量加减），将热路径降为 O(1)。
    - 或在 `counter` 中缓存汇总并在 flush/evict 时更新。

- [ ] 锁粒度偏粗，每 key 吞吐受串行化限制
  - 位置：`allowed`、`record` 全流程均依赖 `sb.mtx`
  - 影响：
    - 同 key 并发请求被严格串行处理，吞吐上限受单锁影响。
    - 在高并发场景会出现明显锁竞争等待。
  - 建议：
    - 保持状态迁移原子性的前提下，缩短 `sb.mtx` 持锁区间。
    - 将纯统计更新与状态判定分离，避免在状态锁中执行昂贵操作。

## 测试补全情况

新增测试：`internal/cli/breaker/subbreaker_test.go`、`internal/cli/breaker/breaker_test.go`

- 并发放行配额不超发：验证 HalfOpen 并发 `allowed` 不超过 `halfOpenAllowed`。
- 并发 `allowed/record` 可完成：验证混合并发调用在限定时间内完成，降低死锁回归风险。
- `Breaker` 首次 key 初始化路径与状态迁移路径已有测试覆盖。

## Race 检测结果

- 执行命令：`CGO_ENABLED=1 go test -race ./internal/cli/breaker -v`
- 执行结果：PASS
- 结论：当前测试覆盖路径下未发现 data race
- 备注：`-race` 结果仅覆盖已执行代码路径，不能替代生产流量下的持续压测与观测

## 结论

- 当前实现的正确性问题（P0/P1）已完成修复，且测试与 race 检测通过。
- 剩余主要风险集中在性能与可观测性：
  - 入队失败静默丢桶（高优先级）。
  - `advance` 与 `Summary` 的线性开销导致高并发下延迟与吞吐压力。
- 建议下一阶段先处理窗口推进语义（显式 oldest-evict），再做批量推进与 rolling summary 优化。
