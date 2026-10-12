<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { client, errorMessage, requireData } from '../../shared/api/client'
import type { components } from '../../shared/api/schema.gen'
const props = defineProps<{ kind?: string }>(); const emit = defineEmits<{ select: [asset: components['schemas']['Asset']] }>()
const values = ref<components['schemas']['Asset'][]>([]); const search = ref(''); const error = ref('')
const filtered = computed(() => values.value.filter(value => (!props.kind || value.kind === props.kind) && (!search.value || value.name.includes(search.value) || value.hash.includes(search.value))))
onMounted(async () => { try { values.value = requireData(await client.GET('/api/admin/assets')) } catch (cause) { error.value = errorMessage(cause) } })
</script>
<template>
  <div class="grid gap-3">
    <input
      v-model="search"
      class="nw-input"
      placeholder="搜索资源"
    ><p
      v-if="error"
      class="text-danger"
    >
      {{ error }}
    </p><button
      v-for="asset in filtered"
      :key="`${asset.kind}:${asset.hash}:${asset.extension}`"
      type="button"
      class="rounded-card border border-border p-3 text-start"
      @click="emit('select', asset)"
    >
      <strong class="block">{{ asset.name }}</strong><span class="text-xs text-muted">{{ asset.kind }} · {{ asset.size }} 字节</span>
    </button>
  </div>
</template>
