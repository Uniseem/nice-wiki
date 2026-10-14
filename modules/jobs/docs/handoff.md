# 任务与事件会话交接

## 本次事实（R1-04 ValidateJobSpec，2026-10-15）

- 独立指定 worktree；分支：`codex/jobs-r1-04-validation`；基线：`82c83226b120b0376fa4ddfe5fe1b58f3c3e7d29`。
- 实现提交：`03ac7eff1ed562be4fdc18794080ae50ecfd900c`（`jobs：实现任务规格纯校验`）；未 push。
- 独占范围：`modules/jobs/`；本次仅新增 `internal/validation.go`，另同步本模块 `STATUS.md`、`progress.json` 与本交接文件；根文件、公共合同、生成物、其他模块、libs、deploy 未修改。

## 本次完成与边界

- `jobs.ValidateJobSpec` 为纯校验：检查 context 取消、可信调用服务是否为任务 owner、owner/type 是否精确命中当前 `config/job-types.json` 登记快照、ObjectRef 的 owner/ID/version/hash/非负 size/媒体类型，以及输入 `dataEpoch` 与可信 `OperationContext.DataEpoch` 是否相等。
- 失败返回零值与登记公共错误：规格字段或引用不合法返回 `VALIDATION`，epoch 缺失/不合法/不一致返回 `VERSION_MISMATCH`；不回显字段值，不写任务状态，不读对象正文，不调用外部服务。
- 去重键按机器合同属于 `CreateJobInput`，不属于 `ValidateJobSpecInput`；本次未向函数输入擅自增加字段。后续 `CreateJob` 必须校验其 UTF-8 字符串长度不超过 8192，并在 `(owner, dataEpoch, dedupeKey)` 范围内执行同输入哈希复用/冲突规则。
- 任务状态机、租约 fencing、事件、定时、屏障、存储和 HTTP handler 仍按既有台账阻塞，未被本次任务伪造实现。

## 检查与验收

- 已核对基线、分支与初始脏文件：路径匹配指定 worktree，初始工作树干净；提交内容仅在 jobs 模块。
- 已运行 `python scripts/check_docs.py --privacy-only`，返回码 0；已运行暂存内容 `git diff --cached --check`，返回码 0。
- 完整 `python scripts/check_docs.py` 返回码 1：公共 `docs/structure/framework.json` 尚未登记本次新增文件；本次按范围未修改公共文件，缺口留给集成会话。
- 当前环境无 `go`、`gofmt`、`make`，未能执行 Go 编译、格式检查、模块检查或行为测试；按仓库规则未用本地替代环境冒充 VPS 证据。
- 本项状态：源码 `implemented`；静态隐私检查已通过，但函数 `verified` 未登记；业务行为 `accepted` 未登记，未新增测试/样例/真实接口/模型/部署。

## 下一步

后续 `CreateJob` 工作包应复用本文件的 owner/type、ObjectRef 与 epoch 规则，并补齐 `CreateJobInput.dedupeKey` 的 UTF-8/长度校验及事务幂等；具备 Go/VPS 工具链后先复核本次实现的编译、gofmt 与行为检查。
