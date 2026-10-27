# 设计校验工具说明

实际工具为根 Makefile、scripts/check_docs.py、scripts/render_docs.py 与 scripts/gen_code.py；它们不执行或模拟业务，不产生测试、样例、数据库或模型调用。Python 仅使用标准库，最低 3.11。VPS 执行 `make check`；本地允许只读文档结构检查。所有问题仅输出仓库相对路径及规则，不输出疑似秘密值。

Makefile 的 `check` 调用 Python 全量校验，`privacy` 只查隐私和 Git 身份，`summary` 输出设计数量；`docs` 调用 render_docs.py 从各域合同重新生成 files.md 与 api.md 接口清单；R1阶段`gen`从唯一机器契约生成Go/TypeScript/API路由及任务权限目录，`build`对现有14个Go模块执行readonly build/vet/gofmt；`test`只提示尚未登记套件，不能当业务测试成功。`PYTHON` 可指定标准库 Python 解释器。

| 函数/准确签名 | 输入与返回 | 作用与步骤 | 副作用/错误 |
|---|---|---|---|
| `fail(label, message)` | label 可转字符串；message 为不含秘密的说明；返回 None | 拼接规则说明加入 ISSUES；不抛业务错误 | 只写进程内问题列表 |
| `relative(path)` | 仓库内绝对 pathlib.Path；返回使用 `/` 的仓库相对 str | 检查相对 ROOT，规范输出路径 | 越界 ValueError，调用者必须先确认边界 |
| `load_json(path)` | 仓库内 JSON 路径；返回解析对象，失败返回空 dict | resolve→缓存→UTF-8 读取→json.loads；失败记录文件和异常类型 | 只读文件，写内存 CACHE/ISSUES，不输出原文 |
| `walk_nodes(value)` | JSON 兼容 dict/list/标量；返回 dict 节点迭代器 | 递归对象值和数组元素，只 yield 对象节点 | 无文件/网络副作用 |
| `check_ref(base, target)` | base 为引用方 JSON 路径；target 为 `$ref` 字符串；返回 None | 分离文件/JSON Pointer→只匹配已登记 `$id` 或仓库内文件→解码 `~0/~1`→确认节点存在 | 禁止联网取远程 schema；错误加入 ISSUES |
| `scan_privacy(label, text)` | 相对路径/用途标签与 UTF-8 文本；返回 None | 检查非代用邮箱、本机用户目录、常见令牌/私钥、非文档 IPv4；不记录命中值 | 仅追加规则问题；模式扫描不能代替人工隐私复核 |
| `check_links(path)` | Markdown 路径；返回 None | 先忽略代码围栏/行内代码中的伪链接→提取 Markdown 文件链接→忽略外部 URL/锚点→解码路径→确认仓库内目标存在 | 只读文件；不访问外部 URL，也不验证锚点标题 |
| `check_modules(modules, permissions, codes, shared=())` | 已解析服务模块数组、权限列表、错误码列表与共享组件数组；返回服务/共享组件/文件/操作/类型计数 dict | 确认每域资料齐全→登记文件（路径在本域内、不重复、有职责）→逐个 OpenAPI operation 核对 operationId、x-handler、权限与响应→核对 consumes 引用的跨域 operation 存在 | 只读文件；问题进入 ISSUES |
| `check_identity()` | 无参数；返回 None | 读取当前 Git 用户名/邮箱→确认 GitHub 代用邮箱→同隐私扫描 | 启动只读 `git config --get` 子进程；未配置时记录失败，不修改配置 |
| `check_events(modules)` | 模块清单；返回 None | 收集唯一生产方→核对字段/交付语义→消费者必须引用同名同版唯一生产方载荷schema→consume条目必须有handler→有消费事件的域须调用jobs订阅与确认，有生产事件的域须调用jobs:publishEvent（以 consumes 为准） | 只读文件；问题进入 ISSUES |
| `check_wiring(modules, shared=())` | 服务模块与共享组件数组；返回 None | 核对后台页面目录与公共schema一致→各域jobTypes/backgroundTasks字段完整，每个任务和事件生产方的managementPermission必须属于已登记具体业务权限且与jobManagementGrants严格一致→非v2的service接口必须有调用方 | 只读文件；问题进入 ISSUES |
| `check_rendered()` | 无参数；返回 None | 导入 render_docs.expected_docs()→逐个比对各域 files.md/api.md/workflows.md 与共享组件生成文档 | 只读文件；不一致时提示运行 make docs，不自动改写 |
| `check_staged()` | 无参数；返回 None | git diff列暂存增加/修改→逐blob隐私扫描→拒绝私有运行目录和未审阅二进制 | 仅调用只读Git子进程，不暂存、不提交 |
| `main()` | 读取命令行 `--privacy-only`、`--summary`、可选 `--message-file`；返回 int 0/1 | 枚举允许文档→扫描隐私/链接/JSON→核对作者→检查引用、模块、事件与生成文档→解析两个脚本的 Python AST→打印结果 | 仅读文件和 Git 配置、输出规则；命令行错误由 argparse 返回 2 |

## scripts/render_docs.py

| 函数/准确签名 | 输入与返回 | 作用与步骤 | 副作用/错误 |
|---|---|---|---|
| `cell(value)` | 字符串或列表；返回表格单元格文本 | 列表用“；”连接，转义竖线并去掉换行 | 无副作用 |
| `render_files(slug, impl)` | 模块 slug 与合同；返回整份 files.md | 统计文件类别，列出路径、类别与职责 | 无副作用 |
| `schema_name(content)` | OpenAPI content 对象；返回 schema 名称、媒体类型或“—” | 取第一个媒体类型的 $ref 末段；二进制返回媒体类型 | 无副作用 |
| `render_api(api, codes)` | 已解析 openapi.json 与错误码列表；返回接口清单 Markdown | 按路径与方法列出 operationId、处理函数、权限、请求→成功响应、摘要与各错误响应中出现的错误码 | 无副作用 |
| `merge_block(current, block, start, end)` | 现有文本或 None、生成块与起止标记；返回完整文本 | 替换标记之间的内容，保留标记外手写说明；缺标记时追加到末尾 | 无副作用 |
| `merge_api(current, block)` | 现有 api.md 文本或 None、生成块；返回完整文本 | 以 openapi 标记调用 merge_block | 无副作用 |
| `render_flows(impl, events)` | 合同与事件合同；返回入口与后台流程 Markdown | 列出消费事件与处理函数、每个jobType的执行函数/触发/定时与步骤、后台任务与运行时机 | 无副作用 |
| `expected_docs()` | 无参数；返回 {Path: 内容} | 读取 modules.json、errors.json 与各域合同，生成每域 files.md、api.md 接口清单与 workflows.md 流程块，以及共享组件的/files.md | 只读文件；JSON 错误直接抛出 |
| `main()` | 命令行可选 `--check`；返回 int 0/1 | 比对期望内容→默认写入过期文件并列出→`--check` 只列出并在有过期时返回 1 | 默认模式只改写各域与共享组件的生成文档或生成块 |

模块 JSON 读取顺序先全部装入 CACHE，再核对 `$id` 引用。每次独立 Python 进程启动时 ISSUES/CACHE 为空；不并发改仓库。各域 implementation.json 登记入口、契约、配置和生成文件，不再登记函数。

当前检查不证明 Go/TypeScript 可编译，不覆盖业务行为、schema 全规范合法性、供应方协议、部署可用性或竞态；这些必须在对应实施与验收阶段落实。

任务目录与权限映射由gen_code.py的gen_jobs_catalog/gen_go_registries从各域jobTypes.managementPermission及根permission-groups.json生成，禁止各域手写第二份映射。上述设计校验只检查目录一致性，不验证运行时授权行为。

客户端生成按OpenAPI声明转发查询参数（原始客户端可用可选url.Values），SSE的Last-Event-ID header与afterSequence query分别按规格生成，不猜测统一lastEventId查询参数；默认分页上限来自schema固定default。
