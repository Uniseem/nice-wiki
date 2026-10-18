# 公开交付当前进度

更新：2026-10-18。用户授权先集中源码至全仓75%，再统一GitLab检查与双推。基线acc9bbc，独占分支codex/public-site-20261018-bulk，源码32ab806。

本批交付既有主要函数和必要参考助手，均blocked（待统一GitLab生成与编译）；未提前记implemented/verified。既有未改verified保留，accepted为0，未验收或部署。精确清单见docs/handoff.md。

语言选择、权威导航/邻文、SEO/hreflang、净化、缓存键、hero/公告、公开索引、Tiptap生产方schema及双渲染已有源码。BuildApplication仍失败关闭。复杂译文映射仍partial，不计完整装配。

chroma v2.20.0为ADR0005既有MIT项目；require已登记，go.sum由根CI通过go命令生成。PayloadValidationSpec等待最终make gen。

第二批源码f1f6aae：精确公开权威、拥有者预览4函数与浏览器预览岛已写入，等待根GitLab生成及检查。LoadPublishedBundle与ResolvePublicMedia仍partial，不纳入完整ready；完整服务仍blocked。
