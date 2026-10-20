import { Node } from '@tiptap/core';

// 图片块：只保存不可变资源引用，字节由 media 提供；编辑器里用同源地址预览。
export const ImageBlock = Node.create({
  name: 'image',
  group: 'block',
  atom: true,
  draggable: true,
  addAttributes() {
    return {
      blockId: { default: null, rendered: false },
      asset: { default: null, rendered: false },
      alt: { default: null },
      widthPercent: { default: null, rendered: false },
    };
  },
  parseHTML() { return [{ tag: 'img[data-image-block]' }]; },
  renderHTML({ node }) {
    const asset = node.attrs.asset as { assetId: string } | null;
    return ['img', {
      'data-image-block': '', src: asset ? `/v1/assets/${asset.assetId}/bytes` : '', alt: node.attrs.alt ?? '',
      style: `max-width:${node.attrs.widthPercent ?? 100}%;height:auto`,
    }];
  },
});
