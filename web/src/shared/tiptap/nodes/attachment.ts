import { Node, mergeAttributes } from '@tiptap/core'

export const Attachment = Node.create({
  name: 'attachment', group: 'block', atom: true, draggable: true,
  addAttributes: () => ({ hash: { default: '' }, extension: { default: '' }, name: { default: '' }, size: { default: 0 }, description: { default: '' } }),
  parseHTML: () => [{ tag: 'div[data-attachment]' }],
  renderHTML: ({ node, HTMLAttributes }) => ['div', mergeAttributes(HTMLAttributes, { 'data-attachment': '', class: 'nw-attachment' }), node.attrs.name],
})
