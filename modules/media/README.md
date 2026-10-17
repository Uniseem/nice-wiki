# 资源媒体（M05）

保持 D24/D26 与资源库产品：内容寻址上传/Range、浏览器WebP/视频封面、四平台公开信息快照、本地封面、Excalidraw三文件，文件不自动删除。

当前第二轮已实现真实资源列表/未引用候选分页及两个HTTP，严格签名scope、独立本地和两源水位、历史风险及未知失败关闭；GitLab pipeline23生成、三分片14模块编译和完整检查通过。列表不产生人工批准或删除，业务验收仍未执行。

既有实现覆盖人工删除Execute、同进程worker注册、完整DELETING恢复与有限reader权威；原批准ObjectRef、任务回执、固定24小时窗口、最终完整两源及marks/readers0共同约束删除。共享bytes与Complete互斥，终局结果与deleted outbox同事务，真实取消仅凭独立只读证明。最终候选通过自部署GitLab CI生成/格式、三分片14模块readonly build/vet/gofmt及完整结构检查；业务验收未执行。

完整服务仍因真实JobsClient适配、bytes、其余HTTP/事件及控制恢复未齐保持blocked；worker与outbox整体职责继续partial。未运行服务或删除业务文件，未新增测试、真实接口、模型调用或部署。逐项状态及准确源码提交见STATUS/progress/handoff。

## 设计索引

- [docs/requirements.md](docs/requirements.md)
- [docs/architecture.md](docs/architecture.md)
- [docs/data.md](docs/data.md)
- [docs/api.md](docs/api.md)
- [docs/workflows.md](docs/workflows.md)
- [docs/files.md](docs/files.md)
- [docs/functions.md](docs/functions.md)
- [docs/operations.md](docs/operations.md)
- [docs/acceptance.md](docs/acceptance.md)

## 机器契约

- [类型与 schema](contracts/types.json)
- [文件与函数实现合同](contracts/implementation.json)
- [OpenAPI 3.1](contracts/openapi.json)
- [事件合同](contracts/events.json)
- [模块工作规则](AGENTS.md)

文件与函数清单及数量见 [docs/files.md](docs/files.md)（由合同生成）。数量只反映登记覆盖，不能视为验收通过。

## 产品来源

- [全局功能保持矩阵](../../docs/product-preservation.md)
- [冻结产品基线](../../docs/product-baseline/README.md)
- [产品来源 editor.md](../../docs/product-baseline/design/editor.md)
- [产品来源 admin.md](../../docs/product-baseline/design/admin.md)
- [产品来源 security-ops.md](../../docs/product-baseline/design/security-ops.md)

确认的old功能/默认行为保持，新的服务私有存储/CAS/fencing只改变内部架构。D55优先于先前初始化AI条款。


## 进度与新会话交接

先读 [本域状态](STATUS.md)、[逐项台账](progress.json)、[会话交接](docs/handoff.md)。有规格不代表已有实现；并行仅改本域，公共文件交集成会话。
