<script setup lang="ts">import { NodeViewWrapper } from '@tiptap/vue-3'; defineProps<{ node: { attrs: Record<string, unknown> }, updateAttributes: (attrs: Record<string, unknown>) => void }>()</script>
<template>
  <NodeViewWrapper class="nw-math-editor">
    <label>LaTeX<textarea
      :value="String(node.attrs.latex ?? '')"
      rows="3"
      class="nw-input"
      @input="updateAttributes({ latex: ($event.target as HTMLTextAreaElement).value })"
    /></label><code>{{ node.attrs.latex }}</code>
  </NodeViewWrapper>
</template>
