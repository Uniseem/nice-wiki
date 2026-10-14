# 身份权限会话交接

## 本次事实（R1-01 启动装配组，2026-10-14）

- 分支：codex/identity-r1-01（worktree `../nice-wiki-identity`）；基线 9c667f3580ee1656d161f3b08311333e08a8ebb5；实现提交 533a1239fa8c48e1a3471005556cdead06669c6f；台账提交紧随其后。
- 独占范围：modules/identity/。公共文件仅在交付提交中同步了 docs/structure/framework.json（登记9个新文件，R1-00先例）与 THIRD_PARTY_NOTICES.md（登记 kin-openapi v0.133.0，ADR0005 已批准、版本取 oapi-codegen v2.5.1 依赖图配对版本），请集成会话复核。
- 未提交文件：无。未新增测试、样例、真实接口或模型调用；未 push、未部署。

## 本次完成

- 创建9文件：cmd/identity/main.go、internal/app/wire.go、internal/transport/server.go、go.mod、go.sum、config/service.defaults.json、deploy/manifest.json、Dockerfile、Makefile。
- identity.Serve/Shutdown/Health/Ready 按合同实现并 verified：HTTP Server ReadHeaderTimeout=5s/MaxHeaderBytes=64KiB、匿名 healthz/readyz 直接注册、sync.Once 优雅关闭、重复 Serve 返回 CONFLICT；进程信号与退出码（0/1/2/75）由 lifecycle.RunService 承担。
- identity.main 步骤1-3、BuildApplication 的配置验证/目录布局/信任锚/签名密钥/运行权威读取/ApplyRestoreSwap 接入已交付；两者整体保持 blocked，缺口逐条见 progress.json（accounts 子命令、业务存储、x-handler 绑定、换向后 InvalidateRestoredSessions 与 registerActivation 回执、ObserveSuperadminResetFile/RelayOutbox 后台循环）。

## 实现口径（供后续与集成对齐）

- signingKeyRef 指向 JSON 文件 `{"keyId","privateKeyRef"}`（privateKeyRef 为绝对路径，PEM 由 serviceauth.LoadSigningKey 校验）；trustAnchorRef 指向 nw-keys 生成的公钥表。deploy 会话（R1-07）按此挂载 secret 文件，与本域 deploy/manifest.json 的 secrets 段一致。
- 换向后回执需要 stageId/manifestHash/域签名（来自暂存/屏障函数，后续工作包）；在此之前 ApplyRestoreSwap 返回非 none 即拒绝开放入口（DEPENDENCY_UNAVAILABLE，进程守护重试），不伪造回执。
- Ready 当前 initialized=false、activeDataEpoch=null、journalRecovered=true（无业务存储）；schema/journal/epoch 深度校验随存储工作包补齐。site-config 不可达时 Ready/启动返回 503/DEPENDENCY_UNAVAILABLE（合同语义，联调等待 R1-02）。
- framework.json 顶层 phase/scope 仍是 design 时期的措辞（R1-00 后已不准确），属集成会话职责，本会话未动。

## 检查证据

- 本地 darwin/arm64 go1.27.1 与隔离 VPS（8核，go1.25.5，/root/build/nice-wiki-r1-01，tar 同步排除 .git/__pycache__/AppleDouble）：`go build ./...`、`go vet ./...`、`gofmt -l` 为空，均通过；根 `make check` 本地与 VPS 均通过（826文件/890函数/372操作登记一致）。
- 未运行业务验收、未启动真实进程联调（site-config 未实现，启动会按合同依赖语义失败）。

## 下一步（一项）

实现 identity 私有持久化：按 [data.md](data.md) 设计打开 active 区 SQLite（modernc.org/sqlite，ADR0005 已批准），落地 WAL/foreign_keys/busy_timeout=3000ms、迁移机制与 journal 恢复记录，并让 BuildApplication 的存储打开步骤与 Ready 的 schema/journal/epoch 校验生效。先读 ADR0005、docs/data.md 与 platform outbox/inbox 的 EnsureSchema 口径；完成条件：存储步骤翻 implemented、Ready 校验有真实依据、检查在本地与 VPS 通过并更新台账。
