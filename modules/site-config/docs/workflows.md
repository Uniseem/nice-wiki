# 生命周期与工作流

初始化 uninitialized→preparing→config_committed→completed；持久commit后恢复完成或blocked。持久化函数的 commit 结果仍停 preparing，不代表配置已提交；config_committed 表示权威文件、COMMITTED、head 和 SQL 水位已提交但初始化完成事务尚未结束；同决定同准备配置摘要重试返回原版本，异摘要或终局决定冲突失败。abort 前不能存在 commit，abort 不删除身份账号或重开注册。单条banner enabled/timewindow决定是否可见，无额外富文公告产品。runtime running→restore_staging→restore_switching→running（或restore_rolling_back→running），任一域未登记回执保持维护。

初始化只需已登录唯一超管填写站点名和基准语言，默认中文zh-Hans。publicBaseURL/部署参数来自受控环境，AI不配置不探测不调用。PrepareDefaultConfig 接受显式 publicBaseURL 与可信 preparedAt，不读取请求 Host 或当前时钟；公开地址通过公共 ServiceConfig 的 publicBaseURL/allowLocalHTTP 边界校验后，启动时调用 ConfigureInitialization 注入；缺少地址不阻止既有读取启动，但初始化使用时失败关闭。先同事务持久完整准备配置、commit 决定和原始业务回执，再由 ResumeInitialization 写 config 权威文件并完成 initialized。

UpdateHome保存全屏hero；video必须poster，public-site输出autoplay muted loop playsinline、图片preload，prefers-reduced-motion视频只poster。UpdateBanner保存唯一公告文字/安全链接/起止时间，文字进localization；前端关闭只localStorage。

恢复由data-protection服务身份携带superadmin委托驱动（ADR0003）；各域control区不随恢复替换。全部有状态域switched到同一targetEpoch才退出维护；整体换回时全部rolled-back才恢复正常。

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
| `site-config.RecoverAnnouncementJournal` | 启动时（入口开放前） | 恢复正文文件与提交日志 |
| `site-config.RelayOutbox` | 启动后持续，空闲每1秒 | 把已提交的outbox事件可靠转交jobs；进程启动先补发，之后后台循环 |
| `site-config.RecoverConfigJournal` | 启动时（入口开放前） | 启动前恢复settings权威文件提交决定 |
| `site-config.RecoverHomeReferences` | 启动前及受信后台周期；每次最多100项，复用公共调度与写窗口 | 按明确决定有界收敛首页媒体pin |
<!-- flows:end -->


一般设置：同站点管理会话 → 规范 URL/稳定请求摘要 → 解析原始 operation 回执 → 首次持久候选配置与固定时钟 → 复用不可变文件/PREPARED → COMMITTED/CAS/head/SQL/outbox → 返回正式配置。成功重试返回原修订；未完成重试不重造时钟；新的维护期写入拒绝，旧成功回执仍可读取。普通候选不是持久发布决定，启动时不能自动发布。

本轮未推进 Home：公开按钮禁用状态与目标 URL 尚无精确返回字段，AI 可用性来源未收口；media acquire→本域 COMMITTED→confirm 的 pinId 与崩溃续办需作为下一紧密组整体设计。不能删按钮代替禁用，也不能只调用成功路径的 pin 接口。
