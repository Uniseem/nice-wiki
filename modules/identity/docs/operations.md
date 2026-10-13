# 运行、恢复与异常处理

Sources：old docs/design/accounts.md、security-ops.md；D21、D35、D55优先。snapshots包含secret时依data-protection加密策略处理；可选不包含keys须明确无法恢复对应密钥，账户hash保留且restore会话全部失效。cookie HttpOnly Secure SameSite=Strict。固定服务器重置文件与controltrust只属于identity进程私有根，其他域不得写该目录。

服务启动顺序：验证配置/schema → 打开本服务私有存储 → 恢复未完成决定日志 → 读取 site-config runtimeState → 校验活动 epoch → 接受允许的读取 → 仅在无维护/屏障时接受业务写入。

超时的写请求必须使用同 operationId 获取原结果。依赖失败不可假定权限、可见性或引用为空。日志记录 requestId、operationId、状态与安全字段，不记录正文、密码、token、原始文件路径或用户 IP。备份导出经只读 ObjectRef API；导出内容与 staging 根由本服务拥有。

快照协议各阶段在 API/函数中登记；portal 仅 gate 屏障。prepare/commit/rollback 均需 data-protection 服务证明、superadmin 委托及维护状态；只要有域激活未完成，site-config 不能退出维护。
