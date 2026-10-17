# 按模块实施

读全局和模块AGENTS、README索引。确认模块规格已被接受，逐条核对requirements→types/API→files/functions；存在缺口先修合同。

入口、契约、配置、部署和约束函数所在文件按files清单创建，约束函数按声明签名实现；internal/与界面src/内的参考拆分可调整，交付提交时同步implementation.json与progress.json并运行make docs。先持久状态/纯领域规则，再远程端口，最后HTTP/UI适配。禁止跨域读写、直接导入内部代码和手改生成文件。

实施尺度按ADR0006：入口之后信任已校验的输入与操作上下文，只写函数特有步骤；合同里的通用约束句、通用错误码和步骤末尾通用句由入口与libs/platform一次满足，不在每个函数重复解析截止、校验schema或映射context错误；不写防御性冗余分支，不复制标准库或平台已有工具；默认不写测试与临时验证程序。

编译门槛：Go代码记implemented前须`make build`通过（全部经用户自部署GitLab CI执行，尽量三个并行分片；不可用时记录blocked，不自行回退本机或SSH编译），go.sum只由go命令生成；无法编译的记blocked（待编译）。跨服务公共机制先用libs/platform已有实现（运行状态客户端、请求校验中间件、outbox/inbox、服务断言），不在本域复制。

编译、lint、类型检查与已有普通测试全部经GitLab CI执行；连接与凭据只读用户指定私有机器管理仓库，不得同步到源码。新增或修改测试和真实验收按AGENTS另行确认。阶段验证及失败如实记录；缺外部配置不能虚构成功。
