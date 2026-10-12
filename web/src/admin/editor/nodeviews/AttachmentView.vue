<script setup lang="ts">import { NodeViewWrapper } from '@tiptap/vue-3'; defineProps<{node:{attrs:Record<string,unknown>},updateAttributes:(attrs:Record<string,unknown>)=>void}>()</script>
<template>
  <NodeViewWrapper class="nw-attachment">
    <strong>{{ node.attrs.name }}</strong><span>{{ node.attrs.size }} 字节</span><textarea
      :value="String(node.attrs.description ?? '')"
      class="nw-input"
      placeholder="附件说明"
      @input="updateAttributes({description:($event.target as HTMLTextAreaElement).value})"
    />
  </NodeViewWrapper>
</template>
