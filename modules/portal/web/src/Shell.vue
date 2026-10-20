<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import type { ModuleUIBootstrap } from '../../../../libs/platform/web/module-api';
import { apiClient, currentGrants } from './api';
import Dashboard from './Dashboard.vue';

defineProps<{ account: any }>();
defineEmits<{ (e: 'sign-out'): void }>();

// 已有界面的模块；portal 只负责导航与挂载，页面由各域自己的 module.js 提供。
const modules = [
  { id: 'dashboard', label: '概览' },
  { id: 'content', label: '文档' },
  { id: 'site-config', label: '站点设置' },
  { id: 'media', label: '资源库' },
  { id: 'identity', label: '账号与权限' },
  { id: 'localization', label: '语言' },
  { id: 'model-gateway', label: '模型接入' },
  { id: 'knowledge', label: '知识库索引' },
  { id: 'memory', label: '长期记忆' },
  { id: 'qa', label: '问答管理' },
  { id: 'traffic-guard', label: '防滥用' },
  { id: 'jobs', label: '任务队列' },
  { id: 'analytics', label: '统计与审计' },
  { id: 'data-protection', label: '备份与恢复' },
];

const path = ref(location.pathname);
const current = computed(() => modules.find((m) => path.value === `/admin/${m.id}` || path.value.startsWith(`/admin/${m.id}/`)) ?? modules[0]);
const builtin = computed(() => current.value.id === 'dashboard');
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
      <a v-for="m in modules" :key="m.id" :href="`/admin/${m.id}`" :class="{ active: current.id === m.id }" @click.prevent="go(m.id)">{{ m.label }}</a>
      <div class="spacer"></div>
      <span class="hint">{{ account?.accountName }}</span>
      <a href="/" target="_blank" rel="noopener">查看站点</a>
      <button @click="$emit('sign-out')">退出登录</button>
    </nav>
    <main>
      <p v-if="failure" class="error">{{ failure }}</p>
      <Dashboard v-if="builtin" />
      <div v-else id="module-root" ref="mountPoint"></div>
    </main>
  </div>
</template>
