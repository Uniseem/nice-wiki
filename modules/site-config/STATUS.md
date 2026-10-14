# 站点配置与初始化当前进度

更新：2026-10-15。阶段：R1-02 实施中；本次完成语言标签纯规则校验，业务验收与部署未开始。

- R1-02 启动装配组已在基线 `ffe1ae4ccbb7ce33f8fbcec19bc888865fdedf24` 上交付：入口、装配、HTTP 生命周期、配置、镜像与 manifest 已实现；原提交为 `5ea6affabea00bba21a614cd58a3dc4b99fa8cd2`。
- 本次 active/journal 工作包基线为 `82c83226b120b0376fa4ddfe5fe1b58f3c3e7d29`，分支为 `codex/site-config-r1-02-active-storage`，独占范围仍为 `modules/site-config/`。
- 本次基线为 `ac593c57861d2242f27d07562a2acae535d18c38`，分支为 `codex/site-config-r1-02-runtime-state`。新增独立 `control/control.db`，采用 modernc.org/sqlite v1.59.0、WAL、foreign_keys、busy_timeout=3000ms、单连接与只追加迁移。
- RuntimeState 现由 control.db 权威读写：首次建库持久化 running/dataEpoch/revision=1；读取校验 phase、maintenance、epoch、requiredModules、activationReceipts 与时间/哈希；运行状态迁移使用 CAS revision 和 operationId+请求摘要回执；RegisterActivation 按服务身份、恢复阶段和 `(restoreId,module)` upsert 回执。
- `localRuntime` 已改为复用本进程控制库句柄；BuildApplication 步骤 2 的自身权威读取不再返回占位 `DEPENDENCY_UNAVAILABLE`；Ready 真实返回活动 epoch、maintenance、publicRequestsAllowed 与 runtime revision；Shutdown 关闭控制库。
- RuntimePhase 矩阵、14 个有状态域回执集合与 CheckWriteEpoch 已落地；本次在独立 active/site-config.db 中加入只追加迁移、epoch/初始化状态与公告 head 索引，并在启动时执行公告 journal 的 COMMITTED 校验和 PREPARED 孤儿隔离。
- `BuildApplication` 现按 control epoch 打开 active 业务库、恢复公告 journal 后才返回应用；`Ready` 校验 active schema、epoch、journal 恢复指纹和初始化索引，任一失败均返回错误阻止就绪。仍未实现 HTTP 业务路由、outbox、后台循环和换目录后的 RegisterActivation 回执装配。
- 规格矛盾已在本模块类型契约中修正：requiredModules 从 15 改为 14，与 ActivationReceipt 枚举和 M03-R06 的 14 个有状态域一致；portal 不登记回执。集成会话需复核该契约修正。
- 本次新增 `internal/language_service.go`，实现 `ValidateLanguageTag`：校验 BCP47 结构、扩展/私有子标签及重复子标签，规范语言/脚本/区域/扩展大小写；代码提交为 `a2599e2`，随后以 `663bf07` 严格收回超出本域 schema 的 grandfathered 标签分支。共享工作区实际分支为 `codex/analytics-r1-storage-contract-audit`，未切换其他会话分支。
- 初始化/配置业务仍被契约缺口阻塞：`PrepareDefaultConfig` 没有受控 `publicBaseURL` 来源，且主题色、资源限额、安全策略、源码链接缺少默认值；未初始化 `InitializationState` 又要求 `operationId`、`siteId` 非空。未据此猜测或生成伪配置。
- 本机没有 Go 工具链，未执行 gofmt、go mod tidy、go build、go vet 或 Go 业务运行；未新增测试、样例、快照、真实接口或模型调用。
- 本模块 JSON 解析、`python scripts/check_docs.py --privacy-only` 与 `git diff --check` 通过；未改生成目录或公共 `framework.json`，完整根检查仍需集成会话复跑。

业务行为验收、部署验收、真实恢复联调均未执行，不得将本次实现或静态检查标记为 accepted。
