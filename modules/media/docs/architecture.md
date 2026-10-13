# 架构与依赖

保持 D24/D26 与资源库产品：内容寻址上传/Range、浏览器WebP/视频封面、四平台公开信息快照、本地封面、Excalidraw三文件，文件不自动删除。Go 服务使用独立进程、独立私有存储根与数据库；Vue3 交互由 portal 装配。服务之间只经已登记 API/事件/对象读取接口交换数据，不能共享可写目录。

原D26浏览器browser-image-compression多尺寸，D24上传视频无ffmpeg服务转码；platform provider统一recognize/buildEmbed/fetchPublic接口，4平台能力adapter保持。媒体服务只签名/MIME/元数据验证，不处理图像重编码/视频帧；封面在浏览器截或公开图下载。本轮不调用真实平台接口，各provider实际端点能力在未来实现逐个核实。

| 依赖 | 方式 | 作用 | 失败政策 |
|---|---|---|---|
| identity | sync | 编辑上传/资源库功能权限与预览账号 | 授权未知关闭 |
| site-config | sync | 配额/runtime与首页品牌引用快照 | 引用未知阻止清理 |
| content | sync | 权威当前/历史refs、publication/preview | 未知refs不当0，公开manifest当前可见性失败关闭 |
| jobs | event | 资源状态和人工批准后延迟清理worker | outbox保留，绝不自动产生清理授权 |

每个内部调用同时验证 callerService、audience、actor 委托、siteId、期限和 dataEpoch；服务身份不自动绕过业务权限。所有事件采用 at-least-once、来源版本单调和 eventId 去重，禁止用事件延迟窗口替代撤回/权限的同步权威校验。
