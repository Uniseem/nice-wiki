import { mergeAttributes, Node } from '@tiptap/core';
import { jsonAttr } from './blocks';

// 图片块：只保存不可变资源引用，字节由 media 提供；编辑器里用同源地址预览。
// 图注和替代文字来源暂时不能在编辑器里改，原样保留。
export const ImageBlock = Node.create({
  name: 'image',
  group: 'block',
  atom: true,
  draggable: true,
  addAttributes() {
    return {
      blockId: { default: null, rendered: false },
      asset: jsonAttr('asset'),
      alt: { default: null },
      altSource: jsonAttr('altSource'),
      caption: jsonAttr('caption', []),
      widthPercent: jsonAttr('widthPercent'),
    };
  },
  parseHTML() { return [{ tag: 'img[data-image-block]' }]; },
  renderHTML({ node, HTMLAttributes }) {
    const asset = node.attrs.asset as { assetId: string; sha256: string } | null;
    return ['img', mergeAttributes(HTMLAttributes, {
      'data-image-block': '', src: asset ? `/v1/assets/${asset.assetId}/bytes?sha=${asset.sha256}` : '', alt: node.attrs.alt ?? '',
      style: `max-width:${node.attrs.widthPercent ?? 100}%;height:auto`,
    })];
  },
});
