<script setup lang="ts">import { NodeViewWrapper } from '@tiptap/vue-3'; defineProps<{ node:{attrs:Record<string,unknown>},updateAttributes:(attrs:Record<string,unknown>)=>void }>()</script>
<template>
  <NodeViewWrapper
    as="figure"
    class="nw-image"
  >
    <img
      :src="`/assets/images/${node.attrs.hash}${node.attrs.extension}`"
      :alt="String(node.attrs.alt ?? '')"
    ><label>宽度<input
      type="range"
      min="10"
      max="100"
      :value="Number(node.attrs.width ?? 100)"
      @input="updateAttributes({width:Number(($event.target as HTMLInputElement).value)})"
    ></label><input
      :value="String(node.attrs.alt ?? '')"
      class="nw-input"
      placeholder="替代文字"
      @input="updateAttributes({alt:($event.target as HTMLInputElement).value})"
    ><input
      :value="String(node.attrs.caption ?? '')"
      class="nw-input"
      placeholder="图注"
      @input="updateAttributes({caption:($event.target as HTMLInputElement).value})"
    >
  </NodeViewWrapper>
</template>
