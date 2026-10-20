<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import type { ModuleUIBootstrap } from '../../../../libs/platform/web/module-api';
import { apiClient, currentGrants } from './api';

defineProps<{ account: any }>();
defineEmits<{ (e: 'sign-out'): void }>();

// 已有界面的模块；portal 只负责导航与挂载，页面由各域自己的 module.js 提供。
const modules = [
  { id: 'content', label: '文档' },
  { id: 'site-config', label: '站点设置' },
  { id: 'identity', label: '账号与权限' },
  { id: 'jobs', label: '任务队列' },
  { id: 'analytics', label: '统计与审计' },
  { id: 'data-protection', label: '备份与恢复' },
];

const path = ref(location.pathname);
const current = computed(() => modules.find((m) => path.value === `/admin/${m.id}` || path.value.startsWith(`/admin/${m.id}/`)) ?? modules[0]);
const mountPoint = ref<HTMLElement | null>(null);
const failure = ref('');

function go(id: string) {
  history.pushState(null, '', `/admin/${id}`);
  path.value = location.pathname;
}
const onPop = () => { path.value = location.pathname; };

async function mountCurrent() {
  failure.value = '';
  await nextTick();
  const el = mountPoint.value;
  if (!el) return;
  el.dispatchEvent(new Event('nice-wiki:unmount'));
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
      <div id="module-root" ref="mountPoint"></div>
    </main>
  </div>
</template>
