# sqlite-vec

> SQLite 的向量检索扩展，纯 C 编写、没有依赖。

| 项目 | 信息 |
|---|---|
| 方向 | 检索问答 |
| 仓库 | asg017/sqlite-vec |
| 许可证 | Apache-2.0 |
| 主要语言 | C |
| Star | 8.2k |
| 最近更新 | 2026-05 |

## 它是什么

提供 `vec0` 虚拟表，在 SQLite 里存储和查询浮点、int8、二值向量。

## 和我们相关的部分

- 官方的 Go 绑定是 CGO 版本（`sqlite-vec-go-bindings/cgo`）。

## 借鉴什么

- 规模变大后可以考虑。目前几千个检索单元，在内存里直接计算更简单。

## 注意

- **和我们选的纯 Go 驱动 modernc.org/sqlite 不直接兼容**。要用它，需要换成 CGO 驱动或者支持扩展的驱动，会影响"单个可执行文件、交叉编译方便"这个优势。

## 资料

- [仓库](https://github.com/asg017/sqlite-vec)
- [Go 用法](https://alexgarcia.xyz/sqlite-vec/go.html)
