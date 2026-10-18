# 问答与缓存当前进度

更新：2026-10-18。R1 实施，业务验收与部署均未开始。

- 本批分支 `codex/qa-20261018-bulk`，基线 `82179b907d25a546e9f470a1fb9cf1584ab45088`。源码 `fea4204`，全部代码就绪条目仍 `blocked` 待根及时送 GitLab 统一生成/格式/编译检查。
- 已编写17个原有业务函数（含共享迁移的 NormalizeQuestion、设计解阻的 EvaluateCacheEligibility）与43个实际私有事务、权威验证、回执及HTTP/SSE助手，共60项。没有把未运行的检查记为完成。
- 已有真实私有 SQL/原回执、准入在线验证消费、会话创建读取、流事件持久回放、精选候选/停用、精确缓存与权威有效性、反馈失效、管理列表/校准/策略和公开配置能力读取、平台模型网关装配，以及对应真实HTTP/SSE映射。
- NormalizeQuestion 共享纯算法迁到 `internal/normalize/question.go`，避免未来 turn→cache→turn 包环；取消通用截止/ctx检查，符合纯函数约定，需新编译证据。Eligibility只接受可信Planner的 resolvedReferences，缺失false，不扫描关键词或接受浏览器自报。
- 完整轮次创建/执行、缓存批准/自动保存/语义与翻译、模型前置/回答、压缩、恢复和完整服务启动尚未完成。本批仅对应函数就绪，含未完成函数的文件保持部分职责 blocked。
- 本机仅阅读、JSON/合同台账同步、Git审阅与 `git diff --check`；未运行Go/gofmt/gen/build/vet/check、测试、真实接口、模型、服务或推送。后续继续本域第二批，根负责及时GitLab检查和推送。
