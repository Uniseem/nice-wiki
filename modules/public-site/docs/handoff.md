# 2026-10-21 B1 metadataRevision缓存键增量

基线 `ddaab834aafb5e66704eef456c32971f5fccec4a`，分支 `codex/public-site-b1-metadata-20261021`，独占 `modules/public-site/`。源码与合同增量提交 `0c25307d`；最终交接提交包含本台账更新，未推送。

## 本次完成

- `contracts/types.json` 的 `BuildCacheKeyInput` 新增必填 `metadataRevision`，并声明它必须是 content 当前公开读取或 `content.metadata.changed/v1` 提供的精确值；权威值为0时才可用0，未知不能按0处理。
- `contracts/implementation.json` 明确 `public-site-renderer-v2` 的字段顺序和 v1 key 不复用规则；`internal/cache_key.go` 已实现同一 v2 纯函数，key 包含 `dataEpoch`、`documentId`、`publicationRevision`、`metadataRevision`、`locale`、`themeRevision`。
- `docs/functions.md` 已由 `make docs` 重生成；`docs/data.md` 与 `docs/requirements.md` 同步缓存列和 etag 维度。

## 精确边界与阻塞

当前 `BuildCacheKey` 没有任何真实调用点。未来调用方若默认构造 `MetadataRevision=0`，分享图变化不会换 key；调用链必须读取 content 当前公开 metadata 水位或消费 `content.metadata.changed/v1` 的精确水位，未知必须失败关闭，不能回退到0或旧发布修订。v2 纯函数已记 `implemented`，未编译验证。

content 的 `content.metadata.changed/v1` 仍由 content 会话冻结，payload 的 `metadataRevision`、当前 `PublishedRevisionRef`、`visibilityRevision` 和可空 `shareImageSource` 不能在本域猜测。`InvalidatePublication`、`ConsumeEvents`、`PrewarmPublication`、当前公开读取和整条 OG 即时更新仍 blocked，不能依赖旧 publication ref 或缓存推断新分享图。

media 的单个 `ContentDeliveryAssetSetProofV2` 固定证明正文 `documentId` 来源，不能授权正文、文档分享图、site-config 默认图三种来源的全集；解除该依赖前必须有多个精确来源证明或逐资产归属证明，并取得 site-config 默认分享图的权威证明。本轮不改 media manifest 草案。

## 检查与下一步

已执行：模块合同 JSON 解析、`make docs`、`git diff --check`。未执行：本机 Go/pnpm/build/check、测试、真实服务/接口/模型。合同摘要已同步到 `progress.json`，`BuildCacheKey` 与 `cache_key.go` 状态为 `implemented`，verification 为 `not_run`，等待用户自部署 GitLab CI。

下一步：待 content 冻结 `content.metadata.changed/v1` 后，先按其精确水位合同修订 `InvalidatePublication` 的事件输入、inbox/单调水位和预热幂等键，再接当前公开读取；media 多来源 delivery proof 与 site-config 默认图证明未就绪时保持失败关闭。

## 2026-10-18 用户收尾指令

用户要求push后停止，全部代理已冻结。本次集成树已通过GitLab pipeline118源码检查；未完成职责保持逐项台账，不将独立WIP计入main。后续仅在新授权下接续，不自动继续。

以下为历史记录。

# 2026-10-18 集成检查通过

pipeline60（生成261、三个Go分片262/263/264、前端265）完整通过，精确生成物已回传。逐项台账只把完整待编译清单转verified；partial、完整启动及既有设计阻塞保持blocked。所有Go模块readonly build/vet/gofmt与go test、完整make check通过；public-site前端类型检查及构建通过。go test没有测试文件，不构成业务验收。accepted=0，未部署。

本批源码汇总d344a37，记录提交等待零生成漂移复核后普通双推。持续向75%推进，用户最新要求及时分批检查/推送，历史停止指令不适用于本次授权。

以下为模块原始交付记录，其中“未CI”只描述交付时。

# 公开交付集中源码交接（2026-10-18，第一批）

基线acc9bbcc1fe0cfbeae91d3c2835b09cea5cbe9bd，分支codex/public-site-20261018-bulk，仅modules/public-site/，源码32ab806。用户本次授权覆盖旧停止指令，先集中源码至全仓75%，再由根统一GitLab检查与双推。本域不push。

## 实际边界

导航和派生索引每次重读content权威与精确发布引用，发送前比较水位。语言停用不输出URL；缺译保留语言URL。正文沿用content唯一schema，保留全部节点和marks。ID链接按当前公开规范路径与稳定块ID映射。

BuildApplication仍blocked，不返回可运行应用。LoadPublishedBundle复杂译文映射缺口仍partial。仅原本一个无marks纯text的paragraph/heading允许替换译文，其他保留原文并提示，不删除格式或链接。

## 集成操作

- 根统一make gen/docs/build/check，本机未执行Go/gofmt/vet/生成或检查。
- 本域OpenAPI登记PublishedDocumentBody生产方runtime schema；最终gen生成PayloadValidationSpec。
- chroma v2.20.0为ADR0005已确认MIT项目，根补THIRD_PARTY_NOTICES并在CI通过go命令更新go.sum。
- 根framework登记新增源码路径；本域implementation已有未来文件。
- 未新增测试、fixture、mock、样例或临时验证程序，未执行真实接口/模型/服务。

下一项：在internal/transport/server.go将已交付公开/预览处理函数绑定到真实handler表，先读当前OpenAPI与libs/platform统一边界。缺任一处理函数、journal或后台装配时仍失败关闭，不把本域标为完整可运行。

## 待CI核验的精确函数ID

- `public-site.SelectLocale`
- `public-site.SanitizeHTML`
- `public-site.RewriteInternalLinks`
- `public-site.BuildNavigation`
- `public-site.BuildBreadcrumbs`
- `public-site.BuildHreflang`
- `public-site.BuildSEOMetadata`
- `public-site.ComposeHTML`
- `public-site.BuildCacheKey`
- `public-site.HandleNavigation`
- `public-site.HandleRobots`
- `public-site.BuildLLMsIndex`
- `public-site.HandleLLMsIndex`
- `public-site.BuildSitemapEntries`
- `public-site.HandleSitemapIndex`
- `public-site.HandleSitemapShard`
- `public-site.HandleRootRedirect`
- `public-site.ParseTiptapDocument`
- `public-site.RenderTiptapHTML`
- `public-site.RenderTiptapMarkdown`
- `public-site.RenderHero`
- `public-site.RenderAnnouncement`
- `public-site.BuildPreviousNext`
- `public-site.HandleLLMsRoot`
- `public-site.HandleLanguageSitemap`
- `public-site.safeLink`
- `public-site.navigationOrder`
- `public-site.readIndexingPolicy`
- `public-site.markdownLabel`
- `public-site.xmlText`
- `public-site.sitemapIndexXML`
- `public-site.sitemapURLSetXML`
- `public-site.attrString`
- `public-site.attrInt`
- `public-site.nodeText`
- `public-site.assetFrom`
- `public-site.markdownEscape`
- `public-site.markdownDestination`
- `public-site.codeFence`
- `public-site.markHref`

## 第二批（源码f1f6aae）

预览OpenAPI已先收口：请求明确draftRevisionId，旧revision可选弃用且不参与授权；返回previewId，GET原token路径新增必填query previewId。直接传递content拥有者grant，不新签第二份token。每次渲染前后都重验本人、同会话及同精确草稿；no-store/private/noindex/no-referrer，资源只接受private_no_store manifest。

PreviewPanel只在内存保留HTML和grant，sandbox iframe无脚本，切稿/卸载/401/403清空；CSRF仅从portal编辑器当前页内存事件传入，不写DOM/存储。独立Vue/Vite工程已登记，锁文件由根CI pnpm生成，使用Node22；Vue3.5.22/Vite7.1.7/TypeScript6.0.0-beta/plugin-vue6.0.1沿用既定工程。根已准许Vue官方language-tools vue-tsc3.1.1作为SFC检查工具，MIT来源github.com/vuejs/language-tools，根补notice。

本批新完整源码待CI共14项，另含第一批RenderTiptapHTML平台点击加载修正；精确ID以progress中源码f1f6aae的记录为准。LoadPublishedBundle与ResolvePublicMedia两项仍partial，禁止纳入完整ready：前者缺复杂富文本/SEO译文回填，后者缺原图尺寸/alt及变体URL。

本域仍没有完整HTTP/router、cache存储及恢复/订阅装配，BuildApplication继续失败关闭。不将纯函数或局部HTTP源码视为服务已可运行。未新增测试/fixture/mock/样例/临时验证程序，未调用真实接口/模型/服务，未本机Go/pnpm。
