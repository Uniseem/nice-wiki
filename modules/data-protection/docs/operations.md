# data-protection 运维与故障

任一必需域停机：备份/恢复不进入ready/completed；maintenance保持失败状态等待明确continue、切换前abort或切换后整体换回。

密钥provider停机：不创建不可解密ready包，不泄漏密钥；已有密文保存。解密校验失败阻止暂存。

site-config停机：不能打开/切代/退出maintenance；各服务readiness fail closed，不假设缓存epoch可继续写。

data-protection停机：已开始maintenance由runtime持久维持；各域不自己解除。重启根据control journal续接。

远程Git停机或分叉：backup ready保持，mirror标failed/conflict；无force push、无真实push在本阶段。

启动：先校验config/schema/外部secret引用→恢复本域journal→读取site-config runtime→核active epoch→恢复控制日志是否有未完成的切换决定→注册ready。任一步关键校验失败则unavailable，不能开放写入；maintenance中仅允许已登记控制操作。关闭：拒新写/claim→取消新外部调用→在途到持久安全点→flush journal→退出；30秒未安全保持durable恢复记录并返回不可安全停机状态。

监控仅模块、errorCode、requestId、状态、队列长度/水位/耗时；不记录IP/问题正文/模型输入输出/secret/token。运维告警来自安全引用事件。每次文件操作检查绝对路径位于本域允许根，拒symlink/traversal。

恢复（ADR0003）：全域snapshot完整→全部域暂存→持久切换决定→各服务重启换入→全部回执与readiness→解除maintenance。control区不随恢复替换。切换后故障可继续等待或整体换回，不允许部分回滚。免费派生索引清空可重建，已付费结果引用必须保留，不能自动发模型请求。

2026-10-17 的启动切片只装配进程生命周期。健康检查不查询远程依赖、数据库或模型。就绪在 journal 未恢复、备份业务未实现或运行视图没有修订号时失败关闭。业务路由不返回空备份、空恢复进度或镜像成功。没有真实服务、模型或 Git 远程操作，没有运行业务验收。

本域Vue库入口mountModuleUI使用portal注入ModuleUIBootstrap.apiClient，不构造认证头。三个既定面板支持完整备份目录/显式创建/ready流式下载、完整恢复预检与批准/只读进度、远程CAS配置与显式镜像同步；实际服务未齐的503明确失败。卸载派发nice-wiki:unmount，取消请求及轮询。只读进度token仅内存/固定header，不进URL或存储；新CreateRestoreResult兼容新增progressToken/progressExpiresAt，服务实际必写。

大型下载固定X-Backup-Download-Token头，原Response.body直接pipeTo浏览器文件WritableStream，任意失败abort，不整包Blob。当前需要File System Access支持；不支持时明确不可用。版本采用已批准Vue3.5.22/Vite7.1.7/plugin-vue6.0.1/TS6.0.0-beta/vue-tsc3.1.1/pnpm9.15.9，lock只CI生成。
