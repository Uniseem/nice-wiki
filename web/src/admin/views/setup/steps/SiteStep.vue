<script setup lang="ts">
import { reactive, ref, onMounted } from 'vue'
import { client, requireData, errorMessage } from '../../../../shared/api/client'
const emit = defineEmits<{ done: [] }>()
const form = reactive({ name: '', description: '', baseURL: '', logoHash: '', themeColor: '#2563eb', footer: '' })
const error = ref('')
const busy = ref(false)
onMounted(async () => { try { Object.assign(form, requireData(await client.GET('/api/admin/settings/site'))) } catch (cause) { error.value = errorMessage(cause) } })
async function save() { busy.value = true; error.value = ''; try { requireData(await client.PUT('/api/admin/settings/site', { body: form })); emit('done') } catch (cause) { error.value = errorMessage(cause) } finally { busy.value = false } }
</script>
<template>
  <form
    class="grid gap-5"
    @submit.prevent="save"
  >
    <label class="grid gap-2 text-sm">站点名称<input
      v-model="form.name"
      required
      maxlength="120"
      class="nw-input"
    ></label>
    <label class="grid gap-2 text-sm">站点地址<input
      v-model="form.baseURL"
      type="url"
      placeholder="https://example.com"
      class="nw-input"
    ></label>
    <p
      v-if="error"
      role="alert"
      class="text-danger"
    >
      {{ error }}
    </p>
    <button
      :disabled="busy"
      class="nw-button"
    >
      保存并继续
    </button>
  </form>
</template>
