import UniqueID from '@tiptap/extension-unique-id'
import { schemaExtensions } from '../../shared/tiptap/extensions'
import { blockTypes } from '../../shared/tiptap/schema'

export const editorExtensions = [
  ...schemaExtensions,
  UniqueID.configure({ types: [...blockTypes] }),
]
