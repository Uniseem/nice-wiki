# 公开交付第四轮支援交接（2026-10-15，纯契约已过VPS设计检查）

## 范围与事实

- 用户持续实施授权范围下由根分配只读首页链接producer合同；分支codex/public-site-link-contract-r1，基线b56430efe56b724d6efdbe8bf3ae39f15fc6803c，接续clean，独占modules/public-site。不得再改identity、根或其他域。
- 仅补POST /v1/public-links:resolve，operationId resolvePublishedLink，binding ResolvePublishedLink(ctx,op,input)置于既有internal/links.go登记；没有Go业务实现、没有新增实际文件或迁移，本域仍无go.mod，全部业务/生成物planned。
- 请求documentId Identifier、language LanguageTag、blockId nullable Identifier均必填；language调用方已按baseLanguage补齐，不使用浏览器偏好。仅已认证site-config纯服务，拒用户委托和其他caller；当前dataEpoch固定。
- 结果oneOf严格限定：true才href/publication/resolvedLanguage非null且available；false三项全null，reason为unavailable/anchor_missing/language_unavailable。不公开不存在/未发布/撤回/删除的区别或标题/metadata/draft/旧路径。未知依赖与并发变化503，不拿旧cache伪造负结果。
- 冻结产品允许正文旧译/原文回退，但不允许已启用请求语言的URL静默改成base URL。成功resolvedLanguage=request.language；仅未启用/停用才language_unavailable；缺译不导致anchor_missing（译文沿用块ID）。
- 真实producer缺口已交根：content.PublicTreeNode需要同一公开树快照required canonicalPath，完整编码层级且不含locale/basePath/query/fragment；公开树可跳过未公开祖先，消费者不能拼parentId/pathSegment。根已在合成快照同步content schema及ReadPublicTree职责，public-site仅依赖现有版本化readPublicTree/readPublishedRevision/resolvePublicPath/validatePublications。
- 基础安全配置只调site-config.readSiteConfig，绝不调readPublicHome造成递归。listLanguages(includeDisabled=false)由根明确public-site纯服务只读范围。复用本域canonical机制组合basePath/请求language/权威路径，JSON取原始URL而非HTML属性转义串；参考BuildCanonical已澄清转义应在HTML输出点。
- 可选块只读当前精确published body并按既有正文hash/schema规则核存在；最终重核ref/path/visibility与config/language水位，变化直接STALE_RESULT/503，不增加自动重算重试。结果no-store且不是后续公开正文授权。
- 修改本域OpenAPI/types/implementation、相关人类文档及台账；生成目录不手改，根VPS docs/gen/check已通过并回传生成物。未本机Go/生成/编译/测试、未新增测试/mock/fixture/样例/临时业务程序、未运行真实接口/模型/服务，未push或部署。设计检查不代表本域实现或行为验收。

## 协议交付证据

2026-10-15 第四轮协议冻结快照在隔离VPS linux/amd64 Go1.25.5、GOMAXPROCS=8通过make docs/gen和完整make check（858文件/1105函数/379操作/2253类型），回传生成物0冲突。根14个现有Go模块readonly build/vet/gofmt同时通过；public-site尚无go.mod或业务源码，本域仅协议/生成物检查，不构成自身Go业务编译、实现或行为验收。pure-null在Go降级副本保留nullable+enum:[null]+x-go-type:any，未放宽3.1/TS源约束。未新增或运行业务测试、真实接口/模型、服务、镜像或部署。

本轮仅本域合同、生成物及文档；准确合同源码SHA在提交后回填。所有业务函数/文件和生成物状态仍planned，implementationCommit仍null，不能把生成代码存在或根14模块通过当本域已实现。

## 下一步（一项）

根另行分配后开始本域既定启动与真实HTTP装配组：先读根service-bootstrap、本域files/functions/operations，核cmd入口、BuildApplication/Serve/Shutdown、go.mod与版本化依赖；包括新resolvePublishedLink必须真实绑定，缺handler/权威依赖不得用占位成功启动。所有编译检查仅VPS，交付前不自开新业务包。
