# 设计校验工具的文件与函数合同

实际工具只有根 Makefile 与 scripts/check_docs.py；它们不执行或模拟业务，不产生测试、样例、数据库或模型调用。Python 仅使用标准库，最低 3.11。VPS 执行 `make check`；本地允许只读文档结构检查。所有问题仅输出仓库相对路径及规则，不输出疑似秘密值。

Makefile 的 `check` 调用 Python 全量校验，`privacy` 只查隐私和 Git 身份，`summary` 输出设计数量；`gen/build/test` 在设计阶段报未实现并退出 1，不能当成功构建。`PYTHON` 可指定标准库 Python 解释器。

| 函数/准确签名 | 输入与返回 | 作用与步骤 | 副作用/错误 |
|---|---|---|---|
| `fail(label, message)` | label 可转字符串；message 为不含秘密的说明；返回 None | 拼接规则说明加入 ISSUES；不抛业务错误 | 只写进程内问题列表 |
| `relative(path)` | 仓库内绝对 pathlib.Path；返回使用 `/` 的仓库相对 str | 检查相对 ROOT，规范输出路径 | 越界 ValueError，调用者必须先确认边界 |
| `load_json(path)` | 仓库内 JSON 路径；返回解析对象，失败返回空 dict | resolve→缓存→UTF-8 读取→json.loads；失败记录文件和异常类型 | 只读文件，写内存 CACHE/ISSUES，不输出原文 |
| `walk_nodes(value)` | JSON 兼容 dict/list/标量；返回 dict 节点迭代器 | 递归对象值和数组元素，只 yield 对象节点 | 无文件/网络副作用 |
| `check_ref(base, target)` | base 为引用方 JSON 路径；target 为 `$ref` 字符串；返回 None | 分离文件/JSON Pointer→只匹配已登记 `$id` 或仓库内文件→解码 `~0/~1`→确认节点存在 | 禁止联网取远程 schema；错误加入 ISSUES |
| `scan_privacy(label, text)` | 相对路径/用途标签与 UTF-8 文本；返回 None | 检查非代用邮箱、本机用户目录、常见令牌/私钥、非文档 IPv4；不记录命中值 | 仅追加规则问题；模式扫描不能代替人工隐私复核 |
| `check_links(path)` | Markdown 路径；返回 None | 提取 Markdown 文件链接→忽略外部 URL/锚点→解码路径→确认仓库内目标存在 | 只读文件；不访问外部 URL，也不验证锚点标题 |
| `check_modules(modules, permissions, codes)` | 已解析模块数组、权限列表、错误码列表；返回模块/文件/函数/操作/类型计数 dict | 确认每域资料齐全→登记文件/函数→检查函数字段和文档签名→核对类型、错误、HTTP处理器/权限/参数→解析跨域 operation 依赖 | 只读合同；问题累积到 ISSUES；数量不等于功能验收 |
| `check_identity()` | 无参数；返回 None | 读取当前 Git 用户名/邮箱→确认 GitHub 代用邮箱→同隐私扫描 | 启动只读 `git config --get` 子进程；未配置时记录失败，不修改配置 |
| `check_events(modules)` | 模块清单；返回 None | 收集唯一生产方→核对字段/交付语义→消费者必须引用同名同版唯一生产方载荷schema | 只读合同，错误追加 ISSUES |
| `check_framework()` | 无参数；返回 None | 枚举实际文档/合同/工具/根文件→与framework.json逐路径比对→报告未登记或缺失 | 不创建或删除文件；不含私有vps.txt、.git |
| `check_staged()` | 无参数；返回 None | git diff列暂存增加/修改→逐blob隐私扫描→拒绝私有运行目录和未审阅二进制 | 仅调用只读Git子进程，不暂存、不提交 |
| `check_progress(modules)` | 模块清单；返回 None | 核对每文件/函数唯一台账、交接资料、状态枚举、已实现提交SHA与已验证证据；设计阶段必须全部planned | 只查记录一致性，不证明记录的业务结论真实；错误追加 ISSUES |
| `main()` | 读取命令行 `--privacy-only`、`--summary`、可选 `--message-file`；返回 int 0/1 | 枚举允许文档→扫描隐私/链接/JSON→核对作者→检查引用和模块→解析自身 Python AST→打印结果 | 仅读文件和 Git 配置、输出规则；命令行错误由 argparse 返回 2 |

模块 JSON 读取顺序先全部装入 CACHE，再核对 `$id` 引用。每次独立 Python 进程启动时 ISSUES/CACHE 为空；不并发改仓库。任何非零退出均不能标记 R0-04 通过。全局框架的实际文件清单登记在 [framework.json](framework.json)，模块未来源码清单登记在各域 implementation.json。

当前检查不证明 Go/TypeScript 可编译，不覆盖业务行为、schema 全规范合法性、供应方协议、部署可用性或竞态；这些必须在对应实施与验收阶段落实。
