<script setup lang="ts">
import { useRouter } from 'vue-router'
import DocMetaPanel from './DocMetaPanel.vue'
import { onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { Editor, EditorContent } from '@tiptap/vue-3'
import type { JSONContent } from '@tiptap/core'
import type { components } from '../../shared/api/schema.gen'
import { schemaVersion } from '../../shared/tiptap/schema'
import { client, requireData, errorMessage } from '../../shared/api/client'
import SlashMenu from './menus/SlashMenu.vue'
import { useUpload } from '../composables/useUpload'
import { insertUploadedAsset } from './upload'
import { editorExtensions } from './extensions'
import { useEditLock } from '../composables/useEditLock'
import { useAutosave } from '../composables/useAutosave'
import { usePermission } from '../composables/usePermission'

const props = defineProps<{ meta: components['schemas']['DocMeta'] }>()
const emit = defineEmits<{ published: [] }>()
const router = useRouter()
const editor = ref<Editor>()
const error = ref('')
const loading = ref(true)
const publishing = ref(false)
const title = ref(props.meta.title)
const slug = ref(props.meta.slug)
const description = ref(props.meta.seoDescription)
const lock = useEditLock(props.meta.id)
const save = useAutosave(props.meta.id, () => lock.token.value, () => !lock.readonly.value)
const { has } = usePermission()
const upload = useUpload()
async function uploadFile(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file || !editor.value || lock.readonly.value) return
  const kind = file.type.startsWith('image/') ? 'images' : file.type.startsWith('video/') ? 'videos' : 'files'
  try { const asset = await upload.upload(file, kind); if (!lock.readonly.value) insertUploadedAsset(editor.value, asset) }
  catch (cause) { error.value = errorMessage(cause) }
  finally { input.value = '' }
}

onMounted(async () => {
  try {
    const draft = requireData(await client.GET('/api/admin/docs/{id}/draft', { params: { path: { id: props.meta.id } } }))
    save.version.value = draft.version
    await lock.acquire()
    editor.value = new Editor({
      extensions: editorExtensions,
      content: draft.document.content as JSONContent,
      editable: !lock.readonly.value,
      editorProps: { attributes: { class: 'nw-content min-h-80 rounded-card border border-border bg-surface p-6 outline-none', 'aria-label': '文档正文' } },
      onUpdate: ({ editor }) => {
        lock.activity()
        const content = JSON.parse(JSON.stringify(editor.getJSON())) as components['schemas']['DocumentContent']['content']
        save.schedule({ schemaVersion, content })
      },
    })
  } catch (cause) { error.value = errorMessage(cause) }
  finally { loading.value = false }
})
watch(lock.readonly, readonly => editor.value?.setEditable(!readonly))
onBeforeUnmount(() => editor.value?.destroy())
async function preview() {
  error.value = ''
  try { await save.flush(); await router.push(`/preview/${props.meta.id}`) }
  catch (cause) { error.value = errorMessage(cause) }
}
async function flush() { await save.flush() }
defineExpose({ flush })
async function publish() {
  publishing.value = true; error.value = ''
  try {
    await save.flush()
    requireData(await client.POST('/api/admin/docs/{id}/publish', {
      params: { path: { id: props.meta.id } }, body: { lockToken: lock.token.value, draftVersion: save.version.value, title: title.value, slug: slug.value, seoDescription: description.value },
    }))
    emit('published')
  } catch (cause) { error.value = errorMessage(cause) }
  finally { publishing.value = false }
}
</script>
<template>
  <section class="grid gap-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <p class="text-sm text-muted">
        {{ save.saving.value ? '保存中…' : save.dirty.value ? '尚未保存' : '草稿已保存' }}
      </p>
      <button
        class="text-sm text-primary"
        @click="preview"
      >
        预览
      </button>
      <button
        v-if="has('publishDocs')"
        :disabled="loading || publishing || lock.readonly.value"
        class="nw-button"
        @click="publish"
      >
        {{ publishing ? '发布中…' : '发布' }}
      </button>
    </div>
    <p
      v-if="lock.notice.value"
      role="status"
      class="text-sm text-danger"
    >
      {{ lock.notice.value }}
    </p>
    <p
      v-if="lock.idleWarning.value"
      role="status"
      class="text-sm text-danger"
    >
      已经 15 分钟没有输入，请继续编辑以保持编辑锁。
    </p>
    <p
      v-if="error || save.error.value"
      role="alert"
      class="text-sm text-danger"
    >
      {{ error || save.error.value }}
    </p>
    <DocMetaPanel
      v-model:title="title"
      v-model:slug="slug"
      v-model:description="description"
      :readonly="lock.readonly.value"
    />
    <div
      v-if="editor && !lock.readonly.value"
      class="flex flex-wrap gap-2 text-sm"
    >
      <SlashMenu :editor="editor" />
      <label class="cursor-pointer rounded border border-border px-3 py-1">{{ upload.uploading.value ? '上传中…' : '上传资源' }}<input
        type="file"
        class="sr-only"
        :disabled="upload.uploading.value"
        @change="uploadFile"
      ></label>
      <button
        class="rounded border border-border px-3 py-1"
        @click="editor.chain().focus().toggleBold().run()"
      >
        加粗
      </button>
      <button
        class="rounded border border-border px-3 py-1"
        @click="editor.chain().focus().toggleItalic().run()"
      >
        斜体
      </button>
      <button
        class="rounded border border-border px-3 py-1"
        @click="editor.chain().focus().toggleBulletList().run()"
      >
        列表
      </button>
      <button
        class="rounded border border-border px-3 py-1"
        @click="editor.chain().focus().toggleCodeBlock().run()"
      >
        代码块
      </button>
    </div>
    <p
      v-if="loading"
      class="text-sm text-muted"
    >
      加载草稿…
    </p>
    <EditorContent
      v-else-if="editor"
      :editor="editor"
      @keydown="lock.activity"
    />
    <button
      v-if="save.dirty.value && !lock.readonly.value"
      class="text-start text-sm text-primary"
      @click="save.flush"
    >
      立即保存草稿
    </button>
  </section>
</template>
