# 运行、恢复与异常处理

SVG（上传和Excalidraw）CSP sandbox/nosniff，不在HTML内联执行；链接只http/https/mailto，iframe固定白名单。Range bytes=N-M/N-/-N，If-Range mismatch全200，single206完整Content-Range；不能满足416 bytes */size。按hash资源字节永久缓存；服务源请求仍验证精确当前发布引用，撤回不自动删除历史bytes；preview资源no-store。

snapshot包含所有COMMITTEDblob/meta/refWatermarks/确认人工plans和journals，不把内存stream句柄导入。恢复专用RecoveryAuthorization本地trust/control批准不覆盖；旧下载proof因epoch无效。历史手动清理造成缺资源保留墓碑，并提供明确缺失给rollback而不造图像。

服务启动顺序：验证配置/schema → 打开本服务私有存储 → 恢复未完成决定日志 → 读取 site-config runtimeState → 校验活动 epoch → 接受允许的读取 → 仅在无维护/屏障时接受业务写入。

超时的写请求必须使用同 operationId 获取原结果。依赖失败不可假定权限、可见性或引用为空。日志记录 requestId、operationId、状态与安全字段，不记录正文、密码、token、原始文件路径或用户 IP。备份导出经只读 ObjectRef API；导出内容与 staging 根由本服务拥有。

快照协议各阶段在 API/函数中登记；portal 仅 gate 屏障。prepare/commit/rollback 均需 data-protection 服务证明、superadmin 委托及维护状态；只要有域激活未完成，site-config 不能退出维护。
