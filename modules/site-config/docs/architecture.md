# 架构与依赖

保持 D17/D34/D55 的全屏首页、公告横幅、站点品牌页脚与无AI初始化；dataEpoch/maintenance唯一控制权威。Go 服务使用独立进程、独立私有存储根与数据库；Vue3 交互由 portal 装配。服务之间只经已登记 API/事件/对象读取接口交换数据，不能共享可写目录。

首超管在identity注册先完成（D21，无初始化码），随后site-config站点名/语言两步（D55）。不创建owner reservation，不收密码。配置文件和控制状态分离，服务之间只API/对象读取/事件。

| 依赖 | 方式 | 作用 | 失败政策 |
|---|---|---|---|
| identity | sync | 首注册已生成唯一superadmin/siteId；权限验证 | 未注册不能初始化，写入关闭 |
| content | sync | 首页文档当前发布引用及可见性 | 未知/撤回引用不公开 |
| media | sync | 首页ready资源与引用pin | 引用不确定禁止写入/清理 |
| jobs | event | 公告计划与配置事件 | 保留outbox；时间窗和撤回仍由权威同步判断 |

每个内部调用同时验证 callerService、audience、actor 委托、siteId、期限和 dataEpoch；服务身份不自动绕过业务权限。所有事件采用 at-least-once、来源版本单调和 eventId 去重，禁止用事件延迟窗口替代撤回/权限的同步权威校验。
