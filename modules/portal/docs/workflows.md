# 生命周期与工作流

初始化→登录→加载壳→可用/降级；编辑导航状态 idle→dirty→saving→saved 或 conflict；维护 gate OPEN→DRAINING→SAFE→OPEN。



业务操作在提交前检查取消和期限；提交后返回已记录回执，客户端超时并不等于未提交。恢复屏障拒绝新业务写入，排空已登记事务后才返回 safe；快照仅覆盖屏障水位，prepare 不修改活动数据，commit/rollback 都有持久决定日志。
