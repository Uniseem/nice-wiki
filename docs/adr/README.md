# 决策记录索引

每条决策一个文件，文件名是"四位编号-英文短名"，例如 `0021-admin-accounts.md`。文档里引用时写作 `D21`。

状态：✅ 已定 · 🟡 倾向（可以推翻）· ❓ 待讨论 · ⛔ 已被取代

## 规则

- **只追加，不改写结论。** 推翻一条决策时，新写一条，在新决策里说明"取代 Dxx"；旧决策的状态改为"⛔ 已被 Dyy 取代"，正文保留。
- 错别字、链接失效这类不改变结论的修正，可以直接改。
- 🟡 倾向状态的决策，在确认之前可以直接修改；确认为 ✅ 之后，就只能追加。
- 实现过程中做的小决定，如果不影响其他模块，记在 push-log 里就够了；会影响其他模块、或者以后可能有人问"为什么"的，写成决策记录。

## 模板

```markdown
# Dn. 标题

- 状态：✅ 已定
- 日期：YYYY-MM-DD
- 取代：Dxx（如果有）

结论和理由。详细设计放在 docs/design/，这里只给链接。
```

## 索引

| 编号 | 决策 | 状态 |
|---|---|---|
| D1 | [技术栈](0001-tech-stack.md) | ✅ |
| D2 | [仓库形态：单仓，部署为单个程序](0002-monorepo-single-binary.md) | ✅ |
| D3 | [定位：一个项目的公开文档站，由少数几个人维护](0003-positioning.md) | ✅ |
| D4 | [编辑器：照思源的交互，用 Tiptap 实现](0004-editor-tiptap.md) | ✅ |
| D5 | [存储：SQLite](0005-sqlite.md) | ✅ |
| D6 | [不做传统搜索，改用 AI 问答](0006-ai-qa.md) | ✅ |
| D7 | [模型统一走 OpenRouter](0007-openrouter.md) | ⛔ 被 D44 取代 |
| D8 | [存储布局：文件是真实数据，SQLite 存索引](0008-storage-layout.md) | ✅ |
| D9 | [块级设计：只做编辑器，不做知识管理](0009-block-design.md) | ✅ |
| D10 | [检索方案](0010-retrieval.md) | 🟡 |
| D11 | [多语言：基准语言 + 大模型全站翻译](0011-i18n-translation.md) | ✅ |
| D12 | [URL 带语言前缀，只在根路径自动跳转](0012-url-language-prefix.md) | 🟡 |
| D13 | [语言切换建议：由快速模型判断](0013-language-suggestion.md) | ✅ |
| D14 | [公开页面由 Go 渲染，交互部分用 Vue 挂载](0014-rendering.md) | ✅ |
| D15 | [草稿与发布分开](0015-draft-publish.md) | ✅ |
| D16 | [版本历史：在 data/ 目录里内置 git](0016-version-history-git.md) | ✅ |
| D17 | [首页：全屏主视觉，后台可配置](0017-home-hero.md) | ✅ |
| D18 | [技术选型](0018-tech-choices.md) | ✅ |
| D19 | [页面布局：左侧文档树 + 右侧本页目录](0019-page-layout.md) | ✅ |
| D20 | [URL 用可读路径](0020-readable-urls.md) | ✅ |
| D21 | [后台账号：超管 + 子账号，账号密码登录](0021-admin-accounts.md) | ✅ |
| D22 | [AI 问答防滥用](0022-ai-abuse-guard.md) | ✅ |
| D23 | [部署：HTTPS 交给反向代理](0023-deploy-reverse-proxy.md) | ✅ |
| D24 | [视频：支持上传和平台嵌入](0024-video.md) | ✅ |
| D25 | [问答记录与回答缓存](0025-qa-records-cache.md) | ✅ |
| D26 | [图片：在浏览器里生成多种尺寸](0026-image-sizes.md) | ✅ |
| D27 | [后台功能与文档生命周期](0027-admin-features.md) | 🟡 |
| D28 | [安全与运维](0028-security-ops.md) | 🟡 |
| D29 | [AI 问答界面：浮窗 + 独立问答页](0029-qa-ui.md) | ✅ |
| D30 | [第一版不做导入](0030-no-import.md) | ✅ |
| D31 | [内置简单统计](0031-analytics.md) | ✅ |
| D32 | [欢迎 AI 爬虫，提供专门的版本](0032-ai-crawlers.md) | ✅ |
| D33 | [不做的小功能](0033-dropped-features.md) | ✅ |
| D34 | [站点公告横幅](0034-announcement-banner.md) | ✅ |
| D35 | [权限按功能分配，编辑时加锁](0035-permissions-edit-lock.md) | ✅ |
| D36 | [文档分类与过程记录（push-log）](0036-engineering-docs.md) | ✅ |
| D37 | [AI 助手说明文件分层](0037-agent-instructions.md) | ✅ |
| D38 | [测试规范：测试由用户决定](0038-testing.md) | ✅ |
| D39 | [前后端接口以 OpenAPI 定义为准](0039-openapi.md) | ✅ |
| D40 | [注释、提交信息、说明文件都用中文](0040-language.md) | ✅ |
| D41 | [提交署名：用户是作者，AI 模型是合著者](0041-commit-attribution.md) | ✅ |
| D42 | [提交中不得包含隐私信息（硬性规定）](0042-no-private-info-in-commits.md) | ✅ |
| D43 | [仓库公开，许可证 AGPL-3.0](0043-public-repository.md) | ✅ |
| D44 | [多厂商模型接入：渠道 → 模型 → 档位 → 任务](0044-multi-provider-models.md) | ✅ |
| D45 | [文件结构是契约：改结构必须先询问](0045-file-structure-contract.md) | ✅ |
| D46 | [长期记忆：独立模块，统一维护，按需取用](0046-long-term-memory.md) | ✅ |
| D47 | [翻译流水线：小批次、逐级降级、翻译记忆、术语抽取](0047-translation-pipeline.md) | ✅ |
| D48 | [长期记忆的细化：单条上限、说明、只读、逐条判断](0048-memory-refinements.md) | ✅ |
| D49 | [回答缓存的细化：两层匹配、默认 AI 判断、分区与校准](0049-answer-cache-refinements.md) | ✅ |
| D50 | [防滥用的细化：明确上限、按花费限流、验证备用方案、对话自动压缩](0050-abuse-limits-and-compaction.md) | ✅ |
| D51 | [借用代码与依赖的选择](0051-borrowed-code-and-dependencies.md) | ✅ |
| D52 | [实施阶段的顺序：先 AI 问答，再多语言](0052-phase-order.md) | ✅ |
| D53 | [档位和任务的模型分配只有超管能改](0053-tier-assignment-superadmin.md) | ✅ |
| D54 | [公开页面的标题不折叠](0054-no-heading-folding.md) | ✅ |

## 待讨论

（暂无。实施计划见 [plan.md](../plan.md)，下一步从第 0 阶段工程准备开始。）
