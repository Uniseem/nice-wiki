# 生命周期与工作流

上传 receiving→validating→ready或blocked；receiving→aborted/expired只临时文件清理。ready永不按年龄自动删除。人工plan proposed→confirmed→waiting24h→deleted/cancelled/blocked；新ref立即cancel/block。asset ready→delete_pending→deleted只有人工授权计划。

上传先quota预留+expectedSHA/size→顺序chunk日志/fsync→流hash/MIME/scene/SVG格式验证→不可变blob rename/fsync→COMMITTEDmeta ready；同SHA dedup仅同site不泄漏。上传完成不自动公开；public-site精确publication manifest验证后产生绑定documentId/publishedRevisionId的稳定SHA URL长期immutable，后续源请求仍查当前发布门禁，管理员draft独立proofURL/no-store。

引用先acquire pendingpin与delete共享对象fence，sourceCOMMITTED后confirm成长期ref；abort不明就保留至少24h。事件完整snapshot版本单调、乱序只ack。资源库明确显示current与history文档引用；未引用只指current0，不抹历史风险。

人工清理先全source authoritative snapshot，未知/不完整拒绝；plan给历史可能缺文件警告与精确hash，管理员确认才等待24h保护迟到引用。执行worker主动jobs claim，最后对象mutex再查generation/fence/pins/readers/sourcewatermarks，任何新引用阻止删。服务端不设自动ready扫描删除，只有receiving过期临时字节可自动回收。

平台视频一次公开信息与封面本地import；失败manual_required手填title/description，不能伪造capability。Bili/YouTube/抖音/TikTok按固定host template；YouTube nocookie。public-site页面先封面按钮，点击才加载第三方player，服务器请求每hop/DNS阻SSRF，不发送登录cookie。

业务操作在提交前检查取消和期限；提交后返回已记录回执，客户端超时并不等于未提交。恢复屏障拒绝新业务写入，排空已登记事务后才返回 safe；快照仅覆盖屏障水位；恢复暂存不修改active数据，active目录只在进程重启时整体换入或换回。

<!-- flows:start -->
## 入口与后台流程

本节由 `make docs` 从 implementation.json 的 jobTypes、backgroundTasks 与 events.json 生成，不手改；节外为手写的状态机与规则。

### 事件处理

| 事件 | 版本 | 处理函数 | 作用 |
|---|---|---|---|
| `content.resources.changed` | 1 | `media.ApplyReferenceEvent` | 更新当前/历史refs，不以迟到事件替代最终权威检查 |

### 异步任务

#### `media.cleanup`

- 执行函数：`media.ExecuteManualCleanup`
- 触发：ConfirmManualCleanup一次人工批准后用固定CleanupApproval ObjectRef/dedupeKey和notBefore≥批准后24小时真实提交jobs；取消不删批准证据，worker必须另核当前计划

1. 仅本域纯media worker及平台CheckLease guard；完整JobExecutionInput保留原ObjectRef/jobId/workerId/fence，ReadCleanupApprovalObject精确读取原规范字节，核原operationId/root/dataEpoch/站点、原AcceptedJob与固定waiting结果；领取时新建十分钟执行期限
2. 终局原结果只回放，原DELETING只收敛；新执行只接受当前完整计划规范字节等于原waiting、notBefore=confirmedAt+24h且已到期、完整精确marks，取消/blocked/未知均拒绝
3. 所有RPC无asset/store/SQL锁；逐资产ValidateCleanupSources取得两源固定水位完整分页及本地pending；新current/pending或原批准历史风险变化则整份CAS blocked并保留marks；未知失败关闭
4. 最终平台CheckLease与公共running写窗口后，完整assetId稳定序锁→store→SQL；核原waiting/完整marks/fence/generation、最终快照generation、无current/pending/DELETING和activeReader及资源/计划修订空间，变化失败关闭
5. 同一事务固定schemaVersion=1执行材料、原批准摘要/job/op/root/site/epoch/waiting修订/decidedAt及全部逐资产决定与DELETING journal；同站点同SHA计划外任一未删除asset存在保留共享bytes，否则仅一项拥有bytes删除；未知Commit不unlink
6. 原决定恢复前完整核批准/marks/资产/水位及保护，store互斥覆盖generated blob普通文件/身份/SHA/长度核对、unlink/目录fsync及终局SQL；Complete同store互斥且未完成同SHA决定阻止新bytes提交/复用
7. 仅原持久决定允许缺失blob恢复；deleted/revision+1、精确mark解除、真实generation的deleted outbox、计划终局及完整原结果/hash同事务。Cancel遇任一DELETING始终CONFLICT，提交未知保留材料
8. 启动exclusive先恢复同opened epoch明确批准决定再核COMMITTED；deleted必须具完整原终局证明，未知/孤立DELETING失败关闭，不扫描ready自动产生授权

### 后台任务

| 函数 | 运行时机 | 作用 |
|---|---|---|
| `media.RecoverBlobJournal` | 启动时（入口开放前） | 恢复blob完成/删除持久日志 |
| `media.ExpireUploadSessions` | 每10分钟 | 仅回收过期未完成临时字节 |
| `media.RelayOutbox` | 启动后持续，空闲每1秒 | 本域已提交outbox的真实单批转发，机制统一由平台执行 |
| `media.ConsumeEvents` | 启动后持续 | 从jobs拉取本域订阅事件，交给events.json登记的处理函数，本地提交后再确认 |
| `media.RunWorker` | 启动后持续 | 主动领取并执行本域登记的持久任务 |
<!-- flows:end -->


第五轮分块：当前授权/epoch → 原操作/来源/hash → 原成功只读返回 → 新操作真实running写窗口 → receiving/expiry/offset/revision/创建epoch → 有界临时chunk与hash/fsync → PREPARED持久 → 追加/fsync → 再核窗口/expiry → SQL水位CAS与COMMITTED原结果。中断后只按SQL已提交水位核前缀/截尾，绝不把PREPARED当已收齐或ready。

第五轮Begin：原严格输入+caller/actor/site/epoch业务hash → 固定原回执 → 新操作running窗口/规范名称/site-config配额 → 创建临时文件fsync → session+创建epoch+原结果同事务。旧成功不再走当前配额；旧缺材料失败要求新操作。

构造先装配公共运行写窗口、核blob和chunk本地日志，再绑定真实已实现HTTP。全部OpenAPI handler未齐时NewServiceHandler失败，关闭存储，不启动部分可用服务；本轮不实现CompleteUpload和引用pin业务。维护仅允许Begin/Chunk的旧成功回放，所有新/未完成写入仍由公共窗口拒绝。

本轮完成链：当前media.upload/已打开epoch → 固定原完成回执 → 新操作running窗口/owner/site/创建epoch/receiving/到期/revision/已收齐 → SQL chunk前缀/完整SHA → MIME与SVG/scene语义 → PREPARED → 复制同时核hash/length → file fsync/rename/两目录fsync → 再核运行窗口 → 同事务ready+complete CAS+COMMITTED+原回执+新资源outbox。校验拒绝只持久blocked及原安全错误，不造ready。原回执不重做新写窗口；所有未提交写入都必须通过公共窗口。完整HTTP、引用pins、公开bytes、RelayOutbox/后台及业务验收仍未完成。

两轮并行第一轮引用：可信content/site-config纯服务 → 已打开epoch与完整pending材料/规范摘要 → 源互斥 → sealed/异材料拒绝或原pin回放 → assetId稳定锁 → store提交互斥 → 公共running写窗口 → SQL内来源复核与真实ready/站点/精确版本/hash/长度/MIME/fence → fence+generation CAS和至少24h pin原结果同事务。空集合走同一来源协议。delete_pending一律失败关闭；尚未实现真实计划取消、Confirm/Release与引用事件，完整服务仍不可运行。

第二轮Confirm/Release：本域当前epoch/可信来源与原材料 → 原终局回执优先 → 无锁固定源决定API与共享schema → 完整材料和明确committed或aborted/rejected → 无锁重新runtime写窗口 → sourceOperation/sourceObject互斥 → 完整受影响asset稳定锁 → store互斥 → SQL内重核来源/终局 → 单调current/history或sealed与原回执同事务。低版本提交历史不丢弃，空集合推进head，无pin终止仍封存，原pin_assets保留，pending/404/未知永不释放。真实清理计划取消仍未实现，Acquire/Confirm保持partial，完整服务仍blocked。


持续目标第一轮：ReadAssetReferences完整两源分页→本地pending与generation复核→ValidateCleanupSources→管理员Propose真实风险计划；只有后来明确确认才能进入delete_pending，目前确认/延迟任务/执行删除仍planned。Acquire/Confirm在任何asset锁之前扩出相交计划完整资产，再稳定锁序和SQL内完整重核→取消整份有效计划/恢复ready/提升水位→同事务pin或confirmed。持久DELETING与未知计划继续阻止，所有回源RPC无asset/store锁。

持续目标第二轮：当前管理员确认原plan/revision/hash/历史风险→两源完整扫描精确匹配原材料→无锁重新写窗口→完整asset稳定锁/SQL CAS→confirmed、人工批准、固定notBefore、delete_pending及精确marks同事务→释放全部业务锁→真实jobs:createJob（固定原op/root/dedupe/inputRef/notBefore）→重新窗口/完整锁核计划与marks→原任务回执、waiting及原成功同事务。未知重试不重复批准；并发取消后的迟到任务回执仅留证据，不复活计划、不返回确认成功。最终删除与worker仍planned；本轮修订后候选已过根统一VPS既有生成/编译/结构门槛，行为验收未执行。
