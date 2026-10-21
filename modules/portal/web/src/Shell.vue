<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import type { ModuleUIBootstrap } from '../../../../libs/platform/web/module-api';
import { apiClient, currentGrants } from './api';
import Dashboard from './Dashboard.vue';
import Profile from './Profile.vue';

const props = defineProps<{ account: any }>();
defineEmits<{ (e: 'sign-out'): void }>();

// 已有界面的模块；portal 只负责导航与挂载，页面由各域自己的 module.js 提供。
const modules = [
  { id: 'dashboard', label: '概览', grants: [] as string[] },
  { id: 'content', label: '文档', grants: ['content.read'] },
  { id: 'site-config', label: '站点设置', grants: ['site.manage'] },
  { id: 'media', label: '资源库', grants: ['media.manage', 'media.upload'] },
  { id: 'identity', label: '账号与权限', grants: ['account.manage'] },
  { id: 'localization', label: '语言', grants: ['localization.manage', 'localization.languages'] },
  { id: 'model-gateway', label: '模型接入', grants: ['model.manage'] },
  { id: 'knowledge', label: '知识库索引', grants: ['knowledge.manage'] },
  { id: 'memory', label: '长期记忆', grants: ['memory.manage'] },
  { id: 'qa', label: '问答管理', grants: ['qa.manage', 'qa.debug'] },
  { id: 'traffic-guard', label: '防滥用', grants: ['guard.manage'] },
  { id: 'jobs', label: '任务队列', grants: ['job.manage'] },
  { id: 'analytics', label: '统计与审计', grants: ['stats.read'] },
  { id: 'data-protection', label: '备份与恢复', grants: ['backup.manage'] },
];

const path = ref(location.pathname);
// 按授权项过滤导航（只是少展示用不了的页面；真正的权限在各服务端校验）。
const visible = computed(() => {
  if (props.account?.mustChangePassword) return [] as typeof modules;
  const grants = currentGrants();
  return modules.filter((m) => !m.grants.length || m.grants.some((g) => grants.includes(g)));
});
const isProfile = computed(() => path.value === '/admin/me' || !!props.account?.mustChangePassword);
const current = computed(() => visible.value.find((m) => path.value === `/admin/${m.id}` || path.value.startsWith(`/admin/${m.id}/`)) ?? visible.value[0] ?? modules[0]);
const builtin = computed(() => isProfile.value || current.value.id === 'dashboard');
const mountPoint = ref<HTMLElement | null>(null);
const failure = ref('');

function go(id: string) {
  history.pushState(null, '', `/admin/${id}`);
  path.value = location.pathname;
}
const onPop = () => { path.value = location.pathname; };

async function mountCurrent() {
  failure.value = '';
  // 先让当前挂着的模块界面卸载（此时旧挂载点还在 DOM 里）。
  mountPoint.value?.dispatchEvent(new Event('nice-wiki:unmount'));
  await nextTick();
  if (builtin.value) return;
  const el = mountPoint.value;
  if (!el) return;
  el.replaceChildren();
  try {
    if (!document.querySelector(`link[data-module="${current.value.id}"]`)) {
      const link = document.createElement('link');
      link.rel = 'stylesheet';
      link.href = `/_admin/modules/${current.value.id}/${current.value.id}-ui.css`;
      link.dataset.module = current.value.id;
      link.onerror = () => link.remove(); // 没有独立样式的模块没有这个文件
      document.head.append(link);
    }
    const mod = await import(/* @vite-ignore */ `/_admin/modules/${current.value.id}/module.js?v=${Date.now()}`);
    const bootstrap: ModuleUIBootstrap = {
      module: current.value.id, apiBaseURL: '', locale: 'zh-Hans', mountSelector: '#module-root',
      capabilities: currentGrants(), buildRevision: String(Date.now()), apiClient,
    };
    mod.mountModuleUI(el, bootstrap);
  } catch (e) {
    failure.value = `页面加载失败：${e instanceof Error ? e.message : e}`;
  }
}

onMounted(() => { addEventListener('popstate', onPop); mountCurrent(); });
onBeforeUnmount(() => removeEventListener('popstate', onPop));
watch(() => current.value.id, mountCurrent);
</script>

<template>
  <div class="shell">
    <nav>
      <div class="brand">后台管理</div>
      <a v-for="m in visible" :key="m.id" :href="`/admin/${m.id}`" :class="{ active: current.id === m.id }" @click.prevent="go(m.id)">{{ m.label }}</a>
      <div class="spacer"></div>
      <a v-if="!account?.mustChangePassword" href="/admin/me" :class="{ active: isProfile }" @click.prevent="go('me')">个人设置（{{ account?.accountName }}）</a>
      <span v-else class="hint">{{ account?.accountName }}</span>
      <a href="/" target="_blank" rel="noopener">查看站点</a>
      <button @click="$emit('sign-out')">退出登录</button>
    </nav>
    <main>
      <p v-if="failure" class="error">{{ failure }}</p>
      <Profile v-if="isProfile" :account="account" />
      <Dashboard v-else-if="builtin" />
      <div v-else id="module-root" ref="mountPoint"></div>
    </main>
  </div>
</template>
