# 文件结构：前端（web/）

> 本文件是契约：**只能创建、修改这里列出的文件**。增加、删除、改名、移动任何文件或目录，都必须先询问用户（D45）。标记说明见 [README.md](README.md)。

一个 Vite 项目，两个入口：
- `admin`：后台单页应用；
- `site`：公开页面上的交互脚本和样式。

两者共用 `src/shared/`。

## 目录之间的依赖规则

- `src/admin/` 和 `src/site/` 之间**不能互相导入**，共用的东西放进 `src/shared/`。
- `src/shared/` 不能导入 `admin/` 或 `site/` 里的任何东西。
- **公开页面要轻**：`site/` 里只有问答和语言建议弹窗用 Vue，其他交互一律用原生 TypeScript。`site/` 不能导入 Tiptap、Excalidraw、React。
- 调用后端统一走 `shared/api/`，不直接写 `fetch`。

## 根目录

| 文件 | 用途 |
|---|---|
| `AGENTS.md` | 前端给 AI 助手的说明：上面的依赖规则、组件写法（`<script setup lang="ts">`）、样式只用 Tailwind 和设计变量、如何添加 shadcn-vue 组件 |
| `CLAUDE.md` | 只有一行 `@AGENTS.md` |
| `package.json` | 依赖和脚本：`dev`、`build`、`lint`、`typecheck`、`gen:api`（生成接口类型） |
| `pnpm-lock.yaml` | 依赖锁定文件 |
| `vite.config.ts` | 两个入口；产物输出到 `../server/internal/webui/dist/` 并生成 manifest；开发时把 `/api` 转发给 Go；`@` 指向 `src/` |
| `tsconfig.json` | TypeScript 项目引用的入口 |
| `tsconfig.app.json` | `src/` 的 TypeScript 配置（严格模式） |
| `tsconfig.node.json` | `vite.config.ts` 等配置文件的 TypeScript 配置 |
| `eslint.config.js` | ESLint 配置，其中包括检查上面的目录依赖规则 |
| `components.json` | shadcn-vue 配置：组件放到 `src/shared/ui/` |
| `env.d.ts` | 全局类型声明（`.vue` 文件、Vite 环境变量） |
| `admin/index.html` | 后台入口页，开发时访问 `/admin/` |

## src/shared —— 两边共用

### src/shared/api

| 文件 | 用途 |
|---|---|
| `schema.gen.ts` | **[生成]** openapi-typescript 根据 `api/openapi.yaml` 生成的类型，不能手改 |
| `client.ts` | 请求客户端（openapi-fetch）：统一的错误处理；后台遇到 401 跳转登录页，遇到"必须改密码"跳转改密码页 |
| `sse.ts` | 读取 SSE 流：问答、调试台 |

### src/shared/tiptap —— 文档结构定义

只定义文档的结构（节点名、属性、HTML 对应关系），不包含编辑交互。后台编辑器在此基础上加交互；一致性测试用它调用 `generateHTML`。

| 文件 | 用途 |
|---|---|
| `schema.ts` | 当前格式版本号（必须和 Go 的 `doc.SchemaVersion` 一致）、块类型清单 |
| `extensions.ts` | 结构层面的完整扩展列表：官方扩展 + 下面的自定义节点 |
| `nodes/callout.ts` | 提示框 |
| `nodes/columns.ts` | 超级块和栏（2–4 栏、栏宽比例、不能嵌套） |
| `nodes/image.ts` | 图片：各尺寸文件、替代文字、图注、宽度 |
| `nodes/attachment.ts` | 附件 |
| `nodes/drawing.ts` | 画板：场景文件和两张 SVG 的 sha256 |
| `nodes/video.ts` | 视频：上传的视频或者平台视频，以及抓取到的公开信息 |
| `nodes/heading.ts` | 标题：只允许 H2–H4 |
| `nodes/internal-link.ts` | 站内链接：存文档 ID 和可选的块 ID，而不是 URL |

### src/shared/styles —— 样式

| 文件 | 用途 |
|---|---|
| `tokens.css` | 设计变量：颜色、字号、间距、圆角，分亮色和暗色两套 |
| `base.css` | Tailwind 入口；用 `@source` 让 Tailwind 也扫描 Go 模板（`server/internal/site/templates/`） |
| `content.css` | 所有块的样式，编辑器和公开页面共用，类名统一以 `nw-` 开头 |
| `code-site.css` | 公开页面代码高亮的配色（对应 chroma 的类名） |
| `code-editor.css` | 编辑器代码高亮的配色（对应 highlight.js 的类名），和上面使用同一套颜色 |

### src/shared 的其他部分

| 文件 | 用途 |
|---|---|
| `ui/` | **[生成]** shadcn-vue 组件，通过它的命令行工具添加。添加新组件也属于结构变动，要先询问用户 |
| `lib/utils.ts` | shadcn-vue 需要的 `cn` 等辅助函数 |
| `i18n/messages.zh-Hans.json` | 公开站点界面文字的原文（开发者维护）。其他语言由翻译流程生成，保存在 `data/i18n/<语言>/_ui.json`，由 Go 注入页面 |
| `i18n/t.ts` | 读取页面注入的界面文字，提供取文字的函数 |

## src/site —— 公开页面

| 文件 | 用途 |
|---|---|
| `main.ts` | 入口：启动各项轻交互；扫描页面上的挂载点，按需加载对应的 Vue 组件 |
| `site.css` | 公开站点的样式入口：引入 `tokens`、`base`、`content`、`code-site` |

### src/site/enhance —— 原生 TypeScript 的轻交互

| 文件 | 用途 |
|---|---|
| `theme.ts` | 亮暗主题切换，并记住选择 |
| `drawer.ts` | 手机端的文档树抽屉、本页目录抽屉 |
| `tree.ts` | 文档树的展开和折叠 |
| `toc-spy.ts` | 滚动时高亮本页目录中当前读到的标题 |
| `code-copy.ts` | 代码块的复制按钮 |
| `copy-markdown.ts` | "复制为 Markdown"按钮 |
| `banner.ts` | 关闭公告横幅，并记在浏览器本地存储里 |
| `video-facade.ts` | 平台视频点击后才加载播放器 |
| `hero-video.ts` | 首页背景视频：开启"减少动态效果"时改为显示封面 |
| `math.ts` | 页面含公式时，按需加载 KaTeX 并渲染 |
| `citation-highlight.ts` | 从 AI 引用跳转过来时（地址带 `#b-<块ID>`），高亮对应的块 |
| `beacon.ts` | 发送浏览统计 |

### src/site/islands —— Vue 组件

| 文件 | 用途 |
|---|---|
| `AskWidget.vue` | 文档页右下角的问答浮窗，能感知当前页面，可以展开到问答页 |
| `AskPage.vue` | 独立问答页 `/<lang>/ask` 的主体 |
| `LanguageSuggest.vue` | 语言切换建议弹窗：请求后端判断，用建议的那种语言显示提示 |

### src/site/chat —— 问答的共用部分

浮窗和问答页共用，对话也共用。

| 文件 | 用途 |
|---|---|
| `useChat.ts` | 对话状态：会话 ID 存在浏览器本地；发送问题；读取 SSE 事件 |
| `markdown.ts` | 把回答渲染成 HTML：不允许原始 HTML，并把块引用转换成引用标签 |
| `ChatPanel.vue` | 对话面板的外框 |
| `MessageList.vue` | 消息列表 |
| `MessageItem.vue` | 单条消息：回答正文、引用、反馈按钮 |
| `CitationChip.vue` | 引用标签：点击后跳转到原文对应的块 |
| `ProgressSteps.vue` | 回答过程中显示进度，例如"正在查找……找到 3 篇相关文章" |
| `ChatInput.vue` | 输入框 |
| `FeedbackButtons.vue` | "有帮助 / 没帮助" |
| `TurnstileGate.vue` | 人机验证：平时无感；判定为可疑时显示交互式挑战，并说明原因 |
| `BanNotice.vue` | 被封禁时的提示：原因和恢复时间 |

## src/admin —— 后台

| 文件 | 用途 |
|---|---|
| `main.ts` | 入口：创建应用，挂载路由和 Pinia |
| `App.vue` | 根组件 |
| `admin.css` | 后台的样式入口：引入 `tokens`、`base`、`content`、`code-editor` |
| `router.ts` | 路由表：每个页面需要的权限、是否只有超管能进；初始化未完成时跳转到向导 |

### src/admin/stores —— Pinia

| 文件 | 用途 |
|---|---|
| `auth.ts` | 当前账号、权限、是否必须改密码 |
| `docTree.ts` | 文档树（编辑页、链接选择器、移动文档时共用） |

### src/admin/layouts

| 文件 | 用途 |
|---|---|
| `AdminLayout.vue` | 后台的外框：侧边导航（按权限显示菜单项）、顶部账号菜单 |
| `AuthLayout.vue` | 注册、登录、改密码、初始化向导的外框 |

### src/admin/composables

| 文件 | 用途 |
|---|---|
| `usePermission.ts` | 判断当前账号有没有某项权限 |
| `useEditLock.ts` | 编辑锁：获取、心跳、被强制释放时切换为只读 |
| `useAutosave.ts` | 草稿自动保存，处理版本冲突 |
| `useUpload.ts` | 上传文件并显示进度 |
| `useImageVariants.ts` | 在浏览器里把图片压缩成几种宽度的 WebP |
| `useVideoPoster.ts` | 在浏览器里截取视频的一帧作为封面 |
| `useCostEstimate.ts` | 先请求费用预估，确认后再执行 |
| `useJobProgress.ts` | 轮询后台任务的进度 |

### src/admin/components —— 后台通用组件

| 文件 | 用途 |
|---|---|
| `DocTree.vue` | 文档树：拖拽排序和移动、新建、重命名 |
| `DocPicker.vue` | 选择一篇文档（以及其中的块），用于插入站内链接、设置跳转目标 |
| `AssetPicker.vue` | 从资源库选择文件 |
| `ModelPicker.vue` | 从已启用的模型里选择，按能力过滤 |
| `CostEstimateDialog.vue` | 显示预估费用并确认 |
| `DiffView.vue` | 版本差异 |
| `BlockSideBySide.vue` | 按块对齐的原文和译文对照 |
| `LockNotice.vue` | "XX 正在编辑"的提示 |
| `PageHeader.vue` | 页面标题和操作按钮区 |
| `ConfirmDialog.vue` | 通用的确认对话框 |

### src/admin/editor —— 文档编辑器

| 文件 | 用途 |
|---|---|
| `DocEditor.vue` | 编辑器主体：加载草稿、编辑、自动保存、编辑锁 |
| `extensions.ts` | 编辑器的完整扩展列表：`shared/tiptap/extensions.ts` + 唯一 ID、拖拽手柄、`/` 菜单、粘贴处理、节点视图 |
| `paste.ts` | 粘贴转换：Word、网页、Markdown 转成对应的块，清理多余的格式 |
| `upload.ts` | 粘贴图片、拖入文件时自动上传 |
| `DocMetaPanel.vue` | 文档属性：标题、路径名、SEO 描述、分享图 |
| `menus/SlashMenu.vue` | `/` 插入菜单 |
| `menus/slash-items.ts` | `/` 菜单的条目清单 |
| `menus/BubbleMenu.vue` | 选中文字时的格式工具条 |
| `menus/BlockMenu.vue` | 块拖拽手柄的菜单：转换类型、复制、删除 |
| `menus/LinkPopover.vue` | 编辑链接：外部地址或者站内文档 |
| `nodeviews/CalloutView.vue` | 提示框的编辑视图：切换类型 |
| `nodeviews/ImageView.vue` | 图片：上传进度、宽度调节、图注、替代文字 |
| `nodeviews/AttachmentView.vue` | 附件卡片 |
| `nodeviews/DrawingView.vue` | 画板预览，点击打开画板编辑 |
| `nodeviews/VideoView.vue` | 视频：上传、粘贴平台链接，抓取失败时手动填写 |
| `nodeviews/ColumnsView.vue` | 超级块：栏宽调节、增减栏 |
| `nodeviews/CodeBlockView.vue` | 代码块：选择语言 |
| `nodeviews/MathView.vue` | 公式：编辑 LaTeX 并预览 |
| `drawing/DrawingDialog.vue` | 全屏的画板编辑，保存时导出场景文件和亮暗两张 SVG |
| `drawing/mount-excalidraw.ts` | 按需加载 React 和 Excalidraw，用 `createElement` 挂载（不使用 JSX） |

### src/admin/views —— 页面

| 文件 | 页面 | 权限 |
|---|---|---|
| `auth/RegisterView.vue` | 注册超管（系统里没有账号时才会出现） | 无 |
| `auth/LoginView.vue` | 登录 | 无 |
| `auth/ChangePasswordView.vue` | 修改密码（初始密码登录后被强制进入） | 登录 |
| `setup/SetupWizardView.vue` | 初始化向导的外框和步骤切换 | 超管 |
| `setup/steps/SiteStep.vue` | 站点名称 | 超管 |
| `setup/steps/LanguageStep.vue` | 基准语言 | 超管 |
| `setup/steps/ChannelStep.vue` | 接入至少一个模型渠道 | 超管 |
| `setup/steps/RestoreStep.vue` | 从备份恢复 | 超管 |
| `DashboardView.vue` | 仪表盘 | 查看统计 |
| `docs/DocsView.vue` | 文档树 + 编辑器 | 编辑文档 |
| `docs/DocPreviewView.vue` | 草稿预览 | 编辑文档 |
| `docs/DocHistoryView.vue` | 历史版本、差异、回滚 | 发布文档 |
| `docs/TrashView.vue` | 回收站 | 发布文档 |
| `assets/AssetsView.vue` | 资源库、引用关系、清理 | 管理资源库 |
| `i18n/LanguagesView.vue` | 启用和停用语言 | 管理语言 |
| `i18n/TranslationProgressView.vue` | 翻译进度、待复核 | 管理译文 |
| `i18n/TranslationEditView.vue` | 原文和译文对照编辑 | 管理译文 |
| `i18n/GlossaryView.vue` | 术语表 | 管理译文 |
| `ai/ChannelsView.vue` | 模型渠道：增删改、密钥、测试连接、启用模型 | 超管 |
| `ai/TiersView.vue` | 档位和任务的模型分配、重建索引 | AI 设置 |
| `ai/AnswerStyleView.vue` | 回答风格 | AI 设置 |
| `ai/QaRecordsView.vue` | 问答记录、设为精选回答 | 问答管理 |
| `ai/QaCacheView.vue` | 回答缓存的设置 | 问答管理 |
| `ai/QaDebugView.vue` | 问答调试台 | AI 设置 |
| `ai/GuardView.vue` | 防滥用：封禁列表、可疑事件、站点概述 | AI 设置 |
| `site/SiteSettingsView.vue` | 站点名称、logo、主题色、页脚 | 站点设置 |
| `site/HomeHeroView.vue` | 首页主视觉 | 站点设置 |
| `site/BannerView.vue` | 公告横幅 | 站点设置 |
| `stats/StatsView.vue` | 访问统计 | 查看统计 |
| `system/AccountsView.vue` | 账号管理 | 超管 |
| `system/AuditLogView.vue` | 操作日志 | 超管 |
| `system/BackupView.vue` | 备份、下载、定时备份 | 超管 |
| `system/SystemSettingsView.vue` | 可信代理、Turnstile、每日花费上限、远程仓库 | 超管 |
| `system/JobsView.vue` | 后台任务队列 | 超管 |
| `ProfileView.vue` | 个人：修改密码、退出所有设备 | 登录 |
| `NotFoundView.vue` | 后台内的 404 | 无 |

## 测试文件 **[测试]**

以下文件**编写前必须征得用户同意**（D38），这里只规定放在哪里：
- 单元测试和被测文件放在一起：`xxx.ts` 对应 `xxx.test.ts`（Vitest）。
- 前后端一致性测试：`src/shared/tiptap/consistency.test.ts`，读取根目录的 `fixtures/`。
- 端到端测试：`e2e/`（Playwright），包括 `e2e/playwright.config.ts` 和 `e2e/*.spec.ts`。
