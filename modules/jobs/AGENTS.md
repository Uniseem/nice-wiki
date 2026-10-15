# jobs 实现工作约束

先读README及全部docs、四份contracts和根产品保留矩阵。当前仅设计，业务验收尚未执行。实现前核对根AGENTS与documentation-standard。

入口、契约、配置和约束函数按files登记路径实现；参考级函数及internal/内拆分可调整，交付时同步合同（ADR0002）；生成文件不可手改。每服务私有SQLite和对象目录，跨域只能调用生产方OpenAPI。所有写入校验epoch/fence；文件与数据库用恢复日志，不能虚称单事务。

不承载翻译/embedding/渲染/恢复算法

不重做已付费成功结果，不持有模型密钥或钱账

不把停止新任务等同所有在途领域写入已安全

恢复控制journal与barrier在control区，不随业务快照替换。备份恢复控制操作只接受data-protection服务身份与superadmin委托；active目录只在进程重启时换入或换回（ADR0003）。

可以读old产品设计/ADR确认产品，后续ADR优先；不复用旧业务代码结构。R1实施中：按 docs/plan.md 工作包与本域合同创建源码；Go 代码记 implemented 前须 make build 通过（全部在VPS执行，连接信息只读被忽略的私有 vps.txt；VPS不可用时记录blocked，未经用户当次明确授权不得回退本机编译或测试）；测试/样例按 ADR0006 默认不写；不执行真实接口/模型；push 须用户当次授权。业务验收保持未勾选。
