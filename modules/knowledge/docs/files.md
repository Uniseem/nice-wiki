# knowledge 文件目录

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改。登记入口、契约、配置、部署和生成文件的位置，其余源码按需要建立，不逐个登记。

文件 2 个：UI 1、生成 1。

| 路径 | 类别 | 职责 |
|---|---|---|
| `modules/knowledge/web/src/gen/api.gen.ts` | 生成 | make gen由openapi-typescript从本域openapi.json生成浏览器类型，配合openapi-fetch经portal同源调用；不手改 |
| `modules/knowledge/web/src/index.tsx` | UI | 知识索引：入库配置、为已发布文档建立索引、索引任务进度与取消 |
