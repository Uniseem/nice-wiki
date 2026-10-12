<script setup lang="ts">import { NodeViewContent, NodeViewWrapper } from '@tiptap/vue-3'; defineProps<{ node: { attrs: Record<string, unknown> }, updateAttributes: (attrs: Record<string, unknown>) => void }>(); const kinds = [{v:'info',n:'信息'},{v:'tip',n:'提示'},{v:'warning',n:'警告'},{v:'danger',n:'危险'}]</script>
<template>
  <NodeViewWrapper
    as="aside"
    :class="`nw-callout nw-callout-${node.attrs.kind}`"
  >
    <select
      :value="node.attrs.kind"
      contenteditable="false"
      @change="updateAttributes({ kind: ($event.target as HTMLSelectElement).value })"
    >
      <option
        v-for="kind in kinds"
        :key="kind.v"
        :value="kind.v"
      >
        {{ kind.n }}
      </option>
    </select><NodeViewContent />
  </NodeViewWrapper>
</template>
