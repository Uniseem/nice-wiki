> 产品需求基线快照：来源 old:docs/adr/0024-video.md，blob 6d30d3dd983deb1a1e33ac829ed358ea54cce28f。保留功能与细节；旧路径、数据布局和代码组织属于原架构，不是新实现结构。后续已确认ADR优先。

# D24. 视频：支持上传和平台嵌入

- 状态：✅ 已定
- 日期：2026-10-12

- 支持上传视频文件，也支持嵌入 Bilibili、YouTube、抖音、TikTok 的视频，统一做成一个视频组件。
- 平台视频：根据公开可获取的信息（标题、简介、作者等）生成描述，供 AI 检索；点击后才加载平台的播放器；封面图下载到本地。

详见 [design/editor.md](https://github.com/Uniseem/nice-wiki/blob/b8190c7/docs/design/editor.md)。
