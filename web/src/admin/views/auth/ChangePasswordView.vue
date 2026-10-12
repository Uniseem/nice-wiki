<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import AuthLayout from '../../layouts/AuthLayout.vue'
import { useAuthStore } from '../../stores/auth'
import { client, requireData, errorMessage } from '../../../shared/api/client'
const oldPassword = ref('')
const newPassword = ref('')
const confirmation = ref('')
const error = ref('')
const busy = ref(false)
const auth = useAuthStore()
const router = useRouter()
async function submit() {
  busy.value = true; error.value = ''
  try {
    requireData(await client.PUT('/api/admin/auth/password', { body: { oldPassword: oldPassword.value, newPassword: newPassword.value, confirmation: confirmation.value } }))
    oldPassword.value = ''; newPassword.value = ''; confirmation.value = ''
    await auth.refresh(); await router.replace('/docs')
  } catch (cause) { error.value = errorMessage(cause) }
  finally { busy.value = false }
}
</script>
<template>
  <AuthLayout
    title="修改密码"
    subtitle="修改后，其他设备的登录会失效。"
  >
    <form
      class="grid gap-5"
      @submit.prevent="submit"
    >
      <label class="grid gap-2 text-sm">旧密码<input
        v-model="oldPassword"
        required
        type="password"
        autocomplete="current-password"
        class="nw-input"
      ></label>
      <label class="grid gap-2 text-sm">新密码<input
        v-model="newPassword"
        required
        minlength="8"
        type="password"
        autocomplete="new-password"
        class="nw-input"
      ></label>
      <label class="grid gap-2 text-sm">确认新密码<input
        v-model="confirmation"
        required
        type="password"
        autocomplete="new-password"
        class="nw-input"
      ></label>
      <p
        v-if="error"
        role="alert"
        class="text-sm text-danger"
      >
        {{ error }}
      </p>
      <button
        :disabled="busy"
        class="nw-button"
      >
        保存新密码
      </button>
    </form>
  </AuthLayout>
</template>
