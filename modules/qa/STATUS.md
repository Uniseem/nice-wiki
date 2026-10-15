# 问答与缓存当前进度

更新：2026-10-15。阶段：R1-16 实施中；本域业务验收与部署未开始。

- 2026-10-15 集成复核修复：`NormalizeQuestion` 先按 BCP 47 规范化语言标签再计算问题哈希，无法解析的标签按 VALIDATION 拒绝。 其余修复与验证见 [handoff](docs/handoff.md)。编译、vet、gofmt 与 make check 已通过，业务验收未执行。
- 本次基线为 `8ae6fcf6302e2fd41a998faef18be52e2a9714c`，实际工作分支为 `main`；独占范围仅为 `modules/qa/`。
- 已实现 `qa.NormalizeQuestion`（提交 `4f4d11a4a6f44f2bd69def6d9a8ee7701a97eb6d`）：NFKC、CRLF/Unicode空白折叠、自然语言大小写规范化、代码/产品标识与否定词保留、500 Unicode 字符/8192 字节限制、确定性问题哈希和最多256个稳定锚点；不读写存储、不接检索、不调用真实模型。
- 新增 `modules/qa/go.mod` 声明本域独立模块及 `x/text`/`libs/platform` 依赖；未手写生成的 `go.sum`。
- 本机没有 Go、gofmt、make，源码格式化、编译、vet、模块依赖生成及业务验收均未运行，因此函数和文件保持 `implemented`/`not_run`，不能标为 `verified` 或 `accepted`。
- 已通过 JSON 解析与 `git diff --check`；根 `docs/structure/framework.json` 的新增文件登记由集成会话处理。未新增测试/样例、真实接口、真实模型、部署或 push。

本域其余 60 个函数仍按 `progress.json` 的逐项状态为 planned。公共契约、根文件、生成物和其他模块均未修改；工作树中已有的 `modules/model-gateway/` 并行改动未触碰、未提交。
