<script setup lang="ts">
import { onMounted, ref } from 'vue'
import PageHeader from '../../components/PageHeader.vue'
import { client, requireData, errorMessage } from '../../../shared/api/client'
import type { components } from '../../../shared/api/schema.gen'
const assets = ref<components['schemas']['Asset'][]>([])
const error = ref('')
const loading = ref(false)
const selected = ref<components['schemas']['Asset']>()
async function refresh() {
  loading.value = true
  try { assets.value = requireData(await client.GET('/api/admin/assets')) }
  catch (cause) { error.value = errorMessage(cause) }
  finally { loading.value = false }
}
onMounted(refresh)
async function remove() {
  const asset = selected.value
  if (!asset) return
  error.value = ''
  try { requireData(await client.DELETE('/api/admin/assets/{id}', { params: { path: { id: asset.hash } } })); selected.value = undefined; await refresh() }
  catch (cause) { error.value = errorMessage(cause) }
}
</script>
<template>
  <PageHeader
    title="资源库"
    description="资源不会自动删除。当前引用包括正文、共享草稿、回收站及站点设置。"
  />
  <p
    v-if="error"
    role="alert"
    class="mb-4 text-danger"
  >
    {{ error }}
  </p>
  <p
    v-if="loading"
    class="text-muted"
  >
    正在刷新引用关系…
  </p>
  <p
    v-else-if="!assets.length"
    class="text-muted"
  >
    还没有上传资源。
  </p>
  <div
    v-else
    class="grid gap-4 md:grid-cols-2 xl:grid-cols-3"
  >
    <article
      v-for="asset in assets"
      :key="asset.hash"
      class="grid gap-3 rounded-card border border-border bg-surface p-5"
    >
      <img
        v-if="asset.kind === 'images'"
        :src="`/assets/${asset.kind}/${asset.hash}${asset.extension}`"
        :alt="asset.name"
        loading="lazy"
        class="max-h-40 max-w-full object-contain"
      >
      <h2 class="break-all text-sm font-semibold">
        {{ asset.name }}
      </h2>
      <p class="text-xs text-muted">
        {{ (asset.size / 1024).toFixed(1) }} KiB · {{ asset.references }} 处当前引用
      </p>
      <a
        :href="`/assets/${asset.kind}/${asset.hash}${asset.extension}`"
        class="text-sm text-primary"
        target="_blank"
        rel="noopener"
      >打开资源</a>
      <button
        v-if="asset.references === 0"
        class="text-start text-sm text-danger"
        @click="selected = asset"
      >
        清理文件
      </button>
    </article>
  </div>
  <section
    v-if="selected"
    role="alertdialog"
    aria-modal="true"
    aria-labelledby="delete-title"
    class="fixed inset-0 z-50 flex items-center justify-center bg-background/90 p-6"
  >
    <div class="max-w-lg rounded-card border border-border bg-surface p-8">
      <h2
        id="delete-title"
        class="mb-4 text-xl font-semibold"
      >
        删除 {{ selected.name }}？
      </h2><p class="mb-6 text-sm text-muted">
        文件目前没有引用，但历史版本可能仍在使用它。删除后，回滚的文档可能缺失该资源。
      </p><div class="flex gap-4">
        <button
          class="nw-button"
          @click="selected = undefined"
        >
          保留文件
        </button><button
          class="text-danger"
          @click="remove"
        >
          确认删除
        </button>
      </div>
    </div>
  </section>
</template>
