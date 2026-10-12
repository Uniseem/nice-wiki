<script setup lang="ts">
import { computed, ref } from 'vue'
import type { components } from '../../shared/api/schema.gen'
const props = defineProps<{ documents: components['schemas']['DocMeta'][], selected?: string }>()
const emit = defineEmits<{ select: [id: string], move: [id: string, parentId: string, beforeId: string, afterId: string] }>()
const dragging = ref('')
const ordered = computed(() => {
  const result: Array<{ value: components['schemas']['DocMeta'], depth: number }> = []
  const visit = (parent: string, depth: number) => props.documents.filter(value => value.parentId === parent).sort((a, b) => a.sortKey.localeCompare(b.sortKey)).forEach(value => { result.push({ value, depth }); visit(value.id, depth + 1) })
  visit('', 0); return result
})
function drop(target: components['schemas']['DocMeta']) {
  if (!dragging.value || dragging.value === target.id) return
  emit('move', dragging.value, target.parentId, target.id, '')
  dragging.value = ''
}
</script>
<template>
  <nav
    aria-label="文档树"
    class="grid gap-1"
  >
    <button
      v-for="entry in ordered"
      :key="entry.value.id"
      draggable="true"
      class="rounded border p-3 text-start text-sm"
      :class="selected === entry.value.id ? 'border-primary' : 'border-border'"
      :style="{ paddingInlineStart: `${.75 + entry.depth * 1.25}rem` }"
      @dragstart="dragging = entry.value.id"
      @dragover.prevent
      @drop.prevent="drop(entry.value)"
      @click="emit('select', entry.value.id)"
    >
      {{ entry.value.title }}<span class="ms-2 text-xs text-muted">{{ entry.value.published ? '已发布' : '草稿' }}</span>
    </button>
  </nav>
</template>
