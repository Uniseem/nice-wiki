# 修改合同

改接口时先改 openapi.json 和 types.json，再跑 `make gen`；改了 implementation.json 跑 `make docs`；最后跑 `make check`。

接口或事件的破坏性修改要升版本，并写清迁移方式。参考级函数不用登记，也不用同步。

新增服务、新协议或新存储，以及删减产品功能，先问用户。
