# 运行、恢复与异常处理

所有有状态快照/barrier完整登记；恢复按ADR0003暂存后重启换入，control区不随恢复替换。content snapshot包含正文+head/journal/tree/redirects/history/IDledger和引用水位，排空保存txn后导出；内存锁不导出，恢复后全部editor重获锁。

D30第一版不做批量导入；粘贴是编辑器交互。D54公开heading不折叠。服务不访问media/data-protection私有目录；go-git镜像由data-protection通过对象API获取已提交文本。

服务启动顺序：验证配置/schema → 打开本服务私有存储 → 恢复未完成决定日志 → 读取 site-config runtimeState → 校验活动 epoch → 接受允许的读取 → 仅在无维护/屏障时接受业务写入。

超时的写请求必须使用同 operationId 获取原结果。依赖失败不可假定权限、可见性或引用为空。日志记录 requestId、operationId、状态与安全字段，不记录正文、密码、token、原始文件路径或用户 IP。备份导出经只读 ObjectRef API；导出内容与 staging 根由本服务拥有。

快照协议各阶段在 API/函数中登记；portal 仅 gate 屏障。stage/discard 均需 data-protection 服务证明、superadmin 委托及维护状态；只要有域未登记切换回执，site-config 不能退出维护。
