# nice-wiki

一个 wiki 系统。前端 Vue 3，后端 Go。

> 目前处于设计阶段，所有设计决策记录在 [docs/adr/](docs/adr/README.md)，一条一个文件。

## 目录结构

```
nice-wiki/
├── server/   # Go 后端：公开页面渲染、JSON 接口、AI、翻译（待搭建）
├── web/      # Vue 3 前端：后台单页应用 + 公开页面上的局部组件（待搭建）
└── docs/     # 设计文档与决策记录
    ├── adr/                  # 决策记录，一条一个文件，README.md 是索引
    └── design/               # 各模块的设计文档
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
| [ai-models.md](docs/design/ai-models.md) | 模型接入：渠道、档位、任务、计费 |
| [admin.md](docs/design/admin.md) | 后台功能清单、文档生命周期 |
| [accounts.md](docs/design/accounts.md) | 后台账号、权限、编辑锁 |
| [security-ops.md](docs/design/security-ops.md) | 安全与运维 |
| [engineering.md](docs/design/engineering.md) | 工程规范：文档体系、过程记录、AI 助手说明文件、测试 |

## 许可证

[AGPL-3.0](LICENSE)。修改本程序后用它对外提供网站服务的，也必须公开修改后的源代码。
