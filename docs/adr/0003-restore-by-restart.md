# ADR0003：恢复改为暂存后重启换目录

- 日期：2026-10-14。
- 状态：用户确认（plan R0-07），已写入合同；业务实现尚未开始。
- 来源：审阅指出在线分阶段恢复是全设计中最复杂、最易出错的部分；全部服务部署在同一台主机，不需要在服务运行中切换数据。

## 问题

原设计让 14 个有状态域在运行中执行 validate→prepare→commit→activate（失败时 rollback），由 data-protection 持久化提交决定协调。为了让 identity 换代后仍能完成提交，又引入专用 RecoveryAuthorization、各域本地信任锚与限定控制分支。每个域都要实现 4 个恢复接口和许可校验，跨服务失败组合很多，难以实现和验证。

旧产品的恢复入口是初始化向导里的“从备份恢复”，只需超管在界面操作。新方案不能要求登录服务器执行命令，否则会退化产品功能。

## 选择

1. 每域数据根分为三部分：`active/`（业务数据，进入快照、恢复时整体替换）、`control/`（屏障、恢复日志、identity 服务签名密钥，永不替换）、`restore/<restoreId>/staged|before/`（暂存数据与换出的旧数据）。
2. 备份流程不变：维护→屏障 safe→各域 `snapshots:export`→封存→解除。
3. 恢复（入口仍是初始化向导和后台恢复页）：
   1. data-protection 校验备份，进入 `restore_staging` 维护，各域屏障 safe。
   2. 各域 `snapshots:stage` 经 `data-protection:serveRestoreArchive` 拉取自己的归档，校验后写入 `restore/<restoreId>/staged`，不动 `active`。任一失败则 `snapshots:discard` 丢弃暂存并退出维护，数据不变。
   3. 全部暂存后，data-protection 在 control 区持久写切换决定，site-config 进入 `restore_switching` 并记录目标 epoch。
   4. 各服务轮询到切换阶段后安全退出（退出码 75），由进程守护重启；启动时把 `active` 改名为 `before`、`staged` 改名为 `active`，丢弃旧代 outbox、租约与缓存，运行 schema 迁移，再向 site-config 登记 `switched` 回执。identity 换入后撤销全部会话。
   5. 全部回执与就绪后，site-config 切到目标 epoch 并解除维护。超时或任一域启动失败则保持维护，管理员可继续等待，或整体换回：进入 `restore_rolling_back`，各服务重启时把 `before` 换回 `active`。
   6. 完成或换回后，管理员可经 `cleanupRestore` 删除各域保留的目录。
4. 删除：各域 `snapshots:validate/prepare/commit/rollback` 共 56 个接口及处理函数；RecoveryAuthorization 类型、identity 签发/撤销接口、各域许可校验函数与 `x-recovery-authorization` 标记；data-protection 的单域激活。data-protection 的恢复编排改为暂存、切换、读取进度三步。
5. 新增：各域 `snapshots:stage/discard`；data-protection `serveRestoreArchive`、`cleanupRestore`，以及凭只读进度令牌查看的 `readRestoreProgress`（恢复期间管理员会话会失效）。启动函数增加换目录与回执步骤，Serve 增加“切换时安全退出”步骤。

## 代价与影响

- 部署必须配置进程守护自动重启（Docker `restart: unless-stopped` 或 systemd `Restart=always`）；没有守护时服务退出后不会回来，恢复会停在维护中。
- 恢复期间站点全部处于维护，包括公开阅读；时长取决于各域重启与迁移。
- 每次恢复会在各域保留一份 `before`，直到管理员清理，需要额外磁盘空间。
- 只适用于各服务能各自重启的部署；以后多机部署仍可沿用，因为换目录只发生在每个服务自己的数据根。
- 业务数据的跨代保护（dataEpoch 比较、旧代 outbox 丢弃）保留，用于防止旧任务或旧事件写入新数据。
