<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import PageHeader from '../../components/PageHeader.vue'
import { client, errorMessage, requireData } from '../../../shared/api/client'
import type { components } from '../../../shared/api/schema.gen'

type Home = components['schemas']['HomeSettings']
const form = reactive<Home>({
  title: '', subtitle: '', backgroundText: 'Welcome To', backgroundHash: '', backgroundKind: 'none',
  videoPosterHash: '', overlay: 35, textColor: 'light', translateBackgroundText: false, links: [],
})
const error = ref(''); const saved = ref(false); const busy = ref(false)
onMounted(async () => { try { Object.assign(form, requireData(await client.GET('/api/admin/settings/home'))) } catch (cause) { error.value = errorMessage(cause) } })
function addLink() { if (form.links.length < 3) form.links.push({ label: '', documentId: '', url: '', style: 'secondary' }) }
function removeLink(index: number) { form.links.splice(index, 1) }
async function save() {
  error.value = ''; saved.value = false; busy.value = true
  try { Object.assign(form, requireData(await client.PUT('/api/admin/settings/home', { body: form }))); saved.value = true }
  catch (cause) { error.value = errorMessage(cause) }
  finally { busy.value = false }
}
</script>
<template>
  <PageHeader
    title="首页主视觉"
    description="首页只显示一屏主视觉；按钮最多三个。"
  />
  <form
    class="grid max-w-3xl gap-5"
    @submit.prevent="save"
  >
    <label class="grid gap-2 text-sm">主标题<input
      v-model="form.title"
      maxlength="200"
      class="nw-input"
    ></label>
    <label class="grid gap-2 text-sm">副标题<textarea
      v-model="form.subtitle"
      maxlength="1000"
      rows="3"
      class="nw-input"
    /></label>
    <label class="grid gap-2 text-sm">背景装饰字<input
      v-model="form.backgroundText"
      maxlength="200"
      class="nw-input"
    ></label>
    <label class="flex items-center gap-2 text-sm"><input
      v-model="form.translateBackgroundText"
      type="checkbox"
    >后续翻译背景装饰字</label>
    <div class="grid gap-4 sm:grid-cols-2">
      <label class="grid gap-2 text-sm">背景类型<select
        v-model="form.backgroundKind"
        class="nw-input"
      ><option value="none">纯色</option><option value="image">图片</option><option value="video">视频</option></select></label>
      <label class="grid gap-2 text-sm">文字颜色<select
        v-model="form.textColor"
        class="nw-input"
      ><option value="light">浅色</option><option value="dark">深色</option></select></label>
      <label class="grid gap-2 text-sm">背景资源哈希<input
        v-model="form.backgroundHash"
        :required="form.backgroundKind !== 'none'"
        pattern="[a-f0-9]{64}"
        class="nw-input"
      ></label>
      <label class="grid gap-2 text-sm">视频封面哈希<input
        v-model="form.videoPosterHash"
        pattern="[a-f0-9]{64}"
        class="nw-input"
      ></label>
      <label class="grid gap-2 text-sm">遮罩深浅：{{ form.overlay }}%<input
        v-model.number="form.overlay"
        type="range"
        min="0"
        max="100"
      ></label>
    </div>
    <section class="grid gap-4 rounded-card border border-border p-4">
      <div class="flex items-center justify-between">
        <h2 class="font-semibold">
          按钮
        </h2><button
          type="button"
          :disabled="form.links.length >= 3"
          class="text-sm text-primary"
          @click="addLink"
        >
          添加按钮
        </button>
      </div>
      <div
        v-for="(link, index) in form.links"
        :key="index"
        class="grid gap-3 border-t border-border pt-4 sm:grid-cols-2"
      >
        <label class="grid gap-1 text-sm">文字<input
          v-model="link.label"
          required
          maxlength="80"
          class="nw-input"
        ></label>
        <label class="grid gap-1 text-sm">样式<select
          v-model="link.style"
          class="nw-input"
        ><option value="primary">主要</option><option value="secondary">次要</option></select></label>
        <label class="grid gap-1 text-sm">站内文档 ID<input
          v-model="link.documentId"
          class="nw-input"
          @input="link.url = ''"
        ></label>
        <label class="grid gap-1 text-sm">或外部地址<input
          v-model="link.url"
          type="url"
          class="nw-input"
          @input="link.documentId = ''"
        ></label>
        <button
          type="button"
          class="text-start text-sm text-danger"
          @click="removeLink(index)"
        >
          删除按钮
        </button>
      </div>
    </section>
    <p
      v-if="error"
      role="alert"
      class="text-danger"
    >
      {{ error }}
    </p><p
      v-if="saved"
      role="status"
      class="text-primary"
    >
      首页设置已保存。
    </p>
    <button
      :disabled="busy"
      class="nw-button"
    >
      保存首页
    </button>
  </form>
</template>
