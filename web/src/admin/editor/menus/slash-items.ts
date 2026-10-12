import type { Editor } from '@tiptap/core'
import { newBlockID } from '../../../shared/tiptap/schema'

export interface InsertItem {
  name: string
  description: string
  run: (editor: Editor) => boolean
}
const paragraph = () => ({ type: 'paragraph', attrs: { id: newBlockID() } })
export const insertItems: InsertItem[] = [
  { name: '段落', description: '普通文字', run: editor => editor.chain().focus().setParagraph().run() },
  ...([2, 3, 4] as const).map(level => ({
    name: `标题 H${level}`, description: '正文标题',
    run: (editor: Editor) => editor.chain().focus().setNode('heading', { level }).run(),
  })),
  { name: '引用', description: '突出引用的内容', run: editor => editor.chain().focus().toggleBlockquote().run() },
  { name: '无序列表', description: '项目清单', run: editor => editor.chain().focus().toggleBulletList().run() },
  { name: '有序列表', description: '步骤清单', run: editor => editor.chain().focus().toggleOrderedList().run() },
  { name: '任务列表', description: '带勾选框的任务', run: editor => editor.chain().focus().toggleTaskList().run() },
  { name: '代码块', description: '保留原始代码', run: editor => editor.chain().focus().toggleCodeBlock().run() },
  { name: '分割线', description: '分隔内容', run: editor => editor.chain().focus().setHorizontalRule().run() },
  { name: '表格', description: '三行三列表格', run: editor => editor.chain().focus().insertTable({ rows: 3, cols: 3, withHeaderRow: true }).run() },
  { name: '提示框', description: '信息提示', run: editor => editor.chain().focus().insertContent({ type: 'callout', attrs: { kind: 'info', id: newBlockID() }, content: [paragraph()] }).run() },
  { name: '两栏', description: '并排呈现内容', run: editor => editor.chain().focus().insertContent({ type: 'columns', attrs: { id: newBlockID(), widths: [1, 1] }, content: [0, 1].map(() => ({ type: 'column', attrs: { id: newBlockID() }, content: [paragraph()] })) }).run() },
  { name: '公式', description: '独立 LaTeX 公式', run: editor => editor.chain().focus().insertContent({ type: 'blockMath', attrs: { id: newBlockID(), latex: 'E = mc^2' } }).run() },
]
