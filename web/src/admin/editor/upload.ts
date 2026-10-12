import type { Editor } from '@tiptap/core'
import type { components } from '../../shared/api/schema.gen'
import { newBlockID } from '../../shared/tiptap/schema'

export function insertUploadedAsset(editor: Editor, asset: components['schemas']['Asset']): boolean {
  const attrs = { id: newBlockID(), hash: asset.hash, extension: asset.extension }
  if (asset.kind === 'images') {
    return editor.chain().focus().insertContent({ type: 'image', attrs: { ...attrs, alt: asset.name, caption: '', width: 100, variants: [] } }).run()
  }
  if (asset.kind === 'videos') {
    return editor.chain().focus().insertContent({ type: 'video', attrs: { ...attrs, source: 'upload', title: asset.name, description: '' } }).run()
  }
  return editor.chain().focus().insertContent({ type: 'attachment', attrs: { ...attrs, name: asset.name, size: asset.size, description: '' } }).run()
}
