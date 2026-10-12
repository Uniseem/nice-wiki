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
        <RouterLink to="/">管理概览</RouterLink>
        <RouterLink
          v-if="has('editDocs')"
          to="/docs"
        >
          文档
        </RouterLink>
        <RouterLink v-if="has('viewStats')" to="/stats">访问统计</RouterLink>
        <RouterLink v-if="auth.account?.superadmin" to="/backups">备份恢复</RouterLink>
        <RouterLink v-if="auth.account?.superadmin" to="/jobs">后台任务</RouterLink>
        <RouterLink v-if="auth.account?.superadmin" to="/system">系统设置</RouterLink>
        <RouterLink v-if="auth.account?.superadmin" to="/models">模型渠道</RouterLink>
        <RouterLink v-if="has('aiSettings')" to="/ai-settings">AI 设置</RouterLink>
        <RouterLink v-if="has('manageQA')" to="/qa">问答记录</RouterLink>
        <RouterLink v-if="has('manageQA')" to="/qa-cache">回答缓存</RouterLink>
        <RouterLink v-if="has('aiSettings')" to="/qa-debug">问答调试</RouterLink>
        <RouterLink v-if="has('aiSettings')" to="/guard">防滥用</RouterLink>
        <RouterLink v-if="has('aiMemory')" to="/memory">长期记忆</RouterLink>
        <RouterLink v-if="has('aiMemory')" to="/memory-tasks">记忆映射</RouterLink>
        <RouterLink v-if="has('manageLanguages')" to="/languages">语言管理</RouterLink>
        <RouterLink v-if="has('manageTranslations')" to="/translations">翻译进度</RouterLink>
        <RouterLink v-if="has('manageTranslations')" to="/glossary">术语表</RouterLink>
        <RouterLink
          v-if="has('publishDocs')"
          to="/trash"
        >
          回收站
        </RouterLink>
        <RouterLink
          v-if="has('siteSettings')"
          to="/site"
        >
          站点设置
        </RouterLink>
        <RouterLink
          v-if="has('siteSettings')"
          to="/site/home"
        >
          首页主视觉
        </RouterLink>
        <RouterLink
          v-if="has('siteSettings')"
          to="/site/banner"
        >
          公告横幅
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
