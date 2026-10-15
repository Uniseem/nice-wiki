# 统计与审计会话交接

## 集成复核修复（2026-10-15，main）

云端两轮切片合入后首次在具备 Go 工具链的环境复核，统一修复如下：

- site-config 运行状态适配改为解开 `{"state": RuntimeState}` 外层，此前字段全空会被平台按“运行状态不完整”拒绝启动。
- 补齐 go.mod 间接依赖与 go.sum，gofmt 整理 `internal/transport/server.go`。

- 后续统一（同日）：`NewRuntimeClient` 改为调用平台 `lifecycle.NewSiteConfigClient`，删除本域 runtimeAuthority 适配；BuildApplication 依赖登记平台请求校验中间件，HTTP 绑定时在 transport 装配。
- 验证：本地 darwin/arm64 go1.27.1 与隔离VPS go1.25.5：`GOFLAGS=-mod=readonly go build ./...`、`go vet ./...` 通过，`gofmt -l` 为空；VPS联网删除go.sum重新`go mod tidy`（sum.golang.org校验）与提交内容一致；根 `make docs` 无漂移、`make gen` 可重复、`make check` 通过。未新增测试、样例或真实接口，未运行业务进程、联调或部署。
- 台账：progress.json 中原先登记、但仓库里不存在的提交 SHA（变基前旧 SHA）已逐行改为 main 上修改该文件的实际提交；本模块本次无新增登记文件或函数。

## 本次事实

2026-10-15 本次工作包在分支 `codex/analytics-r1-storage-contract-audit`、基线
`3db911390e35a07007d70a1d2f5cc15fe4092667` 上复核了交接指定的
`migrations/0001_domain.sql` 与 `analytics.BuildApplication` 依赖。未实现业务代码；仅更新本域台账并记录阻塞。

2026-10-15 已完成本域 R1 启动与契约生成准备。基线为
`ac593c57861d2242f27d07562a2acae535d18c38`，独立分支为
`codex/analytics-r1-bootstrap`，代码提交为
`7d97e7e0e25b260d0d8edcd58802fe20613fc42a`，未 push。

本次只修改 `modules/analytics/`：

- `cmd/analytics/main.go`：只装配 `platform/lifecycle.RunService`、本域 `BuildApplication` 和运行权威客户端。
- `internal/app/wire.go`：建立本域 `active/control/restore` 目录，加载信任锚与签名引用，接入 site-config 运行权威和 `ApplyRestoreSwap`，构造 HTTP 应用。
- `internal/transport/server.go`：实现 `Serve`、`Shutdown`、`Health`、`Ready`，并完整绑定生成 server interface 的 17 个路由方法。
- `go.mod`、`go.sum`、`Makefile`、`config/defaults.json`、`deploy/manifest.json`、`Dockerfile`：固定依赖、非敏感默认值和独立部署入口。

业务路由没有伪造成功：匿名 beacon、审计、访问/费用汇总、导出、屏障和快照方法当前统一返回
`DEPENDENCY_UNAVAILABLE`。生成目录中的 `openapi.gen.go`、`clients.gen.go`、`oapi-codegen.yaml` 和浏览器类型未手改。

## 台账状态

`progress.json` 已切换为 `implementation`，记录本次分支、基线和提交。入口、生命周期、健康/就绪、配置与部署文件为 `implemented`；`BuildApplication` 文件与函数为 `blocked`，因为本域 SQLite/journal、恢复激活回执、后台任务/事件循环和真实 x-handler 业务函数尚未实现。其余业务文件/函数保持 `planned`。实现、验证、行为验收和部署状态严格区分。

## 检查

- `python scripts/check_docs.py --privacy-only`：通过。
- 配置与 manifest JSON 解析：通过。
- 生成 server interface 的 17 个方法静态覆盖核对：通过。
- `git diff --check`：通过。
- `make gen`：未执行成功，当前环境没有 `make`；因此未触碰生成目录。
- Go、gofmt、make 均不在当前环境，未运行编译、vet、格式化、业务测试、真实接口、真实模型或部署验收。

根 `framework.json`、根 STATUS/plan、公共合同和其他模块属于集成会话所有权，本次未修改；集成时需登记新增 9 个本域文件并在可用 Go/VPS 环境复跑阶段检查。

## 本次合同阻塞

交接要求的“SQLite/迁移/journal”不是当前机器合同中的可实施切片：

- `implementation.json` 只有迁移文件的概括性 purpose，没有登记迁移执行器、SQLite 打开/配置、control journal 恢复、fsync/完整性收敛的文件、函数、精确签名、错误、副作用、并发和调用边界。
- `analytics.BuildApplication` 只将“本域journal恢复函数”列为 resource，未给出可实现的函数合同。
- `types.json` 只有供 `Application` 使用的 `*sql.DB` 和状态布尔字段，没有上述内部存储/journal接口或恢复记录类型。

因此本次没有创建 `migrations/0001_domain.sql`，没有新增 `internal/storage.go` 或 `internal/journal.go`，也没有接线 `BuildApplication`；否则会把未定义行为藏入实现，违反本域 AGENTS.md 的“缺约束先修契约”要求。需要公共/本域合同所有者先登记该切片后，才能继续编码。

## 阻塞与下一项

阻塞：`Ready` 会在本域 journal 未恢复时返回 `DEPENDENCY_UNAVAILABLE`；`BuildApplication` 在恢复发生换目录时也拒绝开放入口，避免把未实现的存储或回执当成成功。本次提交只有台账记录，没有业务验收结论。

下一项：由合同所有者先登记 SQLite/journal 切片（目标文件、约束级函数/签名、迁移版本、journal状态机、错误与恢复边界）；登记完成后再按 `data.md`、ADR0003 和该合同实现 `migrations/0001_domain.sql`、私有存储/journal，并回到 `analytics.BuildApplication` 接入。完成条件是失败可恢复、当前 dataEpoch 校验、所有 OpenAPI x-handler 不再依赖失败关闭，并在 Go/VPS 环境通过允许检查。
