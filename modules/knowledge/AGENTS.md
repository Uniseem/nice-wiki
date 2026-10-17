# 知识入库与混合检索实现约束

先读README索引全部合同和根product-preservation。用户授权参考old产品设计/确认ADR，所有功能细节保持，不能复制旧业务代码、结构、函数和接口。较新ADR优先。机器与人类契约矛盾必须先明确修订，不默改协议。未来实现只能访问本域私有数据；调用对方公开HTTP/事件接口。约束级函数的签名/输入输出/步骤/错误/副作用/授权/并发/幂等以implementation为准；参考级函数可在实现时调整，交付时同步合同（ADR0002）。费用全部通过model-gateway；非MG不得创建账本或费用额度，匿名频率控制归guard。普通写入校验active dataEpoch和本地维护屏障；备份恢复控制只接受data-protection服务身份与superadmin委托，control区不随恢复替换（ADR0003）。

不读旧业务代码。R1实施中：按 docs/plan.md 工作包与本域合同创建源码；Go 代码记 implemented 前须 make build 通过（全部经用户自部署GitLab CI执行，尽量三个并行任务；连接与凭据仅从用户指定私有机器管理仓库读取且不入本仓库，优先专用编译Runner；GitLab不可用时记录blocked，未经用户当次明确授权不得回退本机或SSH编译）；测试/样例按 ADR0006 默认不写；不执行真实接口/模型；push 须用户当次授权。业务验收保持未勾选。
