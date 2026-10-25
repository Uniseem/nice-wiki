# agent 未来文件目录

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改。仅登记未来实现位置，尚未创建这些源码；函数行为见 [functions.md](functions.md)。

文件 4 个：手写 2、生成 2。函数 8 个：约束 8，参考 0。入口、契约、配置和部署文件按清单建立；internal/ 与界面 src/ 下的拆分属于参考，实现时可调整并在交付时同步合同。

| 未来路径 | 类别 | 职责 | 函数 |
|---|---|---|---|
| `modules/agent/cmd/agent/main.go` | 手写 | 进程入口 | `agent.main` |
| `modules/agent/internal/app/application.go` | 手写 | HTTP 与原生运行时装配 | `agent.BuildApplication`、`agent.Health`、`agent.Ready`、`agent.StartRun`、`agent.ReadEvents`、`agent.Reply`、`agent.Cancel` |
| `modules/agent/internal/gen/openapi.gen.go` | 生成 | 从合同生成接口 | 无 |
| `modules/agent/internal/gen/clients.gen.go` | 生成 | 从合同生成接口 | 无 |
