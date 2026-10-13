<script setup lang="ts">
import { reactive, ref, onMounted } from 'vue'
import { client, requireData, errorMessage } from '../../../../shared/api/client'
const emit = defineEmits<{ done: [] }>()
const form = reactive({ name: '', provider: '', baseURL: '', protocol: 'openai' as const, apiKey: '', enabled: true, timeoutSeconds: 90, streamTimeoutSeconds: 30, concurrency: 4 })
const error = ref('')
const busy = ref(false)
const existing = ref(false)
onMounted(async () => { try { existing.value = requireData(await client.GET('/api/admin/models/channels')).length > 0 } catch (cause) { error.value = errorMessage(cause) } })
async function save() { busy.value = true; error.value = ''; try { requireData(await client.POST('/api/admin/models/channels', { body: form })); form.apiKey = ''; emit('done') } catch (cause) { error.value = errorMessage(cause) } finally { busy.value = false } }
</script>
<template>
  <form
    class="grid gap-5"
    @submit.prevent="save"
  >
    <p class="text-sm text-muted">
      登记一个兼容 OpenAI 协议的渠道。此步骤保存配置，不发送模型请求；AI 功能将在后续接入。
    </p>
    <label class="grid gap-2 text-sm">渠道名称<input
      v-model="form.name"
      required
      maxlength="120"
      class="nw-input"
    ></label>
    <label class="grid gap-2 text-sm">models.dev provider ID（可选）<input
      v-model="form.provider"
      maxlength="128"
      class="nw-input"
      placeholder="例如 openai"
    ></label>
    <label class="grid gap-2 text-sm">接口地址<input
      v-model="form.baseURL"
      required
      type="url"
      placeholder="https://example.com/v1"
      class="nw-input"
    ></label>
    <label class="grid gap-2 text-sm">API 密钥<input
      v-model="form.apiKey"
      required
      type="password"
      autocomplete="off"
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
      保存渠道并完成
    </button>
    <button
      v-if="existing"
      type="button"
      class="text-sm text-primary"
      @click="emit('done')"
    >
      使用已登记渠道
    </button>
  </form>
</template>
