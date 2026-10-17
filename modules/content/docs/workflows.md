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

## 2026-10-17资源来源查询

只追加0005维护全站reference_scan_state；来源表的插入/更新/删除与原事务一起推进水位，耗尽回滚。每页同SQL快照完整核元数据/head/COMMITTED/回执/引用覆盖，并在签名前释放SQL；签名游标固定过滤、分页大小、主体/会话/代次、水位和位置。跨页水位变化409必须从头，不拼接快照、不持久化读快照。来源扫描各20000预算超限503，不把不完整扫描解释为空。

current_draft与history取真实索引分类；pending_pin来自Acquire前已持久的完整保存意图，retained=true只表示保护，不是COMMITTED。精确COMMITTED资格优先，即使原intent终态尚未补齐；明确aborted/rejected且无COMMITTED才排除。未知已发布/分享图生产资格、缺索引、缺原回执或坏材料均失败关闭。本轮没有实现发布或元数据分享图生产者，也不通过空结果掩盖它们。

两个只读来源HTTP路由使用平台统一服务身份/当前epoch/schema边界；media源决定限定纯服务，列表固定media/site-config/data-protection纯服务或portal即时content.read。RelayOutbox只把真实本域outbox与jobs客户端交给平台单批机制；后台循环和完整启动继续未完成。本轮源码已通过根自部署GitLab CI pipeline 12生成/格式/重复生成、三个实际同时分片覆盖14模块readonly build/vet/gofmt及完整make check（878文件/1435函数/379HTTP/2328类型），未执行SQL/HTTP行为验收或部署。


COMMITTED是不可撤销历史决定；后来正文/文档隔离不撤销完整精确资格、原回执与refs的媒体保护。当前正文读取仍拒绝隔离，缺资格或材料矛盾仍INTEGRITY_ERROR，绝不降pending/abort。

## 2026-10-17第二轮文档树

文档树同快照核真实站点/代次、树水位、原始metadata schema、head/COMMITTED资格、无环与活动父/同父orderKey及slug唯一；活动树按parent/orderKey/ID前序，回收站独立列表，签名cursor绑定主体/会话/权限版本/state/limit、树水位及真实reference_scan_state来源水位，任一水位变化409从头；既有0005触发器覆盖metadata/title/state/head/隔离写，确保SaveDraft仅推进head时也不混完整metadata快照。10000仅为新树查询/操作的明确扫描/输出预算，purged墓碑不计入；超限503不截断。旧Create nil回调无新增总量限制，内部Ensure/Plan可信完整树输入取消maxItems，不新增站点配额。

四个树action只在core按所需功能一次即时identity，DB/锁之前执行；平台仍严格验证服务身份/委托/epoch/schema，仅对实际注册四action不重复授权RPC。trash列表需content.publish，活动读content.read，移动/重排content.edit。真实树写在锁外取得当前写窗口，短SQL做树/metadata CAS、完整原回执和最小content.tree.changed outbox（业务content.edit、jobs事件管理仍沿全域content.publish）；0006仅追加原回执，未知提交按原op查回执。重排要求完整同父活动集合，trash/purged不混入；均匀base62 key不改变slug。

PlanOldPathRedirects完整规划node及所有曾发布后代的无语言前缀旧路径→稳定docId，未知祖先slug失败关闭；非空规划因真实版本化redirect manifest生产链未齐而使Move明确503、不提交，Move及文件继续partial。没有新增假manifest、发布生产者或消费者；当前事件只可靠排队，完整应用因写窗口/全量handler/恢复/后台未齐仍blocked。本轮源码已通过GitLab pipeline23生成/格式/重复生成、三并行分片14模块readonly build/vet/gofmt及完整make check；pipeline21真实编译失败、pipeline22取消均未计通过。未做业务验收或部署，缺可信写窗口在原回执后、新写前503。公共纯服务本站传递仍是后续协议前置，当前不声称跨服务来源链已联通。

## 2026-10-17第三轮metadata

元数据编辑在DB/锁前一次即时content.edit，同代完整scope+规范请求摘要优先回放完整原结果，核result_hash与严格结果schema；新写锁外取得当前平台窗口，短SQL核活动文档、真实metadata/tree CAS并提交标题/SEO/路径、原结果和最小tree.changed。人工SEO非null为manual（包括空串），null为pending；不触发模型或发布共享正文。

仅根级保留navigation、ask；子级允许，同父活动slug唯一。robots.txt/llms.txt/sitemap.xml由slug字符规则自然排除，不另禁无点词、api/admin/v1等。校验使用调用方同SQL可信完整树，纯助手复用给Move，未来发布也沿同规则。PUT metadata与既有树入口均由core一次即时授权，公共边界仍校身份/epoch/schema，无私有缓存marker。

只追加0007保存完整同代原回执，无影子树/正文/媒体来源。shareImage实际改变因精确metadata pin/引用源协议未齐503，原完整ref可保持；曾发布节点或后代路径变化因真实版本化redirect manifest未齐503，Update整体继续partial。当前写窗口缺失新写503，完整启动/行为验收/部署未完成；本轮已通过GitLab pipeline32生成/重复生成、三个并行分片14模块readonly build/vet/gofmt与完整check；pipeline30取消、31 Go过但缺两类型导致整体检查失败均如实记录，生成物精确回传不手改。根同轮补可信服务本站签名与jobs控制proof清理，只是源码/结构证据，未运行真实跨域或业务验收。
