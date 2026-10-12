# 开源参考资料总结

> 调研时间：2026-10-12。每个项目一份文档，记录它是什么、和我们相关的部分、借鉴什么、注意事项。所有说法都对照过各项目的源码、README 或官方文档；来自二手资料的，在对应文档里注明了。

## 按方向汇总

### 翻译

| 项目 | 许可证 | 主要借鉴 |
|---|---|---|
| [translation-agent](translation-agent.md) | MIT | 整篇原文作上下文，用标签圈出要翻译的部分；每批约 1000 token；可选的"反思改进" |
| [Lingo.dev](lingo-dev.md) | Apache-2.0 | 按内容哈希增量判断（含"内容相同就复用"）；每批 25 条、约 250 词；带上已有译文作参考；术语表符合度检查 |
| [BabelDOC](babeldoc.md) | AGPL-3.0 | 每批 200 token 或 5 段；条数校验和逐段降级；"译文等于原文"判为失败；自动术语抽取；带上章节标题 |
| [LLM-Subtrans](llm-subtrans.md) | MIT | 出错先对半切分重试；撞到长度上限时去掉上下文重试；列出失败的部分 |
| [lobe-i18n](lobe-i18n.md) | MIT | Markdown 先解析成语法树，只把文字交给模型 |

**结论**：成熟项目都用**小批次**，不依赖模型标称的输出上限，可靠性靠"小批次 + 条数校验 + 逐级降级"来保证。

### 长期记忆

| 项目 | 许可证 | 主要借鉴 |
|---|---|---|
| [Letta](letta.md) | Apache-2.0 | 记忆块（标签、内容、上限、说明）、一个块挂到多个智能体上、只读块。和我们的设计几乎一致 |
| [mem0](mem0.md) | Apache-2.0 | 整理记忆时逐条判断：新增、更新、删除、不变 |

**结论**：我们"一套记忆、各任务按需取用"的结构，和 Letta 的共享记忆块是同一种成熟做法。

### 回答缓存

| 项目 | 许可证 | 主要借鉴 |
|---|---|---|
| [GPTCache](gptcache.md) | MIT | 精确匹配 + 语义匹配两层；用模型复核是否同义；阈值必须按嵌入模型校准 |
| [RedisVL](redisvl.md) | MIT | 默认阈值偏严格；按过滤字段分区；有效期 |

**结论**：最大的风险是误命中。默认阈值要严格，缓存要分区，阈值要能校准。

### 防滥用

| 项目 | 许可证 | 主要借鉴 |
|---|---|---|
| [OWASP 大模型十大风险](owasp-llm-top10.md) | — | LLM10"无节制消耗"：每次调用限制输出长度、限制输入和历史长度、按花费而不只是次数限流、多层限流 |
| [NeMo Guardrails](nemo-guardrails.md) | Apache-2.0 | 用小模型做话题护栏；护栏叠得越多误伤越多 |
| [LLM Guard](llm-guard.md) | MIT | 已归档，不依赖 |
| [Anubis](anubis.md) | MIT | 自托管的工作量证明挑战，可作为 Turnstile 的备用方案 |
| [CrowdSec](crowdsec.md) | MIT | 反向代理层的恶意 IP 拦截，作为可选的部署加固 |
| [go-limiter](go-limiter.md) | Apache-2.0 | Go 限流库，带 HTTP 中间件 |

### 检索问答

| 项目 | 许可证 | 主要借鉴 |
|---|---|---|
| [Contextual Retrieval 示例](contextual-retrieval.md) | MIT | 入库补背景的官方实现 |
| [PageIndex](pageindex.md) | MIT | 沿目录树推理检索，可以作为检索员的补充工具 |
| [RAGFlow](ragflow.md) | Apache-2.0 | 切块可解释、可查看 |
| [DocsGPT](docsgpt.md) | MIT | 文档站上的问答小部件 |
| [Onyx](onyx.md) | MIT（社区版） | 先建索引比临时搜索更快更便宜 |
| [Dify](dify.md) | 修改版 Apache-2.0 | 检索参数和模型供应商界面；不借用代码 |
| [sqlite-vec](sqlite-vec.md) | Apache-2.0 | 规模变大后的向量方案；和纯 Go 驱动不兼容 |
| [simple](simple-tokenizer.md) | MIT / GPL-3.0 | 中文分词扩展；和纯 Go 驱动不兼容，印证二元组方案 |

### 整体架构与站点

| 项目 | 许可证 | 主要借鉴 |
|---|---|---|
| [Gitea](gitea.md) | MIT | **Go 模板 + Vue 组件 + Vite + 单个可执行文件**，和我们的架构完全一致 |
| [Memos](memos.md) | MIT | 前端产物嵌入 Go 程序；SQLite 存储层 |
| [VitePress](vitepress.md) | MIT | 三栏布局；"最后更新于"取自 git |
| [Starlight](starlight.md) | MIT | 未翻译时显示原文并提示"尚未翻译" |
| [llms.txt 规范](llms-txt.md) | Apache-2.0 | `.md` 版本；用 HTTP `Link` 响应头指向 Markdown 版本 |
| [Outline](outline.md) | BSL 1.1 | 只参考交互，**不能借用代码** |
| [Gollum](gollum.md) | MIT | 印证"文件即数据 + git 记录历史" |
| [思源笔记](siyuan.md) | AGPL-3.0 | 编辑器交互、文件为真实数据 |

### 编辑器

| 项目 | 许可证 | 主要借鉴 |
|---|---|---|
| [Docmost](docmost.md) | AGPL-3.0 | **最重要的参考**：Tiptap 扩展清单和我们的块类型高度重合（提示框、多栏、公式、Excalidraw、视频嵌入、附件），同为 AGPL-3.0，可以借用代码 |
| [Halo 的编辑器](halo-editor.md) | GPL-3.0 | Vue 版的 Tiptap 节点视图和命令菜单 |
| [Umo Editor](umo-editor.md) | MIT | Vue 3 + Tiptap 3 的集成写法 |
| [Tiptap](tiptap.md) | MIT | 编辑器基础 |
| [Excalidraw](excalidraw.md) | MIT | 画板组件 |
| [lite-youtube-embed](lite-youtube-embed.md) | Apache-2.0 | 点击后才加载播放器 |
| [browser-image-compression](browser-image-compression.md) | MIT | 浏览器端生成多种尺寸 |
| [fracdex](fracdex.md) | CC0-1.0 | 文档树排序的分数索引（Go 版和 JS 版） |

### 模型接入

| 项目 | 许可证 | 主要借鉴 |
|---|---|---|
| [opencode](opencode.md) | MIT | 渠道配置、主模型与轻量模型（已吸收进 D44） |
| [models.dev](models-dev.md) | MIT | 内置模型目录 |
| [New API](new-api.md) | AGPL-3.0 | **Go 端协议适配器的参考实现**（OpenAI、Anthropic、Gemini），同为 AGPL-3.0 |
| [One API](one-api.md) | MIT | 渠道管理界面、失败重试和切换 |
| [LiteLLM](litellm.md) | MIT（主体） | 备用切换、花费统计、预算 |

### 统计

| 项目 | 许可证 | 主要借鉴 |
|---|---|---|
| [Plausible](plausible.md) | AGPL-3.0 | 每日盐哈希识别独立访客（D31 已采用） |
| [GoatCounter](goatcounter.md) | 修改版 EUPL | Go 写的无 cookie 统计 |
| [Umami](umami.md) | MIT | 统计界面 |

### 存储与运维

| 项目 | 许可证 | 主要借鉴 |
|---|---|---|
| [go-git](go-git.md) | Apache-2.0 | 版本历史 |
| [Backlite](backlite.md) | MIT | **可以直接用来实现任务队列**，支持 modernc.org/sqlite |
| [goqite](goqite.md) | MIT | 更底层的 SQLite 队列，备选 |
| [Litestream](litestream.md) | Apache-2.0 | 可选的 SQLite 持续备份 |
| [golang.org/x/text/language](x-text-language.md) | BSD-3-Clause | 解析和匹配 `Accept-Language` |
| [maxminddb-golang](maxminddb-golang.md) | ISC | 读取 GeoIP 数据库 |

## 许可证须知

我们是 AGPL-3.0（D43）：
- **可以借用代码**（保留版权声明、注明出处）：MIT、Apache-2.0、BSD、ISC、CC0、AGPL-3.0 的项目。GPL-3.0 的代码可以和 AGPL-3.0 组合使用，借用前确认具体文件的许可证。
- **只参考思路，不借用代码**：Outline（BSL 1.1）、Dify（附加条件）、GoatCounter（修改版 EUPL），以及企业版目录里的文件（Docmost、Onyx、LiteLLM）。
- **不依赖**：LLM Guard（已归档）。

## 对设计的改进建议（待用户确认）

调研得出的改进建议如下。**确认之后才会改进设计文档和文件结构。**

### 翻译（i18n.md）

1. **小批次**：每批预估输出不超过约 1000 token（可调），在块的边界切分，各批大小尽量平均，优先在标题处切开。
2. **整篇原文作上下文，用标签圈出本批要翻译的块。**
3. **失败逐级降级**：按块 ID 返回并校验条数 → 不对就对半切分重试 → 最后逐块单独翻译；译文和原文相同视为失败；最终仍失败的块列在后台。
4. **同一篇文档的批次按顺序执行**，后面的批次带上前面批次的译文。
5. **自动术语抽取**：第一次把一篇文档翻译成某种语言之前，先抽取关键术语并给出译法，作为"待确认"的词条进入术语表。
6. **翻译记忆**：原文哈希相同的块（复制、移动来的内容）直接复用已有译文，不调用模型。
7. **术语表符合度检查**：原文里出现了术语表的词，译文里却没用规定的译法时，标记出来。
8. **未翻译提示**：显示原文代替译文时，加一句"该内容尚未翻译"。
9. **可选的反思润色**：按语言开启，默认关闭（成本约 3 倍）。

### 长期记忆（memory.md）

10. **每个条目单独设上限**，每个任务注入的总量仍然不超过 5KB。
11. **注入时带上条目的说明**，告诉模型这条记忆的用途。
12. **条目可以设为只读**：助手不能改，只有人能改。
13. **助手整理时逐条判断"新增、更新、删除、不变"**，用来去重和处理冲突，结果以差异的形式展示。

### 回答缓存（ai-qa.md）

14. **先精确匹配**（规范化后的问题哈希），再做语义匹配。
15. **新增"重排模型复核"的命中方式**，介于"阈值"和"AI 判断"之间，比 AI 判断便宜。
16. **按语言、嵌入模型、问答记忆的版本分区**；更换嵌入模型时整个缓存失效。
17. **阈值校准工具**：调试台里用历史问答测误命中率；仪表盘显示命中率和误命中数（被点了"没帮助"的缓存回答）。

### 防滥用（ai-guard.md）

18. **按 OWASP 补上明确的上限**：问题长度、对话历史长度、每个任务的输出上限、检索轮数、每个会话同一时间只处理一个问题。
19. **按花费限流**：每个 IP 每小时的花费上限，与每日总上限配合。
20. **人机验证的备用方案**：Turnstile 加载失败时（国内网络访问 Cloudflare 有时不稳定），改用自托管的工作量证明挑战，只用于问答接口。
21. **不再叠加更多的模型护栏**，保持"程序检查 + 前置模型"两层。
22. **部署文档里推荐 CrowdSec 和 Litestream** 作为可选的加固和持续备份。

### 其他

23. **编辑器扩展对照 Docmost 实现**，许可证允许时直接借用并注明出处；Vue 节点视图参考 Halo。
24. **公开站点的挂载方式、Vite 配合方式对照 Gitea。**
25. **Go 端的协议适配器参考 New API。**
26. **任务队列直接使用 Backlite**，不自己写（需要再评估维护风险）。
27. **直接使用的库**：`golang.org/x/text/language`（语言匹配）、`maxminddb-golang`（GeoIP）、fracdex（排序）、go-limiter（限流）、browser-image-compression（图片压缩）。
28. **检索员加一个"查看目录树"的工具**（参考 PageIndex），混合检索不够时沿着目录查找。
29. **llms.txt**：除了 HTML 里的 `<link>`，再在 HTTP 响应头里加 `Link` 指向 Markdown 版本。
