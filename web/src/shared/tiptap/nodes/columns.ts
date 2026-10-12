import { Node, mergeAttributes } from '@tiptap/core'

export const Columns = Node.create({
  name: 'columns', group: 'block', content: 'column{2,4}', defining: true, isolating: true,
  addAttributes: () => ({ widths: { default: [1, 1] } }),
  parseHTML: () => [{ tag: 'div[data-columns]' }],
  renderHTML: ({ node, HTMLAttributes }) => {
    const widths: number[] = Array.isArray(node.attrs.widths) ? node.attrs.widths : [1, 1]
    const safe = widths.slice(0, 4).map(value => Number.isFinite(value) && value > 0 ? value : 1)
    return ['div', mergeAttributes(HTMLAttributes, { 'data-columns': '', class: 'nw-columns', style: `grid-template-columns:${safe.map(value => `${value}fr`).join(' ')}` }), 0]
  },
})
export const Column = Node.create({
  name: 'column', content: 'block+', defining: true, isolating: true,
  parseHTML: () => [{ tag: 'div[data-column]' }],
  renderHTML: ({ HTMLAttributes }) => ['div', mergeAttributes(HTMLAttributes, { 'data-column': '', class: 'nw-column' }), 0],
})
