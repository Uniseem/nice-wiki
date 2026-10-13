# site-config 未来文件目录

仅登记未来实现位置，未创建这些源码。启动蓝图已合入；完整函数行为见functions.md与implementation.json。

文件数=35；分类={"handwritten": 29, "config": 5, "generated": 1}。

| 未来路径 | 类别 | 职责 | 函数ID |
|---|---|---|---|
| `modules/site-config/internal/config_service.go` | handwritten | 读取后台站点配置 | `site-config.ReadSiteConfig`, `site-config.UpdateSiteConfig` |
| `modules/site-config/internal/config_validator.go` | handwritten | 校验配置所有字段和关联约束 | `site-config.ValidateSiteConfig`, `site-config.NormalizeBaseURL` |
| `modules/site-config/internal/language_service.go` | handwritten | 规范基准语言 tag | `site-config.ValidateLanguageTag` |
| `modules/site-config/internal/home_service.go` | handwritten | 配置首页介绍、卡片和精选文档 | `site-config.UpdateHome`, `site-config.ReadPublicHome` |
| `modules/site-config/internal/home_validator.go` | handwritten | 验证首页目标与引用 | `site-config.ValidateHomeLinks` |
| `modules/site-config/internal/runtime_state_service.go` | handwritten | 读取独立控制库权威运行状态 | `site-config.ReadRuntimeState` |
| `modules/site-config/internal/runtime_transition.go` | handwritten | 按CAS切换epoch与维护阶段 | `site-config.TransitionRuntimeState`, `site-config.ValidateRuntimeTransition`, `site-config.RegisterActivation` |
| `modules/site-config/internal/write_epoch.go` | handwritten | 所有业务写入执行epoch与屏障检查 | `site-config.CheckWriteEpoch` |
| `modules/site-config/internal/initialization_service.go` | handwritten | 无需AI完成首站点初始化 | `site-config.InitializeSite`, `site-config.ReadInitializationStatus`, `site-config.ResumeInitialization`, `site-config.AbortInitialization` |
| `modules/site-config/internal/initialization_defaults.go` | handwritten | 确定性生成首次基础配置 | `site-config.PrepareDefaultConfig` |
| `modules/site-config/internal/initialization_repository.go` | handwritten | 持久化不可反悔的初始化决定 | `site-config.PersistInitializationDecision` |
| `modules/site-config/internal/announcement_validator.go` | handwritten | 校验公告纯文本正文并规范序列化 | `site-config.ValidateAnnouncementBody` |
| `modules/site-config/internal/announcement_files.go` | handwritten | 持久写不可变公告正文文件 | `site-config.WriteAnnouncementRevision`, `site-config.CommitAnnouncementHead`, `site-config.RecoverAnnouncementJournal` |
| `modules/site-config/internal/operation_repository.go` | handwritten | 查询配置幂等回执 | `site-config.ResolveConfigOperation` |
| `modules/site-config/internal/outbox_repository.go` | handwritten | 将配置事件与业务提交原子登记 | `site-config.PublishConfigEvent` |
| `modules/site-config/internal/object_reader.go` | handwritten | 经协议读取本域正文或快照对象 | `site-config.ReadConfigObject` |
| `modules/site-config/internal/barrier_service.go` | handwritten | 取得可恢复的数据屏障 | `site-config.AcquireBarrier`, `site-config.ReleaseBarrier` |
| `modules/site-config/internal/snapshot_service.go` | handwritten | 导出屏障水位的一致快照 | `site-config.ExportSnapshot`, `site-config.PrepareSnapshot` |
| `modules/site-config/internal/snapshot_validator.go` | handwritten | 验证快照可恢复性 | `site-config.ValidateSnapshot` |
| `modules/site-config/internal/snapshot_repository.go` | handwritten | 激活已准备恢复数据 | `site-config.CommitSnapshot`, `site-config.RollbackSnapshot` |
| `modules/site-config/internal/branding_service.go` | handwritten | 更新站名/logo/图标/主题色/默认分享图 | `site-config.UpdateBranding` |
| `modules/site-config/internal/footer_service.go` | handwritten | 更新页脚版权/自定义链接/ICP备案/源码链接 | `site-config.UpdateFooter` |
| `modules/site-config/internal/banner_service.go` | handwritten | 保存单条可关闭公告横幅 | `site-config.UpdateBanner` |
| `modules/site-config/internal/config_files.go` | handwritten | 写权威版本化站点设置文件 | `site-config.WriteConfigRevision`, `site-config.CommitConfigHead` |
| `modules/site-config/internal/config_history.go` | handwritten | 读取可回滚的站点设置历史 | `site-config.ReadConfigHistory`, `site-config.RestoreConfigRevision` |
| `modules/site-config/internal/resource_reference_reader.go` | handwritten | 向媒体返回站点设置权威资源引用快照 | `site-config.ListResourceReferences` |
| `modules/site-config/cmd/site-config/main.go` | handwritten | 本域唯一Go可执行入口；不放产品算法 | `M03-F901` |
| `modules/site-config/internal/app/wire.go` | handwritten | 本域依赖装配和完整HTTP路由绑定；不共享存储 | `M03-F902` |
| `modules/site-config/internal/transport/server.go` | handwritten | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 | `M03-F903`, `M03-F904`, `M03-F905`, `M03-F906` |
| `modules/site-config/go.mod` | config | 独立module nice-wiki/modules/site-config；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 |  |
| `modules/site-config/go.sum` | generated | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 |  |
| `modules/site-config/Dockerfile` | config | 本域多阶段独立镜像：按go.mod工具链构建./cmd/site-config；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS |  |
| `modules/site-config/Makefile` | config | generate校验并生成合同绑定；build执行go build ./cmd/site-config与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 |  |
| `modules/site-config/config/service.defaults.json` | config | ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref |  |
| `modules/site-config/deploy/manifest.json` | config | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 |  |
