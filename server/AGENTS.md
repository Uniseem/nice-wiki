# Go 端约定

遵守根目录 AGENTS.md。包分层见 `../docs/structure/server.md`：基础工具 → 基础设施 → 业务 → api/site/webui → app。同层业务协作由使用方定义接口，在 app 注入；只允许契约列出的跨业务直接导入。store 仅被业务层及以上导入，jobs 通过注入获得数据库能力。

使用标准库 net/http、log/slog。返回错误携带操作上下文，不包含密钥或请求正文；关闭、事务回滚和锁释放错误不能静默忽略。用 context 取消长操作，配置路径统一由 datadir.Layout 定义，文件更新用 fsutil 原子写入。

先追加迁移、再写 queries、运行 make gen；不得修改已执行迁移或手改 sqlc、oapi-codegen 输出。路由在 OpenAPI 声明权限，由 api 统一验证。生成目标为标准库 HTTP。

普通检查使用 gofmt、go vet、golangci-lint 与已有测试；新测试必须确认，不调用真实模型。代码注释使用中文。
