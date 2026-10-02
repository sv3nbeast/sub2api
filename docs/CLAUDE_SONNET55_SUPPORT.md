# Claude Sonnet 5.5 支持与验证记录

核实日期：2026-10-02。代码基线：`ad31837ab2ba280ab5227f5f0d3fe3201bf014dd`。
实施分支：`codex/claude-sonnet-5-5`。

本次完成代码、配置迁移和本地验证，尚未发布。生产仅进行了只读查询和有界上游探测，没有修改生产账号、分组、定价或镜像。

## 官方规格

| 项目 | 规格 |
| --- | --- |
| 官方请求 ID | `claude-sonnet-5-5` |
| 发布日期 | 2026-09-28 |
| 上下文 / 最大输出 | 1,000,000 / 128,000 Token |
| 输入 / 输出 | $2 / $10 每百万 Token |
| 缓存读 | $0.20 每百万 Token |
| 缓存写：5 分钟 / 1 小时 | $2.50 / $4 每百万 Token |
| 最小缓存前缀 | 512 Token |
| 长上下文价格 | 全 1M 使用相同价格，无额外长上下文加价 |
| Batch / Fast | Batch 50%；无 Sonnet Fast 模式 |
| 默认思考模式 | adaptive，默认 effort=high |
| adaptive effort | low / medium / high / xhigh / max |
| between_tools effort | low / medium / high |

官方文档：

- [模型概览](https://platform.claude.com/docs/en/models/sonnet-5-5/overview)
- [新特性](https://platform.claude.com/docs/en/models/sonnet-5-5/whats-new-sonnet-5-5)
- [迁移说明](https://platform.claude.com/docs/en/models/sonnet-5-5/migration-guide)
- [官方定价](https://platform.claude.com/docs/en/about-claude/pricing)
- [思考模式、effort 与成本](https://platform.claude.com/docs/en/build-with-claude/thinking-steering-and-cost)

模型支持文本、图像、PDF、工具和结构化输出。未将旧版 computer tool 协议伪装成新的 agent 协议；相关客户端需要按官方迁移指南升级。

## 别名与验证范围

以下四个请求名归于同一模型和独立价卡：

```text
claude-sonnet-5-5
claude-sonnet-5-5-thinking
claude-sonnet-5.5
claude-sonnet-5.5-thinking
```

| 账号 / 路径 | 上游 ID | 状态 |
| --- | --- | --- |
| Anthropic 协议官转 API Key，`/v1/messages` | `claude-sonnet-5-5` | 文本、adaptive、工具、非流式、between_tools 实测成功 |
| Kiro OAuth，AWS Q `generateAssistantResponse` | `claude-sonnet-5.5` | 文本、adaptive、原生工具实测成功 |
| 原生 Anthropic OAuth | 不新增专用持久化映射 | 缺少健康账号，未作执行验证 |
| Bedrock / Vertex / Antigravity / Kiro CLI Key | 不新增默认映射 | 未作执行验证，不声明支持 |

前端仅在 Claude/Kiro 选择器中新增条目。未验证平台不继承新增映射；Antigravity 生成配置也排除新模型。创建/编辑 Kiro 的默认映射来源是后端 API，API 失败时的前端预设回退也已覆盖。

## 上游证据

探测使用现有健康账号：AWS Q OAuth 账号 2713、官转 API Key 账号 2587。每项只作少量短请求；不据此推断生产 SLA。表中的耗时是完整探测耗时，首字节时间不是下游首语义 Token 时间。

| 路径 | 探测 | HTTP | 上游请求 ID | 耗时 |
| --- | --- | --- | --- | --- |
| AWS Q | 文本 | 200 | `ce91ad42-7577-40a3-88cc-f072fab718fe` | 4224ms |
| AWS Q | adaptive | 200 | `bbf046a2-a97f-4968-9bdb-f450688244bd` | 3600ms |
| AWS Q | 工具 | 200 | `8e8e419c-d596-44ee-b0ed-fa7a3ea2a5c8` | 1255ms |
| AWS Q | 最终项目 builder 的 adaptive | 200 | `2c4f3a8e-5e74-4824-9d08-198924b40a5d` | 1915ms |
| 官转 API Key | 文本 | 200 | `59bd5fff-ff65-4f18-a87a-b6420b8c0cdc` | 2038ms |
| 官转 API Key | adaptive | 200 | `40d3e069-dbc0-4329-8207-142531d09d2b` | 2147ms |
| 官转 API Key | 工具 | 200 | `33168e17-e66b-48dd-8347-544b6443a14b` | 2509ms |

官转三条流均有 usage 和恰好一个 `message_stop`；工具请求以 `tool_use` 结束。AWS Q 工具探测收到结构化 `toolUseEvent`，三个真实 EventStream 均经项目现有解析器解析，分别得到恰好一个 `message_stop`，工具流得到一个合法 `tool_use`。

AWS Q 模型元数据请求 `76cf32bf-731d-4b0c-ad97-b35bea0a4697` 返回 native ID、1M/128K、五档 effort 和 credit 倍率 1.3。这个倍率属于上游 credit 消耗，不用于替代上述官方美元价卡。编造的 `claude-sonnet-5.9` 返回 400 `INVALID_MODEL_ID`，没有回退成旧模型。

### 已确认的上游限制

AWS Q 元数据仍将此模型标为 experimental preview。虽然其 schema 列出 `between_tools`，使用最终项目 builder 构造该模式实测返回：

```json
{"message":"Improperly formed request.","reason":"REQUEST_BODY_INVALID"}
```

因此 AWS Q 的已验证默认路径为 adaptive。显式 between_tools 请求保持其语义并返回真实上游错误，不静默变成 adaptive。官转相同模式实测 200。

官转探测站的 `/v1/messages/count_tokens` 返回 404；网关 count_tokens 请求构建入口已验证，但不声称该站提供此能力。

## 请求兼容与历史保护

只针对上述四个新模型别名作以下兼容：

- 旧 `thinking.type=enabled` 转为 adaptive，去掉 `budget_tokens`。
- `disabled` 转为 between_tools；thinking 对象仅保留 type，xhigh/max effort 转为 high。
- 删除不支持的 sampler 字段 `temperature`、`top_p`、`top_k`。
- 不改原始 messages、system、工具定义、cache_control 或有效 thinking/signature 历史。
- 保留客户端强制 tool_choice，由上游真实校验，不悄悄修改成 auto。

Messages 普通转发、API Key 透传、Chat Completions、Responses、count_tokens 和两套 Kiro translator 均接入兼容逻辑。旧模型早退出，不扫描或重写其请求体。正常历史过滤识别 between_tools，保留有效签名块；不伪造签名、不跨模型复用签名。

Kiro 工具保护继续按映射后的 Claude 家族和工具能力生效。没有增加调度重试、冷却、超时或新模型名白名单，也没有改流生命周期。新增测试以合成未来 `claude-sonnet-9-1` 证明工具保护不依赖已发布模型名称。

## 定价与迁移

`251_add_claude_sonnet55_support.sql` 为新的前向迁移，未改历史迁移。

- 只扩展已具等价 Sonnet 5 / Opus 5.5 映射的 Kiro OAuth 和 Anthropic API Key 主账号；跳过删除账号、影子账号和通配映射，已有显式映射值优先。
- 只扩展最新版官转/AWS标准分组的模型数组，保留 enabled 标志、已有顺序并去重。订阅和历史分组的模型数组不自动扩展。
- 最新官转/AWS用户计费渠道各新增独立、启用、后台可见的 Sonnet 5.5 价卡，包含四个计费别名。
- 已有独立自定义价卡保留价格和区间；显式混合价卡拆分时复制原自定义价格与区间，移除其他启用行的重复别名，空行保留但禁用。
- `channel_account_stats_model_pricing` 与区间表独立处理，仅整理其已显式覆盖新别名的规则，不从用户售价推导上游成本。
- 仅匹配官方基础价格、无区间的用户价卡补齐缺失的 5m/1h 缓存写价，不覆盖自定义分层价格。

临时 PostgreSQL 集成测试覆盖上述情况，并验证连续执行两次后行数、内容、时间戳和价格一致。对等后台查询投影验证新增规则数量以及四别名独立行。

发布后仍须只读核对生产迁移和后台定价结果，再从实际用量记录分别复算输入、输出、缓存读、5m/1h 缓存写费用；本次没有生产结算验证。

## 测试结果

| 验证 | 结果 |
| --- | --- |
| `go -C backend test ./... -count=1` | PASS，包括完整 service、translator、handler 包 |
| `go -C backend test -tags=unit ./internal/handler/admin -count=1` | PASS，包括模型列表、默认映射和渠道价格 API |
| Sonnet 5.5 / 缓存并发 / 失败 / TTL / 历史保护的 focused `-race` | PASS |
| `go -C backend build ./...` | PASS |
| 临时 PG 的 `TestSonnet55Migration`，迁移执行两次 | PASS |
| `npm run typecheck` / `npm run build` | PASS |
| 新增和修改的前端测试文件 | PASS |
| 前端 `npm test -- --run` | 2605 通过，3 项严格复现的基线失败；373 个测试文件通过 |
| `git diff --check` / 定价 JSON 解析 | PASS |

前端完整测试在基线 `ad31837ab...` 的独立 worktree 和候选代码中使用相同命令、依赖运行。两者都只有未修改的 `HomeView.compact.spec.ts` 的以下三项失败：

1. `links unauthenticated visitors to login`：期望 `/login`，得到 `/rankings`。
2. `links authenticated users to their dashboard`：期望 `/dashboard`，得到 `/rankings`。
3. `links administrators to the admin dashboard`：期望 `/admin/dashboard`，得到 `/rankings`。

它们分类为严格复现的基线失败，不作为新增测试失败豁免依据。初次验证发现的未验证平台模型曝光问题已修正，并增加配置生成回归测试。

## 网关回归审查

发现：没有未解决的、归属于本次变更的 P0/P1。AWS Q between_tools 的 preview 限制按 P2 记录，默认配置使用已验证的 adaptive；不声明该模式在 AWS Q 上可用。官转 count_tokens 的 404 属于已确认站点能力限制。

```text
Verdict: PASS（限已验证的默认 adaptive 能力）
Reviewed range: ad31837ab2ba280ab5227f5f0d3fe3201bf014dd + 本分支任务变更
Affected matrix: Messages/Chat/Responses × stream/nonstream；普通/透传/计数请求；
                 官转 API Key/Kiro 两套 translator；5m/1h 缓存与独立计费
Four invariants: stream=PASS, cache-hit=PASS, recreate=PASS, latency=PASS（本地代码路径）
Residual risk: AWS Q between_tools 400；未验证的 Provider 不作支持声明；
               生产迁移、实际结算与发布后暖态性能仍需发布流程验证。
```

流完成证据：新模型与合成未来模型均通过三协议、两流模式的真实 Forward 入口测试。无效 standalone-call 尝试在客户端输出前保持私有，沿用既有至多一次恢复；正常短答不误伤；耗尽时不报告成功；成功流恰好一个合法终止事件。现有 EOF、错误、取消和超时测试由完整 service 测试覆盖；本次未修改其资源所有权。

缓存证据：只有新模型的最小阈值改为 512。模型身份、稳定前缀、TTL、提交、失败、并发去重和切号路径未改变。新模型的 5m/1h 首次创建、重复读命中且不重复创建有直接测试，既有 appended/failover/expiry/concurrent/failed-attempt 测试继续通过。

延迟证据：没有新网络调用、锁、sleep、数据库或 Redis 查询，没有新增流响应缓冲。200KB请求的 normalizer benchmark 为 229–233µs/op、134–140B/op、1 alloc/op。首输出计时和响应消费路径未改；未用稀疏上游探测替代暖态生产性能比较。

## 发布时检查

本次不自动发布。后续显式发布应使用 `sub2api-production-release`，以届时生产版本为新基线核对兼容和回退点，检查是否有新迁移号冲突，执行迁移后验证独立价卡和真实结算。不要以“列表可见”替代工具、终止事件和账单验证。
