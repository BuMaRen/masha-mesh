# 熔断器设计与实现设计（成熟版）

## 1. 背景与目标

当前熔断器实现存在以下典型问题：

- 状态迁移依赖全局定时循环，和请求流量耦合不清晰。
- 计数逻辑分散，窗口推进与状态判断职责边界不明确。
- 半开状态在低流量场景下可能长时间停滞，缺少统一超时语义。
- 并发访问时状态与计数的一致性约束不够清晰。

本设计目标：

- 提供生产可用的熔断器状态模型与实现模型。
- 支持滑动窗口统计，不依赖后台定时器也能正确推进时间。
- 明确半开状态探测策略、超时策略、回退策略。
- 给出并发安全、可观测性、参数治理、测试验证和上线迁移方案。

非目标：

- 本文不提供具体代码实现。
- 本文不覆盖重试、限流、隔离舱等完整容错体系，只关注熔断器。

## 2. 设计原则

- 请求路径驱动：状态计算由 Allowed 与 Record 触发，不依赖独立定时线程。
- 最小状态原则：仅保存必要状态，统计尽量从滑动窗口聚合得到。
- 明确时间语义：Open、HalfOpen 的超时和窗口边界必须可解释。
- 并发一致性优先：确保状态切换原子性和计数可见性。
- 保守默认值：默认策略优先保护下游而非追求吞吐。

## 3. 状态模型

### 3.1 状态定义

- Closed：正常放量，持续统计失败率。
- Open：拒绝请求，等待冷却时间结束。
- HalfOpen：仅放行有限探测请求，根据结果决定恢复或重新熔断。

### 3.2 状态语义

- Closed 到 Open：窗口失败率超过阈值且达到最小样本数。
- Open 到 HalfOpen：当前时间超过 openUntil。
- HalfOpen 到 Closed：探测请求达到 n 且全部成功。
- HalfOpen 到 Open：任一探测失败，或半开超时未完成探测。

## 4. 时间窗口与统计模型

### 4.1 窗口结构

采用固定容量的环形时间窗口（Ring Buckets）：

- bucketCount：窗口桶个数。
- bucketDuration：每个桶的时间长度。
- windowDuration = bucketCount × bucketDuration。
- 每桶统计字段：total、success、businessFailure、networkFailure、timeout。

实现语义上，窗口不是一个支持历史随机访问的通用数组，而是一个只围绕“当前尾桶”推进的时间容器：

- 允许写入当前尾桶，或者在时间推进时创建新桶。
- 当窗口满了以后，新桶进入，最旧桶被淘汰。
- 历史桶只作为聚合统计的一部分存在，不作为独立查询目标。

### 4.2 无定时器窗口推进

在每次 Allowed 或 Record 时执行 advance(now)：

- 计算距离上次推进经过了多少个桶。
- 向前滑动对应桶数；每次滑动都通过新增尾桶、淘汰最旧桶来完成。
- 对于中间没有请求落入的时间片，也必须补齐对应数量的空桶，空桶统计值全部为 0。
- 若跨度大于等于 bucketCount，直接清空整窗并重新建立当前尾桶。
- 更新 lastTick。

说明：

- 即使没有后台 ticker，只要有请求进入，就能把时间推进到当前点。
- 长时间无请求时，窗口不会主动生成新桶；但下一次请求到达时，advance(now) 仍要按真实时间差一次性补齐缺失的空桶。
- 因此，无定时器推进改变的是推进时机，而不是时间语义；其统计结果应与后台定时补桶模型一致。
- 状态迁移也不会自动发生，这是请求驱动模型的预期行为。

### 4.3 统计口径建议

- 熔断失败建议仅计入 networkFailure 与 timeout。
- businessFailure 默认不计入熔断失败率，避免业务逻辑失败误触发基础设施熔断。
- 失败率公式：

failureRate = (networkFailure + timeout) / total

- 仅当 total >= minRequestCount 时才允许触发 Closed 到 Open。

### 4.4 advance 职责定义

advance(now) 是窗口与状态机共享的时间对齐入口，其职责应固定为：

- 时间推进：根据 now 与 lastTick 的差值补齐缺失时间片，对应创建空桶并淘汰过期桶。
- 统计对齐：保证后续读取到的窗口统计始终表示“最近 windowDuration 的真实时间”。
- 过期清理：当时间跨度大于等于 bucketCount 时，直接清空整窗，避免历史样本污染当前判断。
- 单一时间基准：为 Allowed 与 Record 提供同一时间基准，避免决策与记账使用不同窗口视图。

advance 只负责时间窗口推进与统计视图对齐，不直接决定 Closed、Open、HalfOpen 的业务迁移条件。

## 5. 核心算法设计

### 5.1 Allowed 决策流程

输入：key、now。
输出：是否放行。

职责边界：

- Allowed 可以执行不依赖本次请求结果的迁移（时间驱动或配额驱动）。
- Allowed 不执行依赖本次请求成功/失败结果的迁移。

流程：

1. 执行窗口推进 advance(now)。
2. 根据当前 state 决策：
   - Closed：
     - 若满足熔断条件，切换到 Open，设置 openUntil，拒绝本次。
     - 否则放行。
   - Open：
     - 若 now < openUntil，拒绝。
     - 若 now >= openUntil，切换到 HalfOpen 并初始化探测计数，再进入 HalfOpen 判定。
   - HalfOpen：
     - 若 now - halfOpenSince > halfOpenMaxDuration，回 Open 并拒绝。
     - 若 probeUsed >= halfOpenAllowed，拒绝。
     - 否则 probeUsed 加一并放行。

Allowed 中允许出现的典型状态切换：

- Open 到 HalfOpen（冷却时间到期）。
- HalfOpen 到 Open（半开超时）。

说明：Allowed 执行 advance(now) 的目的是保证放行决策基于当前时间窗口，而不是基于过期统计。

### 5.2 Record 反馈流程

输入：key、result、now。
输出：更新状态与统计。

职责边界：

- Record 负责执行依赖请求结果的迁移。
- HalfOpen 到 Closed 必须在 Record 中判定，不能在 Allowed 中提前判定。

流程：

1. 执行窗口推进 advance(now)。
2. 更新当前桶计数。
3. 若 state 为 HalfOpen：
   - result 为失败：立即切 Open，设置 openUntil，清空探测计数。
   - result 为成功：probeSuccess 加一；若 probeSuccess == halfOpenAllowed，切 Closed。
4. 切回 Closed 后，建议重置窗口统计，避免历史失败样本立即再次触发 Open。

说明：Record 执行 advance(now) 的目的是保证回写结果落入正确时间片，并与 Allowed 使用一致窗口视图。

### 5.3 长时间无请求场景

在请求驱动模型下：

- 无请求期间不进行状态迁移。
- 下一次请求到达时再执行迁移判断。
- HalfOpen 不应无限期悬挂，必须配置 halfOpenMaxDuration。

推荐策略：

- HalfOpen 超时后默认回 Open（保守）。
- 对低风险服务可允许超时后回 Closed（激进），但需显式配置。

## 6. 数据结构设计

## 6.1 单路由熔断单元（subBreaker）

建议字段：

- state
- openUntil
- halfOpenSince
- halfOpenAllowed
- probeUsed
- probeSuccess
- minRequestCount
- failureRateThreshold
- halfOpenMaxDuration
- windowCounter（仅支持尾部推进与最旧桶淘汰的环形统计器）
- lock 或原子字段

### 6.2 全局管理器（Breakers）

职责：

- 维护 key 到 subBreaker 的映射。
- 提供 Allowed、RecordSuccess、RecordFailure 接口。
- 首次访问懒加载 subBreaker。
- 仅负责生命周期与索引，不承担状态计算细节。

## 7. 并发与一致性设计

### 7.1 一致性要求

- 同一 key 的 Allowed 与 Record 必须串行化或原子化。
- 状态切换与探测计数更新必须在同一临界区内完成。
- 避免同一时刻多个请求重复执行 Open 到 HalfOpen 的迁移。

### 7.2 推荐并发策略

- 方案 A：每个 subBreaker 使用互斥锁。
- 方案 B：状态字段用原子变量，复杂迁移仍使用轻量锁保护。

建议优先采用方案 A，逻辑更清晰、可维护性更高。

### 7.3 关键竞态点

- Open 临界时间点并发请求同时进入 HalfOpen。
- HalfOpen 放行配额并发超发。
- Record 与 Allowed 并发导致窗口推进与计数冲突。

## 8. 参数设计与默认值建议

建议参数及默认值：

- failureRateThreshold: 0.5
- minRequestCount: 20
- bucketDuration: 1s
- bucketCount: 30
- openStateDuration: 30s
- halfOpenAllowed: 10
- halfOpenMaxDuration: 60s

参数调优建议：

- 低流量服务应降低 minRequestCount，但不建议低于 5。
- 高频服务可增大 bucketCount 或减小 bucketDuration，提高敏感度。
- halfOpenAllowed 越大，恢复判断越稳，但恢复速度越慢。

## 9. 可观测性设计

最小指标集：

- breaker_state{key}
- breaker_allow_total{key,result}
- breaker_transition_total{key,from,to,reason}
- breaker_probe_total{key,result}
- breaker_window_failure_rate{key}

建议日志事件：

- 状态切换日志（含 from、to、reason、统计摘要）。
- 半开探测日志（配额使用、成功数、失败原因）。
- 配置热更新日志（旧值、新值、生效范围）。

## 10. 失败场景与降级策略

- 窗口统计异常：保守返回 Open 或拒绝放量。
- 时钟异常跳变：记录异常并限制单次 advance 步数。
- key 爆炸增长：增加最大 breaker 数与淘汰策略（LRU/TTL）。
- 依赖偶发抖动：配合指数退避重试与请求超时统一治理。

## 11. 测试设计

单元测试：

- Closed 到 Open 触发阈值。
- Open 冷却期拒绝与到期迁移。
- HalfOpen 全成功关闭、单失败回开。
- HalfOpen 超时迁移。
- 长时间无请求后的首次请求迁移。
- 窗口大跨度推进（delta >= bucketCount）正确清空。

并发测试：

- 并发 Allowed 不超发 halfOpen 配额。
- 并发 Record 不丢计数。
- 迁移边界条件下无重复切换与脏状态。

稳定性测试：

- 高并发长时间压测无数据竞争与内存泄漏。
- 通过 race 检测和 pprof 验证。

## 12. 落地迁移方案

阶段 1：引入新模型（保留旧接口）

- 保持现有 Breakers 对外 API 不变。
- 在 subBreaker 内部切换为请求驱动迁移。
- 暂时保留旧状态循环开关，默认关闭。

阶段 2：灰度与对比

- 按 key 范围或服务维度灰度启用。
- 对比指标：拒绝率、恢复时间、误熔断率、下游错误率。

阶段 3：全量与清理

- 全量切换后移除 ticker 线程及旧迁移逻辑。
- 收敛配置项，仅保留新模型参数。

## 13. 实施清单

- 明确状态字段与时间字段定义。
- 完成窗口推进 advance 语义统一。
- 重构 Allowed 与 Record 为唯一状态迁移入口。
- 补齐半开超时策略与可配置项。
- 增加指标与关键日志。
- 完成单测、并发测试、压测与灰度验证。

## 14. 结论

成熟熔断器不依赖独立定时器也能正确工作。关键在于：

- 通过请求路径驱动时间推进与状态迁移。
- 保留最小但必要的状态字段保证 HalfOpen 语义完整。
- 用清晰的并发策略保障状态与计数一致性。
- 通过可观测性与灰度策略确保可运营、可回滚、可持续优化。
