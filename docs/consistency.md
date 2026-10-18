# 数据权威与一致性

## 文档发布

正文不可变修订文件是权威；索引、head、历史和待发事件由同域恢复日志协调。发布顺序为校验权限/租约/版本→准备不可变修订与资源引用预留→持久提交记录→写正文并校验哈希→提交head/历史/事件状态→确认资源引用→标完成→投递。任何中断由journal状态恢复；不能宣称单个SQL事务原子提交了文件系统。

读取只看完成修订。发布API返回正文修订与派生任务状态，索引或翻译晚完成不伪装为正文发布失败，也不自动回滚已完成正文。撤回先权威visibilityRevision递增，再发失效事件；公开HTML、问答缓存与索引引用须查当前权威门禁，依赖故障拒绝输出旧内容。

## 人工与派生结果

译文写回同时比较源修订、人工generation、术语/记忆版本和目标任务代次。描述/alt提议交content按预期修订应用，不直接改其文件。自动流程不得覆盖人工标志，旧任务必须拒写。派生收费结果以内容哈希和能力/模型版本保存；索引可重建不等于paid description和embedding可丢弃。

## 租约与资源

编辑、并发准入、任务领取各有独立租约和fencingToken；心跳不能复活已被接管的租约。media在删除前使用权威引用确认和保护窗口，未知状态保留；上传、引用预留、确认、释放都有可恢复operation，避免迟到引用误删。

## 费用

model-gateway是唯一费用和费用额度权威；调用前原子预留所有相关作用域，真实/预估/未决费用分别记录；未知实际值保留保守预留并进入对账。task重试和供应方fallback共享根操作上限，不能组合成无限调用。analytics显示统计水位，不能用延迟报表控制实时额度。

## 备份恢复

site-config的dataEpoch和maintenanceRevision是权威。data-protection持久记录恢复决策，逐域安全屏障停止新写并等待在途调用到可结算安全点。所有域export校验和后才封存完整清单；任一失败不标完整成功。

恢复按[ADR0003](adr/0003-restore-by-restart.md)执行：先校验备份，再让每个域把归档暂存到自己的restore/<restoreId>/staged（不动active）；全部暂存后持久写切换决定，并在site-config记录目标epoch；各服务安全退出、由进程守护重启，启动时把active换成暂存数据并登记回执，全部回执齐后解除维护。任一域未确认则保持维护，可重试或整体换回（启动时把before换回active），不提供混代读写。旧队列、令牌、缓存和租约随代次失效；免费索引可重建，付费派生结果从备份恢复，绝不自动重新调用模型。

每域数据根分为active（业务数据，进入快照、恢复时整体替换）、control（屏障、恢复日志、identity服务签名密钥，永不替换）与restore/<restoreId>/staged|before（暂存与换出的旧数据）。恢复不签发专用许可：stage/discard只接受data-protection服务身份携带的superadmin委托，换目录发生在进程启动时、不需要跨服务授权。identity换入后撤销全部会话，管理员用备份中的账号重新登录；恢复进度凭只读进度令牌查看。部署必须配置进程守护自动重启。

data git只是允许数据的版本镜像，源历史在各域持久保存；镜像延迟显式可见，镜像不含账号/配置密钥/数据库/资源，不能代替完整备份。

任务领取、完成和失败写回同时绑定owner、worker、持久fence与dataEpoch；队列持久保留rootOperationId，worker从所属域权威任务读取原主体、预算和期限，不能用启动上下文或重试生成新付费根。执行结果携完整可空ObjectRef与paidResultRefs，取消同样保留已付费结果。过期执行权先隔离为orphan并提升fence，只有owner持久确认completed或safeToRetry后才能复用结果或重排；未知结果不能凭进程健康重试。

取消意图以持久cancel_requested为准，失败、重排、领取、心跳均保留；只有owner明确确认安全取消才终局cancelled。已取消任务的不明确失败进入orphan，保留已付费结果并由owner核实，不自动重跑。owner已落盘成功与取消竞争时保留成功结果。

恢复隔离保留原输入、原付费结果、原来源与未决费用证据；撤销旧epoch job/fence/event/outbox的活动资格，不做通用epoch改写或重新派发。拥有者必须按已验证来源登记只读投影/新代免费通知，未知跨域闭包保持Ready失败关闭。目标schema在staged迁移并形成完整目标树指纹，切换只接受同一restore/barrier/target/stage证明。
