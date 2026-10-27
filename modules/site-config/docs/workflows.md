# 生命周期与工作流

初始化 uninitialized→preparing→config_committed→completed；持久commit后恢复完成或blocked。持久化函数的 commit 结果仍停 preparing，不代表配置已提交；config_committed 表示权威文件、COMMITTED、head 和 SQL 水位已提交但初始化完成事务尚未结束；同决定同准备配置摘要重试返回原版本，异摘要或终局决定冲突失败。abort 前不能存在 commit，abort 不删除身份账号或重开注册。单条banner enabled/timewindow决定是否可见，无额外富文公告产品。

初始化只需已登录唯一超管填写站点名和基准语言，默认中文zh-Hans。publicBaseURL/部署参数来自受控环境，AI不配置不探测不调用。PrepareDefaultConfig 接受显式 publicBaseURL 与可信 preparedAt，不读取请求 Host 或当前时钟；公开地址通过公共 ServiceConfig 的 publicBaseURL/allowLocalHTTP 边界校验后，启动时调用 ConfigureInitialization 注入；缺少地址不阻止既有读取启动，但初始化使用时失败关闭。先同事务持久完整准备配置、commit 决定和原始业务回执，再由 ResumeInitialization 写 config 权威文件并完成 initialized。

UpdateHome保存全屏hero；video必须poster，public-site输出autoplay muted loop playsinline、图片preload，prefers-reduced-motion视频只poster。UpdateBanner保存唯一公告文字/安全链接/起止时间，文字进localization；前端关闭只localStorage。

各域control区不随恢复替换。全部有状态域switched到同一targetEpoch才退出维护；整体换回时全部rolled-back才恢复正常。

业务操作在提交前检查取消和期限；提交后返回已记录回执，客户端超时并不等于未提交。

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
| `site-config.RecoverAnnouncementJournal` | 启动时（入口开放前） | 恢复正文文件与提交日志 |
| `site-config.RelayOutbox` | 启动后持续，空闲每1秒 | 执行一次单批outbox转发；启动补发与轮询循环只由完整Application后台入口装配 |
| `site-config.RecoverConfigJournal` | 启动时（入口开放前） | 启动前恢复settings权威文件提交决定 |
| `site-config.RecoverHomeReferences` | 完整Application构造后由公共RunRecoveryLoop持续运行；每1秒单批最多100项，维护和未知安全窗口暂停 | 按明确决定有界收敛首页媒体pin |
<!-- flows:end -->


一般设置：同站点管理会话 → 规范 URL/稳定请求摘要 → 解析原始 operation 回执 → 首次持久候选配置与固定时钟 → 复用不可变文件/PREPARED → COMMITTED/CAS/head/SQL/outbox → 返回正式配置。成功重试返回原修订；未完成重试不重造时钟；新的维护期写入拒绝，旧成功回执仍可读取。普通候选不是持久发布决定，启动时不能自动发布。

资源引用读取：受信媒体服务或同站点配置管理员 → 固定当前 config watermark → 校验当前 head/index/COMMITTED/权威文件 → 逐条验证该 epoch 的 COMMITTED revisions → 按既有 home/branding 来源标识生成 current/history refs → 验签游标并比较 payload watermark 与当前水位后分页。scope 固定绑定查询身份与筛选条件，水位变化返回 VERSION_MISMATCH；分页未完成前，任何媒体清理不得把当前页外资源视为未引用。

首页保存：原始输入业务摘要 → 原成功只读回放 → 新/未完成写窗口与配置/home双CAS → 免费公开链接和ready资源验证 → 固定候选及完整引用intent同事务 → acquire并持久原pin → 既有不可变文件/COMMITTED/CAS/head/SQL/outbox → 返回已保存。confirm_pending由公共后台续办，后续网络失败不改变原成功。已有COMMITTED的重放仅补原正式决定，不重查新可见性/ready/期限。

首页终止恢复：无COMMITTED时与提交共锁持久aborted；随后直接Release完整source和可空pinId，media回源核明确终局并原子sealed，有无pin都完成来源封存。绝不为了终止恢复重新Acquire，也不将NOT_FOUND/404/pending/未知当释放证明。COMMITTED只confirm持久pin，缺材料失败关闭。

公开首页：独立公开水位读取 → 保留全部hero按钮 → document读取当前发布链接/语言/锚点，qa读取免费能力，external沿既有安全规则 → 不可用按钮明确disabled且敏感目标和href/publication不返回。hero、公开brand/footer与当前窗口banner一起返回；不会因单个链接或AI能力失联隐藏hero。

构造只做本地引用恢复；全部真实handler齐备后才通过BackgroundTasks交公共RunRecoveryLoop周期收敛。每个RPC前后复用NewWriteContext核安全窗口与active epoch；维护暂停，未知保留。退出时公共生命周期先取消、Drain停止HTTP并等待请求，再等后台退出，最后Shutdown关闭DB。

单批返回只描述本批，不代表全域事件已送达；启动补发与轮询循环仍属于完整 Application 装配。


回调通过 `outbox.MigrateRestoredDatabase` 在隔离库内执行本域已登记迁移、更新目标 active epoch、核验配置状态并重建公告派生索引；公共内核负责旧 outbox 资格隔离和 hostHash 重封，site-config 不把旧事件重标当前 epoch，也不改写原 payload、root 或费用事实。
