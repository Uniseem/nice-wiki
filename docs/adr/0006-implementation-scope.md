# ADR0006：实现尺度——边界一次校验，内部信任输入，默认不写测试

- 日期：2026-10-15。
- 状态：用户要求并确认（2026-10-15），立即适用于全部 R1 后续实现；已写代码不强制回改，修改到时按本 ADR 收敛。
- 来源：集成复核 R1 云端切片时发现，函数合同模板里的通用约束被逐个函数照字面重复实现：16 个文件各自解析 `op.Deadline`，15 个自写的 context 错误映射函数，5 个重造的字符串查找或 ID/哈希校验工具，纯计算函数也检查截止时间与取消。代码量膨胀、风格不一，真正的业务规则反而被淹没。

## 问题

implementation.json 为 905 个函数生成了同一套通用条文：输入“必须通过 types.json 对应 schema、拒绝未知字段”，ctx“与 op.deadline 取最早截止”，op“dataEpoch 一致、权限按资源再次检查”，错误表里普遍列出 CANCELLED/DEADLINE_EXCEEDED/UNAUTHENTICATED/FORBIDDEN，223 个函数的步骤末尾附“业务写入前校验 dataEpoch……截止=min(ctx deadline, op.deadline, 本函数 N ms)”。这些条文描述的是调用链整体要满足的保证，却被理解为每个函数都要自己实现一遍。

## 选择

1. **边界一次校验。** 请求结构（schema、未知字段、长度与范围）由 make gen 生成的严格服务端在 HTTP 入口校验；服务身份、用户委托、dataEpoch、截止时间与 costClass 由 libs/platform 在入口建立 OperationContext 时校验；事件与任务入口由平台循环建立。入口之后的内部函数信任已校验的 `input` 与 `op`，不重复解析 deadline、不重做 schema 校验、不重查调用方身份。
   - 实现：`libs/platform/apivalidate.NewRequestValidator` 按 ADR0005 用 kin-openapi 基于生成代码内嵌规格（`gen.GetSwagger()`）校验请求，各域 HTTP 绑定工作包在 transport 装配该中间件；不得在函数里逐个补写 schema 校验替代。
2. **通用条文按调用链解释。** functions.md 与 implementation.json 中 inputs/outputs 的通用约束句、错误表里的通用错误码、步骤末尾的通用句，都由入口与平台一次满足，不要求逐函数实现。函数只实现自己特有的步骤，以及 `authorization.rule` 中写明的本函数专属判断（例如“仅 data-protection 服务身份”）。错误表列的是可能透出的码，不是每个都要在本函数构造。
3. **取消与截止只靠传播。** 把 ctx 传给数据库、HTTP、文件等下游 IO 即可，长循环可在每轮读一次 `ctx.Err()`。不写自定义的截止解析和 context 错误映射；没有 IO 的纯函数不检查 ctx。通用的毫秒预算只在入口层设一次。
4. **不写防御性冗余。** 不写不可能发生的分支，不对本函数刚构造的值再校验，不加“以防万一”的重试、降级或兜底，不复制标准库或 libs/platform 已有的工具（`strings.Contains`、`platform.ValidIdentifier`/`ValidHash`、错误映射等）。缺少的公共工具由集成会话在 libs/platform 补一次，不在各域复制。
5. **真实边界照旧失败关闭。** 权限与站点作用域、dataEpoch 与维护屏障、revision CAS、幂等回执、内容哈希核对、恢复日志与隔离等合同步骤明确交给本函数负责的检查，照合同实现，不因“精简”放松。判断标准只有一条：这项检查是否写在本函数自己的 steps/authorization 里。
6. **默认不写测试。** 不新增 `_test.go`、测试脚本、fixture、mock、样例数据或临时验证程序（包括在交接里“用内存数据库试跑一次”）。验证只用已有检查：`go build`/`go vet`/gofmt、`make check`、`make gen`/`make docs` 可重复。确有必要时，在交接“建议”里写一句理由，用户当次明确同意后才写；“下一步”不得把补测试列为任务。
7. **合同模板暂不逐条删改。** 已有 905 条通用句保留，以本 ADR 的解释为准，生成的 functions.md 页首同步说明；以后重写合同模板时删去通用句，只保留函数特有规则。

## 代价

- 内部函数被入口之外的代码误用时，不会再逐层报错；靠“服务间只经版本化 API/事件”和代码审查约束调用边界。
- 少了单元测试，行为正确性要到 R2 集成与业务验收才有证据，台账的 verified 仍只代表编译与结构检查。

## 影响

- 根 AGENTS.md 增“实现尺度”一节，`docs/playbooks/implement-module.md` 增实施尺度说明，`scripts/render_docs.py` 在每份 functions.md 页首写明通用条文的解释。
- 代码审查与集成复核以本 ADR 判断冗余：多余的重复校验、自写工具和测试应删除，而不是保留。
