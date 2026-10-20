# 语言、翻译与术语实现约束

先读README索引全部合同和根product-preservation。用户授权参考old产品设计/确认ADR，所有功能细节保持，不能复制旧业务代码、结构、函数和接口。较新ADR优先。机器与人类契约矛盾必须先明确修订，不默改协议。未来实现只能访问本域私有数据；调用对方公开HTTP/事件接口。约束级函数的签名/输入输出/步骤/错误/副作用/授权/并发/幂等以implementation为准；参考级函数可自由调整，不用回写合同（ADR0002）。费用全部通过model-gateway；非MG不得创建账本或费用额度，匿名频率控制归guard。普通写入校验active dataEpoch和本地维护屏障；备份恢复控制只接受data-protection服务身份与superadmin委托，control区不随恢复替换（ADR0003）。

不读旧业务代码。流程以根 AGENTS.md 为准：本地 `make build`/`make check` 通过即可提交，可在本地启动服务验证；测试、真实模型调用和推送按根规则处理。
