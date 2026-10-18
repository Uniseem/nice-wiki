备份入口源码 525e43bdd8a80f323db9bf204f2ea09917cf1f57：ValidateBackupRequest/CreateBackup与3实际助手就绪，累计18既有业务/界面+53助手待统一GitLab CI。control V3持久原批准、不可变任务输入、requested/outbox和真实jobs回执，提交失败保留意图而不伪成功。完整备份执行/归档/恢复链与启动仍未齐。

UI源码 351f17e936549171d39759a3b28c3912dc6da085：3既定面板与mount入口及13实际助手就绪，累计16既有业务/界面+50助手待统一GitLab CI。后端备份/恢复/镜像大链尚未齐，503真实显示；不声称系统可服务。固定token header/stream依赖root770b93c及portal；卸载nice-wiki:unmount取消轮询。lock只CI生成；大型下载需浏览器File System Access，未做行为验收。

远程配置源码 4fc21593670b7171e56d40b874324441c72bdb75：ValidateRemote/ConfigureRemote与HTTP助手就绪，累计12既有业务+37助手待统一GitLab CI。白名单及凭据ID仅可信部署注入，CAS/原回执同事务，无网络或真实Git操作。依赖root新GitCredentialRefs字段，未本地编译生成。

V2源码 5b60328a703a35a6a4dff73965c24c2123364acc；精确新增/改动函数见progress.readyAwaitingCI最新batch。新增3助手，业务ready总数仍10；全部改动待统一GitLab CI。

V2独立vault小批：control目录/对象/journal不随active替换；未知旧对象、旧记录或vault文件维护拒绝不删除。完整索引及完成journal同事务。新增3实际助手，累计10既有业务+36助手待统一GitLab CI，无本地编译/生成/真实操作。

ManifestV1小批源码1ce78c9cc5c9327a1eb5888e1cdf8842840be048：BuildManifest/ValidateCrossDomainRefs及4实际参考助手就绪，累计10既有业务+33助手待统一GitLab CI。轻量top绑定14域同一backup/barrier/sourceEpoch、真实原manifest指纹与完整来源ref；实际认证DomainManifests另作证据，只有自身owner对象及content发布修订可声明权威。对象/发布完整身份不同hash拒绝；缺目标报告valid=false，封存入口必须拒绝；全局索引和top各2MiB，超限不截断。ReadBackup严格V1/canonical投影，不能透传私有header。需root common833与gen，未本地生成或编译/测试/真实调用。

下一项按root已批准非递归布局：cipher=dataRoot/vault、backup_runs/backup_objects/stored_objects等controller事实=control，active只安全设置/镜像；旧active/objects非空或旧表未知记录维护拒绝，追加迁移不静默删。自身快照只历史审计投影，跨实例旧包无实际vault时真实failed，不把旧包递归嵌进新包或伪装活跃引用。root59提供RuntimeView真实revision/AllowedGitHosts/二进制响应，公共控制lane及snapshot工具仍待接装配。

安全设置读取源码ea262f42c3d3a15b6656f3660d8d5c78f8549ce8：ReadProtectionSettings与实际HTTP helper就绪，累计8个既有业务函数+29助手待统一GitLab CI。schedule/remote兼容可选，只有真实已保存配置才返回，安全DTO不含actor/credentialRef，不拿当前读者假造批准事实。根已批准后续轻量BackupManifestV1和可信AllowedGitHosts配置字段，下一步先写真实Manifest/ref验证，不从请求或内部依赖推断远程allowlist。无本地编译/gen/测试/真实业务调用，工作树此记录提交后应干净。

# R1第二批：私有分片AEAD与journal（2026-10-18）

源码6a115d0e1c710454ed07b746c6fd71e9430d5902：EncryptBackupObject、DecryptForValidation、RecoverProtectionJournal及15个实际参考助手交付代码，累计7个既有业务函数+28助手；本批全部待统一GitLab CI，accepted=0。Root通知首批pipeline75 Go/普通test/前端已过，4条name修正同步；不把旧证据套本批改动。

精确格式在crypto-format.md及CryptoMetaV1/CryptoHeaderV1。32-byte可信KEK只包装独立随机DEK；1MiB有界AEAD数据帧和强制终帧，完整EOF/源hash/密文hash后才激活隔离明文。原输入摘要/回执、control FULL同步journal、文件及目录fsync到dataRoot、原子安装和active索引有真实顺序；未知key不接任意路径，未知journal/认证文件缺失不会假称controlReady。未完成真实切换决定只保持维护，不推断外域已safe/已switched。ready备份缺域或密文时真实failed+安全outbox，同步revision；不清除旧完整对象。

完整启动、公共快照传输/屏障、全备份/恢复/镜像/UI仍未齐；未真实执行备份恢复/接口/服务/部署或push，未本地Go/gofmt/gen/测试/完整check，只做Git差异及源合同静态审阅。普通test由CI执行且当前无测试文件，不等于验收。

## 下一项与公共接装配

根已明确common DomainSnapshotManifest（8332b6c）：未压缩tar，首manifest.json canonical≤16MiB，其余files/<path>；真实files/objects/publications/references回调枚举。下一项按该公共格式实现ManifestV1/跨域引用与便携ZIP校验，缺真实引用报告失败，不填空数组冒未知。

root会提供公共serveSnapshotArchive、屏障inflight/安全点/归档导入工具及RuntimeView可信revision；不在本域复制。安全截点需排除当前controller产生的archive/运行记录，冻结历史ready catalog；当前crypto vault在active/objects，own CreateSnapshot回调不得把当前operation新对象混进已固定目录水位。若公共safe门禁严格禁止全部active写，需按根给controller窗口或vault布局统一接装配，不能自行放松。Root索引框架须登记新增docs/crypto-format.md。

工作区提交后干净。所有本批精确ready/改动函数及实际新增助手见progress；root合入时保留pipeline75未改项证据。本域未自行push，无临时业务验证程序、fixture、mock、样例或新增测试。

---

# 2026-10-18 继续全部剩余代码：首批检查通过

用户新授权继续三个子代理，目标完成全部剩余代码，及时GitLab检查后普通双推；此前75%停止仅为历史。GitLab pipeline76（8332b6c；生成341，三分片342/343/344，前端345）全部成功。16个Go模块readonly build/vet/gofmt、普通go test与完整make check、Vue类型检查/构建通过；159373字节生成物已精确回传至0881c49c085bbbfc643e43998e9fd673608ef534。普通go test无测试文件；不代表业务验收。

本域本批17项完整源码按精确交付清单verified，partial/完整启动保持原阻塞，accepted=0。未本机/SSH编译、未新增测试/样例/临时业务程序、未真实接口模型或部署。后续代码继续独占worktree实施，不因本次交付自动停止。

以下为历史记录。

# R1收尾：保护目录首批（2026-10-18）

源码88cef9718165e87d431e36b327f23cb207792388，基线770ea1c。4个既有业务函数（ListBackups、ReadBackup、ReadMirrorStatus、VerifyObjectHash）和13个实际参考支持已写，均blocked待统一GitLab CI。active/control独立SQLite迁移及目录查询已接真实受信HTTP；游标签名复用platform，目录不存在真实NOT_FOUND，不伪造备份成功。原启动入口仍因journal/备份恢复/快照协议未齐blocked，accepted=0。

只改本域，未push；未本地或SSH编译/格式/gen/测试/完整check、无真实接口/备份恢复/部署。git diff --check通过。复用仓库已批准modernc.org/sqlite v1.59.0，go.sum由统一GitLab命令生成，未手写。

## 公共缺口及下一项

根已接统一serveSnapshotArchive/平台屏障归档协议请求；SnapshotExportResult现无归档流获取能力，不能把ObjectRef猜成路径/URL。根批准32-byte本地可信KEK与1MiB分片AEAD容器V1，下一项在crypto.go与本域合同写精确格式后实现流封存/完整验证，不复制外部crypto源码。不以缺失refs返回valid=true；ManifestV1与各域真实归档引用格式待根统一。

数据库：active/protection.db含目录/对象/镜像/配置与revision；control/protection-control.db含restore_runs/decisions/domain_stages/barrier/journal，永不随active恢复替换。每库user_version=1事务迁移，nw_inbox/outbox复用平台EnsureSchema。HTTP只实际开放已写目录函数，其余具体失败关闭；Ready仍明确未完成。query游标绑定稳定身份/session/权限修订/epoch/limit及目录revision，10分钟过期。

精确ready IDs见progress.json.readyAwaitingCI。Git工作区此记录提交后应干净；根集成后执行gen/docs/依赖锁定与三分片检查，未过CI不得提升verified。

---

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
