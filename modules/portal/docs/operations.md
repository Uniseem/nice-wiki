# 运行、恢复与异常处理

入口服务无 snapshots；只登记屏障。重启读取 site-config runtimeState 后才 ready。持久运行配置来自受控部署，route config 哈希可用于审计。

来源：old admin.md/accounts.md/security-ops.md，D21/D35/D55。会话cookie SameSite=Strict；模型管理入口独立，不挂载初始化AI步骤；所有AI缺项仅禁用AI功能并显示缺项。

服务启动顺序：验证配置/schema → 打开本服务私有存储 → 恢复未完成决定日志 → 读取 site-config runtimeState → 校验活动 epoch → 接受允许的读取 → 仅在无维护/屏障时接受业务写入。

超时的写请求必须使用同 operationId 获取原结果。依赖失败不可假定权限、可见性或引用为空。日志记录 requestId、operationId、状态与安全字段，不记录正文、密码、token、原始文件路径或用户 IP。备份导出经只读 ObjectRef API；导出内容与 staging 根由本服务拥有。

快照协议各阶段在 API/函数中登记；portal 仅 gate 屏障。stage/discard 均需 data-protection 服务证明、superadmin 委托及维护状态；只要有域未登记切换回执，site-config 不能退出维护。
