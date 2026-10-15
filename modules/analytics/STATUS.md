# 统计与审计当前进度

更新：2026-10-15。阶段：implementation；本域业务尚未验收、尚未部署。

- 2026-10-15 集成复核修复：site-config 运行状态适配改为解开 `{"state": RuntimeState}` 外层，此前字段全空会被平台按“运行状态不完整”拒绝启动。 其余修复与验证见 [handoff](docs/handoff.md)。编译、vet、gofmt 与 make check 已通过，业务验收未执行。
- 本次复核基线为 `3db911390e35a07007d70a1d2f5cc15fe4092667`，独占分支为 `codex/analytics-r1-storage-contract-audit`；本次无业务代码交付，未 push。
- 已建立 `cmd/analytics` 入口、应用装配、HTTP 生命周期、匿名 health/ready 路由、独立 Go module、默认配置、镜像和部署 manifest。生成目录未手改。
- 所有已登记 OpenAPI 路由均绑定到生成 server interface；beacon、审计、访问/费用汇总、导出、屏障和快照业务尚未实现，统一按合同返回 `DEPENDENCY_UNAVAILABLE`，不返回伪造成功结果。
- 本次按交接复核 `migrations/0001_domain.sql` 与私有存储/journal切片时发现合同缺口：`implementation.json` 未登记 SQLite 打开/迁移执行、control journal 恢复、fsync/完整性收敛所需的文件、函数、签名、错误、副作用与调用边界；`types.json` 也没有对应内部接口。按 AGENTS.md 约定停止实现，不猜测实现，不创建半套迁移。
- `BuildApplication` 与 `Ready` 的域存储/journal、恢复激活回执、后台任务/事件循环和真实业务 handler 仍阻塞；就绪不会把未恢复的本域能力报告为可用。逐项事实见 [progress.json](progress.json)。
- 已通过 JSON 解析、生成 handler 覆盖静态核对、隐私检查和 `git diff --check`；本机没有 Go、gofmt 或 make，未执行编译、格式化、`make gen` 及业务验收。

并行工作只改本域，公共资料由集成会话维护；本次仅更新本域台账，未修改根文件、公共契约、生成物、其他模块、测试、样例、真实接口或模型。
