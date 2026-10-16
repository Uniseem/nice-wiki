# 运行、恢复与异常处理

所有有状态快照/barrier完整登记；恢复按ADR0003暂存后重启换入，control区不随恢复替换。content snapshot包含正文+head/journal/tree/redirects/history/IDledger和引用水位，排空保存txn后导出；内存锁不导出，恢复后全部editor重获锁。

D30第一版不做批量导入；粘贴是编辑器交互。D54公开heading不折叠。服务不访问media/data-protection私有目录；go-git镜像由data-protection通过对象API获取已提交文本。

服务启动顺序：验证配置/schema → 打开本服务私有存储 → 恢复未完成决定日志 → 读取 site-config runtimeState → 校验活动 epoch → 接受允许的读取 → 仅在无维护/屏障时接受业务写入。

超时的写请求必须使用同 operationId 获取原结果。依赖失败不可假定权限、可见性或引用为空。日志记录 requestId、operationId、状态与安全字段，不记录正文、密码、token、原始文件路径或用户 IP。备份导出经只读 ObjectRef API；导出内容与 staging 根由本服务拥有。

快照协议各阶段在 API/函数中登记；portal 仅 gate 屏障。stage/discard 均需 data-protection 服务证明、superadmin 委托及维护状态；只要有域未登记切换回执，site-config 不能退出维护。


第一轮真实正文存储已接启动open/migrate/recover，完整服务继续blocked。迁移只追加，已应用SQL哈希不同或私有库epoch不符时停止；本轮不实现恢复换目录后的epoch重绑定和activation。正文恢复的COMMITTED必须同时有真实私有提交资格及原回执，不能由JSONL单独升级。根统一在VPS做格式/生成/编译/结构检查，业务验收未执行。

首个可信新库的epoch与第1迁移及记录同SQL事务提交；JSONL追加/fsync与所有journal扫描共享RWMutex，读快照与追加互斥，锁内不取SQL/Store互斥。journal使用标准NOFOLLOW并在打开前后/读写后验证SameFile及公共私有路径，不持DB事务跨文件IO。

持久prepared/committed决定必须有完整同材料JSONL；Find缺日志即完整性失败。启动反向核对全部已准备/提交索引，不能只扫描现存日志而漏丢失/截空记录；唯一允许无日志的是尚未append的preparing且没有提交资格，缺日志不猜补。


编辑锁原操作回执属于不可变历史结果，不能作为启动重建租约的来源；进程重启和恢复均清空锁并改bootId。原回执重放携旧boot不会恢复写资格。当前boot的未知SQL提交pending会阻止同doc后续编辑/保存，只有真实原回执和候选hash一致才安装，确定没有回执才丢弃。通知生产者outbox/本地审计已编写候选，完整转发与在线消费尚待后续装配，未做业务验收。
