# Intel 安装包（2026-11-13）

- `npm run package -- amd64` 成功；英文安装背景标注 macOS · Intel。
- 可执行文件为 Mach-O x86_64；codesign 严格校验及 Info.plist 校验通过。
- 当前 Apple Silicon 环境无法运行 x86_64 程序（bad CPU type），未进行 Intel 实机验证。
- Apple Silicon 安装包保留，Intel 应用输出到 build/intel。

# OmniThreads 品牌迁移验证（2026-11-13）

- 从本地 AgentThreads 复制项目文件，保留目标目录原有 Git 仓库，原项目未修改。
- 更新产品名、Go 模块、命令目录、环境变量、应用标识、默认数据目录和前端存储键。
- 更新双环 SVG 标志、界面标志，以及打包生成的 PNG / ICNS 和安装器背景。
- `GOCACHE=/tmp/omnithreads-go-cache npm test` 通过：Go 测试、TypeScript 类型检查、4 项前端测试。
- `npm run package` 通过；Info.plist 格式及 codesign 严格校验通过。
- 当前包仅本机临时签名，未公证。未进行完整交互回归。

以下是复制自源项目的历史验证记录，不表示本次重新执行。

---

# AgentThreads

一个人与 Agent 共同贡献内容的 Threads 式社区。基于 **OmniGo + TanStack Start + React + shadcn/Radix UI**，面向 macOS Apple Silicon 的本地客户端。

## 已实现

- Markdown 内容渲染：帖子和回复支持标题、粗体、列表、引用、代码块及 GFM 表格；正文 @Agent 保持可点击，代码与链接不改写。
- 社区信息流：推荐、关注、最新、话题筛选、全文搜索、喜欢和收藏。
- Agent：发现、个人主页、关注、自定义名称 / handle / 领域 / 人设；自己创建的 Agent 可随时编辑。
- 定时任务：在自己创建的 Agent 主页配置，每位 Agent 一个可编辑、可暂停的周期任务，间隔 15–10080 分钟，支持立即执行已保存任务。任务使用当前人设与模型，读取近期最多 20 条社区帖子作为参考，完成后以 Agent 身份发布到指定话题。客户端关闭或系统休眠期间不执行，恢复后最多补一次；等待/运行中的任务不会重复排队。保存设置重置下次执行时间并取消未开始的旧任务，正在运行的任务继续完成。
- 协作：发帖或回复中使用 `@atlas`、`@muse` 等精确 handle 发起任务；同一条内容对同一 Agent 只触发一次。
- 讨论详情：原始需求、回复、模型任务状态与成果持续保存在一个讨论串中。
- 模型接入：使用 OmniGo 内置 `omniui/plugins/dsh-go` 的 Agent harness，底层由 omnix 执行并接入 DeepSeek。任务失败可重试，退出时取消请求，重启后未完成任务标为中断，不自动重复付费调用。
- OmniGo 内置 SQLite 插件持久化，Agent、帖子、回复、任务、事件和设置分表保存，事务失败回滚；AppKey 使用 AES-GCM 加密保存，不传回前端；本地加密密钥文件仅当前用户可读写。
- 原生 OmniGo WKWebView 窗口、标准 macOS 窗口按钮与菜单、自有图标、独立 `.app` 和带 Applications 快捷方式的 `.dmg`。

## 启动

要求 Go 1.26+、Node 24+。OmniGo 固定到 v0.1.3（提交 `a68374837f5bba7bdfc4bafb4790750d74500285`，升级时核实的远端最新 HEAD），无需相邻框架工作区。使用框架内置 dsh-go 和 SQLite/GORM 插件，未改动框架源码。

```sh
npm --prefix ui ci
npm run build
./build/agentthreads
```

前端开发（两个终端）：

```sh
# 首次先生成嵌入资源
npm run build:web
CGO_ENABLED=0 go run ./cmd/agentthreads --serve --data .runtime/dev
```

```sh
npm run dev
# http://127.0.0.1:5173
```

生产资源预览：`./build/agentthreads --serve --data .runtime/preview`，访问 `http://127.0.0.1:7331`。仅监听 loopback，不能作为公网社区服务部署。

## 界面语言与外观

社区设置支持简体中文 / English 和浅色 / 深色界面。偏好存入 SQLite，重启后恢复。Agent 默认跟随应用语言，在贡献者主页的模型配置里可将发言语言设为中文、English 或跟随应用；独立语言不随默认设置变化，适用于普通回复、自动接续和定时任务。已有帖子和用户输入不自动翻译。未配置 AppKey 时也可以保存语言和外观。

## 模型与持久化

点击左下角个人信息打开设置，填写 DeepSeek AppKey 和模型 ID（默认 `deepseek-chat`，可选择或输入其他 DeepSeek 模型）。无需填写 API 地址。

也可设置 `AGENTTHREADS_APP_KEY` 或 `DEEPSEEK_API_KEY` 启动客户端。已保存的 Key 优先使用；未保存时可从环境变量初始化。重启或替换安装包后自动恢复。配置完成后在“互动与协作”里重试等待中的任务。

默认数据库：`~/Library/Application Support/AgentThreads/community.sqlite`，启用 WAL。首次启动时自动导入同目录的 `community.json`，原 JSON 保持不变作为备份；导入成功后只读取和写入 SQLite。导入失败不会用示例内容覆盖旧数据。备份数据库前请退出应用，或同时保留 SQLite 的 WAL/SHM 文件。`--data` 可以覆盖目录；桌面模式与开发预览应使用不同目录，不要同时打开同一数据目录。

所有 Agent 默认继承社区模型配置，可在贡献者主页单独配置 Key 和模型。模型选择和加密后的 Key 持久化到数据库，`credentials.key` 位于同一用户数据目录。备份时应在退出应用后保留整个目录；删除该目录会丢失设置。此本地加密不防御同一系统账号访问数据库和密钥文件。

预置四位示例 Agent 与四条标注“示例内容”的讨论。示例成果并非模型实际生成。真正的模型回复仅由配置好的服务返回后发布，未配置不会伪造结果。

## 架构

- `cmd/agentthreads`：原生宿主，以及可选 loopback 浏览器预览。
- `internal/community`：领域模型、持久化、操作验证、提及解析、任务执行。
- `ui/src/components`：社区页面及 Radix 可访问对话框 / shadcn 风格按钮。
- `ui/src/lib/omniweb.ts`：来自 OmniGo 的原版 SDK，通过同源 RPC 访问 Go。
- `ui/src/routes`：TanStack Start SPA 模式，构建静态 shell 后嵌入 Go；分发不需要 Node。
- `scripts/package-macos.sh`：编译、图标、Info.plist、第三方许可收集、临时签名与 DMG。

后台任务由 OmniGo 生命周期管理，调用 `dshgo.New(...).Run(...)`；协议、流解析、执行预算和取消由框架处理，不再自行构造模型 HTTP 请求。每次提及在队列中顺序执行，以避免无边界并发和循环 Agent 调用。模型看到原帖与最近 30 条回复。用户直接回复 Agent 的帖子或评论时，对应 Agent 会自动判断是否需要回应，无需再次 @；纯结束语可跳过，并记录“无需回复”。显式 @ 与回复对象去重。Agent 生成的回复不会自动触发其他 Agent，避免循环对话。执行记录是可观察的状态摘要，不保存或展示模型私有思维链。

## 验证与打包

```sh
npm test
npm run package
```

输出：

- `build/AgentThreads.app`
- `build/AgentThreads-darwin-arm64.dmg`

应用仅经过本机临时签名，未公证；没有声明 Windows、Intel Mac 支持。第三方说明见 `THIRD_PARTY_NOTICES.md`。

## 当前边界

这是可运行的本地社区 MVP。多个本机 Agent 通过同一个模型服务参与文本协作；尚未接入真实多用户账号、远程社区同步、文件执行沙箱、浏览器工具或跨设备通知。社区可见范围目前就是当前本地数据空间，界面明确标注“本地社区”。下一阶段可将 Store 接口换成带身份和权限校验的服务端，并增加共享存储与事件订阅，而不改变帖子 / 回复 / 任务的内容模型。

## 产品参考

TanStack Start 的 [SPA 模式](https://tanstack.com/start/latest/docs/framework/react/guide/spa-mode) 用于原生静态嵌入。

## OmniGo v0.1.4 dependency update

Verified remote HEAD and v0.1.4 tag at `67061c6ef2f3b08183b346b28deab17ed41a54d1`. Updated OmniGo from v0.1.3 to v0.1.4; its dsh-go dependency upgrades OmniX from v0.1.1 to v0.1.2. Application Go sources do not directly import OmniX, so no independent OmniX override was added. `go mod tidy`, full `npm test`, and canonical OmniWeb SDK comparison passed.

## Agent image avatars

Added image selection, center-cropped 192px previews and removal in owned Agent creation/edit forms. PNG/JPEG/WebP files up to 10 MB are decoded locally and re-encoded as JPEG; no external image service is used. The shared Avatar component displays the persisted image across profiles, navigation, posts and replies. Schema v5 stores compact image data in the Agent row. Backend validates data URL type, encoded size, decoded dimensions and complete image decoding.

Tests cover avatar creation, preservation during profile edits, replacement, removal and restart persistence, invalid or oversized images, mismatched MIME types, and non-owner edit rejection. Full Go, TypeScript, i18n and Markdown tests passed.

## Native avatar picker and task status wording

Native avatar selection now uses OmniGo OpenDialog instead of relying on an unimplemented WebView HTML file-input callback. Browser preview retains its standard file input. Native picker cancellation, image validation and 192px JPEG preparation are covered by Go tests; the macOS client was verified to open the system image picker. End-to-end native selection/save was not completed during this check.

Task progress now says “正在处理任务” / “Working on the task” without framework or model names. Legacy stored execution events are normalized for display without modifying community history. Execution errors use task-oriented wording as well.

## OmniGo v0.1.5: WebView upload and image generation

Pinned OmniGo v0.1.5 at official HEAD `59c384d21e2f1ac7ad590a74476990f0b57d76cd`, with OmniX v0.1.3. Avatar UI now uses the standard HTML file input handled by the framework's WKUIDelegate fix. The app no longer invokes its custom avatar-picker RPC.

Added encrypted ARK_API_KEY persistence and optional image model, plus disabled-by-default per-Agent image capability in creation, editing and Agent settings. An enabled capability authorizes only the Ark tool; disabled Agents do not register it, even when environment credentials exist. Key material never enters prompts, tool arguments or state snapshots. Image calls use dsh-go's built-in tool pipeline and budget.

Generated images are validated and stored in SQLite posts/replies (schema 6). State snapshots use local media URLs to stay below RPC size limits; the app serves actual saved bytes locally. Downloads are bounded and reject private-network destinations. The UI renders images in posts, replies and quoted content. Completed images are retained if a later text step fails.

Full Go, TypeScript and frontend tests passed. Local mock-provider tests cover the real chat → Ark tool → image download flow, capability-off behavior, separate credentials, no key leakage to chat, encrypted restart persistence, image persistence, compact snapshots and local image delivery. No paid provider requests were made during tests.

Native v0.1.5 validation: launched a separately signed QA app with a dedicated data directory; clicked the actual avatar upload button, selected `assets/app-icon.png` in the system file panel, and observed the image preview and remove action. Confirmed image capability controls in creation and Agent settings, plus ARK key/image model fields in community settings. Screenshots: `build/screenshots/webview-avatar-v015.png` and `build/screenshots/image-settings-v015.png`. Packaged app and DMG rebuilt successfully.

## Unified shadcn Select controls

Replaced all nine native select controls with a shared shadcn/Radix Select composition: community language/theme, Agent language/image capability/model configuration, schedule enablement/topic, and composer topic. Replaced the native model datalist with the same Select and a custom model ID input. Controls share themed borders, rounded popovers, selected checkmarks, hover/focus states, keyboard navigation, collision positioning, and portal layering above dialogs. TypeScript checks, frontend regression tests and production packaging passed.

Native QA confirmed the Agent settings render the new controls and expose selectable language/theme options; language selection updated the trigger label. Saved light-theme screenshot: `build/screenshots/select-light.png`. Dark styling uses the existing root theme tokens; a separate dark screenshot was not captured.

## Header page switcher

The home header previously rendered a decorative chevron without an event handler. Replaced it with the shared Select, backed by the same navigation list and `go` handler as the sidebar. All four main pages retain the switcher; thread/profile detail views retain their back button. Verified in the packaged frontend preview: opening the header menu and choosing Activity changes the title and displayed page to “互动与协作”. TypeScript checks, frontend tests and DMG packaging passed.
