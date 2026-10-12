<script setup lang="ts">
import { onMounted, ref } from 'vue'
import PageHeader from '../../components/PageHeader.vue'
import { useAuthStore } from '../../stores/auth'
import { client, errorMessage, requireData } from '../../../shared/api/client'
import type { components } from '../../../shared/api/schema.gen'

const auth = useAuthStore(); const values = ref<components['schemas']['DocMeta'][]>([]); const error = ref(''); const busy = ref('')
async function refresh() { values.value = requireData(await client.GET('/api/admin/trash')) }
onMounted(async () => { try { await refresh() } catch (cause) { error.value = errorMessage(cause) } })
async function restore(id: string) { busy.value = id; try { requireData(await client.POST('/api/admin/trash/{id}/restore', { params: { path: { id } } })); await refresh() } catch (cause) { error.value = errorMessage(cause) } finally { busy.value = '' } }
async function purge(id: string) { if (!confirm('彻底删除后，当前回收站快照无法恢复；历史版本和资源文件仍会保留。确定继续？')) return; busy.value = id; try { requireData(await client.DELETE('/api/admin/trash/{id}', { params: { path: { id } } })); await refresh() } catch (cause) { error.value = errorMessage(cause) } finally { busy.value = '' } }
</script>
<template>
  <PageHeader
    title="回收站"
    description="恢复时会检查父文档和同级路径冲突。"
  />
  <p
    v-if="error"
    role="alert"
    class="mb-4 text-danger"
  >
    {{ error }}
  </p>
  <div class="grid gap-3">
    <article
      v-for="value in values"
      :key="value.id"
      class="flex items-center gap-4 rounded-card border border-border p-4"
    >
      <div class="grow">
        <p class="font-medium">
          {{ value.title }}
        </p><p class="text-xs text-muted">
          {{ value.slug }}
        </p>
      </div><button
        :disabled="busy === value.id"
        class="text-sm text-primary"
        @click="restore(value.id)"
      >
        恢复
      </button><button
        v-if="auth.account?.superadmin"
        :disabled="busy === value.id"
        class="text-sm text-danger"
        @click="purge(value.id)"
      >
        彻底删除
      </button>
    </article><p
      v-if="!values.length"
      class="text-sm text-muted"
    >
      回收站为空。
    </p>
  </div>
</template>
