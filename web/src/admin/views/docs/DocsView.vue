<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import PageHeader from '../../components/PageHeader.vue'
import DocEditor from '../../editor/DocEditor.vue'
import { useDocTreeStore } from '../../stores/docTree'
import { client, requireData, errorMessage } from '../../../shared/api/client'
import DocTree from '../../components/DocTree.vue'
const store = useDocTreeStore()
const selected = ref('')
const title = ref('')
const slug = ref('')
const error = ref('')
const creating = ref(false)
const editor = ref<InstanceType<typeof DocEditor>>()
const current = computed(() => store.documents.find(document => document.id === selected.value))
onMounted(async () => { try { await store.refresh() } catch (cause) { error.value = errorMessage(cause) } })
async function select(id: string) { try { await editor.value?.flush(); selected.value = id; error.value = '' } catch (cause) { error.value = errorMessage(cause) } }
async function create() {
  error.value = ''; creating.value = true
  try { await editor.value?.flush(); const value = requireData(await client.POST('/api/admin/docs', { body: { parentId: '', title: title.value, slug: slug.value } })); await store.refresh(); selected.value = value.id; title.value = ''; slug.value = '' }
  catch (cause) { error.value = errorMessage(cause) }
  finally { creating.value = false }
}
async function move(id: string, parentId: string, beforeId: string, afterId: string) { try { requireData(await client.PUT('/api/admin/docs/{id}/move', { params: { path: { id } }, body: { parentId, beforeId, afterId } })); await store.refresh() } catch (cause) { error.value = errorMessage(cause) } }
</script>
<template>
  <PageHeader
    title="文档"
    description="草稿自动保存。发布后，访客才能看到正文。"
  />
  <p
    v-if="error"
    role="alert"
    class="mb-5 text-sm text-danger"
  >
    {{ error }}
  </p>
  <div class="grid gap-8 xl:grid-cols-[16rem_1fr]">
    <aside class="grid content-start gap-4">
      <form
        class="grid gap-3 rounded-card border border-border p-4"
        @submit.prevent="create"
      >
        <label class="grid gap-1 text-sm">新文档标题<input
          v-model="title"
          required
          maxlength="200"
          class="nw-input"
        ></label>
        <label class="grid gap-1 text-sm">英文路径名<input
          v-model="slug"
          required
          pattern="[a-z0-9]+(-[a-z0-9]+)*"
          maxlength="120"
          class="nw-input"
        ></label>
        <button
          :disabled="creating"
          class="nw-button"
        >
          新建文档
        </button>
      </form>
      <p
        v-if="store.loading"
        class="text-sm text-muted"
      >
        加载文档树…
      </p>
      <DocTree
        :documents="store.documents"
        :selected="selected"
        @select="select"
        @move="move"
      />
    </aside>
    <DocEditor
      v-if="current"
      :key="current.id"
      ref="editor"
      :meta="current"
      @published="store.refresh"
    />
    <p
      v-else
      class="py-12 text-sm text-muted"
    >
      新建或选择一篇文档，开始编辑。
    </p>
  </div>
  <RouterLink
    v-if="current"
    :to="`/docs/${current.id}/history`"
    class="mt-6 inline-block text-sm text-primary"
  >
    查看版本历史
  </RouterLink>
</template>
