> 产品需求基线快照：来源 old:docs/adr/0028-security-ops.md，blob da4f958472e9940eb6f6680f8b8241dba63e3d3e。保留功能与细节；旧路径、数据布局和代码组织属于原架构，不是新实现结构。后续已确认ADR优先。

# D28. 安全与运维

- 状态：🟡 倾向（可以推翻）
- 日期：2026-10-12

详见 [design/security-ops.md](https://github.com/Uniseem/nice-wiki/blob/b8190c7/docs/design/security-ops.md)。要点：
- 安全：
  - 渲染时转义，过滤危险链接；
  - 上传的 SVG 禁止执行脚本；
  - AI 回答不允许原始 HTML；
  - 后台检查 `Origin` 防 CSRF；
  - 全站设置 CSP 响应头。
- 运维：
  - 流式输出时关闭反向代理的缓冲；
  - 一键备份、定时备份、从备份恢复；
  - 文档 JSON 带格式版本号；
  - 调用模型渠道失败时重试，再切换到档位里的备用模型（D44）。
