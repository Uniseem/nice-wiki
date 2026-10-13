# 公共协议

## HTTP与类型

服务内部API采用OpenAPI 3.1和JSON Schema Draft2020-12；每域唯一来源在contracts/openapi.json和types.json。请求用JSON，文件用二进制/分段，长结果用SSE。JSON不接受未声明字段，required、null、范围和枚举按schema；未知版本明确拒绝。公共类型在 [common.schema.json](../contracts/common.schema.json)，操作权限与错误码各有全局登记。

每operation明确operationId、x-handler函数ID、x-permission、请求、成功及失败响应；有路径变量必须声明参数。响应不把内部堆栈、密钥、路径或提问内容混进Problem.detail。问题响应采用application/problem+json，字段参照 [RFC9457](https://www.rfc-editor.org/rfc/rfc9457)。协议来源参照 [OpenAPI3.1](https://spec.openapis.org/oas/v3.1.0)。

## 身份与上下文

服务凭证确定调用服务，用户委托确定业务主体与权限。OperationContext含requestId、operationId、rootOperationId、callerService、actor、deadline、dataEpoch；付费调用加可信costClass和取消标识。公开客户端不能指定费用类别、匿名网络作用域或后台身份。grants不含资源可见性的推测：每域仍按资源归属执行检查。

身份令牌只接受固定发行方、固定签名算法、audience、有效期及可撤销版本；拒绝客户端选择算法。具体密钥产生/轮换见identity函数合同；JWT字段含义参照 [RFC7519](https://www.rfc-editor.org/rfc/rfc7519)，不复制第三方实现。

## 时间、版本、错误与幂等

机器时间统一RFC3339 UTC；界面转换用户时区。钱统一USD整数micros，priceVersion随费用记录，禁止浮点累计。页大小默认20、上限100，游标签名且绑定过滤条件。写入含expectedVersion或expectedRevision；锁带lease与单调fencingToken，过期锁不能覆盖后继锁。

幂等键绑定调用方、业务操作、请求体哈希和dataEpoch；同键不同请求409，同键重放返回原operation，不能再次调模型。网络超时不证明供应方未接收；供应方attempt与业务operation分别记录。错误码/HTTP映射见 [errors.json](../contracts/errors.json)。

## SSE与取消

先accepted(operation/call ID)，再progress、delta、citation，终态completed或error。序号单调，重连从持久事件水位读取；已输出文本后不将备用模型结果拼成同一回答。心跳不作为业务进度。取消向子任务传播，停止未开始的付费步骤，已开始调用用独立有限结算上下文记录费用。

## 事件与公共管理

EventEnvelope必须符合生产方events.json；至少一次持久投递，按对象revision拒绝旧事件，按eventId去重，dataEpoch不匹配拒写。事件传必要元数据与不可变ObjectRef，不任意抓客户端提供URL。

有状态域的管理API为barriers和snapshots export/validate/prepare/commit/rollback；request/response使用公共schema。入口无业务存储只参加维护，不虚构快照。所有管理路径受服务调用方、超管委托、屏障以及dataEpoch共同限制。
