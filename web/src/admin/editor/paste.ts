import type { Editor } from '@tiptap/core'
import { ensureBlockIDs } from '../../shared/tiptap/schema'

export function installPasteCleanup(editor: Editor): () => void {
  const handler = (event: ClipboardEvent) => {
    const html = event.clipboardData?.getData('text/html')
    if (!html) return
    const parsed = new DOMParser().parseFromString(html, 'text/html')
    parsed.querySelectorAll('script,style,iframe,object,embed,form').forEach(node => node.remove())
    parsed.querySelectorAll<HTMLElement>('*').forEach(node => {
      for (const attribute of [...node.attributes]) if (attribute.name === 'style' || attribute.name.startsWith('on') || attribute.name === 'class') node.removeAttribute(attribute.name)
    })
    event.preventDefault()
    editor.commands.insertContent(ensureBlockIDs({ type: 'doc', content: [{ type: 'paragraph', content: [{ type: 'text', text: parsed.body.textContent ?? '' }] }] }).content ?? [])
  }
  editor.view.dom.addEventListener('paste', handler)
  return () => editor.view.dom.removeEventListener('paste', handler)
}
