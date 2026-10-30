# portal 文件目录

本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改。登记入口、契约、配置、部署和生成文件的位置，其余源码按需要建立，不逐个登记。

文件 3 个：UI 1、生成 2。

| 路径 | 类别 | 职责 |
|---|---|---|
| `modules/portal/config/routes.json` | 生成 | 由全部模块openapi.json生成的入口路由表：非service接口的方法/路径→owner、operationId、x-permission、x-admission、超时与流式标志；Rust edge（rust/edge）启动时加载，未登记路径一律404 |
| `modules/portal/web/src/gen/api.gen.ts` | 生成 | make gen由openapi-typescript生成浏览器DTO；本域OpenAPI的x-browser-auth-adapters替换identity四认证操作的浏览器输入/输出，内部identity Go契约和真实路由归属保持生产方；生成目录不手改 |
| `modules/portal/web/src/App.tsx` | UI | 后台单页应用根：登录/初始化向导、后台外壳与按 permission-groups.adminRoutes 装配的各域页面 |
