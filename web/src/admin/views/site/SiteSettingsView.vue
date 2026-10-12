<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import PageHeader from '../../components/PageHeader.vue'
import { client, requireData, errorMessage } from '../../../shared/api/client'
const form = reactive({ name: '', description: '', baseURL: '', themeColor: '#2563eb', logoHash: '', footer: '' })
const error = ref('')
const saved = ref(false)
const busy = ref(false)
onMounted(async () => { try { Object.assign(form, requireData(await client.GET('/api/admin/settings/site'))) } catch (cause) { error.value = errorMessage(cause) } })
async function save() { error.value = ''; saved.value = false; busy.value = true; try { requireData(await client.PUT('/api/admin/settings/site', { body: form })); saved.value = true } catch (cause) { error.value = errorMessage(cause) } finally { busy.value = false } }
</script>
<template>
  <PageHeader title="站点设置" />
  <form
    class="grid max-w-2xl gap-5"
    @submit.prevent="save"
  >
    <label class="grid gap-2 text-sm">站点名称<input
      v-model="form.name"
      required
      maxlength="120"
      class="nw-input"
    ></label>
    <label class="grid gap-2 text-sm">站点简介<textarea
      v-model="form.description"
      maxlength="500"
      rows="3"
      class="nw-input"
    /></label>
    <label class="grid gap-2 text-sm">站点地址<input
      v-model="form.baseURL"
      type="url"
      class="nw-input"
    ></label>
    <label class="grid gap-2 text-sm">主题色<input
      v-model="form.themeColor"
      type="color"
      class="h-10 w-20 rounded border border-border"
    ></label>
    <label class="grid gap-2 text-sm">页脚文字<textarea
      v-model="form.footer"
      maxlength="4000"
      rows="4"
      class="nw-input"
    /></label>
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
      设置已保存。
    </p>
    <button
      :disabled="busy"
      class="nw-button"
    >
      保存设置
    </button>
  </form>
</template>
