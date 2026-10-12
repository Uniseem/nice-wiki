<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import AuthLayout from '../../layouts/AuthLayout.vue'
import { useAuthStore } from '../../stores/auth'
import { errorMessage } from '../../../shared/api/client'
import RestoreStep from '../setup/steps/RestoreStep.vue'
const username = ref('')
const password = ref('')
const confirmation = ref('')
const error = ref('')
const busy = ref(false)
const auth = useAuthStore()
const router = useRouter()
async function submit() {
  error.value = ''
  if (password.value !== confirmation.value) { error.value = '两次密码不一致'; return }
  busy.value = true
  try { await auth.register(username.value, password.value); password.value = ''; confirmation.value = ''; await router.replace('/setup') }
  catch (cause) { error.value = errorMessage(cause) }
  finally { busy.value = false }
}
</script>
<template>
  <AuthLayout
    title="注册超级管理员"
    subtitle="这是站点唯一的超级管理员账号，请妥善保存密码。"
  >
    <form
      class="grid gap-5"
      @submit.prevent="submit"
    >
      <label class="grid gap-2 text-sm">账号<input
        v-model="username"
        required
        maxlength="64"
        autocomplete="username"
        class="nw-input"
      ></label>
      <label class="grid gap-2 text-sm">密码<input
        v-model="password"
        type="password"
        required
        minlength="8"
        autocomplete="new-password"
        class="nw-input"
      ></label>
      <label class="grid gap-2 text-sm">确认密码<input
        v-model="confirmation"
        type="password"
        required
        minlength="8"
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
        {{ busy ? '注册中…' : '创建账号' }}
      </button>
    </form>
    <RestoreStep />
  </AuthLayout>
</template>
