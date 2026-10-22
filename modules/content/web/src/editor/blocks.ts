import { mergeAttributes, Node } from '@tiptap/core';
import { t } from '@ui';

// 资源引用、图注、栏宽这类结构化属性：写进 data-nw-* 的 JSON，随 HTML 剪贴板往返，粘贴后仍能保存。
// 稳定 ID 故意不输出，复制出来的节点由 StableIds 换新 ID，原节点身份不变。
export function jsonAttr(name: string, fallback: unknown = null) {
  const key = `data-nw-${name.replace(/[A-Z]/g, (c) => `-${c.toLowerCase()}`)}`;
  return {
    default: fallback,
    parseHTML: (element: HTMLElement) => {
      const raw = element.getAttribute(key);
      if (raw === null) return fallback;
      try { return JSON.parse(raw); } catch { return fallback; }
    },
    renderHTML: (attrs: Record<string, unknown>) => (attrs[name] === null || attrs[name] === undefined ? {} : { [key]: JSON.stringify(attrs[name]) }),
  };
}

// 提示框：info / tip / warning / danger，内容是普通块。
export const Callout = Node.create({
  name: 'callout',
  group: 'block',
  content: 'block+',
  defining: true,
  addAttributes() {
    return { blockId: { default: null, rendered: false }, kind: { default: 'info', rendered: false, parseHTML: (element: HTMLElement) => element.getAttribute('data-callout') || 'info' } };
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
    return { blockId: { default: null, rendered: false }, latex: jsonAttr('latex', '') };
  },
  parseHTML() { return [{ tag: 'pre[data-math]' }]; },
  renderHTML({ node, HTMLAttributes }) { return ['pre', mergeAttributes(HTMLAttributes, { 'data-math': '', class: 'math-block' }), `$$ ${node.attrs.latex} $$`]; },
});

// 附件：只保存不可变资源引用和显示名。
export const Attachment = Node.create({
  name: 'attachment',
  group: 'block',
  atom: true,
  draggable: true,
  addAttributes() {
    return { blockId: { default: null, rendered: false }, asset: jsonAttr('asset'), displayName: jsonAttr('displayName', t('附件')) };
  },
  parseHTML() { return [{ tag: 'div[data-attachment]' }]; },
  renderHTML({ node, HTMLAttributes }) { return ['div', mergeAttributes(HTMLAttributes, { 'data-attachment': '', class: 'attachment' }), `📎 ${node.attrs.displayName}`]; },
});

// 多栏：2–4 栏并排，每栏是普通块；保留原有栏宽比例，栏数变了才重新等分；栏不能再嵌套多栏。
export const Columns = Node.create({
  name: 'columns',
  group: 'block',
  content: 'column{2,4}',
  isolating: true,
  defining: true,
  addAttributes() {
    return { blockId: { default: null, rendered: false }, widthRatios: jsonAttr('widthRatios') };
  },
  parseHTML() { return [{ tag: 'div[data-columns]' }]; },
  renderHTML({ HTMLAttributes }) { return ['div', mergeAttributes(HTMLAttributes, { 'data-columns': '', class: 'columns' }), 0]; },
});

export const Column = Node.create({
  name: 'column',
  content: 'block+',
  isolating: true,
  addAttributes() {
    return { columnId: { default: null, rendered: false } };
  },
  parseHTML() { return [{ tag: 'div[data-column]' }]; },
  renderHTML() { return ['div', { 'data-column': '', class: 'column' }, 0]; },
});
