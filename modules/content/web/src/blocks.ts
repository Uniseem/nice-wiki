import { Node } from '@tiptap/core';

// 提示框：info / tip / warning / danger，内容是普通块。
export const Callout = Node.create({
  name: 'callout',
  group: 'block',
  content: 'block+',
  defining: true,
  addAttributes() {
    return { blockId: { default: null, rendered: false }, kind: { default: 'info', rendered: false } };
  },
  parseHTML() { return [{ tag: 'div[data-callout]' }]; },
  renderHTML({ node }) { return ['div', { 'data-callout': node.attrs.kind, class: `callout ${node.attrs.kind}` }, 0]; },
});

// 公式块：保存原始 LaTeX，编辑器里以等宽文字显示。
export const MathBlock = Node.create({
  name: 'mathBlock',
  group: 'block',
  atom: true,
  addAttributes() {
    return { blockId: { default: null, rendered: false }, latex: { default: '', rendered: false } };
  },
  parseHTML() { return [{ tag: 'pre[data-math]' }]; },
  renderHTML({ node }) { return ['pre', { 'data-math': '', class: 'math-block' }, `$$ ${node.attrs.latex} $$`]; },
});

// 附件：只保存不可变资源引用和显示名。
export const Attachment = Node.create({
  name: 'attachment',
  group: 'block',
  atom: true,
  draggable: true,
  addAttributes() {
    return { blockId: { default: null, rendered: false }, asset: { default: null, rendered: false }, displayName: { default: '附件', rendered: false } };
  },
  parseHTML() { return [{ tag: 'div[data-attachment]' }]; },
  renderHTML({ node }) { return ['div', { 'data-attachment': '', class: 'attachment' }, `📎 ${node.attrs.displayName}`]; },
});
