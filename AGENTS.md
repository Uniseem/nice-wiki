# 助手约定

nice-wiki 从零重建为可独立开发部署的微服务文档站。旧实现、旧文档与未提交会话记录归档于 old 分支；旧业务代码不复制、不作为新函数/文件结构的模板；用户已授权参考旧产品设计和上下文/缓存思路，站点功能、细节及已确认默认按old保留。界面、文档、注释及提交说明用中文，标识符用英文。

## 阅读与权威来源

先读 STATUS.md、docs/plan.md、docs/README.md，再读目标模块 AGENTS.md、README.md 及所索引子文档。全局要求和公共契约优先于模块文档，已确认 ADR 优先于草案。

新会话还须读目标模块 STATUS.md、progress.json、docs/handoff.md 并核对Git分支/HEAD/脏文件，按 docs/playbooks/handoff.md 接续。并行会话每模块独立worktree及互斥所有权；公共文件由集成会话统一修改，按 docs/playbooks/parallel-development.md 合并。模块会话只更新自己进度与交接，根状态由集成会话汇总，不能并行改同一Git index。

2026-10-20 分工约定：主代理只负责拆解、指挥、协调与复核；实际源码/文档修改、集成与 CI 操作由多个子代理执行。每模块使用独立 worktree 和互斥所有权，最多 3 个模块并行；公共文件由唯一集成子代理维护。

每模块 contracts/openapi.json 是 HTTP 唯一来源，contracts/types.json 是业务类型来源，contracts/implementation.json 是未来文件、函数及职责的机器契约，contracts/events.json 是其事件来源。人类文档必须与机器契约一致，发现矛盾先修设计，不猜测实现。

## 按契约实施

- 当前为 R1 实施阶段：按 docs/plan.md 工作包与各域合同实现；行为验收与部署另行授权。
- 函数合同分两级（ADR0002）：HTTP处理、事件转发/订阅/处理、任务执行与启动入口为约束级，名称、精确签名、字段约束、步骤、错误、副作用、权限、幂等、并发与调用边界照合同实现；其余内部函数为参考级，实现时可增删改名，无需事先登记，模块交付提交时把合同和台账同步为实际。缺约束函数或行为分支时先修契约，不把模糊步骤藏进实现。
- 本次用户授权整体归档与文档框架重建；后续结构外增加、删除、改名、移动与职责变化先取得授权，并同步结构和契约。
- API 先改 OpenAPI 再生成，生成目录不手改。服务间只走版本化 API/事件，不导入对方内部业务代码，不共享可写目录或业务表。
- 事件只经jobs中转：生产方在业务事务内写outbox，由本域转发循环提交给jobs；消费方由本域订阅循环拉取，交给events.json登记的handler，本地提交后才确认。
- 异步任务与后台函数登记在implementation.json的jobTypes与backgroundTasks；除界面函数外所有函数都必须能从入口到达（ADR0004）。服务身份、outbox/inbox、任务与订阅循环、模型调用、恢复换目录等机制只在libs/platform实现，服务入口函数只装配本域参数（ADR0005）。
- 块 ID 稳定且唯一，复制另生成 ID；格式改变升级 schemaVersion 并声明迁移。
- 每服务显式校验服务身份与用户权限，隐藏菜单不能替代服务端授权。
- 草稿保存不调模型；只有发布及用户明确批准的后台操作触发付费任务。保留、按哈希复用付费结果，不自动重算。
- 正文文件是权威数据；元数据、任务和费用状态有独立持久记录，不把它们全部当可重建索引。
- 配置、账号和密钥不进入 data git；日志不记录密码、模型密钥、提问正文或原始访客 IP。

## 实现尺度（ADR0006）

- 边界一次校验：请求结构由生成的严格服务端校验，服务身份、委托、dataEpoch、截止与 costClass 由 libs/platform 在入口建立；入口之后的函数信任 input 与 op，不重复解析 deadline、重做 schema 校验或重查调用方身份。
- 函数合同里的通用约束句、通用错误码和步骤末尾的通用句描述调用链整体保证，不逐函数实现；只实现本函数特有的 steps 与 authorization.rule。取消只靠把 ctx 传给下游 IO，纯函数不检查 ctx。
- 不写不可能发生的分支、对刚构造值的再校验、“以防万一”的重试/降级/兜底，不复制标准库或 libs/platform 已有工具；缺公共工具由集成会话在 libs/platform 补一次。
- 合同写明由本函数负责的权限、epoch/屏障、CAS、幂等、哈希核对与恢复隔离照旧失败关闭，不因精简放松。

## 检查与记录

编译门槛：Go 代码记 implemented/verified 前须 `make build`（各模块 `-mod=readonly` 的 go build、go vet 与 gofmt）通过；go.sum 与前端 lockfile 只由 go/pnpm 命令生成，不得手写。本机已装 Go，找不到 go 先检查 PATH；确实无法编译的记 blocked（待编译），不得合入 main 记作 implemented。跨服务公共机制先查 libs/platform 已有实现（运行状态客户端 `lifecycle.NewSiteConfigClient`、请求校验 `apivalidate.NewRequestValidator` 等），不在各域复制。

默认不写测试（ADR0006）：不新增测试文件、测试脚本、fixture、mock、样例或临时验证程序，验证只用已有的格式、vet、编译、make check/gen/docs。新增或修改测试、样例、快照、E2E、真实接口、真实模型和 AI 评测须用户当次明确同意；交接“下一步”不得列补测试。已有普通测试可运行，不改删跳过测试掩盖失败。

改合同后运行 make docs 重新生成各域函数与文件目录；结束前运行当阶段 make check，如实更新 STATUS.md、docs/plan.md、docs/push-log/_pending.md。当前检查仅验证设计结构、类型引用、接口/函数映射、依赖、文档链接和隐私，不代表业务验收。只对达到对应验收标准的事项打勾。

编译、lint、类型检查及已有普通测试全部通过用户自部署 GitLab CI 执行，尽量同时运行三个任务（2026-10-17 用户新指令，取代旧 VPS 直连规则）。只读用户指定私有机器管理仓库取得连接与凭据，禁止复制其敏感内容到本仓库、提交或交接。CI 只接收源码，排除私有配置、环境文件、凭据和运行数据；优先专用编译 Runner，不占用业务 Runner。生成后按三个分片执行全部 Go 模块的 readonly build/vet/gofmt，并运行完整 make check；内存密集任务限制单任务并行度。GitLab 不可用时记录 blocked 并等待恢复，未经用户当次明确授权不得回退本机或 SSH 编译。文档链接与结构读取、Git/隐私审阅可在本地执行。

## Git、隐私和外部代码

不主动 push、不强推；每次明确授权后按 docs/playbooks/push.md 执行。保持用户 Git 身份，不自行改配置，邮箱须为 GitHub 代用地址；实际 AI 合著者如实署名，GPT-6 使用 noreply@openai.com，Claude 使用 noreply@anthropic.com，GLM（ZCode）使用 noreply@z.ai。

扫描暂存内容、提交信息、作者、提交者和待推送提交。禁止真实个人邮箱、姓名、地址、电话、本机用户路径、真实 IP、密钥、访客数据；示例用保留域名和文档地址，署名代用地址例外。

借外部代码先核对许可，新借用项目先确认；文件标注出处、版本、版权，更新 THIRD_PARTY_NOTICES.md；GPL 项目只参考不复制；唯一例外是 Docmost 社区版编辑器扩展（AGPL-3.0，与本仓库许可相同，D51 与 ADR0005 已确认），借用时标注出处并登记。技术设计不等于授权复制第三方代码或调用外部服务。

---

## 双远程推送 / Dual-push (GitHub + GitLab)

自 2026-10-16 起本仓库镜像到自建 GitLab。提交后请同时推送：

| Remote | URL |
|---|---|
| `origin` (GitHub) | `https://github.com/Uniseem/nice-wiki.git` |
| `gitlab` | `https://gitlab.anylocate.cc/anylocate/nice-wiki.git` |

```bash
git push origin HEAD && git push gitlab HEAD
```

Fleet / SSH / Mesh / Runner 规则见 **Uniseem/machine-management**（`anylocate/machine-management`）。CI：仅有 GitHub Actions、无现成可跑 `.gitlab-ci.yml` 时 GitLab 侧为代码镜像 only，勿发明会失败的流水线。清单：machine-management `GitLab/镜像清单.md`。

