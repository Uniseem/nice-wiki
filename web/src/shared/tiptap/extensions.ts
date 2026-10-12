import { Node } from '@tiptap/core'
import StarterKit from '@tiptap/starter-kit'
import { Table, TableRow, TableHeader, TableCell } from '@tiptap/extension-table'
import TaskList from '@tiptap/extension-task-list'
import TaskItem from '@tiptap/extension-task-item'
import Highlight from '@tiptap/extension-highlight'
import UniqueID from '@tiptap/extension-unique-id'
import { Callout } from './nodes/callout'
import { Columns, Column } from './nodes/columns'
import { WikiImage } from './nodes/image'
import { Attachment } from './nodes/attachment'
import { Drawing } from './nodes/drawing'
import { Video } from './nodes/video'
import { WikiHeading } from './nodes/heading'
import { InternalLink } from './nodes/internal-link'

const InlineMath = Node.create({
  name: 'inlineMath', group: 'inline', inline: true, atom: true,
  addAttributes: () => ({ latex: { default: '' } }),
  parseHTML: () => [{ tag: 'span[data-latex]' }],
  renderHTML: ({ HTMLAttributes }) => ['span', { ...HTMLAttributes, class: 'nw-math', 'data-latex': HTMLAttributes.latex }, HTMLAttributes.latex],
})
const BlockMath = Node.create({
  name: 'blockMath', group: 'block', atom: true,
  addAttributes: () => ({ latex: { default: '' } }),
  parseHTML: () => [{ tag: 'div[data-latex]' }],
  renderHTML: ({ HTMLAttributes }) => ['div', { ...HTMLAttributes, class: 'nw-math', 'data-latex': HTMLAttributes.latex }, HTMLAttributes.latex],
})

export const schemaExtensions = [
  UniqueID.configure({
    types: [
      'paragraph', 'heading', 'blockquote', 'horizontalRule', 'bulletList', 'orderedList',
      'listItem', 'taskList', 'taskItem', 'callout', 'table', 'tableRow', 'tableCell',
      'tableHeader', 'codeBlock', 'blockMath', 'image', 'attachment', 'drawing', 'video',
      'columns', 'column',
    ],
    attributeName: 'id',
    generateID: () => crypto.randomUUID().replaceAll('-', ''),
  }),
  StarterKit.configure({ heading: false }),
  WikiHeading, Table, TableRow, TableHeader, TableCell,
  TaskList, TaskItem.configure({ nested: true }), Highlight,
  Callout, Columns, Column, WikiImage, Attachment, Drawing, Video,
  InternalLink, InlineMath, BlockMath,
]
