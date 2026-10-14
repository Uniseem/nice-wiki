# 站点配置与初始化会话交接

## 本次事实（R1-02 启动装配组，2026-10-14）

- 分支：codex/site-config-r1-02（worktree `../nice-wiki-site-config`）；基线 ffe1ae4ccbb7ce33f8fbcec19bc888865fdedf24；实现提交 5ea6affabea00bba21a614cd58a3dc4b99fa8cd2；台账提交紧随其后。
- 独占范围：modules/site-config/。公共文件仅在实现提交中同步了 docs/structure/framework.json（登记9个新文件，identity 先例），请集成会话复核。
- 未提交文件：无。未新增测试、样例、真实接口或模型调用；未 push、未部署。

## 本次完成

- 创建9文件：cmd/site-config/main.go、internal/app/wire.go、internal/transport/server.go、go.mod、go.sum、config/service.defaults.json、deploy/manifest.json、Dockerfile、Makefile。
- site-config.main/Serve/Shutdown/Health/Ready 按合同实现并 verified：入口三步全由 lifecycle.RunService 承担（本域无 identity 那样的 accounts 运维子命令）；HTTP Server ReadHeaderTimeout=5s/MaxHeaderBytes=64KiB、匿名 healthz/readyz 直接注册、sync.Once 优雅关闭、重复 Serve 返回 CONFLICT；运行监视每5秒读自身权威，恢复切换退出码75。
- BuildApplication 的配置验证/三区目录布局/信任锚/签名密钥/自身权威读取接入/ApplyRestoreSwap 已交付；整体保持 blocked，缺口逐条见 progress.json（运行控制库读取、active 区 DB 打开与迁移、公告 journal 恢复、x-handler 绑定、换向后 RegisterActivation 回执、RecoverAnnouncementJournal/RelayOutbox 后台循环）。

## 与 identity 模式的差异（运行权威读取方式）

- identity 经 HTTP 客户端（gen.SiteConfigClient）读 site-config /v1/runtime-state；本域即权威，wire.go 的 NewRuntimeClient 返回 localRuntime（进程模板与 Ready 共用），从本域控制库读取，不发起自调用 HTTP，也不依赖自身依赖地址。
- 控制库（独立 control.db，data.md：RuntimeState 权威、不随业务快照恢复覆盖）随 runtime_state_service 工作包落地；落地前 localRuntime.ReadRuntimeState 如实返回 DEPENDENCY_UNAVAILABLE——启动装配失败交进程守护重试、readyz 返回 503，均为合同语义，不用占位 RuntimeView 冒充。
- 换向后回执同样需要 stageId/manifestHash/域签名（暂存/屏障函数，后续工作包）；在此之前 ApplyRestoreSwap 返回非 none 即拒绝开放入口，不伪造回执。

## 实现口径（供后续与集成对齐）

- 配置格式与 identity 相同：signingKeyRef 指向 JSON 文件 `{"keyId","privateKeyRef"}`（privateKeyRef 为绝对路径，PEM 由 serviceauth.LoadSigningKey 校验）；trustAnchorRef 指向 nw-keys 生成的公钥表。defaults 端口 8102；manifest 依赖按本域 implementation.json dependencies 登记 identity(8101)/content(8103)/media(8104)/jobs(8111) 容器服务名端点，env 为 NW_DEP_IDENTITY/CONTENT/MEDIA/JOBS。
- 本域无独立 UI（蓝图：无独立UI合同），Dockerfile 运行层只带 server/config。
- 依赖等待：identity/content/media 的 sync 客户端与 jobs 事件转发（outbox→jobs）随业务工作包装配；当前不构造占位联调。

## 检查证据

- 本地 darwin/arm64 go1.27.1 与隔离 VPS（8核，go1.25.5，/root/build/nice-wiki-site-config，tar 同步排除 .git/__pycache__/AppleDouble）：`go build ./...`、`go vet ./...`、`gofmt -l` 为空，均通过；根 `make check` 本地与 VPS 均通过。
- 未运行业务验收、未启动真实进程联调（本域控制库未实现，启动会按合同依赖语义失败）。

## 下一步（一项）

实现本域私有持久化：按 [data.md](data.md) 打开独立 control.db（modernc.org/sqlite，ADR0005 已批准），落地 RuntimeState 权威读写（RuntimePhase 迁移矩阵、requiredModules 与 activationReceipts、CAS revision）并替换 localRuntime 的如实报错，使 BuildApplication 步骤2 的自身权威读取与 Ready 的 epoch/maintenance 判定真实生效；active 区业务 DB 与公告 journal 恢复随后续包。先读 ADR0005、本域 data.md/workflows.md 与 platform lifecycle.ReadRuntime 口径；完成条件：localRuntime 读取真实控制库、相应台账翻 implemented/verified、检查在本地与 VPS 通过并更新台账。

详见根 [交接流程](../../../docs/playbooks/handoff.md) 与 [并行规则](../../../docs/playbooks/parallel-development.md)。
