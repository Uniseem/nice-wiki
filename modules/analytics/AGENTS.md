# analytics 实现工作约束

先读README及全部docs、四份contracts和根产品保留矩阵。当前仅设计，业务验收尚未执行。实现前核对根AGENTS与documentation-standard。

只实现files登记路径，新增任何私有辅助函数也先补合同；生成文件不可手改。每服务私有SQLite和对象目录，跨域只能调用生产方OpenAPI。所有写入校验epoch/fence；文件与数据库用恢复日志，不能虚称单事务。

不记录IP/提问正文/模型输入输出/secret/token

不追踪跨站访客或持久设备指纹

不控制实时额度，不写费用账，不把派生汇总当结算真相

恢复控制journal、barrier和信任锚独立于业务快照。维护下验证RecoveryAuthorization的签名、audience、restore/barrier、两epoch和期限，不回查已被替换的identity数据。不得用恢复许可执行普通业务。

可以读old产品设计/ADR确认产品，后续ADR优先；不复用旧业务代码结构，不读vps.txt。本文不授权本阶段源码、测试样例、真实模型或接口、远程push。
