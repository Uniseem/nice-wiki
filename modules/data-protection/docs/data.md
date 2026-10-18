# data-protection 数据与持久化

未来私有目录 `data/data-protection/active/` 保存SQLite和领域对象；`data/data-protection/restore/<restoreId>/staged|before/` 为恢复暂存与换出的旧数据；`data/data-protection/control/` 保存不随业务恢复替换的barrier、恢复回执和操作journal。外域不得直接读这些路径。

- `backup_runs(backup_id PK, source_epoch, barrier_id, status, manifest_hash, encrypted_manifest_ref, key_id, created_at, completed_at)`

- `backup_objects(backup_id, owner, snapshot_id, schema_version, watermark, object_hash, encrypted_ref; PK backup_id/owner)`

- `restore_runs(restore_id PK, backup_id, source_epoch, target_epoch, barrier_id, status, runtime_revision, committed_at)`

- `restore_decisions(decision_id PK, restore_id UNIQUE, target_epoch, decision(switch/abort/rollback), staged_manifest_hash, decided_at)`，位于control区，不随恢复替换

- `domain_stages(restore_id, owner, snapshot_id, snapshot_hash, state(staged/switched/rolled-back/discarded), last_error_code; PK restore_id/owner)`

- `mirror_runs(mirror_id PK, backup_id, manifest_hash, local_commit, remote_ref, state)`

- `remote_config(config_id PK, url_without_credentials, branch, credential_ref, revision)`

- `outbox(event_id PK, payload_ref, state, retry_at)`

SQLite WAL、foreign_keys=ON、busy_timeout=3000ms；同一服务使用串行短写事务，读使用快照。正文字节或导出文件与DB关联均先临时写+fsync+rename再索引提交，journal记录prepare/object-installed/index-committed/complete；启动按journal核hash补齐索引或清未提交对象，不能宣称一个SQLite事务覆盖文件。

修订单调；代次opaque Identifier只比较相等；唯一约束绑定epoch，旧epoch索引/lease/token禁止复活。金额只出现analytics派生投影，按USD整数micros处理。

快照只经HTTP管理协议（barrier/export/stage/discard），在safe屏障固定revisionWatermark；恢复先全域暂存，写切换决定后各服务重启换入，全部域登记回执才解维护。control区不随恢复替换。换入后先schema迁移与完整性验证，失败拒绝启动并可整体换回。

R1实现收口：active/protection.db保存安全设置与镜像；control/protection-control.db保存备份及对象权威、vault_meta、恢复运行、决定、域回执、屏障及journal，不参与业务恢复。各库user_version=2，未知更高版本拒绝启动。实际迁移0001_domain.sql与0002_control.sql在SQLite短事务应用；平台inbox/outbox直接EnsureSchema，不复制结构。对象只允许本域active/incoming、vault及restore隔离根，不能读取其他域私有路径。

目录分页由created_at/backup_id倒序keyset和目录revision绑定，平台签名游标固定稳定身份/会话/权限修订/epoch/limit，10分钟过期；水位变化拒绝混页。未配置镜像或不存在备份返回NOT_FOUND，不用空成功伪造历史。

分片私有密文格式见[crypto-format.md](crypto-format.md)与CryptoMetaV1/CryptoHeaderV1；配置MasterKeyRef只接受32字节私有KEK，启动keyring按指纹绑定路径，不从恢复输入推断路径。stored_objects只保存密文安全元数据，wrappedDEK仅在私有header。控制journal保存原输入摘要与真实完整引用/解密源摘要；备份历史/已付费对象不因失败清掉，未认证明文temp不得交付。源码只实现协议，不执行真实备份或恢复。

设置读取使用独立安全DTO：schedule/remote均兼容可选，未配置即省略。schedule只给实际保存的timezone、keepCount、includeSecrets、scheduleId及revision；remote只给无凭据URL、branch及CAS修订。不返回原批准AuthContext或credentialRef，不使用当前读者伪造原策略批准事实。

BackupManifestV1采用轻量设计：14份真实DomainSnapshotManifest的hash/来源SnapshotRef、同backupId/barrierId/sourceEpoch及完整来源绑定引用；原域manifest留在原tar。全局top JSON及对象/发布/引用索引≤2MiB，域manifest≤16MiB。ValidateCrossDomainRefs另接完整认证的真实域清单，以所有精确ref字段闭包验证；快照存在不等于对象存在。发布权威只取content，历史ref的epoch/schema等必须真实完全对应，不补造别名。keyId是创建来源指纹，便携导入不依赖源KEK，本机vault的实际keyId另存私有backup_runs/header。

## V2独立vault

密文在dataRoot/vault，不在active/control/restore内。0003建立control备份目录、对象及vault_meta；0004仅在旧active三张表确认为空后移除旧表。旧objects非空、旧控制记录或vault未登记文件均维护拒绝，不自动搬迁、遗漏或删除。完整密文与目录fsync后，真实索引及原journal完成状态同一control事务持久。游标绑定vault_meta revision。

backup_runs持久approved_by/approval_permission_revision/approval_kind/site_id/root_operation_id/input_hash/include_secrets/source_epoch。HTTP普通备份核backup.manage，secret另核superadmin；worker只在固定批准范围和当前epoch/jobs fence执行，不保存复用过期portal proof。

自身快照只导出安全设置与历史目录/完成审计投影，不打包历史vault实物或恢复controller授权。历史密文是明确external-vault审计metadata，追溯backupId/cipher SHA/bytes/keyId，不伪装活跃业务ObjectRef。恢复时真实核本机vault；跨实例缺旧包应failed+安全issue，不捏造ready。本次业务及付费结果由真正拥有域的完整快照保存。

远程配置实施：可信AllowedGitHosts精确匹配ASCII主机，仅HTTPS443/SSH22及refs/heads完整安全分支；HTTP credentialRef仅选部署GitCredentialRefs的ID，空表示无凭据，不读请求路径。配置按expectedRevision CAS，原操作摘要/结果同事务幂等；修改配置不触发网络或同步。

control V3追加backup_jobs：canonical不可变{backupId}与其真实ObjectRef、原subject/operation/root/site/epoch/inputHash和jobs原回执。原批准与requested状态/outbox同事务持久，再以清portal proof的本域服务身份单次提交jobs；失败保留原意图供相同操作重试，不自建领取/重试队列。requested事件不含尚不存在的manifestHash；只有真实封存ready才必填并显式核验，不依赖生成器单独支持if/then。

ValidateBackupRequest只核管理员/secret额外超管、当前running同epoch、真实本机KEK及隔离目录，valid是基础条件通过，capacityVerified始终false；取得14域完整真实manifest后再按staging/密文/便携总预算与free bytes核容量，并在有界写前维持配额。
