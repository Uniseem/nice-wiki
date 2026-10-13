<script setup lang="ts">
import { onMounted, ref } from 'vue'
import PageHeader from '../../components/PageHeader.vue'
import { client, errorMessage, requireData } from '../../../shared/api/client'
import type { components } from '../../../shared/api/schema.gen'

const values = ref<components['schemas']['GuardEvent'][]>([])
const error = ref('')

async function refresh() {
  values.value = requireData(await client.GET('/api/admin/guard/events')).slice().reverse()
}

onMounted(async () => {
  try { await refresh() } catch (cause) { error.value = errorMessage(cause) }
})

async function unban(scope: string) {
  try {
    error.value = ''
    requireData(await client.DELETE('/api/admin/guard/bans/{key}', { params: { path: { key: scope } } }))
  } catch (cause) { error.value = errorMessage(cause) }
}
</script>

<template>
  <PageHeader
    title="问答防滥用"
    description="展示程序检查、模型风险和预算触发的事件；范围值是进程内加盐标识，不包含提问正文或原始 IP。"
  />
  <p
    v-if="error"
    class="text-danger"
  >
    {{ error }}
  </p>
  <p
    v-if="!values.length"
    class="text-muted"
  >
    暂无风险事件。
  </p>
  <table
    v-else
    class="w-full"
  >
    <thead><tr><th>时间</th><th>风险</th><th>原因</th><th>范围</th><th /></tr></thead>
    <tbody>
      <tr
        v-for="value in values"
        :key="`${value.created}:${value.scope}`"
      >
        <td>{{ new Date(value.created).toLocaleString() }}</td><td>{{ value.risk }}</td><td>{{ value.reason }}</td><td class="font-mono text-xs">
          {{ value.scope }}
        </td>
        <td>
          <button
            v-if="value.risk==='high'"
            type="button"
            @click="unban(value.scope)"
          >
            解除封禁
          </button>
        </td>
      </tr>
    </tbody>
  </table>
</template>
