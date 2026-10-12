<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useDocTreeStore } from '../stores/docTree'
const props = defineProps<{ modelValue?: string, exclude?: string }>()
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
const store = useDocTreeStore(); const search = ref('')
const options = computed(() => store.documents.filter(value => value.id !== props.exclude && (!search.value || value.title.includes(search.value) || value.slug.includes(search.value))))
onMounted(() => { if (!store.documents.length) void store.refresh() })
</script>
<template>
  <div class="grid gap-2">
    <input
      v-model="search"
      class="nw-input"
      placeholder="搜索文档"
    ><select
      :value="modelValue"
      class="nw-input"
      @change="emit('update:modelValue', ($event.target as HTMLSelectElement).value)"
    >
      <option value="">
        不选择文档
      </option><option
        v-for="value in options"
        :key="value.id"
        :value="value.id"
      >
        {{ value.title }}（{{ value.slug }}）
      </option>
    </select>
  </div>
</template>
