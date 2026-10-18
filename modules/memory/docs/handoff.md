# 长期记忆会话交接

## 本轮交付（2026-10-18）

独占 `modules/memory/`，分支 `codex/memory-20261018-bulk`；从 acc9bbc 开始。公共模型适配 8c593f3 在本工作树 cherry-pick 为 a56407e，集成时跳过这条已归根所有的公共提交。业务源码提交为 `bb4b678c4c1f78a59b837f9a5bdaeac8b1888ce8`，其后另提交本记录。准确完整 ready 函数 ID 见 progress.json.readyAwaitingCI：39 个既有业务/已修改函数、52 个参考级辅助。全部 blocked 待统一 GitLab CI，不是已编译/验收。

人工条目链、映射与预算链、上下文完整预算/renderer/manifest 追溯、助手对话与候选管理、人工批准入口/记忆应用/术语本地命令、助手固定任务输入与执行提交、持久取消及原任务重投均已有源码。存储字段/handler 类型在 contracts 来源中同步；OpenAPI 需统一生成 readProfiles 客户端、MemoryProposal.sourceLanguage 及 PayloadValidationSpec 后才能编译。新增迁移是 002_business.sql，001 未改。

对象迁至 active/objects 随恢复换目录；非空旧根 objects 拒启动，不自动丢弃/迁移。JSON frontmatter/schemaVersion=1、精确 renderer、预算revision/语言fallback/bodyHash进入manifest的规则见 data/workflows。入口用平台严格请求校验及即时身份权限；内层信任入口，不逐函数重复schema/身份。收费助手只选择当前任务 accurate 对应 admin-probe 的真实 profileId；纯服务配置读取清 portal proof、保留 site/root/epoch。

## 尚未完成的真实依赖

BuildApplication 仍由完整HTTP工厂拒绝：Acquire/ReleaseBarrier、Export/Stage/DiscardSnapshot 未实现；事件订阅及术语终态处理、命令dispatcher、RunWorker与UpdateProjectOverview/CountPublication未齐。不能把上述39项源码就绪等同完整助手或服务已可运行。助手创建先本地固定 queued/待投递请求，恢复函数只重投同去重键；worker需在下一切片接平台循环，未伪造执行成功。

无用户当次行为验收授权；不列补测试计划。保留既有未改 verified 证据；本轮修改项仅 blocked（待统一GitLab CI）。未执行本地/SSH工具链，不得回退；需在 GitLab 统一生成、gofmt、14模块 readonly build/vet和完整 make check，生成补丁由根回传。Git/隐私审阅与 git diff --check 是本轮仅有的本地检查范围。

## 下一步（一项）

先读本域 implementation.json jobTypes/backgroundTasks、workflows 与平台 worker/client 契约，在 runtime/worker.go 和 app/wire.go 接好受信任务工厂与 RunAssistant、UpdateProjectOverview 执行器；所有任务须固定本域输入对象、root/site/epoch/原操作，并由平台检查租约/取消。worker 不得猜ID，不得重新创建收费尝试。完成后立即交付小切片供根统一 GitLab CI，再补其余事件/恢复控制。
