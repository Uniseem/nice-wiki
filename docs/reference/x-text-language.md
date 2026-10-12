# golang.org/x/text/language

> Go 官方的语言标签处理包，用来解析和匹配 `Accept-Language`。

| 项目 | 信息 |
|---|---|
| 方向 | 存储与运维 |
| 仓库 | golang/text |
| 许可证 | BSD-3-Clause |
| 主要语言 | Go |
| Star | 0.8k（GitHub 镜像） |
| 最近更新 | 2026-10 |

## 它是什么

Go 团队维护的扩展库，处理 BCP 47 语言标签。

## 和我们相关的部分

- `ParseAcceptLanguage` 解析浏览器的语言偏好；`NewMatcher` 在支持的语言里找最合适的一个，能处理地区变体（例如 `zh-TW` 匹配到繁体中文）。

## 借鉴什么

- 根路径的语言跳转（D12）直接用它，不要自己写匹配规则。

## 资料

- [包文档](https://pkg.go.dev/golang.org/x/text/language)
- [GitHub 镜像](https://github.com/golang/text)
