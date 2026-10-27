# content 文件目录

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改。登记入口、契约、配置、部署和生成文件的位置，其余源码按需要建立，不逐个登记。

文件 57 个：手写 46、UI 1、生成 4、配置 6。

| 路径 | 类别 | 职责 |
|---|---|---|
| `modules/content/internal/document_service.go` | 手写 | 新建共享草稿文档 |
| `modules/content/internal/tree_service.go` | 手写 | 后台读取整站文档树 |
| `modules/content/internal/tree_validator.go` | 手写 | 检查树无环和层级约束 |
| `modules/content/internal/metadata_service.go` | 手写 | 修改标题/路径/SEO描述/分享图 |
| `modules/content/internal/path_service.go` | 手写 | 校验英文可读路径与同父唯一 |
| `modules/content/internal/edit_lock_service.go` | 手写 | 单leader内存租约、共享序号和同文档提交guard；改变租约统一经可靠操作事务 |
| `modules/content/internal/draft_service.go` | 手写 | 读取全篇共享草稿及编辑状态 |
| `modules/content/internal/block_validator.go` | 手写 | 验证全部Tiptap块及稳定ID |
| `modules/content/internal/table_validator.go` | 手写 | 验证表格合并单元格覆盖 |
| `modules/content/internal/columns_validator.go` | 手写 | 校验2–4栏比例和禁止嵌套 |
| `modules/content/internal/reference_extractor.go` | 手写 | 扫描所有嵌套块资源引用 |
| `modules/content/internal/body_canonicalizer.go` | 手写 | 规范序列化正文并计算hash |
| `modules/content/internal/revision_files.go` | 手写 | 持久写权威正文文件 |
| `modules/content/internal/revision_repository.go` | 手写 | 原子提交共享草稿head |
| `modules/content/internal/revision_reader.go` | 手写 | 读取COMMITTED正文并核真相 |
| `modules/content/internal/publication_metadata.go` | 手写 | 人工优先准备首次slug和SEO |
| `modules/content/internal/publication_service.go` | 手写 | 点击发布才公开精确共享草稿版本 |
| `modules/content/internal/publication_repository.go` | 手写 | 原子提交当前发布快照与署名历史 |
| `modules/content/internal/visibility_service.go` | 手写 | 同步核公开引用是否仍可见且当前 |
| `modules/content/internal/publication_reader.go` | 手写 | 获取仍公开的精确正文快照 |
| `modules/content/internal/trash_service.go` | 手写 | 删除进回收站，可选旧URL替代跳转 |
| `modules/content/internal/history_service.go` | 手写 | 查看署名发布/git版本历史 |
| `modules/content/internal/preview_service.go` | 手写 | 管理员预览草稿公开样式 |
| `modules/content/internal/reference_reader.go` | 手写 | 向资源库列出哪些文档引用文件 |
| `modules/content/internal/object_reader.go` | 手写 | 经对象API读取私有文件而不共享目录 |
| `modules/content/internal/outbox_repository.go` | 手写 | 与发布/撤回提交原子登记最小事件 |
| `modules/content/internal/public_route_reader.go` | 手写 | 公开导航读取已发布文档树 |
| `modules/content/cmd/content/main.go` | 手写 | 本域唯一Go可执行入口；不放产品算法 |
| `modules/content/internal/app/wire.go` | 手写 | 本域依赖装配和完整HTTP路由绑定；不共享存储 |
| `modules/content/internal/transport/server.go` | 手写 | HTTP/worker生命周期、优雅关闭、匿名健康与就绪 |
| `modules/content/go.mod` | 配置 | 独立module nice-wiki/modules/content；固定Go工具链和本域实际库依赖，不能import另一服务internal或用latest浮动依赖 |
| `modules/content/go.sum` | 生成 | go.mod依赖校验锁；由go mod download生成，纳入版本以可重现构建 |
| `modules/content/Dockerfile` | 配置 | 本域多阶段独立镜像：按go.mod工具链构建./cmd/content；如有UI根先pnpm frozen-lockfile构建dist；运行只带server/config/dist，以非root用户挂本域独占data卷，HTTP由反代提供HTTPS |
| `modules/content/Makefile` | 配置 | generate校验并生成合同绑定；build执行go build ./cmd/content与本域UI build；run传配置；verify核编译和合同引用。无共享业务启动器，不创建测试样例 |
| `modules/content/config/service.defaults.json` | 配置 | ServiceConfig默认值/严格字段及CLI>env>file>defaults覆盖；现有配置文件扩展基础启动字段而不重复维护；secret仅ref |
| `modules/content/deploy/manifest.json` | 配置 | 唯一服务slug/image/command/http端口/本域data卷/外部secret refs/所需dependency endpoints/healthz-readyz探针；不塞真实主机凭证 |
| `modules/content/internal/model_client.go` | 手写 | 经模型网关执行一次调用并读取到终态 |
| `modules/content/internal/gen/openapi.gen.go` | 生成 | make gen由oapi-codegen从本域contracts/openapi.json（含types.json与公共schema引用）生成DTO与严格服务端接口；不手改 |
| `modules/content/internal/gen/clients.gen.go` | 生成 | make gen由oapi-codegen按本域dependencies中的生产方operation生成调用客户端，自动附platform服务断言；不手改 |
| `modules/content/internal/gen/oapi-codegen.yaml` | 配置 | oapi-codegen配置：输出包gen、strict-server、只生成本域与所需生产方operation |
| `modules/content/web/src/gen/api.gen.ts` | 生成 | make gen由openapi-typescript从本域openapi.json生成浏览器类型，配合openapi-fetch经portal同源调用；不手改 |
| `modules/content/internal/storage.go` | 手写 | 正文私有SQLite、迁移、准备/提交资格/隔离与独占启动恢复适配 |
| `modules/content/migrations/migrations.go` | 手写 | 内嵌本域只追加迁移的唯一来源 |
| `modules/content/migrations/0001_domain.sql` | 手写 | 初始正文权威schema：绑定、准备、精确已提交索引、回执、head和隔离；不创建文档/锁/提交 |
| `modules/content/internal/document_edit_state.go` | 手写 | 持久文档存在/生命周期/删除fence及隔离读取 |
| `modules/content/migrations/0002_document_edit_state.sql` | 手写 | 追加documents身份/生命周期/删除fence表，不插入文档或持久租约 |
| `modules/content/internal/edit_lock_operations.go` | 手写 | 完整编辑锁用例、即时授权与安全姓名、可靠状态事件/本地审计/原回执及未知提交隔离 |
| `modules/content/migrations/0003_edit_lock_operations.sql` | 手写 | 只追加编辑原回执、跨boot状态事件修订和本地审计；不恢复权威租约 |
| `modules/content/internal/draft_commit_store.go` | 手写 | 真实延迟草稿SQL事务、创建保存意图和稳定ID/引用/完整回执原子提交 |
| `modules/content/migrations/0004_draft_commits.sql` | 手写 | 只追加站点树版本、元数据、草稿意图、历史IDledger与引用权威表 |
| `modules/content/migrations/0005_reference_snapshots.sql` | 手写 | 只追加全站来源扫描单调水位；触发器在原来源事务内推进，耗尽回滚；不持久化读快照 |
| `modules/content/migrations/0006_tree_operations.sql` | 手写 | 只追加树操作完整scope/动作/摘要与原结果回执；不另建影子树或虚假manifest |
| `modules/content/migrations/0007_metadata_operations.sql` | 手写 | 只追加完整同代metadata原回执：scope/caller/actor/request hash/result/hash，同真实CAS/outbox事务，不建资源影子来源 |
| `modules/content/migrations/0008_publication_snapshots.sql` | 手写 | 只追加发布快照、署名历史、已发布引用和版本化重定向清单 |
| `modules/content/migrations/0009_model_suggestions.sql` | 手写 | 只追加首次发布模型建议的按哈希结果缓存 |
| `modules/content/migrations/0010_purged_reference_releases.sql` | 手写 | 记录已向资源服务发出空引用集合释放事件的彻底删除文档；引用列表只跳过这些文档，未释放的旧文档由事件转发循环补发 |
| `modules/content/web/src/index.tsx` | UI | 文档管理：文档树、Tiptap 编辑器（稳定块 ID、斜杠菜单、画板、公式、上传）、编辑锁与自动保存、发布/撤回/历史/回收站 |
