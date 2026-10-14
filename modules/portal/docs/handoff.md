# 主系统会话交接

## 本次事实

2026-10-15 在分支 `codex/portal-route-gate`、基线 `dc803f1d92f3b9d528ec92e11e929f544bd6a9a1` 完成本域首片。独占范围始终为 `modules/portal/`，未改根文件、公共契约、生成物、其他模块，未新增测试或真实接口调用。

已实现文件：

- `internal/route_registry.go`：静态路由编译、路径穿越/双重编码拒绝、精确/最长匹配。
- `internal/request_validation.go`：请求体/头/媒体类型/operationId 预算与可信头清洗。
- `internal/origin_guard.go`：同源、常量时间 CSRF 比对和登记的 identity CSRF 客户端适配。
- `internal/auth_proof.go`、`internal/proxy_stream.go`：预授权、短期委托 proof、有界上游转发、背压流回传和失败映射。
- `internal/runtime_state_client.go`、`internal/maintenance_gate.go`：site-config runtime 读取、维护入口门禁和 data-protection 屏障状态。
- `internal/bootstrap_service.go`：权限能力与固定导航纯规则。
- `internal/app/wire.go`、`internal/transport/server.go`、`cmd/portal/main.go`：独立服务装配、健康/就绪、HTTP 生命周期和代理外壳。
- `go.mod`、`go.sum`、`config/service.defaults.json`、`deploy/manifest.json`：本域工程装配资料。

## 检查证据

- JSON 解析：portal contracts、routes、service defaults、manifest、progress 均通过。
- `git diff --check`：通过。
- 隐私/路径扫描：本次新增源码未写入密钥、正文、原始 IP 或私有地址凭据；配置仅使用示例/部署端点和外部 secret 引用。
- `python scripts/check_docs.py`：未通过，原因是并行会话新增文件及本次 portal 新文件尚未由集成会话登记根 `docs/structure/framework.json`；本域未越权修改根登记。
- Go/gofmt/make：本机未安装，未编译、格式化或运行测试。

## 未完成与下一项

`LoadBootstrap`、偏好、SSE、审计、完整 identity 授权上下文、traffic-guard admission、UI 和容器构建仍为 `planned`/依赖阻塞；当前 generic 本域业务入口只返回 `DEPENDENCY_UNAVAILABLE`，不伪造成功结果。

下一项：先由集成会话登记本次实际文件到根结构清单并在具备 Go/VPS 工具链后复核本地编译；之后再单独认领 `LoadBootstrap` 与 identity 授权客户端，完成条件是按 identity/site-config 合同接入并通过对应编译/静态检查。
