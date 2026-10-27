# 未执行的业务验收规格

本阶段分批实现基础函数；以下业务验收均未执行，不标记通过。这里描述未来验收行为，不授权写测试、样例或调用真实接口/模型。

- 未执行 M05-R01：静图原图+多宽WebP，动图不压缩；视频MP4/WebM无服务转码，浏览器截封面/自传。
- 未执行 M05-R02：每文件SHA不可变；中断可续offset；Range/If-Range/HEAD/416支持iOS，不信任路径/MIME。
- 未执行 M05-R03：Bili/YouTube/抖音/TikTok统一provider；插入一次抓公开metadata，失败手填；点击才加载白名单embed。
- 未执行 M05-R04：所有图片附件视频画板显示文档引用；无当前引用单列，历史风险明确确认，ready文件绝不自动删除。
- 未执行 M05-R05：unknown source/pendingpin/新generation/readers阻止finaldelete；同fence保护新引用，事件重复乱序不回退。
- 未执行 M05-R06：scene JSON+亮暗SVG各hash保存，旧历史保留；SVG CSP sandbox，public不React。
- 未执行 M05-R07：blobjournal/hash完整；snapshot与control区分离，确认人工delete决定才恢复补删。

- 未执行：输入未知字段、schema 升级/不支持版本与完整性破坏时拒绝/隔离行为。
