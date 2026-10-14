# 独立服务构建、启动与部署蓝图

15域均为独立Go module、独立可执行入口、镜像及私有data目录。本文件补启动装配缺口，不改变old产品行为、不放领域算法；本轮不创建未来业务源码。机器合同：[service-bootstrap.json](../contracts/service-bootstrap.json)。

普通匿名HTTP `GET /healthz` 只检查进程；`GET /readyz` 检查本域配置/schema/journal/control区、服务身份信任锚与epoch。维护中已换入目标数据可control-ready，但publicRequestsAllowed=false，不能提前开放业务；全部域登记切换回执后由runtime解除维护。无AI配置不阻塞初始化、文档管理和公开阅读。

## 每域独立文件

| 域 | Go入口 | 装配 | HTTP生命周期 | 实际UI根 | 配置 |
|---|---|---|---|---|---|
| portal | `modules/portal/cmd/portal/main.go` | `modules/portal/internal/app/wire.go` | `modules/portal/internal/transport/server.go` | `modules/portal/web` | `modules/portal/config/service.defaults.json` |
| identity | `modules/identity/cmd/identity/main.go` | `modules/identity/internal/app/wire.go` | `modules/identity/internal/transport/server.go` | 无独立UI合同，使用既有portal界面 | `modules/identity/config/service.defaults.json` |
| site-config | `modules/site-config/cmd/site-config/main.go` | `modules/site-config/internal/app/wire.go` | `modules/site-config/internal/transport/server.go` | 无独立UI合同，使用既有portal界面 | `modules/site-config/config/service.defaults.json` |
| content | `modules/content/cmd/content/main.go` | `modules/content/internal/app/wire.go` | `modules/content/internal/transport/server.go` | `modules/content/web` | `modules/content/config/service.defaults.json` |
| media | `modules/media/cmd/media/main.go` | `modules/media/internal/app/wire.go` | `modules/media/internal/transport/server.go` | 无独立UI合同，使用既有portal界面 | `modules/media/config/service.defaults.json` |
| public-site | `modules/public-site/cmd/public-site/main.go` | `modules/public-site/internal/app/wire.go` | `modules/public-site/internal/transport/server.go` | `modules/public-site/web` | `modules/public-site/config/defaults.json` |
| model-gateway | `modules/model-gateway/cmd/model-gateway/main.go` | `modules/model-gateway/internal/app/wire.go` | `modules/model-gateway/internal/transport/server.go` | `modules/model-gateway/ui` | `modules/model-gateway/deploy/service.config.json` |
| knowledge | `modules/knowledge/cmd/knowledge/main.go` | `modules/knowledge/internal/app/wire.go` | `modules/knowledge/internal/transport/server.go` | `modules/knowledge/ui` | `modules/knowledge/deploy/service.config.json` |
| qa | `modules/qa/cmd/qa/main.go` | `modules/qa/internal/app/wire.go` | `modules/qa/internal/transport/server.go` | `modules/qa/ui` | `modules/qa/deploy/service.config.json` |
| localization | `modules/localization/cmd/localization/main.go` | `modules/localization/internal/app/wire.go` | `modules/localization/internal/transport/server.go` | `modules/localization/ui` | `modules/localization/deploy/service.config.json` |
| memory | `modules/memory/cmd/memory/main.go` | `modules/memory/internal/app/wire.go` | `modules/memory/internal/transport/server.go` | `modules/memory/ui` | `modules/memory/deploy/service.config.json` |
| traffic-guard | `modules/traffic-guard/cmd/traffic-guard/main.go` | `modules/traffic-guard/internal/app/wire.go` | `modules/traffic-guard/internal/transport/server.go` | `modules/traffic-guard/web` | `modules/traffic-guard/config/defaults.json` |
| jobs | `modules/jobs/cmd/jobs/main.go` | `modules/jobs/internal/app/wire.go` | `modules/jobs/internal/transport/server.go` | `modules/jobs/web` | `modules/jobs/config/defaults.json` |
| analytics | `modules/analytics/cmd/analytics/main.go` | `modules/analytics/internal/app/wire.go` | `modules/analytics/internal/transport/server.go` | `modules/analytics/web` | `modules/analytics/config/defaults.json` |
| data-protection | `modules/data-protection/cmd/data-protection/main.go` | `modules/data-protection/internal/app/wire.go` | `modules/data-protection/internal/transport/server.go` | `modules/data-protection/web` | `modules/data-protection/config/defaults.json` |

每域同时登记 `go.mod`、`go.sum`、`Dockerfile`、`Makefile`、`deploy/manifest.json`。已有web/ui根登记package.json/pnpm-lock.yaml/vite.config.ts/tsconfig.json/index.html/src/main.ts；沿用原根，不复制业务。go.mod/package.json是各域精确版本唯一声明，依赖锁配合Docker可重现构建；TypeScript保留old D18的6.0.x。Docker仅本域构建产物，非root运行且本域独占data卷，HTTPS交可信反代。

## 六个Go启动函数与UI入口

以下精确签名在每域实例化为“模块slug.函数名”（如content.BuildApplication），均为约束级；独立UI根另有界面挂载入口。输入/输出schema和完整错误、副作用、授权、并发、幂等字段都在机器合同，合并后进入各域functions/files文档。

### main

```go
func main()
```

独立进程可执行入口

1. 读取--config/--data-dir/--listen及环境覆盖，严格解析ServiceConfig；配置错误安全stderr后exit2
2. signal.NotifyContext注册SIGINT/SIGTERM；调用BuildApplication，失败exit1且只输出errorCode
3. 调用Serve；正常关闭或signal后以30秒context调用Shutdown；安全退出0，无法安全关闭exit1且保留journal

错误：VALIDATION, INTERNAL。副作用：打开本域资源、监听HTTP；关闭按持久安全点。
并发：一个进程一个Application；OS信号处理只触发一次drain。幂等：进程重启先journal恢复；不重放已付费任务。

### BuildApplication

```go
func BuildApplication(ctx context.Context, config ServiceConfig) (*Application, error)
```

装配本域存储、客户端、函数路由与worker

1. 验证module/listen/data目录/secretRef；建立本域控制区和业务数据区；portal不虚构业务DB
2. 先读control区恢复日志与site-config runtime：phase为restore_switching且本域有该restoreId的staged目录、尚未换入时，把active改名为restore/<restoreId>/before、staged改名为active并fsync父目录后写control日志；phase为restore_rolling_back且已换入时反向换回；换目录后丢弃旧dataEpoch未转发outbox、租约与可重建缓存。随后打开active区DB运行schema迁移与文件journal恢复；control区不随恢复替换
3. 按implementation登记dependencies装配HTTP/JSON/SSE客户端；绑定每个OpenAPI x-handler至本域手写函数；健康路由直接注册，缺任何handler则启动失败
4. 读取runtime epoch/maintenance；维护期间只开放健康/就绪与控制管理接口、不启动worker；本次启动完成换目录或换回时调用site-config:registerActivation登记回执；注册本域主动claim worker，有登记生产/消费事件时启动本域outbox转发与事件订阅循环，并配置停止新副作用/安全点机制
5. UI仅加载本域dist静态资源；没有AI渠道仍装配基础文档/初始化能力，不探测模型；失败逆序关闭已打开资源

错误：VALIDATION, INTEGRITY_ERROR, DEPENDENCY_UNAVAILABLE。副作用：打开本域DB/文件资源、注册router和本域worker；失败资源回滚。
并发：装配单flight；ready前不接受业务；不共享DB或执行外域算法。幂等：启动重入按journal和epoch恢复，不新建paid operation。

### Serve

```go
func Serve(ctx context.Context, app *Application) error
```

运行独立HTTP/JSON/SSE服务

1. 仅HTTP启动net/http.Server，ReadHeaderTimeout=5s、MaxHeaderBytes=64KiB；写deadline由各路由决定，SSE不用统一短WriteTimeout
2. 启动本域worker循环，匿名Health/Ready优先可访问；普通路由执行权限/epoch/maintenance门禁，SSE响应X-Accel-Buffering:no
3. 运行中每5秒读取site-config runtime；phase进入restore_switching或restore_rolling_back且本域有该restoreId的staged或已换入目录时，停止新请求、在途操作到安全点后以退出码75退出，由进程守护重启并在BuildApplication换目录
4. ctx取消即停止新入口并调用Shutdown；ListenAndServe的http.ErrServerClosed为正常结束，其他错误保存安全失败状态

错误：INTERNAL。副作用：监听本域HTTP端口、服务请求和本域worker。
并发：只有一Serve调用；每请求取消独立，worker只本域。幂等：重复Serve返回CONFLICT语义，不开第二监听。

### Shutdown

```go
func Shutdown(ctx context.Context, app *Application) error
```

停止入口并使在途操作到可恢复安全点

1. CAS lifecycle ready→draining，拒新写/claim/paid步骤并关闭SSE接受新订阅
2. 取消本域worker的新步骤；等待已有DB事务、文件journal、模型可结算阶段安全确认；不能因超时假定供应方免费或删除paid refs
3. http.Server.Shutdown等在途响应；flush本域outbox/journal、close DB与文件句柄；正常lifecycle closed
4. deadline未安全返回DEADLINE_EXCEEDED并保留完整journal/未决费用引用，启动恢复收敛；不清掉活跃控制记录

错误：DEADLINE_EXCEEDED, INTERNAL。副作用：本域drain、资源关闭、durable恢复日志。
并发：sync.Once启动drain；其他调用等待同一次结果。幂等：重复Shutdown返回同一次关闭结果；已closed成功。

### Health

```go
func Health(ctx context.Context, app *Application) (HealthStatus, error)
```

GET /healthz普通HTTP存活检查

1. 仅读本进程lifecycle与HTTP循环是否存活；不查询远程依赖/模型/数据库
2. 存活且未closed返回200 HealthStatus；closed或server不可用返回503；不泄漏路径、端点或secret

错误：DEPENDENCY_UNAVAILABLE。副作用：无。
并发：无锁或原子读取lifecycle；不进行外部调用。幂等：GET只读重复安全。

### Ready

```go
func Ready(ctx context.Context, app *Application) (ServiceReadiness, error)
```

GET /readyz普通HTTP就绪检查

1. 确认配置/本域schema/journal/control区与服务身份信任锚完整并在lifecycle ready或维护控制ready；返回当前active数据代次与本域恢复回执状态
2. 已初始化时核activeDataEpoch等于runtime目标代次；关键运行权威不可达或旧epoch返回503；site-config本域读取自身权威
3. 维护中本域已换入目标数据可返回200 control-ready但publicRequestsAllowed=false，使全域回执检查不会和维护门禁互相等待；所有域完成后runtime解除维护才开放
4. 未初始化可仅带初始化能力ready，activeDataEpoch=null；无AI渠道不使基础内容能力失败，能力缺项显式呈现

错误：DEPENDENCY_UNAVAILABLE。副作用：无。
并发：有界只读快照；依赖请求总deadline1500ms，禁止模型探测。幂等：GET只读重复安全。

## 配置与内部依赖容器

ServiceConfig明确module/listen/dataDirectory/超时/日志/HTTP依赖端点/外部trust-anchor与服务身份ref/UI dist。严格未知字段拒绝，CLI > env > file > 默认。Application标x-internal，HTTP router/server/sql.DB/cancel句柄用标准库x-go-type，绝不序列化为HTTP业务DTO。BuildApplication必须绑定本域全部OpenAPI handler，不允许只启动一个空server。失败逆序关闭已打开资源。

## 已合入模块合同

本轮启动条目已合入每域implementation/types/openapi及functions/files/api文档；后续以模块契约为唯一实现来源。本蓝图说明共同规则，变更必须同步模块实例。业务源码和业务验收尚未开始。

## 平台库实现（[ADR0005](adr/0005-engineering-and-deployment.md)）

上述 main、BuildApplication、Serve、Shutdown 的通用部分由 libs/platform 的 RunService、LoadServiceConfig、ApplyRestoreSwap、ReadRuntime、WatchRuntime、BuildOperationContext、WriteProblem 实现；各服务函数只提供本域装配回调、后台任务清单与任务执行函数表。
