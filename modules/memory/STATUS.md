# 长期记忆当前进度

更新：2026-10-15。阶段：R1 实现准备；本域业务尚未实现、尚未行为验收、尚未部署。

- 本次认领“启动与契约生成准备”，基线为 `ac593c57861d2242f27d07562a2acae535d18c38`，分支为 `codex/memory-r1-13-bootstrap`。
- 已提交 `fa8de2243d3865be49411739ea3afc5ccb3c68b9`：建立 `cmd/memory` 入口、`BuildApplication` 前置装配、HTTP 生命周期函数、独立 `go.mod`、Makefile、非敏感默认配置、manifest 与非 root Dockerfile；未 push。
- `BuildApplication` 已按合同校验模块、依赖、目录、信任锚、签名密钥、运行态和恢复换目录；因业务存储、全部 OpenAPI handler、激活回执、outbox/订阅、任务执行器与后台循环尚未实现，明确返回 `DEPENDENCY_UNAVAILABLE`，没有空路由或伪成功。
- `Serve`、`Shutdown`、`Health`、`Ready` 已写入，但尚未通过 Go 编译、gofmt、vet；`main` 与 `BuildApplication` 保持 blocked。生成的 Go/TS 文件未手改、未运行全仓 `make gen`。
- 本域未来目录仍为 58 个文件、60 个函数；除本次启动组外的业务文件/函数保持 planned。`go.sum` 尚未生成：本机缺少 Go 工具链，待 VPS 执行 `go mod tidy`/下载后登记。

检查记录：

- `python scripts/check_docs.py --privacy-only`：通过。
- memory 合同、配置、进度 JSON 解析：通过；`git diff --check`：通过。
- `python scripts/check_docs.py`：按预期因根 `docs/structure/framework.json` 尚未登记本次 8 个新文件失败；模块会话不修改根公共结构文件。
- 本机未找到 Go、gofmt、make、Docker；未运行 `make check`、`make gen`、编译、vet、镜像或业务测试。未调用真实接口/模型，未新增测试/样例。

下一项：实现 memory 私有持久化与启动恢复，先读本域 data/workflows、`memory.LoadRuntime` 合同及 `migrations/001_initial.sql` 登记，再按 CAS、journal、dataEpoch 和正文文件权威规则落地；完成前不得接入业务 handler 或 worker。
