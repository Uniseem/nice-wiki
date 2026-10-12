<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import PageHeader from '../components/PageHeader.vue'
import { useAuthStore } from '../stores/auth'
import { client, requireData, errorMessage } from '../../shared/api/client'
const auth = useAuthStore()
const router = useRouter()
const error = ref('')
async function logoutAll() { try { requireData(await client.POST('/api/admin/auth/logout-all')); auth.account = null; await router.replace('/login') } catch (cause) { error.value = errorMessage(cause) } }
</script>
<template>
  <PageHeader title="个人账号" />
  <p class="mb-6 text-muted">
    当前账号：{{ auth.account?.username }}
  </p>
  <div class="flex flex-wrap gap-4">
    <RouterLink
      to="/password"
      class="nw-button"
    >
      修改密码
    </RouterLink><button
      class="rounded border border-border px-4 py-2 text-danger"
      @click="logoutAll"
    >
      退出所有设备
    </button>
  </div>
  <p
    v-if="error"
    role="alert"
    class="mt-5 text-danger"
  >
    {{ error }}
  </p>
</template>
