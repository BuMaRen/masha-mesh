# Pod Sidecar iptables 劫持机制说明（基于 tests/server/deployment.yml）

## 1. 目标与场景

这份 Deployment 在 Pod 启动时通过 initContainer 写入 iptables 规则，把业务流量透明导入 sidecar（监听 15001）。

目标是：
- 出站 TCP：默认先进入 sidecar。
- 入站到 5678 的 TCP：先进入 sidecar。
- sidecar 自身流量与关键本地流量不被重复劫持，避免环路。

---

## 2. 关键前提：Pod 共享网络命名空间

同一个 Pod 内的多个容器共享同一个 network namespace：
- 共享同一套网卡、路由、iptables、conntrack。
- 因此 initContainer 写入的规则会影响该 Pod 后续所有容器。

这也是为什么在 sidecar 场景里，一个 initContainer 就能给整个 Pod 生效。

---

## 3. 规则逐条解释

来自 deployment 中的命令（按顺序）：

1) `iptables -t nat -A OUTPUT -m owner --uid-owner 1337 -j RETURN`
- 在 nat/OUTPUT 链中，匹配“本机发起且进程 uid=1337”的包，直接返回。
- 目的：sidecar 进程通常就是 uid=1337，避免 sidecar 自己发出的流量再次被重定向到自己。

2) `iptables -t nat -A OUTPUT -d 127.0.0.1/32 -j RETURN`
- 放行发往 localhost 的流量。
- 目的：保留本地回环通信语义，避免无意义代理与潜在问题。

3) `iptables -t nat -A OUTPUT -p tcp --dport 15001 -j RETURN`
- 放行目标端口是 15001 的 TCP。
- 目的：避免“本来就要发给 sidecar”的连接再次被重定向，防止循环。

4) `iptables -t nat -A OUTPUT -p tcp -j REDIRECT --to-port 15001`
- 将其余 OUTPUT 的 TCP 透明重定向到本地 15001。
- 这条是“默认劫持规则”，使应用出站连接先进入 sidecar。

5) `iptables -t nat -A PREROUTING -p tcp --dport 5678 -j REDIRECT --to-port 15001`
- 对进入该 Pod 网络栈且目标端口为 5678 的 TCP，在 PREROUTING 阶段重定向到 15001。
- 作用：入站到业务端口 5678 的连接先给 sidecar。

顺序很重要：
- nat/OUTPUT 上方三条 RETURN 是排除项。
- 最后一条 OUTPUT REDIRECT 是兜底项。

---

## 4. 出站连接完整路径（以 app 发起 HTTP 为例）

假设 app 进程发起 `connect(dstIP:dstPort)`：

1. 应用发起 connect（HTTP 只是 TCP 之上的协议）。
2. 首包进入 nat/OUTPUT。
3. 按规则依次匹配：
   - 若 uid=1337，放行。
   - 若目标是 127.0.0.1，放行。
   - 若目标端口是 15001，放行。
   - 否则命中 REDIRECT 到 15001。
4. 内核将该连接目标改写为本地 15001，sidecar 接收到连接。
5. sidecar 读取原始目标信息（通过内核/套接字机制获取），再自己建立到真实上游的连接。
6. 最终形成两段连接：
   - app <-> sidecar（Pod 内本地段）
   - sidecar <-> upstream（真实网络段）

因此从 app 视角，它“以为”自己连的是原目标；实际上连接被透明接管。

---

## 5. 入站连接完整路径（访问 Pod:5678）

1. 外部流量进入 Pod 网络栈。
2. 在 nat/PREROUTING 命中 `--dport 5678`，被重定向到 15001。
3. sidecar 接收连接，再按代理逻辑转发到真实服务（通常是同 Pod 的业务进程）。

---

## 6. 重点：连接建立后，write 数据是否“继续受 iptables 影响”？

简答：
- 会经过网络栈，但 nat 的重定向决策通常按“连接”做一次并由 conntrack 复用。
- 不会在每次 write 都重新做一遍 nat 规则决策。

详细过程：

1. 应用 `write()` 只是把字节交给 TCP 栈。
2. TCP 栈把字节分片成段（segment）后发包。
3. 发包路径仍会经过 netfilter 钩子。
4. 对 nat 表而言：
   - 新连接首包（典型是 SYN）命中 nat 规则并建立 NAT 映射。
   - 后续同一连接的数据包使用 conntrack 中已建立的映射。
5. 所以连接建立后，后续 write 的数据沿用既定映射到 sidecar，不是每个包重新“选一次目标”。

可以理解为：
- 规则匹配是“建连定向”。
- 连接存续期间是“按连接状态转发”。

---

## 7. conntrack 在这里的作用

conntrack 维护连接状态与 NAT 映射，常见状态包括：
- NEW：新连接首包。
- ESTABLISHED：已建立连接后续包。
- RELATED：与现有连接相关的包。

在本场景中：
- NEW 阶段由 nat 规则确定是否 REDIRECT 到 15001。
- ESTABLISHED 阶段复用映射，保证同一连接行为一致。

这也是“改了 nat 规则后，旧连接通常不变、新连接才生效”的根本原因。

---

## 8. 为什么需要 uid 排除与端口排除

如果没有排除规则，可能出现：
- sidecar 自己发出的上游连接又被重定向回 15001，造成代理自环。
- 发往 15001 的流量再次被改写，引发递归。
- 本地回环通信被劫持，破坏应用预期。

当前三条 RETURN 正是为了避免这些问题。

---

## 9. 常见误区澄清

1) “iptables 劫持的是进程吗？”
- 本质是劫持经过网络栈的数据包。
- 但可通过 owner match（如 uid）按进程属性筛选。

2) “每次 write 都重新匹配 nat 吗？”
- 一般不是。nat 主要在连接首包决策，后续走 conntrack 映射。

3) “HTTP 和 TCP 是否不同？”
- 劫持发生在 L3/L4（IP/TCP）层。
- HTTP 只是承载在 TCP 上，随 TCP 路径被透明代理。

---

## 10. 可观测与排障建议

1. 查看规则与命中计数：
- `iptables -t nat -vnL OUTPUT`
- `iptables -t nat -vnL PREROUTING`

2. 查看连接跟踪：
- `conntrack -L -p tcp`

3. 抓包确认流向：
- 在 Pod 内抓 `15001` 和业务端口，观察是否先进入 sidecar。

4. 验证排除是否生效：
- 确认 sidecar 进程 uid 与规则一致（此处应为 1337）。

---

## 11. 一句话总结

这套机制是“initContainer 预置 iptables + sidecar 代理 + conntrack 连接复用”：
- 新连接在 nat 阶段被透明改道到 15001；
- 连接建立后数据按既有映射持续转发；
- 通过 uid/本地地址/代理端口排除，避免环路并保持稳定。
