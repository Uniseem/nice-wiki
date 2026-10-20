# 文件结构契约

各模块源码文件和约束级函数登记在 contracts/implementation.json，`make docs` 由它生成 docs/files.md 和 docs/functions.md。共享组件（libs/platform、deploy）同理。

只有约束级函数（HTTP 处理、事件处理、任务执行、启动入口）需要登记，内部辅助文件和函数不用登记。函数字段规则见 [函数登记要求](functions.md)，检查工具见 [tooling.md](tooling.md)。
