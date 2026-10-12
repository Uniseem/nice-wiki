import type { JSONContent } from '@tiptap/core'

export const schemaVersion = 1
export const blockTypes = [
  'paragraph', 'heading', 'blockquote', 'horizontalRule', 'bulletList', 'orderedList',
  'listItem', 'taskList', 'taskItem', 'callout', 'table', 'tableRow', 'tableCell',
  'tableHeader', 'codeBlock', 'blockMath', 'image', 'attachment', 'drawing', 'video',
  'columns', 'column',
] as const
export interface WikiDocument {
  schemaVersion: number
  content: JSONContent
}

export function newBlockID(): string { return crypto.randomUUID().replaceAll('-', '') }

// 复制块必须重新分配整棵子树的块 ID。
export function duplicateBlock(source: JSONContent): JSONContent {
  const result = structuredClone(source)
  const visit = (node: JSONContent) => {
    if (blockTypes.includes(node.type as typeof blockTypes[number])) {
      node.attrs = { ...node.attrs, id: newBlockID() }
    }
    node.content?.forEach(visit)
  }
  visit(result)
  return result
}
