import { Node, mergeAttributes } from '@tiptap/core'

export const WikiHeading = Node.create({
  name: 'heading', group: 'block', content: 'inline*', defining: true,
  addAttributes: () => ({ level: { default: 2 } }),
  parseHTML: () => [2, 3, 4].map(level => ({ tag: `h${level}`, attrs: { level } })),
  renderHTML: ({ node, HTMLAttributes }) => {
    const level = [2, 3, 4].includes(node.attrs.level) ? node.attrs.level : 2
    return [`h${level}`, mergeAttributes(HTMLAttributes, { class: 'nw-heading' }), 0]
  },
})
