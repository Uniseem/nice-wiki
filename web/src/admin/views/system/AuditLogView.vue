<script setup lang="ts">
import { onMounted, ref } from 'vue'
import PageHeader from '../../components/PageHeader.vue'
import { client, requireData, errorMessage } from '../../../shared/api/client'
import type { components } from '../../../shared/api/schema.gen'
const events = ref<components['schemas']['AuditEvent'][]>([])
const error = ref('')
onMounted(async () => { try { events.value = requireData(await client.GET('/api/admin/audit')) } catch (cause) { error.value = errorMessage(cause) } })
</script>
<template>
  <PageHeader
    title="操作日志"
    description="最近 100 条操作，只允许超级管理员查看。"
  />
  <p
    v-if="error"
    role="alert"
    class="text-danger"
  >
    {{ error }}
  </p>
  <p
    v-if="!events.length"
    class="text-muted"
  >
    暂无日志。
  </p>
  <div class="overflow-x-auto">
    <table
      v-if="events.length"
      class="w-full text-start text-sm"
    >
      <thead>
        <tr>
          <th class="p-3 text-start">
            时间
          </th><th class="p-3 text-start">
            账号
          </th><th class="p-3 text-start">
            操作
          </th><th class="p-3 text-start">
            目标
          </th>
        </tr>
      </thead><tbody>
        <tr
          v-for="event in events"
          :key="event.id"
          class="border-t border-border"
        >
          <td class="p-3">
            {{ new Date(event.createdAt * 1000).toLocaleString('zh-CN') }}
          </td><td class="p-3">
            {{ event.accountName || '匿名' }}
          </td><td class="p-3">
            {{ event.action }}
          </td><td class="p-3 font-mono text-xs">
            {{ event.targetId }}
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
