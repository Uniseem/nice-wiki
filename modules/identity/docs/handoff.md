# 身份权限会话交接

## 本次事实（R1-01 超管不可变纯规则，2026-10-15）

- 分支：`codex/identity-r1-05-superadmin`；基线 `82c83226b120b0376fa4ddfe5fe1b58f3c3e7d29`；源码提交 `7a059d4e95f8043808cecc21537afc5a18f38a6a`。未 push、未部署。
- 独占范围：仅 `modules/identity/`；本次实际修改 `internal/account_validator.go`、`STATUS.md`、`progress.json`、`docs/handoff.md`。未修改 contracts、生成目录、根文件、其他模块或公共库。
- `identity.AssertSuperadminImmutable` 已实现为纯规则函数：输入状态枚举先校验；`isSuperadmin=true` 仅允许当前与目标状态均为 `active`；非超管按 `active↔disabled→deleted` 状态机判断；不从 `permissions` 推断超管身份；无持久化、网络、会话或HTTP副作用。
- 状态口径：源码与台账为 `implemented`；因本环境无 Go/gofmt，验证为 `not_run`；M02-R01 业务验收为未执行、未接受。
- 未新增测试、样例、快照、E2E、真实接口或真实模型调用。

## 本次检查与阻塞

- `git diff --check` 与提交前暂存差异检查已通过。
- Go、gofmt、make 在当前环境不可用，未运行 `go build ./...`、`go vet ./...`、`gofmt -l` 或 `make check`；不可将本次实现标为 verified。
- 根 `docs/structure/framework.json` 与 `THIRD_PARTY_NOTICES.md` 的既有公共登记缺口仍由集成会话处理；本模块会话未修改公共文件。

## 本次事实（R1-01 纯规则函数组，2026-10-15）

- 分支：`codex/identity-r1-04-pure-rules`（managed worktree）；基线 `ac593c57861d2242f27d07562a2acae535d18c38`；源码与依赖提交 `bdcb9128d3ec5983d54c57f0fddc7abb793e93f4`。无未提交文件、未 push、未部署。
- 独占范围：仅 `modules/identity/`；新增/修改 `internal/account_validator.go`、`internal/credential_service.go`、`go.mod`、`go.sum`。未改 contracts、生成目录、根文件、其他模块或公共库。
- 未新增测试、样例、快照、E2E、真实接口或真实模型调用；业务验收仍未执行。

## 本次完成

- `identity.NormalizeAccountName`：先验证 UTF-8，再做 NFC；拒绝空值、控制字符、首尾 Unicode 空白和超过 64 个 Unicode 字符；不 trim、不大小写折叠、不回显原值。
- `identity.HashPassword`：使用 `x/crypto/argon2` 的 Argon2id v19，固定 `memory=65536 KiB`、`time=3`、`parallelism=4`、`hash=32`、CSPRNG 16 字节盐；输出 PHC 编码，受限并发，检查 context/operation deadline，清理敏感缓冲。
- `identity.VerifyPassword`：解析并限制 PHC Argon2id 成本不超过当前上限，派生后常量时间比对；仅对有效口令返回 `needsRehash`，损坏或越界编码返回 `INTEGRITY_ERROR`。
- `identity.ValidatePasswordChange`：按 schema 验证旧/新口令长度与 UTF-8，要求两次新口令完全相同且与旧口令不同；语义不满足返回 `valid=false`，不记录或回显口令。
- `go.mod` 新增固定版本 `golang.org/x/crypto v0.28.0`、`golang.org/x/text v0.20.0`；两者许可证核对为 BSD-3-Clause。根 `THIRD_PARTY_NOTICES.md` 尚未登记，交集成会话补登记。

## 检查证据

- 基线与提交前 `git status --short --branch`、`git diff --cached --check`、作者/邮箱和暂存隐私扫描通过；提交内容仅在 identity 模块内。
- `git diff --check` 通过。当前环境没有 Go 工具链、gofmt 或 make，未执行 `go build ./...`、`go vet ./...`、`gofmt -l`；不可把源码实现标为 verified。
- `python scripts/check_docs.py` 已运行但因根 `docs/structure/framework.json` 尚未登记两个新增源码文件而失败；`make check` 因 make 不可用未执行。根 framework 与第三方声明是公共文件，按并行规则交集成会话处理。
- 无行为测试或业务验收证据；M02-R01～R06 仍未执行。

## 依赖与边界

- 本次只落地纯规则函数；会话、账号 CRUD、HTTP 路由、后台循环、`AssertSuperadminImmutable`、`ChangePassword` 均未实现。
- 公共登记缺口：根 `THIRD_PARTY_NOTICES.md` 需登记 x/crypto 与 x/text；根 `docs/structure/framework.json` 需登记两个新源码文件。未在本会话修改公共文件。
- 依赖版本已固定在 identity module；Go 构建需在具备 Go 1.25 工具链的 VPS 或集成环境复核。

## 本次事实（R1-01 私有持久化，2026-10-14）

- 分支：codex/identity-r1-01-storage（worktree `../nice-wiki-identity`）；基线 bbb1e535c2f1（main）；实现提交 28af316bf6a42a2378f30f17a2fe036380af4ff9；台账提交紧随其后。
- 独占范围：modules/identity/。公共文件仅在交付提交中同步 docs/structure/framework.json（+3 文件）、THIRD_PARTY_NOTICES.md（modernc.org/sqlite v1.59.0），请集成会话复核。
- 未提交文件：无。未新增测试/样例/真实接口调用；未 push、未部署。
- 沿用说明：migrations/migrations.go 与 0001_domain.sql 初稿来自并行子代理（因用量上限中断），本会话核对其符合 data.md/ADR0005 后沿用并补齐存储层、装配接线与台账。

## 本次完成

- internal/storage.go：identity.OpenDatabase（WAL/foreign_keys/busy_timeout=3000ms、单连接串行、control 区决定日志 0600 追加句柄）、identity.RunMigrations（migrations.All 校验编号连续，事务执行并登记 schema_migrations，集外版本 INTEGRITY_ERROR）、identity.RecoverJournal（逐行末行状态生效，open 条目隔离）；AppendDecision/CompleteDecision/HasSuperadmin/Close 为未登记参考级助手，随首个调用方交付时登记。
- BuildApplication 在 ApplyRestoreSwap 后打开业务库并执行迁移与恢复；Ready 补 initialized（唯一超管存在）/journalRecovered/activeDataEpoch（打开时代次，运行代次不一致 503）；Shutdown 在 HTTP 关闭后关闭业务库。
- 合同同步：implementation.json 增 internal/storage.go、migrations/migrations.go（函数 identity.All）与 0001_domain.sql（migration 类），BuildApplication 依赖补三个存储函数；make docs 已重生成 files.md/functions.md；progress.json contractSha256 已重算并新增 4 函数/3 文件行。

## 实现口径（供后续与集成对齐）

- 业务库 active/identity.db，决定日志 control/identity-journal.jsonl（control 区不随恢复替换）。0001 权威表按 types.json 公开 DTO 建列；sessions 的 refresh 族/令牌哈希列不在 0001，随会话工作包追加 0002（迁移只追加）。
- 平台 nw_outbox/nw_inbox 由 EnsureSchema 创建（outbox 工作包接入时），不进本域迁移。
- 换向后回执、InvalidateRestoredSessions、x-handler 绑定与后台循环仍 blocked，缺口见 progress.json；site-config 控制库（R1-02 下一包）就绪前运行权威读取等待依赖。

## 检查证据

- 本地 darwin/arm64 go1.27.1 与隔离 VPS（8核，go1.25.5，/root/build/nice-wiki-identity-persistence，tar 同步排除 .git/__pycache__/AppleDouble）：`go build ./...`、`go vet ./...`、`gofmt -l` 为空均通过；根 `make check` 均通过（829文件/894函数/372操作一致）。
- 未运行业务进程与业务验收（site-config 未实现，启动按合同 DEPENDENCY_UNAVAILABLE；无已授权测试）。

## 下一步（一项）

实现 `modules/identity/internal/account_validator.go` 的 `identity.AssertSuperadminImmutable`：先读 `docs/functions.md`、`types.json` 和 M02-R01 条款，保持纯函数边界，完成后仅更新本域台账/STATUS/交接并在具备 Go 工具链的环境执行既有检查；不得顺手实现账号状态接口或会话逻辑。
