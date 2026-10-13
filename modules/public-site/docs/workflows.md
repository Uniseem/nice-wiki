# public-site 流程

公开页面：Tiptap JSON权威正文→发布事件预渲染；每次请求仍POST content/publications:validate→同修订cache或JSON渲染→最终权威校验→no-cache+etag的200/304；原文撤回立刻门禁拒输出；已完成最终校验开始发送的有限响应无法召回。

公开派生清单：content 返回修订快照→生成导航/sitemap/llms→最终批量状态校验→发送；修订变化重新生成一次，否则 503。

预览：后台已签入身份→content明确草稿修订grant→本人每次content重验→公开样式noindex/no-store HTML；有效期内刷新可重复，token不进入快照/git/访问日志。

恢复管理校验：维护前由identity为每域签发RecoveryAuthorization；本域控制记录/屏障/信任锚独立于业务导入。维护中的prepare/commit/rollback按本地签名、audience、restore/barrier、两epoch、到期和决策状态校验，不回查已换代身份业务DB；普通业务不接受此许可；全域完成后撤销。

来源：old docs/design/{site,home,i18n,editor,security-ops,admin}.md；ADR D12/13/14/16/17/19/20/23/26/28/29/31/32/34/54/55。后续ADR优先；产品细节保留，存储/事务重新按微服务划分。

状态迁移：

缓存 absent→building→ready→invalidated；building 不阻止其他修订的构建

公开输出要求 content authority 判定 published 且请求 publicationRevision 等于权威修订

预览grant由content拥有：active→expired/revoked；绑定本人身份/草稿修订/epoch，在有效期内重复预览；每次content校验权限，不能匿名分享

并发和幂等：每个修改函数明确CAS、fencing/租约或事务边界，见functions；公共事件至少一次，不能依赖全局顺序。接收eventId相同但hash不同拒CONFLICT；较低sourceRevision记已见但不应用；旧dataEpoch拒绝并不能重绑新epoch。

持久事件由M06-F060消费content.publication.changed/content.publication.withdrawn/translation.ready，按inbox与单调revision失效/预热。配置与启用语言每请求读当前权威值；事件只是投影优化，最终发布门禁不能省略。
