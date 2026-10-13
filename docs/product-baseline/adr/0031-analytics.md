> 产品需求基线快照：来源 old:docs/adr/0031-analytics.md，blob f620f7412f79cfbefe322bb12ad5443d61d5f171。保留功能与细节；旧路径、数据布局和代码组织属于原架构，不是新实现结构。后续已确认ADR优先。

# D31. 内置简单统计

- 状态：✅ 已定
- 日期：2026-10-12

不依赖第三方，不使用 cookie。统计浏览量、独立访客、热门文档、来源、国家、语言分布。独立访客用每天更换盐的哈希计算，不保存原始 IP。详见 [design/site.md](https://github.com/Uniseem/nice-wiki/blob/b8190c7/docs/design/site.md)。
