# AI 接入当前进度

更新：2026-10-16。第六轮只实现QA免费任务配置可用性读取，分支codex/model-task-availability-r1，基线cbe93952464bc9bd4a159e4d870c1b191855bc3a，独占本域。源码、合同与VPS生成产物已交付，完整12项相关函数及对应完整迁移/生成文件verified；task.go和runtime/bootstrap.go各自未完成职责保持partial，完整启动仍blocked，业务验收未执行。

- ReadTaskAvailability仅QA纯服务、固定三任务，一次当前epoch SQL快照；缺任务/QA档位为not_configured，已配无可用元数据候选为unavailable；空/异代、损坏与查询未知失败关闭。
- 单任务覆盖替换默认主模型，保留原第二项起有序备用并去重；免费读取固定QA档位，不要求op.costClass变为qa。凭据只查存在/代次及nonce/ciphertext长度，不SELECT密文/主密钥/URL/价格/预算/余额，不探测或调用供应方。
- 新实际路径仅已登记internal/catalog/task.go及本轮批准追加migrations/002_task_availability.sql；profiles/assignment完整schema、AES-GCM凭据结构，无种子或费用写入。配置写路径及实际加密/轮换仍未实施，不能凭schema宣称已可配置运行。
- BuildApplication真实OpenDatabase/InstallStore并将单项映射交公共完整工厂，其他handler、权威代次初始化、恢复回执与后台缺失仍拒绝启动；不为免费读取调用需masterKey的LoadRuntime，不覆写未知epoch。
- 未新增测试/mock/fixture/样例或临时业务程序；未本机Go/gofmt/gen/编译/vet/check，未SQL业务验证、真实接口/模型、服务、部署或push。本包收尾后按用户要求停止，不开启后包或自行push，集成会话统一复核。
- 源码提交：23db600ead941c088344af2c3a33bf4204277229。2026-10-16 集成会话最终合成快照：VPS linux/amd64 go1.25.5，GOMAXPROCS=8，make docs/gen、14个现有Go模块-mod=readonly build/vet/gofmt及完整make check全部通过（861文件/1182函数/379操作/2269类型）；包含256项持久配置、先eligible过滤后16项路由参考裁决、最终只读代码和002 schema，生成/格式回传无冲突。仅本阶段源码/格式/结构检查，未执行SQL业务验证、业务测试、真实接口/模型、服务、部署或push。

以下为既有交付历史，当前以页首和progress.json为准。

更新：2026-10-15。阶段：R1实施中；本域已完成目录校验、模型身份纯规则和私有SQLite最小持久化切片，尚未业务验收或部署。

- 2026-10-15 集成复核修复：`PutProvider`/`PutModel` 事件改经平台 `outbox.EnqueueEvent` 写 nw_outbox，删除自建 outbox 表（迁移与内联副本），平台转发循环才能投递（ADR0005）。 其余修复与验证见 [handoff](docs/handoff.md)。编译、vet、gofmt 与 make check 已通过，业务验收未执行。
- 已实现：`LoadRuntime` 建立 active/control/restore 目录、严格迁移和32字节主密钥引用检查；`ValidateProvider`/`ValidateModel` 完成无网络纯校验；`PutProvider`/`PutModel` 完成CAS、幂等回执、屏障检查和变更outbox事务。
- 本地提交：`4eec945320f8cf20bfc6673a8c3c8490385aca2f`（实现切片）。
- 当前仍阻塞：BuildApplication 尚未接入本域完整数据库/所有OpenAPI handler、site-config代次接线、activation回执和后台循环；真实模型、连接探测、密钥加密/轮换与费用预留/结算均未实现。
- `EmbeddingSpace` 现有合同没有独立归一化规则字段，本切片只按 provider/modelVersion/dimensions/preprocessingVersion 计算spaceId，矛盾已记录交接；不得自行扩展公共合同。
- 本机无Go、gofmt、make，未执行编译、迁移运行、静态检查或行为验收；未新增测试/样例、未调用真实接口或模型、未push。
- 下一步：先在有Go工具链的VPS完成格式/编译验证，再接入BuildApplication与全部handler；随后实现凭据加密、档位/路由和预留账本。

并行工作只改本域，公共资料交集成会话；开始与结束均同步本状态、台账与交接。
