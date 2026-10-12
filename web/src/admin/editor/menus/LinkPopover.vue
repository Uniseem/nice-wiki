<script setup lang="ts">
import { ref } from 'vue'; import type { Editor } from '@tiptap/core'; import DocPicker from '../../components/DocPicker.vue'
const props = defineProps<{ editor: Editor }>(); const external = ref(''); const documentId = ref(''); const blockId = ref('')
function setExternal() { if (!external.value) return; props.editor.chain().focus().extendMarkRange('link').setLink({ href: external.value }).run(); external.value = '' }
function setInternal() { if (!documentId.value) return; props.editor.chain().focus().extendMarkRange('internalLink').setMark('internalLink', { documentId: documentId.value, blockId: blockId.value }).run() }
</script>
<template>
  <details class="relative">
    <summary class="cursor-pointer text-sm">
      链接
    </summary><section class="absolute z-20 mt-2 grid w-80 gap-3 rounded-card border border-border bg-surface p-4">
      <form
        class="flex gap-2"
        @submit.prevent="setExternal"
      >
        <input
          v-model="external"
          type="url"
          class="nw-input"
          placeholder="https://example.com"
        ><button>外部链接</button>
      </form><DocPicker v-model="documentId" /><input
        v-model="blockId"
        class="nw-input"
        placeholder="可选块 ID"
      ><button
        type="button"
        @click="setInternal"
      >
        站内链接
      </button><button
        type="button"
        @click="editor.chain().focus().unsetLink().unsetMark('internalLink').run()"
      >
        移除链接
      </button>
    </section>
  </details>
</template>
