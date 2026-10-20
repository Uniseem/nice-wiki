# 运行、恢复与异常处理

来源 old docs/design/home.md/site.md/admin.md/i18n.md/security-ops.md，D17/D34/D55。themeColor/logo/favicon/defaultShareImage/footer均配置完整。明文settings不含凭据，恢复/镜像对象经hash和epoch验证。窗口公开失联保守关闭。

服务启动顺序：验证配置/schema → 打开本服务私有存储 → 恢复未完成决定日志 → 读取 site-config runtimeState → 校验活动 epoch → 接受允许的读取 → 仅在无维护/屏障时接受业务写入。

超时的写请求必须使用同 operationId 获取原结果。依赖失败不可假定权限、可见性或引用为空。日志记录 requestId、operationId、状态与安全字段，不记录正文、密码、token、原始文件路径或用户 IP。备份导出经只读 ObjectRef API；导出内容与 staging 根由本服务拥有。

快照协议各阶段在 API/函数中登记；portal 仅 gate 屏障。stage/discard 均需 data-protection 服务证明、superadmin 委托及维护状态；只要有域未登记切换回执，site-config 不能退出维护。


初始化公开地址由受信进程配置 `publicBaseURL` 或 `NW_PUBLIC_BASE_URL` 注入，不能从请求 Host 推导。公开地址要求 https；只有显式 `allowLocalHTTP`/`NW_ALLOW_LOCAL_HTTP` 且主机为 loopback 才允许 http 本地开发。未配置时已有读取与健康端点仍可装配，新的初始化返回依赖配置缺失。

配置恢复在入口开放前执行 RecoverConfigJournal，然后对非维护状态下尚未完成的初始化 commit 执行 ResumeInitialization。COMMITTED 决定成功而 SQL 失败，使用同一 operation 补完；不能删除 journal、重新生成 preparedAt 或另造请求以绕过冲突。旧准备决定缺原始 actor 回执、文件哈希损坏、未知 schema 或 journal 水位断裂均失败关闭，不猜测主体、不从准备 JSON 重造已提交权威文件。

恢复 Stage 通过登记的 data-protection 服务依赖读取本域归档；配置与部署清单只保存私有服务地址引用，不保存凭据或真实主机信息。协调与全域封口接口在共享合同冻结前保持失败关闭。

Build 在 active 迁移后登记 `site_config_state`、`config_revision_index`、`announcement_heads`、初始化决定、配置操作和首页引用决定表。维护态启动只提供健康、就绪与控制接口，不启动首页恢复和 outbox worker；Ready 使用同一 SnapshotManager 的内存屏障状态投影 `safePoint` 与 `publicRequestsAllowed`，没有状态记录时按无本域屏障处理，未知状态失败关闭。

公告恢复逐条核对 COMMITTED 的 head、正文、大小、hash 和 schema。缺失或不匹配的单条公告写入本域 quarantine 并计入隔离结果，其他有效公告继续恢复；结构损坏、迁移损坏和控制水位矛盾仍阻止启动。阶段迁移只接收公共认证的隔离数据库，回调内完成本域 schema 与派生公告索引校验，不能直接操作 active 目录。
