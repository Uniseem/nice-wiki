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

待实现SaveDraft的媒体abort恢复对齐新版ReleaseResourceReferences：先源持久aborted/rejected，再携sourceOperationId/siteId/sourceObjectId/sourceRevisionId/sourceVersion/referenceDigest、nullable pinId、reason请求媒体原子封存。pinId未知不先重新Acquire，避免资源已不可用时恢复永久卡住；COMMITTED优先且不释放已提交历史引用。此条仍planned，不代表媒体流程已实现。
