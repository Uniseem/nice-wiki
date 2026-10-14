# 需求与边界

保持 D24/D26 与资源库产品：内容寻址上传/Range、浏览器WebP/视频封面、四平台公开信息快照、本地封面、Excalidraw三文件，文件不自动删除。

| ID | 行为要求 | 验收判定 |
|---|---|---|
| M05-R01 | 浏览器派生图/视频处理保持old | 静图原图+多宽WebP，动图不压缩；视频MP4/WebM无服务转码，浏览器截封面/自传 |
| M05-R02 | 完整上传/内容寻址/Range | 每文件SHA不可变；中断可续offset；Range/If-Range/HEAD/416支持iOS，不信任路径/MIME |
| M05-R03 | 四平台一次公开快照和本地封面 | Bili/YouTube/抖音/TikTok统一provider；插入一次抓公开metadata，失败手填；点击才加载白名单embed |
| M05-R04 | 资源库与纯手动清理 | 所有图片附件视频画板显示文档引用；无当前引用单列，历史风险明确确认，ready文件绝不自动删除 |
| M05-R05 | 迟到引用与删除竞态保护 | unknown source/pendingpin/新generation/readers阻止finaldelete；同fence保护新引用，事件重复乱序不回退 |
| M05-R06 | Excalidraw三文件/SVG安全 | scene JSON+亮暗SVG各hash保存，旧历史保留；SVG CSP sandbox，public不React |
| M05-R07 | 私有存储与恢复 | blobjournal/hash完整；snapshot与control区分离，确认人工delete决定才恢复补删 |

每项当前状态均为未实现、未执行验收。


来源：[产品保持矩阵](../../../docs/product-preservation.md)、[冻结产品基线](../../../docs/product-baseline/README.md)；具体产品来源与ADR在README来源索引和本域operations列出。所有需求状态未执行，不能以合同数量视为完成业务。
