# data-protection 需求

**M15-R001** 在全域维护安全点创建一致备份、校验并恢复统一代次；把各域已有历史投影成 data git 版本镜像并管理远程同步。

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M15-R002** 全站备份manifest、校验报告、恢复暂存与切换决策日志及恢复进度

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M15-R003** 加密备份对象、保留策略、下载授权

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M15-R004** data git确定性版本镜像与受配置远程同步

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M15-R005** 不成为content/localization/memory等源历史权威

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M15-R006** 不读取共享数据库或其他服务私有磁盘；仅HTTP导出/暂存协议

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M15-R007** 密钥/凭证/日志/提问正文/活动租约不进git镜像；本阶段不push远程

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M15-R008** 服务间HTTP/JSON/SSE；事件使用公共EventEnvelope并标版本，至少一次按eventId和源修订去重

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M15-R009** 所有写入验证当前dataEpoch，恢复后旧任务/租约/token/缓存不得修改或授权新数据

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M15-R010** 私有存储；外域只通过拥有者HTTP合同读写，不共享数据库或直接读私有文件

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M15-R011** 所有有状态域实现统一barrier/export/stage/discard；恢复时重启换入暂存数据，全部域登记switched回执后才解除维护

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M15-R012** 日志、事件与镜像不记录IP、提问正文或secret；结构化字段仅allowlist

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M15-R013** 未来Go后端、Vue3模块界面按文件/函数合同手写；本阶段只登记设计，无业务源码和测试样例

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M15-R014** 任一必需域停机：备份/恢复不进入ready/completed；maintenance保持失败状态等待明确continue、切换前abort或切换后整体换回。

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M15-R015** 密钥provider停机：不创建不可解密ready包，不泄漏密钥；已有密文保存。解密校验失败阻止暂存。

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M15-R016** site-config停机：不能打开/切代/退出maintenance；各服务readiness fail closed，不假设缓存epoch可继续写。

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M15-R017** data-protection停机：已开始maintenance由runtime持久维持；各域不自己解除。重启根据control journal续接。

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

**M15-R018** 远程Git停机或分叉：backup ready保持，mirror标failed/conflict；无force push、无真实push在本阶段。

验收观察：按acceptance.md观察该行为与对应故障分支；本阶段未实现、未运行业务验收

首版默认参数：

```json
{
  "barrierDeadlineSeconds": 360,
  "domainCallDeadlineMs": 5000,
  "parallelSnapshotExports": 3,
  "parallelImportValidations": 3,
  "backupRetentionCount": 20,
  "backupRetentionDays": 0,
  "archiveCipher": "AES-256-GCM 每对象独立随机96-bit nonce，keyId外部KEK封装DEK；无密钥明文入manifest",
  "hashAlgorithm": "SHA-256",
  "restoreRequiresAllStatefulDomains": true,
  "mirrorMaxTextFileBytes": 10485760,
  "gitSyncTimeoutSeconds": 60,
  "gitAutoPushDefault": false,
  "gitRemoteDefault": "unset",
  "mirrorPushPolicy": "仅显式已配置远程+backup.manage授权触发；禁止force push，分叉返回冲突",
  "recoverAfterCommit": "continue-activation-only；不得回滚已提交一部分域",
  "localBackupSchedule": "每日02:00站点IANA时区，保留最近N份，默认N=20",
  "includeSecretsDefault": false,
  "includeSecretsAllowed": "超管明确勾选完整备份可includeSecrets=true；secret通过拥有者加密export，秘钥不进git/日志",
  "restoreEntry": "初始化向导可从备份恢复；不要求AI配置",
  "gitImplementation": "go-git，未来不依赖系统git",
  "gitAllowedPaths": [
    "docs/",
    "i18n/",
    "glossary/",
    "site-settings/",
    "redirects.json"
  ],
  "gitExcluded": [
    "数据库",
    "drafts",
    "assets",
    "config秘密",
    "账号",
    "token",
    "IP",
    "活动lease"
  ],
  "gitCommitAuthor": "原发布账号，源发布完成时间；每次发布一次版本镜像",
  "restoreApproval": "已有superadmin恢复权限；初始化向导使用首次setup受控授权，不新增多重人工批准",
  "portableBackup": "下载时源服务器透明解密at-rest对象并输出完整ZIP；本地文件可在源服务/key失效后于另一实例恢复，不新增密码/客户端解密/二次审批",
  "archiveUpload": "初始化/后台既有恢复入口上传便携ZIP，校验checksum/schema/zip路径和全域refs"
}
```

精确参数约束以types.json为准；不从客户端接受成本分类、可信actor、IP或secret。
