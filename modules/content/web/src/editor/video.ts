import { mergeAttributes, Node } from '@tiptap/core';
import { jsonAttr } from './blocks';

// 视频块（上传来源）：只保存不可变资源引用；服务端不转码，编辑器里用同源地址预览。
export const VideoBlock = Node.create({
  name: 'video',
  group: 'block',
  atom: true,
  draggable: true,
  addAttributes() {
    return {
      blockId: { default: null, rendered: false },
      source: jsonAttr('source'),
      title: jsonAttr('title', ''),
      description: jsonAttr('description', ''),
      poster: jsonAttr('poster'),
      widthPercent: jsonAttr('widthPercent'),
    };
  },
  parseHTML() { return [{ tag: 'video[data-video-block]' }]; },
  renderHTML({ node, HTMLAttributes }) {
    const asset = node.attrs.source?.asset as { assetId: string; sha256: string } | undefined;
    const poster = node.attrs.poster as { assetId: string; sha256: string } | null;
    return ['video', mergeAttributes(HTMLAttributes, {
      ...(poster ? { poster: `/v1/assets/${poster.assetId}/bytes?sha=${poster.sha256}` } : {}),
      'data-video-block': '', controls: '', preload: 'metadata', src: asset ? `/v1/assets/${asset.assetId}/bytes?sha=${asset.sha256}` : '',
      style: `max-width:${node.attrs.widthPercent ?? 100}%;height:auto`,
    })];
  },
});
