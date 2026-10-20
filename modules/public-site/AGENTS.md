# public-site 实现工作约束

先读README及全部docs、四份contracts和根产品保留矩阵。当前仅设计，业务验收尚未执行。实现前核对根AGENTS与documentation-standard。

入口、契约、配置和约束函数按files登记路径实现；参考级函数及internal/内拆分可调整，交付时同步合同（ADR0002）；生成文件不可手改。每服务私有SQLite和对象目录，跨域只能调用生产方OpenAPI。所有写入校验epoch/fence；文件与数据库用恢复日志，不能虚称单事务。

不持有正文源历史，不修改发布状态

不根据缓存或异步事件单独判断页面仍可公开

不执行翻译算法、不调用付费模型、不控制费用账本

恢复控制journal与barrier在control区，不随业务快照替换。备份恢复控制操作只接受data-protection服务身份与superadmin委托；active目录只在进程重启时换入或换回（ADR0003）。

可以读old产品设计/ADR确认产品，后续ADR优先；不复用旧业务代码结构。流程以根 AGENTS.md 为准：本地 `make build`/`make check` 通过即可提交，可在本地启动服务验证；测试、真实模型调用和推送按根规则处理。
