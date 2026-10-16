# 生命周期与工作流

文档draft↔published→trash→draft恢复或purged。撤回回draft但保留历史；删除旧URL410或替代docIdredirect。内存锁FREE→HELD→idleWarning→RELEASED，2min无心跳与15min输入提示后2min不操作释放；重启全锁清空，serverBootId区别旧fence。共享草稿每次save版本CAS，同人双tab不覆盖。

SaveDraft：authorize+epoch→lease/fence+expectedHead→schema/资源pin→不可变文件PREPARED/fsync/rename→txn再次fence/CAS→COMMITTED head/refs/receipt/outbox→confirm pins；任一步失败未提交文件隔离，外部重试查operationId。

Publish：管理员手填元数据优先，首次英文slug/SEO仅AI配置时可生成；无AI空slug明确补填，基础发布人工合法slug可完成，不自动doc-ID/摘取假输出。精确draft+hash+meta/tree CAS事务选发布ref、visibilityRevision/history/作者+outbox；后续render/translate/index/data git镜像不反向修改正文。

Withdraw/trash先权威增visibility并清发布引用再发失效事件。公开/qa/knowledge在答复与传输前validate当前ref；旧事件低水位不能复活。Purge只有超管明确确认，仅删本域活跃数据，data git历史保留；media不自动清除。

媒体历史文件可经管理员手动清理并有风险提示。回滚先列缺失文件，需要显式承认后按历史恢复，新发布可能有缺资源占位；不得造bytes。预览只有已登录管理账号，public样式但noindex/no-store，不提供匿名分享。

业务操作在提交前检查取消和期限；提交后返回已记录回执，客户端超时并不等于未提交。恢复屏障拒绝新业务写入，排空已登记事务后才返回 safe；快照仅覆盖屏障水位；恢复暂存不修改active数据，active目录只在进程重启时整体换入或换回。

<!-- flows:start -->
## 入口与后台流程

本节由 `make docs` 从 implementation.json 的 jobTypes、backgroundTasks 与 events.json 生成，不手改；节外为手写的状态机与规则。

### 事件处理

本域不消费事件。

### 异步任务

本域没有登记的异步任务类型。

### 后台任务

| 函数 | 运行时机 | 作用 |
|---|---|---|
| `content.RecoverPreparedCommits` | 启动时（入口开放前） | 恢复崩溃前正文提交日志 |
| `content.ExpireIdleLocks` | 每30秒 | 释放失联或提示后仍空闲锁 |
| `content.RelayOutbox` | 启动后持续，空闲每1秒 | 把已提交的outbox事件可靠转交jobs；进程启动先补发，之后后台循环 |
<!-- flows:end -->

第五轮文件核心不接HTTP或业务保存成功：上游授权/CAS快照→规范hash→私有原准备回放→新操作当前站点大小及本树语义→公共写窗口+真实持久preparing→原8字段PREPARED/fsync→tmp/fsync/不覆盖rename/目录同步→MarkPrepared。head/稳定ID ledger/refs/receipt/outbox仍在后续既有CommitDraftTransaction同事务完成。Read只允许受信COMMITTED资格，保留历史节点语义，坏文件隔离而不从旧head或缓存降级。

第五轮尚未实现SaveDraft时确定的媒体abort协议，已由持续目标第二轮代码接通新版ReleaseResourceReferences：先源持久aborted/rejected，再携sourceOperationId/siteId/sourceObjectId/sourceRevisionId/sourceVersion/referenceDigest、nullable pinId、reason请求媒体原子封存。pinId未知不先重新Acquire，避免资源已不可用时恢复永久卡住；COMMITTED优先且不释放已提交历史引用。本轮已编写该保存来源流程并通过编译/结构检查；真实媒体接口和业务验收未执行。

本轮树纯规则先核完整原树，再核拟移子树的完整高度：根为第1层，总深度最多32，禁止移入自己或后代。返回后代固定为原树ID字典序，不修改输入；MoveDocument/redirect事务仍待后续实现。本轮资源纯规则只提取已核正文的精确引用，不申请pin、不提交引用、不探测资源，也不修改head。


第二轮只交付参考锁核心：已即时授权用例和identity姓名→真实持久doc状态→单leader内存账户共享lease；新心跳序号续TTL，新输入序号更新服务活动→固定15分钟提醒/17分钟绝对失效→同docmutex释放/提高fence。返回快照仍须由完整用例可靠通知和审计，不能直接绑定为已成功的HTTP。提交前AcquireSaveFence持生命读锁/docmutex→核真实doc/delete/隔离和lease/boot/fence→未来短SQL事务内再核CAS/ledger/refs/pin/receipt/outbox→Release；当前完整事务链仍blocked。


## 完整编辑锁用例的提交边界

本轮Acquire/Renew/Release/Force先经identity即时核当前会话和功能权限，安全姓名仅来自同次AuthorizationDecision.accountName。别人持有有效锁时，原AcquireEditLockResult成功返回lease+writable=false；LOCKED保留给文档不可编辑、租约失效或维护窗口等。旧operationId先回原历史结果，再来的原租约不能因此复活；同键异动作、文档或完整参数返回CONFLICT。

当前平台写窗口在文档互斥前建立；同doc mutex内复制候选，真实文档状态/deleteFence/隔离在短事务重核。原回执、跨boot单调revision的content.edit-lock.changed v1 outbox与本地审计提交成功，才把候选安装到权威内存。Commit结果未知时仅保留内存pending；所有编辑和保存guard先读取真实原回执决定安装或丢弃，读取失败保持关闭。匹配fence/到期是纯检查，不能偷偷清权威租约而漏发通知。fence耗尽不安装失败候选；旧租约仍按原到期界点拒绝保存。

Expire使用本域调度器可信now，同操作局部失败后将已完成scope一起回放，逐文档事务不重复事件。15分钟提醒固定于LastInputAt+15，17分钟绝对失效；新输入清提醒发送active。状态载荷没有姓名、正文或leaseId，通知只供界面失效提示，不代替保存入口同步核租约。原回执虽然保留历史lease结果，授权仍只在当前boot内存，恢复/重启绝不据此重建。

本轮只实现生产者可靠排队和本域持久审计，RelayOutbox/HTTP绑定、portal在线通知消费/流与启动后台尚未装配；不声称在线编辑器已收到强释，也不声称完整服务可运行。

## 真实创建、读取与保存候选

创建先即时content.edit授权和站点配置读取，读取本站完整treeRevision快照，固定新文档ID/首次修订/空Tiptap正文/基准语言/有界fractional order并持久专用意图。文件准备后，短SQL以站点树CAS和文档不存在谓词建立metadata/head/COMMITTED/完整原结果，重试回原结果；不会调用模型。相邻key间无剩余空间时明确CONFLICT，重排用例后续实现。

ReadDraft即时content.read授权，以同一SQL快照读取元数据、共享head及COMMITTED原回执；文件校hash结束后才短取同doc guard，返回当前未过期内存lease。读操作不重建或自动续租，不从不存在或未知正文降级。

SaveDraft初始短guard核真实lease与head后释放；固定完整引用源意图→真实Acquire pin→不可变文件准备→当前窗口→真实SaveFenceGuard→短SQL再次核lease/deleteFence/head CAS并原子提交ledger/COMMITTED/refs/完整回执/outbox→原pin确认。Commit未知或完整性损坏保留保护；已知CAS/锁拒绝必须先同事务证明没有COMMITTED并封存源，再隔离未提交正文和Release原pin。完成函数候选不等于HTTP已绑定、通知已送达或服务可运行。

文档资源来源水位使用documents.reference_version，独立于draftVersion。创建初始化为1；保存首次意图固定ExpectedReferenceVersion/SourceVersion，最终同head/ledger/refs事务CAS并推进。发布/撤回等后续引用生产者也必须使用同一水位，不能按草稿版本推导；公共Revision上限耗尽失败。旧行默认0只表示无本轮引用资格，存在旧head却无真实水位时失败关闭，禁止从文件/JSONL猜补；原op重试保持原来源材料，不重新分配水位或Acquire替代pin。

保存首次新操作在持久intent或Acquire pin前读取当前正文限额并按规范UTF8长度拒绝超额请求；COMMITTED原回执仍优先，已prepared材料不因后来降低配额重做。pending重试在任何续作RPC/文件IO前短核原真实租约及head/引用CAS，返回时释放guard；明确失败经统一无COMMITTED证明后封存并Release nullable原pin，未知或正文完整性错误保持资源保护。
