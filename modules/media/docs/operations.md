# 运行、恢复与异常处理

SVG（上传和Excalidraw）CSP sandbox/nosniff，不在HTML内联执行；链接只http/https/mailto，iframe固定白名单。Range bytes=N-M/N-/-N，If-Range mismatch全200，single206完整Content-Range；不能满足416 bytes */size。按hash资源字节永久缓存；服务源请求仍验证精确当前发布引用，撤回不自动删除历史bytes；preview资源no-store。

snapshot包含所有COMMITTEDblob/meta/refWatermarks/确认人工plans和journals，不把内存stream句柄导入。control区不随恢复替换；旧下载proof因epoch无效。历史手动清理造成缺资源保留墓碑，并提供明确缺失给rollback而不造图像。

服务启动顺序：验证配置/schema → 打开本服务私有存储 → 恢复未完成决定日志 → 读取 site-config runtimeState → 校验活动 epoch → 接受允许的读取 → 仅在无维护/屏障时接受业务写入。

超时的写请求必须使用同 operationId 获取原结果。依赖失败不可假定权限、可见性或引用为空。日志记录 requestId、operationId、状态与安全字段，不记录正文、密码、token、原始文件路径或用户 IP。备份导出经只读 ObjectRef API；导出内容与 staging 根由本服务拥有。

快照协议各阶段在 API/函数中登记；portal 仅 gate 屏障。stage/discard 均需 data-protection 服务证明、superadmin 委托及维护状态；只要有域未登记切换回执，site-config 不能退出维护。

## 本轮人工删除代码边界

media.cleanup由RunWorker注册到platform.RunWorkerLoop。只接受受信装配注入的真实platform.WorkerJobsClient；本域不复制claim/续租/取消/fence机制，不造接口空方法。完整ClaimedJob ObjectRef和原任务回执绑定原批准站点/op/root/epoch，领取时新建十分钟执行期限。真实适配与单批周期装配通过本轮GitLab pipeline32既有检查；完整服务因bytes、其他HTTP/事件/控制恢复未齐继续blocked；本轮没有运行worker、真实接口、SQL行为验收或删除任何业务文件。

任务取消只报告可证明安全点：ctx取消或公共CANCELLED cause后，一秒独立只读快照核原任务/批准/计划与execution；已有完整终局优先成功，无execution才能JobCancelled，未终局DELETING与读取未知保持决定，不谎报安全取消。该证明不执行写入或RPC，jobs最后写回仍受平台有效fence约束。


## 真实任务与背景关闭装配（2026-10-17，GitLab pipeline32检查通过）

域仅构建jobs拥有的六Result校验器，公共窄WorkerJobsClient适配负责真实HTTP转换；Claim使用system服务范围，领取后精确原批准恢复真实本站对象范围。背景任务只有worker和单批relay的统一周期包装，完整应用门槛未齐时不安装运行。producer事件广播可用system基础op，数据仅来自已提交outbox；不从任意payload指定RPC站点。

背景关闭统一为取消→HTTP Drain→等待背景安全点→Shutdown关库，不能先关DB再等任务。RelayOutbox只单批，平台RunRecoveryLoop每秒串行触发，网络/行租约/退避/隔离仍由RunRelayBatch处理；本批计数不代表全域已转发。所有runtime校验schema显式由OpenAPI manifest选取并生成自包含闭包，避免HTTP prune导致运行初始化缺规格，不把编译成功当行为验收。
