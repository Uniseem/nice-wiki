<script setup lang="ts">
import { computed, ref } from 'vue'
import type { Editor } from '@tiptap/core'
import { insertItems } from './slash-items'
const props = defineProps<{ editor: Editor }>()
const search = ref('')
const open = ref(false)
const items = computed(() => insertItems.filter(item => item.name.includes(search.value) || item.description.includes(search.value)))
function insert(index: number) {
  const item = items.value[index]
  if (!item) return
  item.run(props.editor); open.value = false; search.value = ''
}
</script>

<template>
  <div class="relative">
    <button
      class="rounded border border-border px-3 py-1 text-sm"
      :aria-expanded="open"
      @click="open = !open"
    >
      插入块
    </button>
    <section
      v-if="open"
      class="absolute inset-s-0 top-full z-30 mt-2 w-72 rounded-card border border-border bg-surface p-3 shadow-lg"
      aria-label="选择块类型"
      @keydown.esc="open = false"
    >
      <input
        v-model="search"
        class="nw-input mb-2"
        placeholder="搜索块类型"
        aria-label="搜索块类型"
      >
      <div class="max-h-80 overflow-y-auto">
        <button
          v-for="(item, index) in items"
          :key="item.name"
          class="block w-full rounded px-3 py-2 text-start hover:bg-background"
          @click="insert(index)"
        >
          <span class="block text-sm font-medium">{{ item.name }}</span><span class="block text-xs text-muted">{{ item.description }}</span>
        </button>
      </div>
      <p
        v-if="!items.length"
        class="px-3 py-4 text-sm text-muted"
      >
        没有匹配的块类型。
      </p>
    </section>
  </div>
</template>
