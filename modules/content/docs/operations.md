# 运行、恢复与异常处理

所有有状态快照/barrier完整登记；恢复按ADR0003暂存后重启换入，control区不随恢复替换。content snapshot包含正文+head/journal/tree/redirects/history/IDledger和引用水位，排空保存txn后导出；内存锁不导出，恢复后全部editor重获锁。

D30第一版不做批量导入；粘贴是编辑器交互。D54公开heading不折叠。服务不访问media/data-protection私有目录；go-git镜像由data-protection通过对象API获取已提交文本。

服务启动顺序：验证配置/schema → 打开本服务私有存储 → 恢复未完成决定日志 → 读取 site-config runtimeState → 校验活动 epoch → 接受允许的读取 → 仅在无维护/屏障时接受业务写入。

超时的写请求必须使用同 operationId 获取原结果。依赖失败不可假定权限、可见性或引用为空。日志记录 requestId、operationId、状态与安全字段，不记录正文、密码、token、原始文件路径或用户 IP。备份导出经只读 ObjectRef API；导出内容与 staging 根由本服务拥有。

快照协议各阶段在 API/函数中登记；portal 仅 gate 屏障。stage/discard 均需 data-protection 服务证明、superadmin 委托及维护状态；只要有域未登记切换回执，site-config 不能退出维护。


第一轮真实正文存储已接启动open/migrate/recover，完整服务继续blocked。迁移只追加，已应用SQL哈希不同或私有库epoch不符时停止；本轮不实现恢复换目录后的epoch重绑定和activation。正文恢复的COMMITTED必须同时有真实私有提交资格及原回执，不能由JSONL单独升级。根统一在VPS做格式/生成/编译/结构检查，业务验收未执行。

首个可信新库的epoch与第1迁移及记录同SQL事务提交；JSONL追加/fsync与所有journal扫描共享RWMutex，读快照与追加互斥，锁内不取SQL/Store互斥。journal使用标准NOFOLLOW并在打开前后/读写后验证SameFile及公共私有路径，不持DB事务跨文件IO。

持久prepared/committed决定必须有完整同材料JSONL；Find缺日志即完整性失败。启动反向核对全部已准备/提交索引，不能只扫描现存日志而漏丢失/截空记录；唯一允许无日志的是尚未append的preparing且没有提交资格，缺日志不猜补。


编辑锁原操作回执属于不可变历史结果，不能作为启动重建租约的来源；进程重启和恢复均清空锁并改bootId。原回执重放携旧boot不会恢复写资格。当前boot的未知SQL提交pending会阻止同doc后续编辑/保存，只有真实原回执和候选hash一致才安装，确定没有回执才丢弃。通知生产者outbox/本地审计已编写候选，完整转发与在线消费尚待后续装配，未做业务验收。

第二轮真实共享草稿实现增加0004只追加迁移；未知既有文档缺完整元数据或站点treeRevision时停止该业务，不推测修复。COMMITTED正文资格、完整原回执和源引用决定同SQL提交；SQL Commit错误不推定失败、不释放pin。最终冻结源码已通过根统一VPS make docs/gen、8源码gofmt、14模块readonly build/vet/gofmt与完整make check（876文件/1403函数/379HTTP/2317类型）；仅证明编译及设计结构。业务验收、全部HTTP/后台/恢复activation及完整启动未完成。

## 2026-10-17资源来源查询

只追加0005维护全站reference_scan_state；来源表的插入/更新/删除与原事务一起推进水位，耗尽回滚。每页同SQL快照完整核元数据/head/COMMITTED/回执/引用覆盖，并在签名前释放SQL；签名游标固定过滤、分页大小、主体/会话/代次、水位和位置。跨页水位变化409必须从头，不拼接快照、不持久化读快照。来源扫描各20000预算超限503，不把不完整扫描解释为空。

current_draft与history取真实索引分类；pending_pin来自Acquire前已持久的完整保存意图，retained=true只表示保护，不是COMMITTED。精确COMMITTED资格优先，即使原intent终态尚未补齐；明确aborted/rejected且无COMMITTED才排除。未知已发布/分享图生产资格、缺索引、缺原回执或坏材料均失败关闭。本轮没有实现发布或元数据分享图生产者，也不通过空结果掩盖它们。

两个只读来源HTTP路由使用平台统一服务身份/当前epoch/schema边界；media源决定限定纯服务，列表固定media/site-config/data-protection纯服务或portal即时content.read。RelayOutbox只把真实本域outbox与jobs客户端交给平台单批机制；后台循环和完整启动继续未完成。本轮源码已通过根自部署GitLab CI pipeline 12生成/格式/重复生成、三个实际同时分片覆盖14模块readonly build/vet/gofmt及完整make check（878文件/1435函数/379HTTP/2328类型），未执行SQL/HTTP行为验收或部署。


COMMITTED是不可撤销历史决定；后来正文/文档隔离不撤销完整精确资格、原回执与refs的媒体保护。当前正文读取仍拒绝隔离，缺资格或材料矛盾仍INTEGRITY_ERROR，绝不降pending/abort。

## 2026-10-17第二轮文档树

文档树同快照核真实站点/代次、树水位、原始metadata schema、head/COMMITTED资格、无环与活动父/同父orderKey及slug唯一；活动树按parent/orderKey/ID前序，回收站独立列表，签名cursor绑定主体/会话/权限版本/state/limit、树水位及真实reference_scan_state来源水位，任一水位变化409从头；既有0005触发器覆盖metadata/title/state/head/隔离写，确保SaveDraft仅推进head时也不混完整metadata快照。10000仅为新树查询/操作的明确扫描/输出预算，purged墓碑不计入；超限503不截断。旧Create nil回调无新增总量限制，内部Ensure/Plan可信完整树输入取消maxItems，不新增站点配额。

四个树action只在core按所需功能一次即时identity，DB/锁之前执行；平台仍严格验证服务身份/委托/epoch/schema，仅对实际注册四action不重复授权RPC。trash列表需content.publish，活动读content.read，移动/重排content.edit。真实树写在锁外取得当前写窗口，短SQL做树/metadata CAS、完整原回执和最小content.tree.changed outbox（业务content.edit、jobs事件管理仍沿全域content.publish）；0006仅追加原回执，未知提交按原op查回执。重排要求完整同父活动集合，trash/purged不混入；均匀base62 key不改变slug。

PlanOldPathRedirects完整规划node及所有曾发布后代的无语言前缀旧路径→稳定docId，未知祖先slug失败关闭；非空规划因真实版本化redirect manifest生产链未齐而使Move明确503、不提交，Move及文件继续partial。没有新增假manifest、发布生产者或消费者；当前事件只可靠排队，完整应用因写窗口/全量handler/恢复/后台未齐仍blocked。本轮源码已通过GitLab pipeline23生成/格式/重复生成、三并行分片14模块readonly build/vet/gofmt及完整make check；pipeline21真实编译失败、pipeline22取消均未计通过。未做业务验收或部署，缺可信写窗口在原回执后、新写前503。公共纯服务本站传递仍是后续协议前置，当前不声称跨服务来源链已联通。

## 2026-10-17第三轮metadata候选

元数据编辑在DB/锁前一次即时content.edit，同代完整scope+规范请求摘要优先回放完整原结果，核result_hash与严格结果schema；新写锁外取得当前平台窗口，短SQL核活动文档、真实metadata/tree CAS并提交标题/SEO/路径、原结果和最小tree.changed。人工SEO非null为manual（包括空串），null为pending；不触发模型或发布共享正文。

仅根级保留navigation、ask；子级允许，同父活动slug唯一。robots.txt/llms.txt/sitemap.xml由slug字符规则自然排除，不另禁无点词、api/admin/v1等。校验使用调用方同SQL可信完整树，纯助手复用给Move，未来发布也沿同规则。PUT metadata与既有树入口均由core一次即时授权，公共边界仍校身份/epoch/schema，无私有缓存marker。

只追加0007保存完整同代原回执，无影子树/正文/媒体来源。shareImage实际改变因精确metadata pin/引用源协议未齐503，原完整ref可保持；曾发布节点或后代路径变化因真实版本化redirect manifest未齐503，Update整体继续partial。当前写窗口缺失新写503，完整启动/行为验收/部署未完成；本轮候选待GitLab，生成物不手改。
