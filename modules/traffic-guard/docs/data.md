# traffic-guard 数据与持久化

未来私有目录 `data/traffic-guard/active/` 保存SQLite和领域对象；`data/traffic-guard/restore/<restoreId>/staged|before/` 为恢复暂存与换出的旧数据；`data/traffic-guard/control/` 保存不随业务恢复替换的barrier、恢复回执和操作journal。外域不得直接读这些路径。

- `guard_policy(site_id, revision, policy_json, data_epoch; PK site_id)`

- `policy_operation_receipts(site_id, operation_id, input_hash, revision, data_epoch, created_at; PK site_id+operation_id；只保存幂等摘要，不保存正文)`

- `buckets(scope_key PK, capacity, tokens_milli, updated_at_ms, data_epoch)`

- `permits(admission_id PK, request_id UNIQUE, subject_key, action, expires_at, hard_deadline, state, policy_revision, data_epoch)`

- `challenges(challenge_id PK, subject_key, nonce_hash, difficulty, expires_at, state, attempts)`

- `bans(ban_id PK, subject_key, scope, starts_at, expires_at, reason_code, revision)`

- `event_inbox(event_id PK, source_revision, data_epoch)`

- `outbox(event_id PK, payload_ref, state, retry_at)`

SQLite WAL、foreign_keys=ON、busy_timeout=3000ms；同一服务使用串行短写事务，读使用快照。正文字节或导出文件与DB关联均先临时写+fsync+rename再索引提交，journal记录prepare/object-installed/index-committed/complete；启动按journal核hash补齐索引或清未提交对象，不能宣称一个SQLite事务覆盖文件。

修订单调；代次opaque Identifier只比较相等；唯一约束绑定epoch，旧epoch索引/lease/token禁止复活。金额只出现analytics派生投影，按USD整数micros处理。

快照只经HTTP管理协议（barrier/export/stage/discard），在safe屏障固定revisionWatermark；恢复先全域暂存，写切换决定后各服务重启换入，全部域登记回执才解维护。control区不随恢复替换。换入后先schema迁移与完整性验证，失败拒绝启动并可整体换回。

登录失败保护由identity提交账号私有HMAC与portal已签名来源的当前/前一日网络HMAC。仅接受`CallerService=identity`且`Actor.Kind=service`；本域库在启动时固定active代次，每个事务匹配`op.DataEpoch`。统一HTTP入口一次核验当前runtime、维护、严格schema；本域不重新查询身份或解析deadline。

追加迁移`0002_login_protection.sql`是以下私有表的唯一运行时schema来源，保留0001及原策略/回执：

- `login_attempts(attempt_id PK, data_epoch, caller, request_id, input_hash, account_key, action, created_at_ns, expires_at_ns, creation_revision, outcome, finished_at_ns)`；`epoch+caller+requestId`唯一；保存原30秒预留与创建revision=1，重放不延长。
- `login_attempt_networks(data_epoch, attempt_id, network_key)`；每个当前/前一日键独立关联、计数与并发，规范排序去重只消除顺序差异，不合并不同网络键。
- `login_failures(data_epoch, scope_kind, scope_key, attempt_id, failed_at_ns)`；按`epoch+account/network+HMAC+attempt`唯一；使用Finish受信服务时间查询滚动15分钟，非固定时间桶。
- `login_locks(data_epoch, scope_kind, scope_key, locked_until_ns)`；阈值首次达到时固定15分钟锁，锁内追加在途失败不续锁，重放及新Begin也不重算锁期；锁结束后按滚动窗正常判定。
- `login_finish_receipts(data_epoch, caller, operation_id, attempt_id, outcome, created_at_ns)`；Finish命名空间与Begin分开，同attempt同outcome换key仍返回原终局，不重复失败或清除；异终局始终冲突。

login/bootstrap共享账号及所有网络的失败窗、锁和并发。账号5次/每网络20次失败触发锁15分钟；账号未过期pending最多1个、每网络最多8个，30秒后自然释放并发，不自动计失败。过期pending只禁首次authenticated；首次late rejected仍按Finish当前服务时间计失败；aborted只释放不清计数。authenticated只清此账号失败与账号锁，不清任何共享网络失败/锁。终局、失败/清除、锁和回执在同一SQLite事务提交；后续session写入失败不改写口令核验事实。

表中只保存不可逆HMAC、安全操作引用和服务时间，不保存原账号、IP、口令、凭证或HMAC原料。当前只实现登录保护事务与HTTP映射；匿名保留清理、全域barrier/恢复和完整服务装配仍未完成，不能由本包宣称已可上线或已验收。

## 匿名风控运行表（2026-10-18源码，待GitLab）

只追加0003_guard_runtime.sql保存组合准入的匿名主体与current/previous网络别名、双桶、持久permit及网络关联、挑战nonce哈希、AI bans、问题信号、风险引用、FIFO等待引用及内部原安全回执。签名token、问题正文和原始IP不入这些表，签名与预算key只用外部引用。相似度与间隔是可选实测列，缺失SQL NULL；禁止用0冒充观测。现策略回执旧表保留，新完整业务回执使用guard_operation_receipts，不手改历史迁移。

同一site/epoch/caller/操作种类/operationId绑定规范输入摘要；管理回执额外绑定账号，服务报告绑定既有caller及请求引用。回执、状态与outbox同一短事务。桶/并发可复用最外层事务，内层不能单独提交。QA问题信号以可信RootOperationID去重，保留最近最多8条10分钟hash/时间信号，不保留问题正文。


## 准入持久记录的本轮规则

owner_caller与audience分开：许可行拥有者为qa或localization，audience固定model-gateway仅表示凭证验签目标。consumed_request_id为空时不可签模型token，消费后必须等于原request_id（稳定RootOperationID）。token、访客cookie与sessionId原值不写业务库或幂等回执；回执只存分配引用、结果布尔或原期限。许可捕获原网络current/previous别名，消费/续租/在线验证及新ban撤销按该组别名核对，不接受QA自报替代scope。

策略旧JSON缺qaPerNetworkPerMinute明确按600读取，新更新规范写入600；无需篡改旧迁移或重置策略。网络次数与费用比例互不推导。guard_buckets按动作、会话/网络别名/站点持久化，三层同事务先检查后扣减。


挑战nonce只存哈希，原nonce由独立预算key及前期过渡key重构；方法、难度、120秒期限与最多5失败次数绑定同挑战。已消费不再使用。通过后的10分钟proof只保存匿名主体/会话scope及通过水位，原sessionId和签名cookie不写业务库、日志或outbox。身份撤销通知使用平台nw_inbox，与guard_identity_revisions最大修订标记同事务；标记不替代即时identity授权。
