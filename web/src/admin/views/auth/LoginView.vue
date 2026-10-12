<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import AuthLayout from '../../layouts/AuthLayout.vue'
import { useAuthStore } from '../../stores/auth'
import { errorMessage } from '../../../shared/api/client'
const username = ref('')
const password = ref('')
const busy = ref(false)
const error = ref('')
const auth = useAuthStore()
const router = useRouter()
async function submit() {
  busy.value = true; error.value = ''
  try { await auth.login(username.value, password.value); password.value = ''; await router.replace('/docs') }
  catch (cause) { error.value = errorMessage(cause) }
  finally { busy.value = false }
}
</script>

<template>
  <AuthLayout
    title="登录后台"
    subtitle="使用管理员分配的账号和密码。"
  >
    <form
      class="grid gap-5"
      @submit.prevent="submit"
    >
      <label class="grid gap-2 text-sm">账号<input
        v-model="username"
        required
        autocomplete="username"
        maxlength="64"
        class="nw-input"
      ></label>
      <label class="grid gap-2 text-sm">密码<input
        v-model="password"
        required
        type="password"
        autocomplete="current-password"
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
        {{ busy ? '登录中…' : '登录' }}
      </button>
    </form>
  </AuthLayout>
</template>
