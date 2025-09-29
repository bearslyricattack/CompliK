# procscan

一个为 Kubernetes 设计的高性能、节点级的可疑进程扫描器，专为自动化检测和响应而生。

---

## ✨ 功能特性

`procscan` 是一个精密的节点安全工具，它以 `DaemonSet` 的形式运行在 Kubernetes 集群的每个节点上，持续扫描可疑进程，并基于一套高度灵活的规则引擎执行自动化响应。

### 1. 核心检测与过滤引擎
gen
- **强大的规则引擎**
  - **黑名单 (Blacklist)**：可基于 `进程名 (processes)` 和 `命令行关键词 (keywords)` 来定义您需要检测的恶意行为模式。
  - **白名单 (Whitelist)**：可基于 `进程名 (processes)`、`命令行 (commands)`、`命名空间 (namespaces)` 和 `Pod 名称 (podNames)` 来定义豁免规则，以精确地忽略特定业务场景，防止误报。
  - **全面正则支持 (Full Regex Support)**：**所有**黑、白名单的匹配规则都使用正则表达式进行匹配，提供了无与伦比的精确性和灵活性。

- **核心安全过滤器**
  - **`ns-` 前缀锁定**：这是一个硬编码的、最高优先级的安全规则。`procscan` **只有**在可疑进程所在的命名空间以 `ns-` 开头时，才会执行后续的告警和自动化操作，这从根本上保证了操作的绝对安全，避免了对系统命名空间的任何影响。

### 2. 自动化响应与处置

- **自动添加注解 (Automatic Annotation)**
  - **功能**：对发现问题的 `ns-` 命名空间，能自动添加一个您预先配置好的注解（例如 `debt.sealos/status: Suspend`），以便触发后续的资源限制等管控策略。
  - **开关**：此功能可通过 `config.yaml` 中的 `actions.annotation.enabled` (true/false) 进行全局启用或禁用。

- **强制清理异常 Pod (Forced Pod Cleanup)**
  - **功能**：在添加注解后，能自动、强制地删除目标命名空间下所有处于 `Terminating`（卡住无法终止）或 `Failed`（已失败）状态的 Pod，以快速释放资源。
  - **实现**：采用了“**清空 finalizers + 0秒宽限期**”的专业级实现，确保了删除操作的成功率。
  - **开关**：此高危操作默认**关闭**，必须在 `config.yaml` 中将 `actions.forceDelete.enabled` 设置为 `true` 才能激活。

### 3. 高性能与高可用架构

- **高性能扫描引擎**
  - **并发处理**：扫描任务在多核 CPU 上并行执行，极大缩短了全盘扫描的时间。
  - **高效缓存**：在每次扫描开始时，程序会一次性构建容器信息缓存，避免了在扫描过程中产生大量低效的网络请求。
  - **混合模式**：通过“缓存优先、按需查询”的机制，既保证了绝大多数情况下的高性能，又解决了缓存可能带来的对“新创建”容器的检测延迟问题。

- **动态配置热加载 (Dynamic Config Hot-Reload)**
  - **无中断更新**：您可以在任何时候修改 `ConfigMap` 中的配置，`procscan` 程序能够自动加载并应用新配置，全程**无需重启 Pod**。
  - **原子性保障**：运行中的扫描任务不会被新配置中断，确保了每次扫描结果的一致性和稳定性。
  - **透明日志**：每一次配置变更，都会在日志中留下详细的、结构化的记录，清晰地展示出是哪个配置项、从什么旧值、变成了什么新值。

### 4. 聚合告警与可观测性

- **全局聚合报告 (Global Aggregated Report)**
  - **单一告警**：每次扫描无论发现多少问题、涉及多少命名空间，都只会发送**唯一一封**飞书告警，彻底解决了“告警风暴”问题。
  - **结构化信息**：告警是一张内容丰富的“全局扫描报告”，采用“**总体摘要 + 分段详情**”的结构，清晰地展示了本次受影响的命名空间列表、每个空间内的详细进程信息、以及程序对每个空间执行的自动化操作结果。

- **结构化日志 (Structured Logging)**
  - 所有日志输出都为 **JSON 格式**。这使得日志易于被机器解析，可以方便地接入 Loki、ELK 等日志聚合平台，进行高效的查询、分析和监控。

---

## ⚙️ 配置

所有配置都通过一个 `config.yaml` 文件进行管理，该文件从 `ConfigMap` 中挂载。配置项已按领域分组，结构清晰，易于扩展。

```yaml
# 扫描器自身配置
scanner:
  proc_path: "/host/proc"
  scan_interval: "100s" # 支持 s, m, h 等时间单位
  log_level: "info"     # 支持 debug, info, warn, error

# 自动化动作配置
actions:
  annotation:
    enabled: true
    data:
      debt.sealos/status: "Suspend"
  forceDelete:
    enabled: false # 默认关闭，保证安全

# 通知渠道配置
notifications:
  lark:
    webhook: "https://open.feishu.cn/open-apis/bot/v2/hook/..."

# 检测规则配置
detectionRules:
  blacklist:
    processes:
      - "^miner$" # 精确匹配 miner
    keywords:
      - "xmr.pool.gulf.obtc.com"

  whitelist:
    processes:
      - "legit-process"
    commands:
      - "/usr/bin/bash -c 'legit-script'"
    namespaces:
      - "ns-dev-test"
    podNames:
      - "^special-debug-pod-" # 支持正则
```

---

## 🚀 部署

`procscan` 以 Kubernetes `DaemonSet` 的形式部署，确保在每个节点上都有一个实例运行。

完整的部署清单（包括 `ConfigMap`, `ClusterRole`, `DaemonSet` 等）位于 `deploy/manifests/deploy.yaml`。

**注意**：在执行 `kubectl apply` 部署前，请务必确认 `DaemonSet` 定义中的 `image` 字段，将其修改为您需要部署的镜像版本标签（例如 `layzer/sealos-procscan:v0.0.2-alpha-4`）。