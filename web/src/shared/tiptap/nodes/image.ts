import { Node, mergeAttributes } from '@tiptap/core'

export const WikiImage = Node.create({
  name: 'image', group: 'block', atom: true, draggable: true,
  addAttributes: () => ({
    hash: { default: '' }, extension: { default: '.webp' }, alt: { default: '' }, caption: { default: '' },
    width: { default: 100 }, variants: { default: [] },
  }),
  parseHTML: () => [{ tag: 'figure[data-image]' }],
  renderHTML: ({ node, HTMLAttributes }) => ['figure', mergeAttributes(HTMLAttributes, { 'data-image': '', class: 'nw-image' }),
    ['img', { src: `/assets/images/${node.attrs.hash}${node.attrs.extension}`, alt: node.attrs.alt, loading: 'lazy' }],
    ['figcaption', {}, node.attrs.caption],
  ],
})
