> 产品需求基线快照：来源 old:docs/adr/0021-admin-accounts.md，blob 7de773fe2fd6add165b2165f2f9637812a8fe221。保留功能与细节；旧路径、数据布局和代码组织属于原架构，不是新实现结构。后续已确认ADR优先。

# D21. 后台账号：超管 + 子账号，账号密码登录

- 状态：✅ 已定
- 日期：2026-10-12
- 初始化渠道条款：已被 [D55](https://github.com/Uniseem/nice-wiki/blob/b8190c7/docs/adr/0055-setup-without-ai.md) 取代；其余账号约定继续有效。

详见 [design/accounts.md](https://github.com/Uniseem/nice-wiki/blob/b8190c7/docs/design/accounts.md)。

- 第一次进入后台显示注册页，第一个注册的人成为超级管理员。之后只有登录页，不再有注册入口。
  - 不加初始化码（用户的选择）。部署后要立即完成注册。
- 超管新建账号，设置初始密码。用初始密码登录的人，必须先修改密码。
- 账号名不能修改。修改密码：输入旧密码，新密码输入两遍。
- 超管忘记账号或密码：
  - 账号在 `data/config/accounts.json` 里查；
  - 密码通过在服务器上放一个重置文件来修改。
- 账号、密钥、配置都存在 `data/config/`，不纳入 git。
- 第一次注册后进入初始化向导：站点名称、基准语言、OpenRouter 密钥。
