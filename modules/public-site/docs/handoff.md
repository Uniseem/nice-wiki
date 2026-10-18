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

下一项：完成internal/authority.go的CheckPublication与internal/content_client.go的精确引用装配，先读content/localization/media生产方合同。

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
