<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { usePermission } from '../composables/usePermission'
const auth = useAuthStore()
const router = useRouter()
const { has } = usePermission()
async function logout() { await auth.logout(); await router.replace('/login') }
</script>

<template>
  <div class="min-h-screen md:grid md:grid-cols-[14rem_1fr]">
    <aside class="border-e border-border bg-surface p-6">
      <p class="mb-8 text-lg font-semibold">
        nice-wiki
      </p>
      <nav class="grid gap-4 text-sm">
        <RouterLink
          v-if="has('editDocs')"
          to="/docs"
        >
          文档
        </RouterLink>
        <RouterLink
          v-if="has('siteSettings')"
          to="/site"
        >
          站点设置
        </RouterLink>
        <RouterLink
          v-if="auth.account?.superadmin"
          to="/accounts"
        >
          账号管理
        </RouterLink>
        <RouterLink
          v-if="auth.account?.superadmin"
          to="/audit"
        >
          操作日志
        </RouterLink>
        <RouterLink to="/profile">
          个人账号
        </RouterLink>
        <a
          href="/"
          target="_blank"
          rel="noopener"
        >查看公开站点 ↗</a>
      </nav>
      <div class="mt-12 border-t border-border pt-4 text-sm">
        <p class="mb-3 text-muted">
          {{ auth.account?.username }}
        </p>
        <button
          class="text-danger"
          @click="logout"
        >
          退出登录
        </button>
      </div>
    </aside>
    <main class="min-w-0 p-6 md:p-10">
      <RouterView />
    </main>
  </div>
</template>
