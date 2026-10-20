## 2026-10-21 B1 metadataRevision缓存键增量

从已绿 B1 集成基线 `ddaab834aafb5e66704eef456c32971f5fccec4a` 建立独立分支 `codex/public-site-b1-metadata-20261021`，本会话独占 `modules/public-site/`。源码与合同提交 `0c25307d`，未推送。

`BuildCacheKeyInput` 新增必填 `metadataRevision`。`BuildCacheKey` 使用稳定的 `public-site-renderer-v2`，按 `dataEpoch`、`documentId`、`publicationRevision`、`metadataRevision`、`locale`、`themeRevision` 长度编码后生成 SHA-256；v1 key 不读取、不复活。缓存表与 etag 规格同步登记 `metadata_revision`。

当前函数可作为独立纯函数记 `implemented`，但没有任何真实调用点，仍未做 Go 编译验证。未来调用方必须从 content 当前公开读取或 `content.metadata.changed/v1` 水位取得精确 `metadataRevision`；未知值必须失败关闭，不能以默认 `0` 或旧 `publicationRevision` 代替。该项不代表分享图或整条 OG 即时更新，`InvalidatePublication`、事件消费/预热和真实公开读取仍 blocked。

跨域审查记录：content 分享图合同仍由 content 会话冻结，public-site 只读；media 当前单个正文 `ContentDeliveryAssetSetProofV2` 不能证明正文、文档自有分享图和 site-config 默认图的来源全集，需多个独立来源证明或逐资产归属证明，并取得默认图的 site-config 权威证明后才能解除 manifest 依赖。本轮未修改 content/media。

本轮静态检查：合同 JSON 解析通过，`make docs` 通过，`git diff --check` 通过；按用户授权未运行本机 Go、pnpm、build 或 `make check`，未执行真实服务、接口、模型和测试。

## 2026-10-18 用户收尾指令

用户要求push后停止，全部代理已冻结。本次集成树已通过GitLab pipeline118源码检查；未完成职责保持逐项台账，不将独立WIP计入main。后续仅在新授权下接续，不自动继续。

以下为历史记录。

# 2026-10-18 集成检查通过

pipeline60（生成261、三个Go分片262/263/264、前端265）完整通过，精确生成物已回传。逐项台账只把完整待编译清单转verified；partial、完整启动及既有设计阻塞保持blocked。所有Go模块readonly build/vet/gofmt与go test、完整make check通过；public-site前端类型检查及构建通过。go test没有测试文件，不构成业务验收。accepted=0，未部署。

本批源码汇总d344a37，记录提交等待零生成漂移复核后普通双推。持续向75%推进，用户最新要求及时分批检查/推送，历史停止指令不适用于本次授权。

以下为模块原始交付记录，其中“未CI”只描述交付时。

# 公开交付当前进度

更新：2026-10-18。用户授权先集中源码至全仓75%，再统一GitLab检查与双推。基线acc9bbc，独占分支codex/public-site-20261018-bulk，源码32ab806。

本批交付既有主要函数和必要参考助手，均blocked（待统一GitLab生成与编译）；未提前记implemented/verified。既有未改verified保留，accepted为0，未验收或部署。精确清单见docs/handoff.md。

语言选择、权威导航/邻文、SEO/hreflang、净化、缓存键、hero/公告、公开索引、Tiptap生产方schema及双渲染已有源码。BuildApplication仍失败关闭。复杂译文映射仍partial，不计完整装配。

chroma v2.20.0为ADR0005既有MIT项目；require已登记，go.sum由根CI通过go命令生成。PayloadValidationSpec等待最终make gen。

第二批源码f1f6aae：精确公开权威、拥有者预览4函数与浏览器预览岛已写入，等待根GitLab生成及检查。LoadPublishedBundle与ResolvePublicMedia仍partial，不纳入完整ready；完整服务仍blocked。
