# data-protection 实现工作约束

先读README及全部docs、四份contracts和根产品保留矩阵。当前仅设计，业务验收尚未执行。实现前核对根AGENTS与documentation-standard。

入口、契约、配置和约束函数按files登记路径实现；参考级函数及internal/内拆分可调整，交付时同步合同（ADR0002）；生成文件不可手改。每服务私有SQLite和对象目录，跨域只能调用生产方OpenAPI。所有写入校验epoch/fence；文件与数据库用恢复日志，不能虚称单事务。

不成为content/localization/memory等源历史权威

不读取共享数据库或其他服务私有磁盘；仅HTTP导出/导入协议

密钥/凭证/日志/提问正文/活动租约不进git镜像；本阶段不push远程

恢复控制journal与barrier在control区，不随业务快照替换。备份恢复控制操作只接受data-protection服务身份与superadmin委托；active目录只在进程重启时换入或换回（ADR0003）。

可以读old产品设计/ADR确认产品，后续ADR优先；不复用旧业务代码结构，不读vps.txt。本文不授权本阶段源码、测试样例、真实模型或接口、远程push。
