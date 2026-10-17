# 运行、恢复与异常处理

SVG（上传和Excalidraw）CSP sandbox/nosniff，不在HTML内联执行；链接只http/https/mailto，iframe固定白名单。Range bytes=N-M/N-/-N，If-Range mismatch全200，single206完整Content-Range；不能满足416 bytes */size。按hash资源字节永久缓存；服务源请求仍验证精确当前发布引用，撤回不自动删除历史bytes；preview资源no-store。

snapshot包含所有COMMITTEDblob/meta/refWatermarks/确认人工plans和journals，不把内存stream句柄导入。control区不随恢复替换；旧下载proof因epoch无效。历史手动清理造成缺资源保留墓碑，并提供明确缺失给rollback而不造图像。

服务启动顺序：验证配置/schema → 打开本服务私有存储 → 恢复未完成决定日志 → 读取 site-config runtimeState → 校验活动 epoch → 接受允许的读取 → 仅在无维护/屏障时接受业务写入。

超时的写请求必须使用同 operationId 获取原结果。依赖失败不可假定权限、可见性或引用为空。日志记录 requestId、operationId、状态与安全字段，不记录正文、密码、token、原始文件路径或用户 IP。备份导出经只读 ObjectRef API；导出内容与 staging 根由本服务拥有。

快照协议各阶段在 API/函数中登记；portal 仅 gate 屏障。stage/discard 均需 data-protection 服务证明、superadmin 委托及维护状态；只要有域未登记切换回执，site-config 不能退出维护。

## 本轮人工删除代码边界

media.cleanup由RunWorker注册到platform.RunWorkerLoop。只接受受信装配注入的真实platform.JobsClient；本域不复制claim/续租/取消/fence机制，不造接口空方法。完整ClaimedJob ObjectRef和原任务回执绑定原批准站点/op/root/epoch，领取时新建十分钟执行期限。完整服务仍因真实适配、bytes、其他HTTP/事件/控制恢复未齐而blocked；本轮没有运行worker、真实接口、SQL行为验收或删除任何业务文件。

任务取消只报告可证明安全点：ctx取消或公共CANCELLED cause后，一秒独立只读快照核原任务/批准/计划与execution；已有完整终局优先成功，无execution才能JobCancelled，未终局DELETING与读取未知保持决定，不谎报安全取消。该证明不执行写入或RPC，jobs最后写回仍受平台有效fence约束。
