# 生命周期与工作流

初始化 uninitialized→preparing→config_committed→completed；持久commit后恢复完成或blocked。单条banner enabled/timewindow决定是否可见，无额外富文公告产品。runtime running→restore_staging→restore_switching→running（或restore_rolling_back→running），任一域未登记回执保持维护。

初始化只需已登录唯一超管填写站点名和基准语言，默认中文zh-Hans。publicBaseURL/部署参数来自受控环境，AI不配置不探测不调用。写config权威文件与决定日志，再initialized完成。

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
<!-- flows:end -->
