# 按模块实施

读全局和模块AGENTS、README索引。确认模块规格已被接受，逐条核对requirements→types/API→files/functions；存在缺口先修合同。

入口、契约、配置、部署和约束函数所在文件按files清单创建，约束函数按声明签名实现；internal/与界面src/内的参考拆分可调整，交付提交时同步implementation.json与progress.json并运行make docs。先持久状态/纯领域规则，再远程端口，最后HTTP/UI适配。禁止跨域读写、直接导入内部代码和手改生成文件。

已有检查优先VPS；新增或修改测试和真实验收按AGENTS另行确认。阶段验证及失败如实记录；缺外部配置不能虚构成功。
