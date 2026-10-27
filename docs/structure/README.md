# 文件结构契约

各模块的入口、契约、配置、部署和生成文件登记在 contracts/implementation.json，`make docs` 由它生成 docs/files.md。共享组件（libs/platform、deploy）同理。

不再逐个登记函数（2026-10-27 起，函数级登记和 functions.md 已删除）：新代码不为它登记函数，内部文件和函数不用登记。implementation.json 还保留 `consumes`（本域调用的其他域接口，用来生成客户端）、`jobTypes` 和 `backgroundTasks`，检查工具见 [tooling.md](tooling.md)。
