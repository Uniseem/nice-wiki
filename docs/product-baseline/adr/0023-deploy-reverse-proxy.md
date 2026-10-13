> 产品需求基线快照：来源 old:docs/adr/0023-deploy-reverse-proxy.md，blob a83beb2ed451ddca2af02d4db30ced320343f762。保留功能与细节；旧路径、数据布局和代码组织属于原架构，不是新实现结构。后续已确认ADR优先。

# D23. 部署：HTTPS 交给反向代理

- 状态：✅ 已定
- 日期：2026-10-12

- 程序只监听 HTTP，HTTPS 由前面的 Nginx、Caddy 或 CDN 负责。
- 配置可信代理的地址，只从它们的请求头里读取访客真实 IP。封禁 IP、判断语言都依赖这个 IP。
- 同时提供可执行文件和 Docker 镜像。🟡
