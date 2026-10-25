# agent 函数合同

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改；先改合同再重新生成。类型字段见 [types.json](../contracts/types.json)，公共类型见 [common.schema.json](../../../contracts/common.schema.json)。

共 8 个函数：约束 8 个，参考 0 个。约束函数（HTTP 处理、事件转发/订阅/处理、任务执行与启动入口）按合同的名称、签名和行为实现；参考函数是建议拆分，实现时可增删改名，模块交付时同步合同。

实现尺度见 [ADR0006](../../../docs/adr/0006-implementation-scope.md)：各函数输入输出的通用约束句、错误表中的通用错误码与步骤末尾的通用句（schema、截止与取消、dataEpoch 与屏障）是调用链整体保证，由入口与 libs/platform 一次满足，不在每个函数重复实现；只实现函数特有的步骤与授权规则。默认不写测试。

## agent.main

- 级别：约束
- 文件：`modules/agent/cmd/agent/main.go`
- 作用：启动服务

```go
func main()
```

**输入**

无

**输出**

无

**步骤**

1. 启动服务
2. 服务身份、原操作归属及运行代次在入口校验；正文与模型密钥不写日志。

**错误**

- 无

- 副作用：进程内独立会话状态
- 授权：仅已签名的 qa 服务及原站点、代次、调用服务、根操作归属。
- 并发：原操作截止和取消贯穿运行；不同会话独立。
- 幂等：读取可重放；原请求编号结果仅接受一次；运行失败不自动重派模型调用。
- 依赖：`agent.BuildApplication`

## agent.BuildApplication

- 级别：约束
- 文件：`modules/agent/internal/app/application.go`
- 作用：装配服务身份、版本化 API 与共享 Node 运行时

```go
func BuildApplication(ctx context.Context, config platform.ServiceConfig) (*Application, error)
```

**输入**

无

**输出**

无

**步骤**

1. 装配服务身份、版本化 API 与共享 Node 运行时
2. 服务身份、原操作归属及运行代次在入口校验；正文与模型密钥不写日志。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：进程内独立会话状态
- 授权：仅已签名的 qa 服务及原站点、代次、调用服务、根操作归属。
- 并发：原操作截止和取消贯穿运行；不同会话独立。
- 幂等：读取可重放；原请求编号结果仅接受一次；运行失败不自动重派模型调用。
- 依赖：无

## agent.Health

- 级别：约束
- 文件：`modules/agent/internal/app/application.go`
- 作用：进程存活

```go
func (a *Application) Health(w http.ResponseWriter, r *http.Request)
```

**输入**

无

**输出**

无

**步骤**

1. 进程存活
2. 服务身份、原操作归属及运行代次在入口校验；正文与模型密钥不写日志。

**错误**

- 无

- 副作用：进程内独立会话状态
- 授权：仅已签名的 qa 服务及原站点、代次、调用服务、根操作归属。
- 并发：原操作截止和取消贯穿运行；不同会话独立。
- 幂等：读取可重放；原请求编号结果仅接受一次；运行失败不自动重派模型调用。
- 依赖：无

## agent.Ready

- 级别：约束
- 文件：`modules/agent/internal/app/application.go`
- 作用：原生运行时就绪

```go
func (a *Application) Ready(w http.ResponseWriter, r *http.Request)
```

**输入**

无

**输出**

无

**步骤**

1. 原生运行时就绪
2. 服务身份、原操作归属及运行代次在入口校验；正文与模型密钥不写日志。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：进程内独立会话状态
- 授权：仅已签名的 qa 服务及原站点、代次、调用服务、根操作归属。
- 并发：原操作截止和取消贯穿运行；不同会话独立。
- 幂等：读取可重放；原请求编号结果仅接受一次；运行失败不自动重派模型调用。
- 依赖：无

## agent.StartRun

- 级别：约束
- 文件：`modules/agent/internal/app/application.go`
- 作用：启动独立原生会话

```go
func (a *Application) StartRun(w http.ResponseWriter, r *http.Request)
```

**输入**

无

**输出**

无

**步骤**

1. 启动独立原生会话
2. 服务身份、原操作归属及运行代次在入口校验；正文与模型密钥不写日志。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：进程内独立会话状态
- 授权：仅已签名的 qa 服务及原站点、代次、调用服务、根操作归属。
- 并发：原操作截止和取消贯穿运行；不同会话独立。
- 幂等：读取可重放；原请求编号结果仅接受一次；运行失败不自动重派模型调用。
- 依赖：无

## agent.ReadEvents

- 级别：约束
- 文件：`modules/agent/internal/app/application.go`
- 作用：读取模型及工具请求

```go
func (a *Application) ReadEvents(w http.ResponseWriter, r *http.Request)
```

**输入**

无

**输出**

无

**步骤**

1. 读取模型及工具请求
2. 服务身份、原操作归属及运行代次在入口校验；正文与模型密钥不写日志。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：进程内独立会话状态
- 授权：仅已签名的 qa 服务及原站点、代次、调用服务、根操作归属。
- 并发：原操作截止和取消贯穿运行；不同会话独立。
- 幂等：读取可重放；原请求编号结果仅接受一次；运行失败不自动重派模型调用。
- 依赖：无

## agent.Reply

- 级别：约束
- 文件：`modules/agent/internal/app/application.go`
- 作用：提交模型或工具结果

```go
func (a *Application) Reply(w http.ResponseWriter, r *http.Request)
```

**输入**

无

**输出**

无

**步骤**

1. 提交模型或工具结果
2. 服务身份、原操作归属及运行代次在入口校验；正文与模型密钥不写日志。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：进程内独立会话状态
- 授权：仅已签名的 qa 服务及原站点、代次、调用服务、根操作归属。
- 并发：原操作截止和取消贯穿运行；不同会话独立。
- 幂等：读取可重放；原请求编号结果仅接受一次；运行失败不自动重派模型调用。
- 依赖：无

## agent.Cancel

- 级别：约束
- 文件：`modules/agent/internal/app/application.go`
- 作用：取消并释放会话

```go
func (a *Application) Cancel(w http.ResponseWriter, r *http.Request)
```

**输入**

无

**输出**

无

**步骤**

1. 取消并释放会话
2. 服务身份、原操作归属及运行代次在入口校验；正文与模型密钥不写日志。

**错误**

- `VALIDATION` / HTTP 400：（不可重试）
- `UNAUTHENTICATED` / HTTP 401：（不可重试）
- `FORBIDDEN` / HTTP 403：（不可重试）
- `NOT_FOUND` / HTTP 404：（不可重试）
- `CONFLICT` / HTTP 409：（不可重试）
- `LOCKED` / HTTP 409：（不可重试）
- `CANCELLED` / HTTP 409：（不可重试）
- `VERSION_MISMATCH` / HTTP 409：（不可重试）
- `INTEGRITY_ERROR` / HTTP 422：（不可重试）
- `DEPENDENCY_UNAVAILABLE` / HTTP 503：（不可重试）
- `DEADLINE_EXCEEDED` / HTTP 504：（不可重试）
- `INTERNAL` / HTTP 500：（不可重试）

- 副作用：进程内独立会话状态
- 授权：仅已签名的 qa 服务及原站点、代次、调用服务、根操作归属。
- 并发：原操作截止和取消贯穿运行；不同会话独立。
- 幂等：读取可重放；原请求编号结果仅接受一次；运行失败不自动重派模型调用。
- 依赖：无
