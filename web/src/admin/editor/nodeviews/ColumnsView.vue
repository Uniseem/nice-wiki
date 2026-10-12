<script setup lang="ts">import { NodeViewContent, NodeViewWrapper } from '@tiptap/vue-3'; const props=defineProps<{ node: { attrs: Record<string, unknown>, childCount: number }, updateAttributes: (attrs: Record<string, unknown>) => void }>(); function change(index:number,value:string){const widths=Array.isArray(props.node.attrs.widths)?[...props.node.attrs.widths] as number[]:Array(props.node.childCount).fill(1);widths[index]=Math.max(1,Math.min(100,Number(value)||1));props.updateAttributes({widths})}</script>
<template>
  <NodeViewWrapper class="nw-columns-editor">
    <div
      class="mb-2 flex gap-2"
      contenteditable="false"
    >
      <label
        v-for="(_, index) in node.childCount"
        :key="index"
        class="text-xs"
      >第 {{ index + 1 }} 栏<input
        type="number"
        min="1"
        max="100"
        :value="(node.attrs.widths as number[] | undefined)?.[index] ?? 1"
        @change="change(index, ($event.target as HTMLInputElement).value)"
      ></label>
    </div><NodeViewContent class="nw-columns" />
  </NodeViewWrapper>
</template>
