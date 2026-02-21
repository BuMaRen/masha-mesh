# pkg/ctrl 单元测试（UT）规范与实践指南

本文档给出 `pkg/ctrl` 目录的 UT 编写规范，目标是：**快速回归、行为可验证、失败易定位**。

## 1. 业界通用原则

- **FIRST 原则**
  - `Fast`：测试执行要快，能频繁运行。
  - `Independent`：测试之间相互独立，无顺序依赖。
  - `Repeatable`：本地/CI 结果一致，可重复。
  - `Self-Validating`：自动断言通过/失败，不靠人工看日志。
  - `Timely`：测试与功能一起写，避免滞后。
- **AAA 模式**：`Arrange`（准备）→ `Act`（执行）→ `Assert`（断言）。
- **单一行为验证**：一个测试只验证一个业务行为，失败定位更清晰。
- **对外行为优先**：优先断言输入输出、状态变化、事件发布，而非实现细节。

## 2. 目录内建议标准

- 测试文件命名：`*_test.go`，与被测文件同包（当前使用 `package ctrl`）。
- 测试函数命名：`Test<Struct>_<Method>_<Scenario>`。
  - 示例：`TestCoreData_OnDeleted_LastSlice_PublishesDeletedAndRemovesService`。
- 用例组织：优先**表驱动测试**（table-driven tests）。
- 断言风格：先检查关键前置（`t.Fatal`），再做细粒度断言（`t.Errorf`/`t.Fatalf`）。

## 3. 覆盖范围要求（建议）

对核心逻辑代码建议至少覆盖以下维度：

- **正常路径（Happy Path）**：输入合法、流程主路径。
- **边界条件（Boundary）**：空值、空集合、单元素、多元素。
- **异常路径（Error/Invalid Path）**：不存在对象、版本不匹配、标签不匹配。
- **副作用验证（Side Effects）**：
  - 状态变化（如 `serviceMap` 增删改）；
  - 事件发布（`Publish` 的服务名、操作类型、payload 类型）。

> 覆盖率目标：
>
> - `statement coverage`：建议不低于 `80%`；
> - 核心分支（缓存变更、事件发布）优先达到高覆盖。

## 4. Mock / Fake 的使用规范

本目录中的 `CoreData` 依赖 `Distributer` 接口，推荐使用轻量 mock：

- 只实现被调用方法（`Publish`）。
- 在 mock 中记录调用历史（`svcName`、`opType`、`payload`）。
- 在断言阶段验证调用次数与关键参数。

这样可以在不引入外部依赖的情况下，保持测试简洁且稳定。

## 5. 与当前代码对应的 UT 清单

### `endpoint_slice_merge.go`

重点验证：

- `OnAdded`：新增、服务不匹配、版本递增/非递增。
- `OnUpdate`：存在/不存在、版本校验、名称变化。
- `OnDelete`：存在/不存在、服务不匹配。
- `Merge` 相关行为（通过 `CoreData` 间接验证）。
- `DeepCopy`：原对象变化不应污染缓存。

### `storage.go`

重点验证：

- `List`：命中与未命中。
- `OnAdded`：创建服务缓存并发布 `ADDED`。
- `OnUpdate`：更新后发布 `MODIFIED`，payload 与状态一致。
- `OnDeleted`：
  - 删除最后一个 slice：发布 `DELETED` 且删除服务；
  - 删除部分 slice：发布 `MODIFIED` 且服务仍存在；
  - 删除不存在服务：不发布事件。

## 6. 常见反模式（避免）

- 测试依赖执行顺序。
- 测试中包含 sleep/真实网络/真实集群依赖（UT 不应依赖外部系统）。
- 断言过弱（只断言“不 panic”）。
- 单个测试覆盖过多行为，导致失败难定位。
- 过度耦合内部实现（重构后测试大量无意义失败）。

## 7. 运行方式

在仓库根目录执行：

```bash
go test ./pkg/ctrl/...
```

只跑某个文件相关测试：

```bash
go test ./pkg/ctrl/... -run TestCoreData
```

查看覆盖率：

```bash
go test -cover ./pkg/ctrl/...
```

## 8. Code Review 检查清单

提交 UT 时建议 reviewer 按以下核查：

- 是否覆盖了新增逻辑的主分支与失败分支？
- 断言是否验证了**业务结果**而非日志文本？
- 测试是否独立、稳定、可重复？
- 命名是否体现“方法 + 场景 + 期望”？
- 是否存在冗余 setup，可抽取 helper 简化？

---

如后续引入并发处理（例如多 goroutine 更新缓存），建议补充：

- `-race` 检测；
- 并发场景下的线程安全 UT；
- 必要时将 UT 与集成测试（IT）分层管理。
