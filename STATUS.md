# 当前状态（2026-10-20，用户恢复R1实施）

主代理已审核通过并冻结 [docs/plan.md](docs/plan.md)；覆盖 15 个业务域、`libs/platform`、UI、任务与事件、保护与恢复、部署源码、R2 和最终发布交接。用户已恢复 R1 源码实施，并改为主代理只拆解、指挥、协调与复核，多个子代理负责实际修改、集成和 CI；每模块独立 worktree，公共文件由唯一集成子代理维护。本轮仍不授权 R2、真实业务接口/模型调用、服务运行、镜像构建或部署。

本次集成已将本地 `main=6204dfc94510f28752fe8da81df9f3a4d3ce781d` 安全合入当前集成树，保留 `e85dde0583938bb4207bc3a1117e62549d21018c` WIP记录；main 已包含 pipeline173/174 验证的 `VerifyActivationReceipt` 与 site-config `RegisterActivation`。当前公共候选继续保留 e85 的 `SignActivationReceipt`、`ObserveTx/SealTx/ResumeTx`，并新增只读 `ReadBarrierStatus`、`MigrateRestoredDatabase`及非jobs不等待最终broker的分阶段封口合同；这些新增/修改源码尚无本次候选GitLab SHA，仍按blocked/in_progress记录。

identity源码交付 `e6c131dfbbb1bf117e60c6eaca1faa5b3a809743`、clean HEAD `0c56660baa6dd2d5a93bb42da550328a45cfccf5`；jobs源码交付 `879a41d430b89af5a0b88dd8d86d24f8f25763e3`、clean HEAD `7bc01a2917302324b84e3f594c5f249b7fc22c4e`。两模块暂不合入：均等待平台公共合同和jobs `step=observe` service-only授权分支明确；identity/site-config的 `VerifyGlobalSeal` 缺协调只读证据时必须失败关闭。用户未跟踪的 `kangaroo-down-under.html` 未触碰。

远端推送基线仍为 `b986443d0dddf5482f7c3430e3dc7c8b99a24232`，对应 pipeline 162 全绿且生成补丁 0。本地计划收尾基线为 `1857ef4bb1a936d3e8c7a8b091a38ee2e2331ef8`，对应 pipeline 163 全绿且生成补丁 0。本轮不把任何源码 WIP 或失败候选计入正式交付。

四路 WIP 原样保留，不 reset、不强推：

- `codex/closeout-20261020-integration`：`e85dde0583938bb4207bc3a1117e62549d21018c`。pipeline 166 生成、三个 Go 分片 build/vet/gofmt、既有普通测试和前端通过，生成补丁 0；完整 check 仍因 5 个公共函数没有真实消费者依赖边而失败，不交付。
- `codex/closeout-20261020-identity`：HEAD `5466c47d22249c224d57009a329786f1d019fa80`。新增后台参考函数尚未完整登记，实际 caller 未接齐，不交付。
- `codex/closeout-20261020-site-config`：HEAD `6d83888b39ebbd3a624f58d820274e7c6f3072c7`，源码链为 `42b30e41`、`b5895d88`。relay 仍不可达，不交付。
- `codex/closeout-20261020-jobs`：HEAD `76325f0d024563daec80037f42e021112de39fad`，源码链为 `34d3fa5a`、`0644f1d1`。真实后台、readiness 与保护来源仍不完整，且没有完整 CI 证据，不交付。

三域 worktree 均干净且代理已关闭，提交只保留在本地 WIP 分支；没有 push、进入 main 或新增完整 CI，完成数不增加。

全仓函数台账仍为 1645/2414（68.1%，已含 platform）：verified 1604、implemented 41；未完成状态 planned 267、in_progress 8、blocked 494，accepted 0。15 域为 2179 项，其中未完成状态 675；platform 为 235 项，其中 blocked 94。两者合计未完成 769，不再另加 platform；41 个 implemented 另行冻结核验。15 域完整 `BuildApplication` 均未闭合。

文件台账仍有 669 项未到 verified（blocked 274、implemented 108、in_progress 13、planned 274；verified 391）。同域 Dockerfile/Makefile/manifest 共 45 项中，verified 3、implemented 29、blocked 3、planned 10；`deploy/` 的 5 个总编排文件仍全部 planned。

冻结覆盖口径：

- 15 域非 verified 716 项（未完成 675、implemented 41）与平台 blocked 94 项全部归入 19 个交付包及 3 个无重复计数的联合闭合门；全仓非 verified 合计 810，769 只计未完成状态。
- 41 个 implemented 函数及 108 个 implemented 文件在最终源码冻结核验；baseline verified 只在改动、依赖变化或实际证据冲突时重开。
- `docs/frontend.md` 的完整后台页面表和公开问答/语言建议岛属于完成范围；identity、site-config、media 的账号、站点和资源库页面由 Portal 组合实现，不能漏项。
- 15 个服务独立镜像/manifest、Compose/Caddy/服务地址/公钥/secret 引用和自动重启属于 R1-D05 与最终冻结证据；退出码 75 仅用于 ADR0003 数据恢复切换/换回后的重启，代码镜像升级回退另按兼容镜像与升级前备份点。实际已核对 portal/public-site/qa 缺 Dockerfile，public-site/qa 缺 deploy manifest；台账状态不能代替文件存在。源码阶段不生成或保存真实私钥，不构建或部署真实实例。
- 无 AI 初始化按 bootstrap claim、site-config 自身 runtime、首次恢复通道和启动依赖表核对，禁止形成“初始化等待服务 ready、服务 ready 等待 initialized”的环。

R1-FZ 通过只表示源码就绪。R2 真实业务/接口证据、真实模型调用和部署分别等待用户当次授权；R2 通过后才可进入指定环境的发布与回滚交接。

本轮实际交付和最终回执见 [2026-10-20-0001](docs/push-log/2026-10-20-0001.md) 及 [push-log 索引](docs/push-log/INDEX.md)。源码工作保持冻结，不自动开始下一包；R2、真实模型/接口、部署未执行。
