<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import PageHeader from '../../components/PageHeader.vue'
import { client, errorMessage, requireData } from '../../../shared/api/client'

const form = reactive({ enabled: false, text: '', url: '', dismissible: true, startsAt: 0, endsAt: 0 })
const starts = ref(''); const ends = ref(''); const error = ref(''); const saved = ref(false); const busy = ref(false)
const localValue = (value: number) => value ? new Date(value * 1000 - new Date().getTimezoneOffset() * 60000).toISOString().slice(0, 16) : ''
const timestamp = (value: string) => value ? Math.floor(new Date(value).getTime() / 1000) : 0
onMounted(async () => {
  try { Object.assign(form, requireData(await client.GET('/api/admin/settings/banner'))); starts.value = localValue(form.startsAt); ends.value = localValue(form.endsAt) }
  catch (cause) { error.value = errorMessage(cause) }
})
async function save() {
  error.value = ''; saved.value = false; busy.value = true; form.startsAt = timestamp(starts.value); form.endsAt = timestamp(ends.value)
  try { Object.assign(form, requireData(await client.PUT('/api/admin/settings/banner', { body: form }))); saved.value = true }
  catch (cause) { error.value = errorMessage(cause) }
  finally { busy.value = false }
}
</script>
<template>
  <PageHeader
    title="公告横幅"
    description="可限制公告的生效时间，访客关闭后会记住当前公告。"
  />
  <form
    class="grid max-w-2xl gap-5"
    @submit.prevent="save"
  >
    <label class="flex items-center gap-2"><input
      v-model="form.enabled"
      type="checkbox"
    >启用公告</label>
    <label class="grid gap-2 text-sm">公告文字<textarea
      v-model="form.text"
      :required="form.enabled"
      maxlength="2000"
      rows="4"
      class="nw-input"
    /></label>
    <label class="grid gap-2 text-sm">链接<input
      v-model="form.url"
      type="url"
      class="nw-input"
    ></label>
    <div class="grid gap-4 sm:grid-cols-2">
      <label class="grid gap-2 text-sm">开始时间<input
        v-model="starts"
        type="datetime-local"
        class="nw-input"
      ></label><label class="grid gap-2 text-sm">结束时间<input
        v-model="ends"
        type="datetime-local"
        class="nw-input"
      ></label>
    </div>
    <label class="flex items-center gap-2"><input
      v-model="form.dismissible"
      type="checkbox"
    >允许访客关闭</label>
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
      公告已保存。
    </p>
    <button
      :disabled="busy"
      class="nw-button"
    >
      保存公告
    </button>
  </form>
</template>
