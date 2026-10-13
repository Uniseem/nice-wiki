# 架构与依赖

入口、后台外壳与模块装配；偏好由 identity 拥有，portal 无业务数据仓库。Go 服务使用独立进程、独立私有存储根与数据库；Vue3 交互由 portal 装配。服务之间只经已登记 API/事件/对象读取接口交换数据，不能共享可写目录。

后台导航严格按10功能组显示，首次注册→站点/基准语言两步初始化（D55），子账号初始口令只能强改页。站点设置hero/banner/branding/footer与AI模型独立模块分组；无AI仍编辑/预览/人工元数据发布。

| 依赖 | 方式 | 作用 | 失败政策 |
|---|---|---|---|
| identity | sync | 身份、CSRF、授权与自身偏好 | 管理请求关闭；匿名公开交付按 public-site 自身策略 |
| site-config | sync | bootstrap 与唯一 runtimeState | 写入关闭，读取显示受控维护 |
| jobs | event | 后台事件与访问元数据 | SSE 降级；不可影响 owning-service 审计提交 |

每个内部调用同时验证 callerService、audience、actor 委托、siteId、期限和 dataEpoch；服务身份不自动绕过业务权限。所有事件采用 at-least-once、来源版本单调和 eventId 去重，禁止用事件延迟窗口替代撤回/权限的同步权威校验。
