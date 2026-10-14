# 统一模型网关实现约束

先读README索引全部合同和根product-preservation。用户授权参考old产品设计/确认ADR，所有功能细节保持，不能复制旧业务代码、结构、函数和接口。较新ADR优先。机器与人类契约矛盾必须先明确修订，不默改协议。未来实现只能访问本域私有数据；调用对方公开HTTP/事件接口。约束级函数的签名/输入输出/步骤/错误/副作用/授权/并发/幂等以implementation为准；参考级函数可在实现时调整，交付时同步合同（ADR0002）。费用全部通过model-gateway；非MG不得创建账本或费用额度，匿名频率控制归guard。普通写入校验active dataEpoch和本地维护屏障；恢复控制仅限定RecoveryAuthorization，control.db与信任锚不导入覆盖。

当前设计阶段禁止创建业务源码、测试/样例、调用真实接口/模型、读旧业务代码、读vps.txt和push。业务验收保持未勾选。
