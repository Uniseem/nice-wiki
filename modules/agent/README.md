# M16 原生 Agent

独立承载 ZCode 原生 AgentRuntime、上下文压缩和工具调度。问答服务持有模型调用身份、文档权限及派生会话记录，通过版本化 API 回送模型与只读工具结果；本服务不持有模型密钥或业务数据库。

`NW_ZCODE_WORKER` 指向 `pnpm build:agent` 生成的 worker。缺少 Node 或运行文件时就绪检查及新调用返回依赖不可用，HTTP 服务仍可启动。
