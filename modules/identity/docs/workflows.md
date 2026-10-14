# 生命周期与工作流

首次无账户→首注册唯一超管→site-config两步向导→后台。子账号初始口令→restricted session→强制改密→后台；active↔disabled→deleted，superadmin仅active。

首注册不加初始化码；部署后立即注册或注册完成前不暴露端口。超管创建子账号默认勾选10项功能并设置初始密码；初始口令session不能读后台业务。改密旧口令+两遍新口令，至少8位、不同旧；其他设备立即失效，当前设备换token保持登录。

服务器查control/account-index.json找账号；忘密在identity私有control/reset-superadmin.txt写新口令，watcher核对固定路径/hash与OS权限，凭据提交后删除该文件，mustChangePassword=true。

恢复由data-protection服务身份携带发起恢复的superadmin委托驱动，不签发专用恢复许可（ADR0003）。identity服务签名密钥在control区，不随恢复替换；换入恢复数据后启动流程撤销全部会话，管理员用备份中的账号重新登录。

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
| `identity.ObserveSuperadminResetFile` | 每5秒 | 检测服务器超管重置文件 |
| `identity.RelayOutbox` | 启动后持续，空闲每1秒 | 把已提交的outbox事件可靠转交jobs；进程启动先补发，之后后台循环 |
<!-- flows:end -->
