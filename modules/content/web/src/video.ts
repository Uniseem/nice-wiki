import { Node } from '@tiptap/core';

// 视频块（上传来源）：只保存不可变资源引用；服务端不转码，编辑器里用同源地址预览。
export const VideoBlock = Node.create({
  name: 'video',
  group: 'block',
  atom: true,
  draggable: true,
  addAttributes() {
    return {
      blockId: { default: null, rendered: false },
      source: { default: null, rendered: false },
      title: { default: '', rendered: false },
      description: { default: '', rendered: false },
      poster: { default: null, rendered: false },
      widthPercent: { default: null, rendered: false },
    };
  },
  parseHTML() { return [{ tag: 'video[data-video-block]' }]; },
  renderHTML({ node }) {
    const asset = node.attrs.source?.asset as { assetId: string } | undefined;
    return ['video', {
      'data-video-block': '', controls: '', preload: 'metadata', src: asset ? `/v1/assets/${asset.assetId}/bytes` : '',
      style: `max-width:${node.attrs.widthPercent ?? 100}%;height:auto`,
    }];
  },
});
