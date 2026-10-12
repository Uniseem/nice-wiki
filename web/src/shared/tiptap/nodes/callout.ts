import { Node, mergeAttributes } from '@tiptap/core'

export const Callout = Node.create({
  name: 'callout', group: 'block', content: 'block+', defining: true,
  addAttributes: () => ({ kind: { default: 'info', parseHTML: element => element.dataset.kind ?? 'info' } }),
  parseHTML: () => [{ tag: 'aside[data-kind]' }],
  renderHTML: ({ node, HTMLAttributes }) => ['aside', mergeAttributes(HTMLAttributes, {
    class: `nw-callout nw-callout-${['info', 'tip', 'warning', 'danger'].includes(node.attrs.kind) ? node.attrs.kind : 'info'}`,
    'data-kind': node.attrs.kind,
  }), 0],
})
