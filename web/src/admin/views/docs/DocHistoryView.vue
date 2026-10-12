<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import PageHeader from '../../components/PageHeader.vue'
import DiffView from '../../components/DiffView.vue'
import { client, errorMessage, requireData } from '../../../shared/api/client'
import type { components } from '../../../shared/api/schema.gen'
import { useEditLock } from '../../composables/useEditLock'

const route = useRoute(); const router = useRouter(); const id = String(route.params.id)
const revisions = ref<components['schemas']['HistoryRevision'][]>([])
const before = ref(''); const after = ref(''); const error = ref(''); const busy = ref(false)
const lock = useEditLock(id)
onMounted(async () => { try { revisions.value = requireData(await client.GET('/api/admin/docs/{id}/history', { params: { path: { id } } })); await lock.acquire() } catch (cause) { error.value = errorMessage(cause) } })
onBeforeUnmount(() => lock.release())
async function compare(index: number) {
  const newer = revisions.value[index]; const older = revisions.value[index + 1]
  if (!newer || !older) return
  try { const value = requireData(await client.GET('/api/admin/docs/{id}/history/diff', { params: { path: { id }, query: { before: older.hash, after: newer.hash } } })); before.value = value.before; after.value = value.after }
  catch (cause) { error.value = errorMessage(cause) }
}
async function restore(revision: string) {
  if (lock.readonly.value) return
  busy.value = true; error.value = ''
  try {
    const draft = requireData(await client.GET('/api/admin/docs/{id}/draft', { params: { path: { id } } }))
    requireData(await client.POST('/api/admin/docs/{id}/history/restore', { params: { path: { id } }, body: { revision, lockToken: lock.token.value, draftVersion: draft.version } }))
    await router.push('/docs')
  } catch (cause) { error.value = errorMessage(cause) }
  finally { busy.value = false }
}
</script>
<template>
  <PageHeader
    title="版本历史"
    description="回滚只会创建新草稿，不会改变访客正在阅读的版本。"
  />
  <p
    v-if="error"
    role="alert"
    class="mb-4 text-danger"
  >
    {{ error }}
  </p>
  <p
    v-if="lock.notice.value"
    class="mb-4 text-sm text-muted"
  >
    {{ lock.notice.value }}
  </p>
  <div class="mb-8 grid gap-3">
    <article
      v-for="(revision, index) in revisions"
      :key="revision.hash"
      class="flex flex-wrap items-center gap-4 rounded-card border border-border p-4"
    >
      <div class="min-w-0 grow">
        <p class="font-medium">
          {{ revision.message }}
        </p><p class="text-xs text-muted">
          {{ revision.author }} · {{ new Date(revision.time).toLocaleString() }} · {{ revision.hash.slice(0, 10) }}
        </p>
      </div>
      <button
        v-if="index + 1 < revisions.length"
        class="text-sm text-primary"
        @click="compare(index)"
      >
        与上版比较
      </button>
      <button
        :disabled="busy || lock.readonly.value"
        class="text-sm text-primary"
        @click="restore(revision.hash)"
      >
        回滚为草稿
      </button>
    </article>
    <p
      v-if="!revisions.length"
      class="text-sm text-muted"
    >
      暂无发布历史。
    </p>
  </div>
  <DiffView
    v-if="before || after"
    :before="before"
    :after="after"
  />
</template>
