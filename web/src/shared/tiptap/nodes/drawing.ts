import { Node, mergeAttributes } from '@tiptap/core'

export const Drawing = Node.create({
  name: 'drawing', group: 'block', atom: true, draggable: true,
  addAttributes: () => ({ sceneHash: { default: '' }, lightHash: { default: '' }, darkHash: { default: '' }, alt: { default: '' }, caption: { default: '' } }),
  parseHTML: () => [{ tag: 'figure[data-drawing]' }],
  renderHTML: ({ node, HTMLAttributes }) => ['figure', mergeAttributes(HTMLAttributes, { 'data-drawing': '', class: 'nw-drawing' }),
    ['img', { src: `/assets/drawings/${node.attrs.lightHash}.svg`, alt: node.attrs.alt }], ['figcaption', {}, node.attrs.caption],
  ],
})
