# 依赖与失效传播

| 调用方 | 直接业务依赖 | 异步输入/输出 |
|---|---|---|
| portal | identity、site-config、被访问模块 | 能力/维护状态通知 |
| identity | 自己的事务存储 | 权限/会话撤销、审计 |
| site-config | identity、media引用管理 | 站点/维护/代次事件 |
| content | media引用管理；可选model-gateway | 发布/撤回/资源引用/历史快照 |
| media | content/site-config权威引用确认 | 引用事件及可用资源 |
| public-site | content权威可见性、site-config；预览授权 | 内容、译文、语言、配置投影 |
| model-gateway | 供应方；本地准入签名验证 | 费用与能力事件 |
| knowledge | content、media、memory、model-gateway | 发布/撤回/译文，索引结果 |
| qa | traffic-guard、content、memory、knowledge、model-gateway；可选localization | 版本失效、反馈与用量 |
| localization | content、memory、model-gateway | 发布/设置、批准术语命令，译文 |
| memory | content、model-gateway | 记忆变化、批准术语命令 |
| traffic-guard | 私有状态与验证供应方 | 行为/封禁与前置风险信号 |
| jobs | 注册执行方领取/上报 | 持久事件和执行记录 |
| analytics | 私有汇总存储 | 访问、审计、费用、任务事件 |
| data-protection | 各域管理快照、site-config、jobs、远程git | 不可变版本镜像 |

身份与管理操作授权是共同依赖；数据保护流程属于控制流程，不插入普通读取链。memory对术语采用批准命令加终态回执，避免localization→memory→localization同步循环。索引结果不触发另一轮发布；元数据提议由content按源修订CAS应用，避免描述更新无限入库。

独立部署不表示没有运行依赖。依赖故障必须按本模块failurePolicy报告；付费链默认拒绝缺准入/额度检查，公开读取不能因门禁失败继续返回缓存。来源投影更新失败可保留旧投影，但公开可见性仍查权威。
