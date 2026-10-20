<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { ApiError, currentGrants, currentSessionId, refresh, request, setCSRF } from './api';
import AuthPage from './AuthPage.vue';
import Shell from './Shell.vue';

type Phase = 'loading' | 'auth' | 'app';
const phase = ref<Phase>('loading');
const account = ref<any>(null);

// 站点初始化只写了站点配置；基准语言要在本地化域登记成默认语言，公开页才能渲染。已登录时补一次，幂等。
async function ensureBaseLanguage() {
  try {
    const languages = await request<{ languages: any[] }>('/v1/languages?includeDisabled=true');
    if (languages.languages.length) return;
    const config = await request<{ config: { baseLanguage: string } }>('/v1/config');
    const tag = config.config.baseLanguage;
    const idempotencyKey = crypto.randomUUID().replaceAll('-', '');
    await request(`/v1/languages/${tag}`, { method: 'PUT', idempotencyKey, body: { expectedVersion: 0, idempotencyKey, language: {
      tag, version: 1, enabled: true, isDefault: true, automaticTranslation: false, direction: 'ltr', reflectionEnabled: false, batchOutputTokens: 1000, translationMemoryMinimumCharacters: 20 } } });
  } catch (e) { console.warn('基准语言登记失败', e); }
}

async function boot() {
  try {
    const self = await request<{ account: any }>('/v1/self');
    account.value = self.account;
    // 授权项只在登录/刷新响应里；新开标签页时会话存储为空，静默刷新一次补上。
    if (!currentGrants().length) await refresh();
    await ensureBaseLanguage();
    phase.value = 'app';
  } catch (error) {
    if (!(error instanceof ApiError) || error.status !== 401) console.error(error);
    phase.value = 'auth';
  }
}

async function signedIn(next: any) {
  account.value = next;
  await ensureBaseLanguage();
  phase.value = 'app';
  if (location.pathname === '/admin/login' || location.pathname === '/admin/setup' || location.pathname === '/admin' || location.pathname === '/admin/') {
    history.replaceState(null, '', '/admin/dashboard');
  }
}

async function signedOut() {
  const id = currentSessionId();
  if (id) {
    try { await request(`/v1/sessions/${id}`, { method: 'DELETE', body: {} }); } catch { /* 退出以清本地状态为准 */ }
  }
  setCSRF(null, null);
  account.value = null;
  phase.value = 'auth';
  history.replaceState(null, '', '/admin/login');
}

onMounted(boot);
</script>

<template>
  <p v-if="phase === 'loading'" class="hint" style="text-align:center;margin-top:20vh">加载中…</p>
  <AuthPage v-else-if="phase === 'auth'" @signed-in="signedIn" />
  <Shell v-else :account="account" @sign-out="signedOut" />
</template>
