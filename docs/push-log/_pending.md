# 待整理会话记录

## 2026-10-14：归档与从零规格重建

- 用户明确放弃旧实现，授权old归档、main重建分级文档与完整文件/函数合同。
- 已保存old提交272bddc，包含原有全部代码文档及前轮未提交记录。用户另行授权修正本仓库作者为既有GitHub代用身份，未修改全局配置。
- 按用户后续要求，旧产品功能/细节及AI上下文/缓存以old设计和已确认ADR为需求基线，保存50份产品设计/决策快照；不复用旧业务源码和函数结构。
- 15域最终规格：753未来文件、860函数、377HTTP；新增独立启动/部署规格、多会话互斥所有权和逐文件/函数交接台账。旧线上实例未改变，私有连接文件未入Git，未调用真实模型，未创建业务代码/测试。
- 隔离VPS make check通过，范围仅设计契约/结构/引用/台账/隐私，不将机械检查冒充业务完成。
- 用户2026-10-14明确授权“写清楚就push”。本轮内容已整理到 [推送记录](2026-10-14-0001.md)，old与main规格基准原子推送成功且已核对远端；本条结果记录随main同步。GPT-6按真实合著者署名。

## 2026-10-14：本地同步与忽略规则

- 按用户要求从GitHub快进同步main至3511403；本地无未推送提交，未push。
- 用户确认删除旧实现遗留的本地被忽略目录server/、web/（仅构建产物与依赖缓存，无跟踪文件），并在.gitignore追加.DS_Store。未改结构契约，未运行业务构建或测试。

## 2026-10-14：规格审阅与修订（ADR0002）

- 用户请Claude审阅GPT-6.1 Sol产出的十五域规格，并授权按审阅结论修改（“你自己帮我改吧”）。审阅抽查发现：多个消费域没有接收事件的路径；部分生产域发件箱无转发；签名与函数ID各域不统一；约一半依赖引用无法机器校验。
- 已修订：事件统一经jobs拉取投递（订阅循环、handler、outbox转发、到期重投/死信）；Go与浏览器函数签名统一；函数ID统一；依赖规范化并新增resources；函数分约束/参考两级；新增scripts/render_docs.py与make docs生成functions.md/files.md；check_docs.py新增对应校验并在改动前的合同上验证能报出原问题；新增ADR0002，更新根/模块AGENTS、文档标准、结构契约、手册、协议、计划与状态。
- 未做：备份恢复简化仅登记为R0-07待确认；未创建业务代码、测试或样例；未调用真实模型；本地make check通过，未在VPS复跑；已本地提交，未push。合著者为Claude Opus 5.5。

## 2026-10-14：恢复改为暂存后重启换目录（ADR0003）

- 用户确认R0-07“按你说的改”。为保持old产品“初始化向导/后台从备份恢复”的界面入口，没有采用需要登录服务器的离线命令，改为各域暂存→切换决定→进程守护重启时换目录。
- 合同：删除14个有状态域的snapshots:validate/prepare/commit/rollback、identity恢复许可签发/撤销、各域许可校验函数与RecoveryAuthorization类型；新增各域snapshots:stage/discard，data-protection的serveRestoreArchive、cleanupRestore与凭进度令牌读取的readRestoreProgress；site-config运行阶段与回执状态改为staging/switching/rolling_back与switched/rolled-back；启动与Serve函数增加换目录、登记回执和切换时安全退出步骤。
- 文档：重写一致性、架构、协议、运维、启动蓝图与各域相关段落；api.md接口清单改由make docs从OpenAPI生成（标记外手写说明保留）。本地make check通过，未在VPS复跑；未创建业务代码或测试；已本地提交，未push。合著者为Claude Opus 5.5。
