# /docs 实测审查与修复结果

日期：2026-10-03。独立分支：`codex/docs-audit-20261003`，基线：`47790e76f`。
预览：`http://127.0.0.1:3187/docs`。
目标与可复用提示词：[DOCS_GUIDE_REPAIR_PROMPT.md](DOCS_GUIDE_REPAIR_PROMPT.md)。

## 结论与证据

用户提供的三张截图和本轮自行访问线上页面的结果一致。主要视觉问题来自 **Vue 样式作用范围失效**，不是刻意的排版。`DocsGuideView.vue` 中通过 `defineComponent` / `h()` 创建的子组件后代没有父组件的 `data-v-*` 标记；原 scoped 选择器无法匹配标题、步骤、图标、代码头与 pre。浅色主题代码因此继承黑色正文颜色，却仍处于黑色代码容器中。

线上浏览器测得章节 h2 为 **16px / 400**，代码 pre 为 **0px padding**。修复后六个章节 h2 均为 **28px / 820**；代码颜色为 `rgb(229, 237, 248)`，pre padding 为 **16px**，代码头与编号列表恢复 flex 布局。样式改为 `.docs-page` 命名空间，静态检查确认所有选择器均局限于文档页面。

| 问题 | 实测影响 | 源头修复 |
| --- | --- | --- |
| 子组件 scoped 样式不匹配 | 黑底黑字、标题无层级、图标/编号/正文错位 | 页面命名空间覆盖渲染函数子树，同时隔离其他路由 |
| 三个主题占两列 | 第三个主题旁出现大片空白 | 最后一张奇数主题跨整行，说明文字限定阅读宽度 |
| 长 Codex 配置并列 | 文件列过窄，代码阅读困难 | 配置文件顺序纵向展示，代码块内部滚动 |
| 健康检查使用 HEAD | 线上 `curl -I /health` 返回 404，被误解为网络不可达 | 使用 GET；说明仅检查网关可达 |
| 首次请求只有 Responses | Claude 用户无法照抄验证；协议和模型容易混用 | Messages、Responses、Chat Completions 三种请求独立生成 |
| Windows 选择未影响请求示例 | Unix 反斜杠命令被复制进 PowerShell | 统一系统选择；PowerShell 使用 Invoke-RestMethod 与 UTF-8 JSON |
| 推荐模型被当成分组权限 | 对不支持默认模型的 Key 发起无效请求 | 模型 ID 可编辑；解释推荐值与 Key 权限的区别 |
| 地址可能重复拼 /v1 | 公共地址含后缀时生成 /v1/v1 | 统一基础地址归一化并保留部署路径前缀 |
| 手机目录整体隐藏 | 只能滚动长页面寻找章节 | 手机原生 details 目录，跳转后收起 |
| 系统控件是假 tablist | 无 tab 语义与键盘状态 | 原生 radio/select，多个选择位置同步 |
| 复制失败仍出现成功图标 | 用户误以为内容已复制 | 使用 composable 的布尔结果，仅成功时更新状态 |
| 通用教程宣称与生成器完全一致 | 未体现分组模板/模型目录/额外配置 | 明确通用样例范围，专用配置优先使用控制台生成器 |
| VS Code 与 CLI 设置混淆 | 仅配置 CLI 文件，扩展仍显示登录界面 | 补充官方扩展的用户设置和重载步骤 |
| Desktop 流程过于概括 | 不清楚菜单路径或适用会话范围 | 完整菜单路径；明确桌面本地 Code 会话、独立设置与组织策略 |
| 小屏长地址截断、按钮过小 | 接口地址尾部不可见，点击复制困难 | URL 换行，移动复制/主题等控件扩大到 44px |

## 配置核验

Anthropic 官方 [网关接入说明](https://code.claude.com/docs/en/llm-gateway-connect) 说明：VS Code 扩展的启动认证读取 `claudeCode.environmentVariables`；CLI 的设置不能代替扩展自身的认证检查。桌面应用使用独立的 Third-Party Inference 配置，本地菜单、组织策略和会话能力也有区别。新教程分别说明这些入口。

根据 [Claude Code 安装说明](https://code.claude.com/docs/en/setup)，原生安装不需要 Node.js；新教程不再将 Node/npm 当作所有用户的必备环境，提供官方安装入口和版本检查命令。

公共 Codex 示例保持 Responses，采用 HTTP/SSE 基线，将凭证存储显式设为 `file`，与旁边的 auth.json 教程一致。去掉无关的旧网络/WSL/功能开关，保留动态推荐模型和 reasoning effort。需要模型目录、分组模板或 WebSocket 时，引导使用实际 Key 的生成器。依据：[配置参考](https://developers.openai.com/codex/config-reference)、[认证说明](https://developers.openai.com/codex/auth)。未调整控制台生成器或客户端已有配置。

## 验证结果

- 4 个测试文件、**17 项测试全部通过**：配置与协议联动、复制失败状态、锚点、JSON 请求结构、真实 Bash 解析、i18n key 完整性和消息编译。
- Unix 请求通过模拟 curl 捕获真实 Bash 参数验证，未发出模型请求；模型中的引号与命令替换文本保持为 JSON 字面值。
- Windows 示例的 here-string JSON 与 UTF-8 字节格式通过测试及浏览器复制核对；本机未运行真实 Windows PowerShell，不宣称完成 Windows 上的实际模型调用。
- `pnpm run build` 通过（包含 i18n 检查、vue-tsc 和 Vite）；针对改动文件 ESLint 通过；SFC 解析、CSS 命名空间检查、`git diff --check` 通过。构建有仓库既有大 chunk 提示，未增加依赖。
- 浏览器检查 **1440px 桌面、390px 手机、320px 小屏**，中英文与明暗主题。390px 可用内容宽度 382px、scrollWidth 382px；320px 可用内容宽度 312px、scrollWidth 312px，没有页面横向溢出。
- 390px 下代码 pre 宽度 348px，长 PowerShell 内容宽度 723px，仅代码块横向滚动；接口地址直接换行。
- 手机目录实际点击后正确设置 hash 并收起；章节 top 约 96px，大于顶部栏 bottom 61px，标题不被遮挡。
- 实际复制后剪贴板内容与显示代码一致，按钮显示“已复制”；语言切换后按钮使用对应语言。
- 已实测 FAQ 展开；全部目录目标均存在。保留首页、价格、服务状态、注册/控制台入口与原章节，未改这些路由的实现。

## 截图与预览边界

自行采集的截图位于本任务 worktree 的 `docs/reviews/docs-guide-20261003/`，为本地验收附件，不进入生产构建：

- `before-desktop-light.png` / `before-desktop-dark.png`：线上代码与主题证据。
- `after-desktop-models.png`：标题、图标与奇数主题布局。
- `after-desktop-code-light.png`：修复后的代码与协议选择。
- `after-desktop-cli.png` / `after-desktop-dark.png`：CLI 与主题。
- `after-mobile-navigation.png` / `after-mobile-code.png` / `after-mobile-faq.png` / `after-mobile-english.png`：手机交互与可读性。
- `after-desktop-full.png` / `after-mobile-full.png`：完整页面。

预览后端仅使用线上公开设置的只读快照，不转发任何生产写操作。测试过程中未用真实用户密钥、未调用模型、未产生模型扣费。本轮不发布生产，不修改共享主工作区 WIP。

本地重启入口：先运行 `/tmp/sub2api-docs-preview-api-20261003.py`；再进入本 worktree 的 frontend，设置 `VITE_DEV_PROXY_TARGET=http://127.0.0.1:18087` 并运行 `pnpm exec vite --host 127.0.0.1 --port 3187 --strictPort`。
