# nice-wiki

一个 wiki 系统。前端 Vue 3，后端 Go。

> 已开始实现，当前进度见 [STATUS.md](STATUS.md)。任务与验收标准见 [实施计划](docs/plan.md)；设计决策记录在 [docs/adr/](docs/adr/README.md)。

## 目录结构

```
nice-wiki/
├── server/   # Go 后端：公开页面渲染、JSON 接口、AI、翻译（待搭建）
├── web/      # Vue 3 前端：后台单页应用 + 公开页面上的局部组件（待搭建）
└── docs/     # 设计文档与决策记录
    ├── plan.md               # 实施计划：阶段、任务、依赖与验收标准
    ├── adr/                  # 决策记录，一条一个文件，README.md 是索引
    ├── design/               # 各模块的设计文档
    ├── reference/            # 开源参考资料：每个项目一份，README.md 是总结
    └── structure/            # 文件结构：每个文件的用途（改结构必须先询问）
```

## 设计文档

| 文档 | 内容 |
|---|---|
| [architecture.md](docs/design/architecture.md) | 整体架构、草稿与发布、版本历史、仓库布局 |
| [editor.md](docs/design/editor.md) | 编辑器：块类型、画板、超级块、视频、图片 |
| [site.md](docs/design/site.md) | 公开站点：布局与 URL |
| [home.md](docs/design/home.md) | 首页主视觉 |
| [i18n.md](docs/design/i18n.md) | 多语言：翻译流程、语言判断与切换 |
| [ai-qa.md](docs/design/ai-qa.md) | AI 问答：检索方案、问答记录与缓存 |
| [ai-guard.md](docs/design/ai-guard.md) | AI 问答防滥用 |
| [memory.md](docs/design/memory.md) | 长期记忆：统一维护，各功能按需取用 |
| [ai-models.md](docs/design/ai-models.md) | 模型接入：渠道、档位、任务、计费 |
| [admin.md](docs/design/admin.md) | 后台功能清单、文档生命周期 |
| [accounts.md](docs/design/accounts.md) | 后台账号、权限、编辑锁 |
| [security-ops.md](docs/design/security-ops.md) | 安全与运维 |
| [engineering.md](docs/design/engineering.md) | 工程规范：文档体系、过程记录、AI 助手说明文件、测试 |

## 许可证

[AGPL-3.0](LICENSE)。修改本程序后用它对外提供网站服务的，也必须公开修改后的源代码。

## 开发与检查

编译与已有测试优先使用 VPS。将私有服务器地址写入根目录 `vps.txt`；该文件已被 git 忽略，禁止上传 GitHub。默认 SSH 用户为 `root`，使用已有 SSH 密钥，可通过 `VPS_USER`、`VPS_PORT` 指定连接参数。

- `make vps-check`：同步排除私有配置的源码，在 8 核、4 GB 构建环境生成接口、构建前端并运行统一检查。
- `make vps-build`：在 VPS 构建前端和纯 Go 可执行文件。
- `make vps-test`：在 VPS 运行已有普通测试；不会新增测试或调用真实模型。
- `make dev`：需要开发工具链时同时启动 Go 与 Vite。Go 参数支持 `--data`、`--addr`、`--dev`、`--log-level`，对应 `NICE_WIKI_*` 环境变量。

远程构建需要已准备的 Go、Node、pnpm、git、make 及 C 编译器（race 检查）。本次构建工具安装在服务器的独立缓存目录，不修改本机 git 身份。首次部署后必须立即注册超级管理员，或先限制后台端口访问。

当前实现仍在验收，AI 问答、多语言翻译与完整备份功能尚未开放。运行时数据位于单独 `data/` 目录；模型渠道只登记配置，不发送模型请求。

当前开发分支正在接入 AI、翻译、统计与一致备份；正式启用条件以后台状态和发布说明为准，未配置完整模型档位时不会投递模型任务。

## 基础站部署

基础站以单个二进制加持久化 `data/` 目录运行。首次启动后的超级管理员抢注风险、反向代理、HTTPS、可信代理和升级注意事项见 [部署说明](deploy/README.md)。
