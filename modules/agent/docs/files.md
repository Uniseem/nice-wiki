# agent 文件目录

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改。登记入口、契约、配置、部署和生成文件的位置，其余源码按需要建立，不逐个登记。

文件 4 个：手写 2、生成 2。

| 路径 | 类别 | 职责 |
|---|---|---|
| `modules/agent/cmd/agent/main.go` | 手写 | 进程入口 |
| `modules/agent/internal/app/application.go` | 手写 | HTTP 与原生运行时装配 |
| `modules/agent/internal/gen/openapi.gen.go` | 生成 | 从合同生成接口 |
| `modules/agent/internal/gen/clients.gen.go` | 生成 | 从合同生成接口 |
