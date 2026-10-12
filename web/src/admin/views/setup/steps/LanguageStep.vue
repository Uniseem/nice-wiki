<script setup lang="ts">
import { ref } from 'vue'
import { client, requireData, errorMessage } from '../../../../shared/api/client'
const emit = defineEmits<{ done: [] }>()
const base = ref('zh-Hans')
const error = ref('')
const busy = ref(false)
async function save() { busy.value = true; error.value = ''; try { requireData(await client.PUT('/api/admin/setup/language', { body: { base: base.value, enabled: [base.value] } })); emit('done') } catch (cause) { error.value = errorMessage(cause) } finally { busy.value = false } }
</script>
<template>
  <form
    class="grid gap-5"
    @submit.prevent="save"
  >
    <label class="grid gap-2 text-sm">文档基准语言<select
      v-model="base"
      class="nw-input"
    ><option value="zh-Hans">简体中文</option><option value="zh-Hant">繁體中文</option><option value="en">English</option><option value="ja">日本語</option><option value="ko">한국어</option><option value="fr">Français</option><option value="de">Deutsch</option><option value="es">Español</option><option value="pt-BR">Português</option><option value="ru">Русский</option></select></label>
    <p class="text-sm text-muted">
      初始化后固定基准语言。目标语言在多语言阶段开放。
    </p>
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
