# AI 接入会话交接

## 集成复核修复（2026-10-15，main）

云端两轮切片合入后首次在具备 Go 工具链的环境复核，统一修复如下：

- `PutProvider`/`PutModel` 事件改经平台 `outbox.EnqueueEvent` 写 nw_outbox，删除自建 outbox 表（迁移与内联副本），平台转发循环才能投递（ADR0005）。
- 迁移改为 go:embed；`cmd` 补 `Application.Serve/Shutdown` 方法；运行状态适配解开 `state` 外层；补齐 go.sum。

- 验证：本地 darwin/arm64 go1.27.1 与隔离VPS go1.25.5：`GOFLAGS=-mod=readonly go build ./...`、`go vet ./...` 通过，`gofmt -l` 为空；VPS联网删除go.sum重新`go mod tidy`（sum.golang.org校验）与提交内容一致；根 `make docs` 无漂移、`make gen` 可重复、`make check` 通过。未新增测试、样例或真实接口，未运行业务进程、联调或部署。
- 台账：progress.json 中原先登记、但仓库里不存在的提交 SHA（变基前旧 SHA）已逐行改为 main 上修改该文件的实际提交；本次新增的迁移包、迁移文件与函数已回填修复提交 `978e0cee659dc342650193c628710ff75115add5` 并按上述检查证据转为 `verified`。
- 遗留：迁移文件 `001_initial.sql` 在未部署前删除了 outbox 表定义；一旦发布即只追加。

## 本次事实

2026-10-15 完成本域最小实施切片，提交 `4eec945320f8cf20bfc6673a8c3c8490385aca2f`。新增 `internal/runtime/bootstrap.go`、`internal/catalog/provider.go`、`internal/catalog/model.go`、`migrations/001_initial.sql`，并为本域 `go.mod` 登记 `modernc.org/sqlite v1.59.0`。

- `LoadRuntime`：校验绝对目录和恰好32字节主密钥引用，建立本域SQLite/WAL、严格schema迁移和运行代次水位；不调用模型。
- `ValidateProvider`：校验HTTPS根URL、协议白名单、超时上限、附加头与Authorization覆盖，拒绝userinfo/fragment/query；不做网络探测。
- `ValidateModel`：校验能力集合、嵌入维度和chat/translation输出上限，计算稳定embedding space hash；不收费。
- `PutProvider`/`PutModel`：完成superadmin前置、dataEpoch水位、屏障、CAS、幂等回执、配置表和provider/model.changed outbox的同事务写入。

真实供应方调用、连接探测、密钥AES-GCM存储、档位/路由、预留/派发/结算/未知收费对账、完整HTTP handler与后台循环仍未实现。未新增测试/样例/部署，未调用真实接口或模型，未push。

## 基线与文件

本域合同摘要及每文件/函数状态在 [progress.json](../progress.json)。设计基线为 `cf841ca3fd6a8a0dcf45cecc3bf9a6b95c228148`；本次交付提交为 `4eec945320f8cf20bfc6673a8c3c8490385aca2f`，分支为 `main`。先用git status核对并行会话留下的未提交工作，不能把它们纳入本域提交。

独占范围是 modules/model-gateway/。根contracts/docs/scripts和其他模块由集成会话维护；禁止跨目录顺手修接口。现有功能、默认及用户操作以 [功能保持矩阵](../../../docs/product-preservation.md) 和本域requirements为准。

## 下一步（一项）

在VPS补跑Go/gofmt/迁移验证后，把本次Store接入 `BuildApplication`：由启动装配把site-config读取到的active dataEpoch写入本域水位，注册全量OpenAPI handler和activation回执；缺任一项继续返回 `DEPENDENCY_UNAVAILABLE`，不开放半成品业务入口。

完成条件：VPS格式/编译通过、Store迁移与代次门禁验证、全量handler绑定与启动失败语义符合合同；新测试、真实接口或模型仍需单独授权。不能将其他业务函数一起标记完成。

## 下一次结束填写

交付提交：`4eec945320f8cf20bfc6673a8c3c8490385aca2f`。实际完成函数：5个（LoadRuntime、ValidateProvider、PutProvider、ValidateModel、PutModel）；实际修改仅限 `modules/model-gateway/`。检查：`git diff --check`通过；因本机无Go/gofmt/make，未编译或运行迁移/行为验收。阻塞：BuildApplication全量接线、归一化规则合同缺口及工具链缺失。

详见根 [交接流程](../../../docs/playbooks/handoff.md) 与 [并行规则](../../../docs/playbooks/parallel-development.md)。
