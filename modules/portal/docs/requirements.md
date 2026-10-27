# 需求与边界

入口、后台外壳与模块装配；偏好由 identity 拥有，portal 无业务数据仓库。

| ID | 行为要求 | 验收判定 |
|---|---|---|
| M01-R01 | 入口静态白名单、CSRF 与 owning-service 双重授权 | 越权、路径穿越与伪造 identity header 均拒绝；凭据不进入日志 |
| M01-R02 | 后台壳支持权限动态撤销与路由恢复 | permissionRevision 增大后取消失权操作，刷新导航；外部 returnLocation 拒绝 |
| M01-R03 | 流式请求有背压、取消和明确未知提交状态 | 断开关闭上游读取；同 operationId 可确认结果且写请求不盲重放 |
| M01-R04 | 用户偏好归 identity 权威 | portal 重启无偏好丢失，入口不维护第二份业务存储 |

每项当前状态均为未实现、未执行验收。`RedactRequest` 只落实 M01-R01 里“凭据不进入日志”的最小字段边界，不把整项需求标为完成。


来源：[产品保持矩阵](../../../docs/product-preservation.md)、[冻结产品基线](../../../docs/product-baseline/README.md)；具体产品来源与ADR在README来源索引和本域operations列出。所有需求状态未执行，不能以合同数量视为完成业务。
