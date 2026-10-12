import { Mark, mergeAttributes } from '@tiptap/core'

export const InternalLink = Mark.create({
  name: 'internalLink', inclusive: false,
  addAttributes: () => ({ documentId: { default: '' }, blockId: { default: '' } }),
  parseHTML: () => [{ tag: 'a[data-document-id]' }],
  renderHTML: ({ mark, HTMLAttributes }) => ['a', mergeAttributes(HTMLAttributes, { 'data-document-id': mark.attrs.documentId, 'data-block-id': mark.attrs.blockId, class: 'nw-internal-link' }), 0],
})
