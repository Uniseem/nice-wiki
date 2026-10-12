<script setup lang="ts">import { NodeViewContent, NodeViewWrapper } from '@tiptap/vue-3'; defineProps<{ node: { attrs: Record<string, unknown> }, updateAttributes: (attrs: Record<string, unknown>) => void }>(); const languages=['','text','bash','go','javascript','typescript','json','html','css','sql','python','rust']</script>
<template>
  <NodeViewWrapper as="div">
    <select
      :value="node.attrs.language"
      contenteditable="false"
      @change="updateAttributes({ language: ($event.target as HTMLSelectElement).value })"
    >
      <option
        v-for="language in languages"
        :key="language"
        :value="language"
      >
        {{ language || '纯文本' }}
      </option>
    </select><pre class="nw-code"><NodeViewContent as="code" /></pre>
  </NodeViewWrapper>
</template>
