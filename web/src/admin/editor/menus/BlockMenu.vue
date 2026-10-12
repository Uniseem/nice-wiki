<script setup lang="ts">
import type { Editor } from '@tiptap/core'
import { duplicateBlock } from '../../../shared/tiptap/schema'
const props = defineProps<{ editor: Editor }>()
function duplicate() { const { $from } = props.editor.state.selection; const depth = $from.depth; const node = $from.node(depth); if (node.type.name === 'doc') return; props.editor.chain().focus().insertContentAt($from.after(depth), duplicateBlock(node.toJSON())).run() }
function remove() { props.editor.chain().focus().deleteSelection().run() }
</script>
<template>
  <div class="flex gap-2 text-sm">
    <button
      type="button"
      @click="duplicate"
    >
      复制当前块
    </button><button
      type="button"
      class="text-danger"
      @click="remove"
    >
      删除选中内容
    </button>
  </div>
</template>
