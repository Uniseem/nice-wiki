# 问答与缓存当前进度

更新：2026-10-17。阶段：实施中。分支 `codex/qa-20261017-r5`，基线 `a3e6af980d5288e42bc8d023410faf8923ff21e8`，源码提交 `7ef0cfb4f1e695a82dcecb5e84e1b0825a7c62fe`。部署 `not_started`。accepted 为 0。

- `qa.EvaluateCacheEligibility` 与 `modules/qa/internal/cache/eligibility.go` 保持 blocked。已补全、非个性化、非敏感动态、仍依赖页面和 out_of_scope 按已有字段拒绝共享缓存。指代残留没有可执行规则，函数不扫描问题文本；上述字段都通过时仍返回 eligible=false、reason=ambiguous。未接语义缓存、嵌入、真实模型或持久化。
- 验证状态为 not_run。本轮未跑 GitLab CI，未执行 go、gofmt、make build、make gen、make check 或 SSH。本域执行了 `make docs`（只调用 Python 渲染）。
- `qa.NormalizeQuestion` 仍为 implemented，验证 not_run。其余函数与文件保持原状态。
- 新文件尚未写入根 `docs/structure/framework.json`，由集成会话登记。

下一步：等合同写明独立问题残留这/它或未命名第二步来源的可执行判定后，再打开 eligible=true。在此之前不要自选文本规则，也不要把本函数记成 verified 或 accepted。

历史记录见下文，其中的旧下一步不再执行。

---

# 问答与缓存当前进度

更新：2026-10-15。阶段：R1-16 实施中；本域业务验收与部署未开始。

- 2026-10-15 集成复核修复：`NormalizeQuestion` 先按 BCP 47 规范化语言标签再计算问题哈希，无法解析的标签按 VALIDATION 拒绝。 其余修复与验证见 [handoff](docs/handoff.md)。编译、vet、gofmt 与 make check 已通过，业务验收未执行。
- 本次基线为 `8ae6fcf6302e2fd41a998faef18be52e2a9714c`，实际工作分支为 `main`；独占范围仅为 `modules/qa/`。
- 已实现 `qa.NormalizeQuestion`（提交 `4f4d11a4a6f44f2bd69def6d9a8ee7701a97eb6d`）：NFKC、CRLF/Unicode空白折叠、自然语言大小写规范化、代码/产品标识与否定词保留、500 Unicode 字符/8192 字节限制、确定性问题哈希和最多256个稳定锚点；不读写存储、不接检索、不调用真实模型。
- 新增 `modules/qa/go.mod` 声明本域独立模块及 `x/text`/`libs/platform` 依赖；未手写生成的 `go.sum`。
- 本机没有 Go、gofmt、make，源码格式化、编译、vet、模块依赖生成及业务验收均未运行，因此函数和文件保持 `implemented`/`not_run`，不能标为 `verified` 或 `accepted`。
- 已通过 JSON 解析与 `git diff --check`；根 `docs/structure/framework.json` 的新增文件登记由集成会话处理。未新增测试/样例、真实接口、真实模型、部署或 push。

本域其余 60 个函数仍按 `progress.json` 的逐项状态为 planned。公共契约、根文件、生成物和其他模块均未修改；工作树中已有的 `modules/model-gateway/` 并行改动未触碰、未提交。
