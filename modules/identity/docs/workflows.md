# 生命周期与工作流

首次无账户→首注册唯一超管→site-config两步向导→后台。子账号初始口令→restricted session→强制改密→后台；active↔disabled→deleted，superadmin仅active。

首注册不加初始化码；部署后立即注册或注册完成前不暴露端口。超管创建子账号默认勾选10项功能并设置初始密码；初始口令session不能读后台业务。改密旧口令+两遍新口令，至少8位、不同旧；其他设备立即失效，当前设备换token保持登录。

会话access由identity固定`iss=identity`、`aud=portal`、`typ=NW-SESSION`的Ed25519令牌表示，15分钟访问期、最长7天绝对期限；refresh和CSRF各为独立32字节随机secret，仅保留SHA256摘要。refresh不依赖未过期access；旧refresh再次使用必须先提交族与会话撤销和安全outbox，再返回未认证，不能因返回错误回滚撤销。浏览器刷新须singleflight；敏感响应不进入可重放普通回执，响应丢失后重新登录。

任务入口能力由根jobManagementGrants生成目录派生：账号已有任一登记任务业务能力才增加job.manage，仍只有既有10项用户开关。portal粗预检通过后，jobs按任务持久owner/type再次请求具体权限；受限初始口令与停用账户不获得该能力。

初始口令的单次授权额外检查可信服务从静态路由绑定产生的`action`，仅`identity:changePassword`、`identity:revokeSession`、`identity:readSelf`与`account.self`组合允许；缺失或其他action失败关闭，不允许会话列表或偏好读取。受限登出只撤销当前设备。action不能由浏览器选择，也不能增加原有功能grants；后续HTTP绑定须按实际handler提供固定值。

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


登录/首次注册只接受portal经服务断言建立的网络HMAC作用域（当前/前一UTC日），不从浏览器字段或普通header兜底。identity通过平台HMAC规范账号名，调用traffic-guard.beginLoginAttempt，再以authenticated/rejected/aborted提交幂等终局；错密码、未知账号、停用账号使用同一未认证结果。authenticated仅表示口令和active核验事实，在会话事务提交前确认；一旦发送authenticated/rejected，即便响应未知，也不改报aborted。未知风控或初始化状态拒绝登录。

首次注册先持久control reserved决定，再写active账户/凭据/会话/安全回执和outbox；active提交后补齐索引并单向标记committed。同原op重试不重发秘密，改密当前设备轮换且其他设备立即撤销；ReadSelf返回credentialRevision供改密CAS。固定nextRoute只从/admin/me（强改视图）、/admin/setup、/admin选择。


第三轮账号管理：只有当前会话权威对应的唯一超管row可管理同站点子账号。列表含deleted墓碑但不含超管，accountName/accountId稳定键集分页的签名scope绑定主体、站点和dataEpoch。已删除墓碑不再写入；唯一超管状态仅active原样无副作用，权限/密码重置入口拒绝修改超管。

状态/权限/重置先匹配HMAC/keyId原成功回执再CAS，重放不覆盖后来权限、撤销后来新会话或再次重置密码。实际停用/删除/重新启用或重置与全session/族撤销、身份事件和安全回执同事务；启用不复活旧登录。功能集合变化只提升permissionRevision并写事件，旧access/委托同步失效，合法refresh可按既定协议升级；无变化请求只记成功回执，不制造版本或撤销。所有索引仍在不可反悔提交前按统一16MiB上限检查。
