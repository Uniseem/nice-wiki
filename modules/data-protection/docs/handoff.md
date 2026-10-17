# 数据保护会话交接

## 本次事实

2026-10-17 完成本域启动与契约生成准备。分支 `codex/data-protection-20261017-r5`，基线 `a3e6af980d5288e42bc8d023410faf8923ff21e8`，源码提交 `d03bad4debb9ddb6ec830baa37794f4330396277`。独占范围是 `modules/data-protection/`。未 push。

已编写：

- `cmd/data-protection/main.go` 只装配 `platform.RunService`、本域 `BuildApplication` 和 site-config 运行权威客户端。
- `internal/app/wire.go` 建立 `active/control/restore`，加载信任锚和签名引用，调用 `ApplyRestoreSwap`。换目录后缺少 `stageId` 与 `manifestHash`，不调用 `registerActivation`，启动失败关闭。
- `internal/transport/server.go` 实现 HTTP `Serve`/`Shutdown`、匿名 `Health`/`Ready`，并绑定生成接口的 27 个路由。备份、恢复和镜像路由返回 `DEPENDENCY_UNAVAILABLE`，不返回空目录或空进度。
- `go.mod`、`Makefile`、`Dockerfile`、`config/defaults.json`、`deploy/manifest.json`。生成目录未手改。`make gen` 未运行。

`Health` 只读本进程生命周期。`Ready` 在 journal 未恢复、业务处理未实现或运行视图没有修订号时返回 503，不把 `publicRequestsAllowed` 报成 true。没有启动 `RunWorker`，没有打开私有库，没有事件循环，没有真实 Git 推送。

## 台账

`progress.json` 的 phase 为 `implementation`。配置和部署文件为 `implemented`。`go.mod` 与 `go.sum` 已由 pipeline 48 的 readonly 编译使用。`main`、`BuildApplication`、`Serve`、`Shutdown`、`Health`、`Ready` 以及对应 Go 文件仍为 `blocked`：包已编译，协议未齐，不记 verified。`migrations/0001_domain.sql` 仍为 `blocked`。其余业务文件和函数保持 `planned`。`accepted` 为 0。合同未改，`contractSha256` 仍是 `97a61469c575aea0e0c3c341b8c8a70e358c8f480782de68e15a237a7df20c1f`。

## 检查

- `make docs`：通过，合同未改，生成文档无漂移。
- 默认配置与 manifest 的 JSON 解析：通过。
- 生成 `ServerInterface` 的 27 个方法与手写路由方法一致。
- pipeline 48 已对本包做 readonly build、vet 和 gofmt。未运行真实接口、真实模型或业务验收。
- 新文件和 `go.sum` 已登记。下面的登记清单是当时交给集成会话的列表，现已完成。

## 需要集成会话登记

请把下列新文件登记进根 `docs/structure/framework.json`，不要由本域会话修改该文件：

| 路径 | 类别 | 职责 |
|---|---|---|
| `modules/data-protection/cmd/data-protection/main.go` | handwritten | 本域唯一 Go 可执行入口；不放产品算法 |
| `modules/data-protection/internal/app/wire.go` | handwritten | 本域依赖装配和完整 HTTP 路由绑定；不共享存储 |
| `modules/data-protection/internal/transport/server.go` | handwritten | HTTP/worker 生命周期、优雅关闭、匿名健康与就绪 |
| `modules/data-protection/go.mod` | config | 独立 module `nice-wiki/modules/data-protection`；固定 Go 工具链和本域实际库依赖 |
| `modules/data-protection/Dockerfile` | config | 本域多阶段独立镜像；当前没有界面产物，只带服务与默认配置 |
| `modules/data-protection/Makefile` | config | generate、build、run、verify；界面构建在 lockfile 出现前失败关闭 |
| `modules/data-protection/config/defaults.json` | config | 平台 `ServiceConfig` 默认值；secret 只保留外部引用 |
| `modules/data-protection/deploy/manifest.json` | config | 服务 slug、端口 8113、数据卷、secret 引用、依赖地址和探针 |

登记后请用 `go mod tidy` 生成 `modules/data-protection/go.sum`，再跑 GitLab CI 的 readonly build、vet 和 gofmt。通过前不要把上述 Go 函数改成 `verified`。

## 仍在等待

- `identity`、`jobs`、`content`、`site-config` 只有生成客户端，启动时不拿它们冒充快照成功。
- `media`、`public-site`、`model-gateway`、`knowledge`、`qa`、`localization`、`memory`、`traffic-guard`、`analytics` 还没有本域快照客户端。
- 本域 `types.json` 的 `ServiceConfig` 仍是 `listenAddress`/`dataDirectory`/`serviceIdentityRef`。运行入口使用平台字段 `listen`/`dataRoot`/`signingKeyRef`。本轮不改类型合同。
- 没有新增测试、fixture、mock 或样例。

## 下一步（一项）

pipeline 48 已编译启动包。六个启动函数仍 blocked。下一步不要实现备份业务，先补齐 journal、迁移执行器和激活回执字段。

详见根 [交接流程](../../../docs/playbooks/handoff.md) 与 [并行规则](../../../docs/playbooks/parallel-development.md)。
