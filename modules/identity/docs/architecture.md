# 架构与依赖

保持 D21/D35 账号产品：唯一超管、账号名口令登录、10项功能授权、初始密码强改与服务器恢复；独立私有身份服务。Go 服务使用独立进程、独立私有存储根与数据库；Vue3 交互由 portal 装配。服务之间只经已登记 API/事件/对象读取接口交换数据，不能共享可写目录。

严格保持old D21/D35功能；删除独立初稿中的email登录、邀请、多owner、继承角色、MFA和邮件恢复。新架构只改变私有存储/服务边界；功能性安全为Origin+SameSite=Strict、argon2id、账号/IP失败临时锁定。

| 依赖 | 方式 | 作用 | 失败政策 |
|---|---|---|---|
| site-config | sync | 初始化向导状态与runtime epoch | 写入关闭，注册不依赖AI |
| traffic-guard | sync | 账号/IP登录失败保护 | 敏感匿名请求受控拒绝 |
| jobs | event | 权限/登录审计事件 | 私有outbox保留，授权仍同步权威检查 |

每个内部调用同时验证 callerService、audience、actor 委托、siteId、期限和 dataEpoch；服务身份不自动绕过业务权限。所有事件采用 at-least-once、来源版本单调和 eventId 去重，禁止用事件延迟窗口替代撤回/权限的同步权威校验。
