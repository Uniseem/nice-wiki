import { Node, mergeAttributes } from '@tiptap/core'

export const Video = Node.create({
  name: 'video', group: 'block', atom: true, draggable: true,
  addAttributes: () => ({ source: { default: 'upload' }, hash: { default: '' }, extension: { default: '.mp4' }, provider: { default: '' }, providerId: { default: '' }, posterHash: { default: '' }, title: { default: '' }, description: { default: '' }, author: { default: '' }, duration: { default: 0 } }),
  parseHTML: () => [{ tag: 'figure[data-video]' }],
  renderHTML: ({ node, HTMLAttributes }) => ['figure', mergeAttributes(HTMLAttributes, { 'data-video': '', class: 'nw-video' }), node.attrs.title],
})
