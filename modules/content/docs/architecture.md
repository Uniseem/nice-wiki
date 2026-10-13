# 架构与依赖

保持 D4/D9/D15/D16/D20/D35/D54/D55：Tiptap3块、共享自动保存草稿、内存编辑锁、发布撤回回收站和署名历史；私有正文文件权威。Go 服务使用独立进程、独立私有存储根与数据库；Vue3 交互由 portal 装配。服务之间只经已登记 API/事件/对象读取接口交换数据，不能共享可写目录。

保持old editor.md D4/D9/D51：Tiptap3/Vue3，Docmost社区扩展已授权借用，注明原版权并由根THIRD_PARTY_NOTICES登记；React节点view改Vue，Halo只参考。画板Vue懒包React Excalidraw，公开不加载React。单active content写leader使内存锁语义成立；读规模由public-site派生实现。

| 依赖 | 方式 | 作用 | 失败政策 |
|---|---|---|---|
| identity | sync | 全站功能权限与即时撤销，唯一超管强释/purge | 未知权限关闭写入 |
| site-config | sync | 基准语言/限制和runtime epoch | 新写关闭，无AI不影响基础功能 |
| media | sync | 不可变资源ready/当前历史引用/pin | 未知当前引用禁止提交，已手清历史缺失明确提示 |
| model-gateway | optional | content.slug/content.seo首发元数据生成 | 人工优先；无渠道提示人工补slug/SEO，不假模型fallback |
| jobs | event | 发布render/index/translation/git镜像和审计 | 原子outbox保留，公开权威立即更新 |

每个内部调用同时验证 callerService、audience、actor 委托、siteId、期限和 dataEpoch；服务身份不自动绕过业务权限。所有事件采用 at-least-once、来源版本单调和 eventId 去重，禁止用事件延迟窗口替代撤回/权限的同步权威校验。
