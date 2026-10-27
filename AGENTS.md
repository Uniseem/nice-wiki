# 助手约定

nice-wiki 是微服务文档站，2026-10-27 起按[新架构设计](docs/architecture-v2.md)改造成多用户平台（6 个服务），在 main 上分阶段进行并复用现有代码；改造前的 16 服务实现留在 archive-16-services 分支。更早的旧实现在 old 分支，只参考其产品设计和上下文/缓存思路，不复制旧代码。界面、文档、注释、提交说明用中文，标识符用英文。

2026-10-21 起流程以效率为先。本文件优先于 ADR、docs/plan.md 和模块文档里的流程性要求（编译方式、记录、授权门槛、并行分工）；ADR 和合同里的技术设计仍然有效，但与新架构设计冲突时以新架构设计为准。

## 工作方式

- 目标是让站点尽快真正跑起来，不是把记录写完整。
- 开工前只读 [STATUS.md](STATUS.md)、[新架构设计](docs/architecture-v2.md) 和要改模块的 README.md、contracts/；其余文档按需查，不要通读。
- 单会话、顺序推进，在 main 上小步提交。不开 worktree、不拆子代理、不建集成/CI 候选分支，除非用户要求。
- 技术细节按最简单可行的方案自己决定，不停下来等授权；只有产品取舍、范围增减、花钱或对外的操作才问用户。
- 不写过程记录：任何文档都不记 SHA、pipeline/job 号、补丁哈希、"证据"、交接流水。git log 就是历史。
- 做完一件事，只更新 STATUS.md 里"现在能用什么 / 下一步"，保持一页以内。

## 编译与检查（全部在 NL 编译机）

- 2026-10-26 起，编译、检查、测试和冒烟实例都放在 NL 编译机上（连接方式与工作区约定见机器管理仓库），本机只改代码、开浏览器。源码用 rsync 同步到 `/srv/build/nice-wiki`（不含 node_modules、.local、web-dist），Go/pnpm 缓存放 `/srv/build/nice-wiki-cache`，可随时清理。
- `make build`：全部 Go 模块 build/vet/gofmt，约 30 秒。改了 Go 就跑。
- `make check`：合同结构、链接、隐私，约 2 秒。改了合同或文档就跑。
- 改 OpenAPI/合同后跑 `make gen`、`make docs`；生成目录不手改，go.sum 和 pnpm lock 只由 go/pnpm 命令生成。
- 冒烟用 NL 上的测试实例（端口偏移、只监听 127.0.0.1），本机经 SSH 隧道用 curl 或浏览器验证；模型联调可接 OpenRouter 的免费模型（`:free`），只给问答档位，后台任务用假供应方。用完删掉实例里的密钥数据。
- GitLab CI 只是推送后的兜底，不等它，也不把它当编译手段。

## 合同

- 每个服务的 contracts/openapi.json 是 HTTP 唯一来源，Go 与 Rust 的接口代码都由它生成；改接口先改 OpenAPI 再 `make gen`。服务间只走版本化 API/事件。
- 不再维护函数级 implementation.json，新代码不为它登记函数；events.json 只登记事件名称、版本和载荷。函数登记和 functions.md 已删除，implementation.json 只留文件、`consumes`、任务类型和后台任务，检查脚本已调整。
- 合同与能跑通的实现冲突时，改合同中被调用方真正依赖的部分，然后继续；不要为了让合同完整而写代码。

## 架构底线

- 服务之间不导入对方内部代码，不共享可写目录或业务表。全部关系数据共用一个 PostgreSQL 实例，每个服务独立 schema 和数据库账号，不读写其他服务的 schema（阶段 2 换库前沿用现有 SQLite）。
- 事件和任务经 PostgreSQL 公共队列：生产方在业务事务内入队，消费方按租约领取、按事件 ID 去重（阶段 2 换库前沿用现有 jobs 中转）。服务身份、队列、任务循环、模型调用这类公共机制，Go 只在 libs/platform 实现，Rust 只在 Rust 平台库实现，不在各域复制。
- 每个服务在服务端校验服务身份和用户权限；隐藏菜单不能代替授权。
- 多用户数据按 site_id / user_id 隔离：所有查询都以 site_id（用户级数据以 user_id）限定，跨站点访问在服务的查询层拒绝。
- 块 ID 稳定唯一，复制时生成新 ID；格式变化升 schemaVersion 并写迁移。
- 草稿保存不调模型；只有发布和用户批准的操作才触发付费任务，付费结果按哈希复用。
- 正文文件是权威数据。
- 配置、账号、密钥不进 git；日志不记录密码、模型密钥、提问正文、原始访客 IP。

## 实现尺度

- 边界只校验一次（生成的服务端校验 + 平台库入口），内部函数信任输入。
- 不写不可能发生的分支，也不写"以防万一"的重试、降级、兜底。
- 先打通主链路，备份恢复这类横切机制放在后面（跨域屏障、快照和换目录恢复协议已删除）。服务能启动优先：未完成的接口返回 DEPENDENCY_UNAVAILABLE，不要因此让整个 BuildApplication 失败。
- 默认不新增测试文件、fixture、mock；需要时先问用户。已有测试可以运行。
- 真实模型调用、真实付费接口、部署到服务器，都需要用户当次同意。

## Git、隐私和外部代码

- 只在用户要求时推送，不强推。推送时双推：`git push origin HEAD && git push gitlab HEAD`。
- 保持用户的 Git 身份（GitHub 代用邮箱），不改 git 配置。AI 合著者如实署名：GPT-6 用 noreply@openai.com，Claude 用 noreply@anthropic.com，GLM 用 noreply@z.ai。
- 提交前跑 `make privacy`。禁止出现真实邮箱、姓名、地址、电话、本机用户路径、真实 IP、密钥和访客数据；示例只用保留域名和文档地址。
- 私有机器的连接信息只在 machine-management 仓库，不复制进本仓库。
- 借用外部代码先核许可，标注出处、版本和版权，并登记 THIRD_PARTY_NOTICES.md。GPL 项目只参考不复制，唯一例外是 Docmost 社区版编辑器扩展（AGPL-3.0）。
