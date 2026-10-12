<script setup lang="ts">import { NodeViewWrapper } from '@tiptap/vue-3'; defineProps<{node:{attrs:Record<string,unknown>},updateAttributes:(attrs:Record<string,unknown>)=>void}>()</script>
<template>
  <NodeViewWrapper class="nw-video">
    <video
      v-if="node.attrs.source === 'upload'"
      controls
      :src="`/assets/videos/${node.attrs.hash}${node.attrs.extension}`"
      :poster="node.attrs.posterHash ? `/assets/images/${node.attrs.posterHash}.webp` : undefined"
    /><p v-else>
      平台视频：{{ node.attrs.provider }} / {{ node.attrs.providerId }}
    </p><input
      :value="String(node.attrs.title ?? '')"
      class="nw-input"
      placeholder="标题"
      @input="updateAttributes({title:($event.target as HTMLInputElement).value})"
    ><textarea
      :value="String(node.attrs.description ?? '')"
      class="nw-input"
      placeholder="说明"
      @input="updateAttributes({description:($event.target as HTMLTextAreaElement).value})"
    />
  </NodeViewWrapper>
</template>
