# 生命周期与工作流

首次无账户→首注册唯一超管→site-config两步向导→后台。子账号初始口令→restricted session→强制改密→后台；active↔disabled→deleted，superadmin仅active。

首注册不加初始化码；部署后立即注册或注册完成前不暴露端口。超管创建子账号默认勾选10项功能并设置初始密码；初始口令session不能读后台业务。改密旧口令+两遍新口令，至少8位、不同旧；其他设备立即失效，当前设备换token保持登录。

服务器查control/account-index.json找账号；忘密在identity私有control/reset-superadmin.txt写新口令，watcher核对固定路径/hash与OS权限，凭据提交后删除该文件，mustChangePassword=true。

超级管理员批准恢复后签发公共RecoveryAuthorization；各域保留不随导入覆盖的本地信任锚，只允许绑定restore/barrier/旧新epoch/audience/action/期限的快照控制。普通业务仍按账号功能授权，不能借恢复许可访问正文或改权限。

业务操作在提交前检查取消和期限；提交后返回已记录回执，客户端超时并不等于未提交。恢复屏障拒绝新业务写入，排空已登记事务后才返回 safe；快照仅覆盖屏障水位，prepare 不修改活动数据，commit/rollback 都有持久决定日志。
