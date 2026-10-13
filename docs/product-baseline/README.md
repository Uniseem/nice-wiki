# 产品需求基线

用户于2026-10-14补充：站点的功能及细节按old，我们只是换架构。本目录只保留产品设计及相关ADR的需求快照，不复制旧业务代码。

功能、界面交互、默认值、权限和业务规则保持旧设计；已确认后续ADR优先旧草案。旧单程序/数据库/文件路径/共享data和函数布局属于可更换的架构实现，不强制继承；来源中的这些段落只用于辨认原语境。D55初始化无需AI、D54标题不折叠、D53档位/任务仅超管等覆盖早期相反草案。

需求追踪见 [功能保留矩阵](../product-preservation.md)，新AI上下文/缓存规则见 [专门规格](../ai-context-and-cache.md)。来源与blob指纹见 [机器清单](../../contracts/product-baseline.json)。来源中的链接保留为已存在提交的公开文档链接，不指向尚未推送的old归档提交。

| 基线文档 | 原始blob |
|---|---|
| [accounts.md](design/accounts.md) | `882960809268f911ddadbd3c852003fc53bac4d4` |
| [admin.md](design/admin.md) | `3404011edac56fb14e23d54d183956974344b5db` |
| [editor.md](design/editor.md) | `60d8e9bb8ca8513f941a8a12b9460fec93d74074` |
| [site.md](design/site.md) | `61000042d4a4ca7dfcc2f791a8562c01440e1a3a` |
| [home.md](design/home.md) | `4f619f940bdf410510b077edd0b3b6abd424ad64` |
| [ai-models.md](design/ai-models.md) | `6383d6eeb038d88bf78a4e1c004f009dd393065b` |
| [ai-qa.md](design/ai-qa.md) | `9f11da6b511bd8e645b3ba2bc6576d4034c7c8c2` |
| [ai-guard.md](design/ai-guard.md) | `9cbb5cbf3e61817556cf5504513d9ee7e865feb5` |
| [memory.md](design/memory.md) | `bd1b090da4db1acdd331799fc593435c5448cbe3` |
| [i18n.md](design/i18n.md) | `f5d7963667132e6bc73dd20354ba18cb8530399d` |
| [security-ops.md](design/security-ops.md) | `c71d7d7f05fa8bd5f13e91f6cc8775fad572f296` |
| [0003-positioning.md](adr/0003-positioning.md) | `a9b432ca8fc2fd61cdc0516d255e0a9fdd65a80f` |
| [0004-editor-tiptap.md](adr/0004-editor-tiptap.md) | `12accd46b9395136d3cd8d0e447a3a4b0190a629` |
| [0006-ai-qa.md](adr/0006-ai-qa.md) | `6ed5a02c09b122812b1cf3a9eb55d47e0994f326` |
| [0009-block-design.md](adr/0009-block-design.md) | `6813e3c761d46d060d1cee7e30afb87665751627` |
| [0010-retrieval.md](adr/0010-retrieval.md) | `c0aef65c26eade05046cb429015e9f3e0ee19966` |
| [0011-i18n-translation.md](adr/0011-i18n-translation.md) | `60c2207d9d976ee975556ec96dd88ad296b32940` |
| [0012-url-language-prefix.md](adr/0012-url-language-prefix.md) | `7d3a2c114e27c635b20529e3e849519d696785b0` |
| [0013-language-suggestion.md](adr/0013-language-suggestion.md) | `7d63b00ef8969a47c94824ac7f96f69eeab99723` |
| [0014-rendering.md](adr/0014-rendering.md) | `afb77864dff47deeb6f8944a7afb3a73ea8f3a41` |
| [0015-draft-publish.md](adr/0015-draft-publish.md) | `6ea156ca4b5d387d52ca19c05935243bb08e5a2a` |
| [0016-version-history-git.md](adr/0016-version-history-git.md) | `f12ae84cf59dbb2fe1846751b8655e1d43346f33` |
| [0017-home-hero.md](adr/0017-home-hero.md) | `a64fc1031a7d716f51894dcd72e97282d6e8c525` |
| [0019-page-layout.md](adr/0019-page-layout.md) | `5f2280c0d7727c12f69036432a9d2210afb330f1` |
| [0020-readable-urls.md](adr/0020-readable-urls.md) | `99ca6676e0624aadcd798bd4d294c64116635990` |
| [0021-admin-accounts.md](adr/0021-admin-accounts.md) | `7de773fe2fd6add165b2165f2f9637812a8fe221` |
| [0022-ai-abuse-guard.md](adr/0022-ai-abuse-guard.md) | `f9e7bbb22388f167c56f22a29fb28dc09a4e5c74` |
| [0023-deploy-reverse-proxy.md](adr/0023-deploy-reverse-proxy.md) | `a83beb2ed451ddca2af02d4db30ced320343f762` |
| [0024-video.md](adr/0024-video.md) | `6d30d3dd983deb1a1e33ac829ed358ea54cce28f` |
| [0025-qa-records-cache.md](adr/0025-qa-records-cache.md) | `4f529dce82a12cb4b82158c88ff3a646043a8b2a` |
| [0026-image-sizes.md](adr/0026-image-sizes.md) | `0c688c79ef51f9a0c218c1a3fc91a08bb3f472b3` |
| [0027-admin-features.md](adr/0027-admin-features.md) | `e2d2b2fd09853bffc25da15d403766d123f5831e` |
| [0028-security-ops.md](adr/0028-security-ops.md) | `da4f958472e9940eb6f6680f8b8241dba63e3d3e` |
| [0029-qa-ui.md](adr/0029-qa-ui.md) | `4a81db413595c551a9ce887a35ef95d95b3ca965` |
| [0030-no-import.md](adr/0030-no-import.md) | `0caf756bf9ef0620433556a7c37bb3ae872da7ea` |
| [0031-analytics.md](adr/0031-analytics.md) | `f620f7412f79cfbefe322bb12ad5443d61d5f171` |
| [0032-ai-crawlers.md](adr/0032-ai-crawlers.md) | `6e12e7af355174a8a6e0dcbf8a01c51c59309dd9` |
| [0033-dropped-features.md](adr/0033-dropped-features.md) | `966463a3c278a637ffe880c8ae16f37aa9fde073` |
| [0034-announcement-banner.md](adr/0034-announcement-banner.md) | `9b71d5f51fc52f89016c6d3b40b0992e573e629d` |
| [0035-permissions-edit-lock.md](adr/0035-permissions-edit-lock.md) | `008290eae497058f1513267281daa01d960a2f63` |
| [0040-language.md](adr/0040-language.md) | `7599da256d8be5f4475117df48589c73a940e0d9` |
| [0044-multi-provider-models.md](adr/0044-multi-provider-models.md) | `e52914f2fe00960a6d2bbc75312baf9db78064eb` |
| [0046-long-term-memory.md](adr/0046-long-term-memory.md) | `05e18d6f91c865426fe0b4bd7eb290c156c02a17` |
| [0047-translation-pipeline.md](adr/0047-translation-pipeline.md) | `e992a32ed5bf37a832bd20ac9ca2ab207e739de2` |
| [0048-memory-refinements.md](adr/0048-memory-refinements.md) | `363ace859adcaafd18091307e3eaa5a7b37fe229` |
| [0049-answer-cache-refinements.md](adr/0049-answer-cache-refinements.md) | `5a942f009facb9c995687ded4765895fdee89f07` |
| [0050-abuse-limits-and-compaction.md](adr/0050-abuse-limits-and-compaction.md) | `5f0cd0738bed70629a0c43438b6329b161bb9f3d` |
| [0053-tier-assignment-superadmin.md](adr/0053-tier-assignment-superadmin.md) | `fc19e88c0598833516230d150f7578f8c9b1c41e` |
| [0054-no-heading-folding.md](adr/0054-no-heading-folding.md) | `b6804d5b5e379a143c4b8542d79e5205b7a3af1f` |
| [0055-setup-without-ai.md](adr/0055-setup-without-ai.md) | `373d3257dbc08f29da7f0893b875a7a02bf4bbbe` |
