<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { ApiError, request, setCSRF } from './api';
import AuthPage from './AuthPage.vue';
import Shell from './Shell.vue';

type Phase = 'loading' | 'auth' | 'app';
const phase = ref<Phase>('loading');
const account = ref<any>(null);

async function boot() {
  try {
    const self = await request<{ account: any }>('/v1/self');
    account.value = self.account;
    phase.value = 'app';
  } catch (error) {
    if (!(error instanceof ApiError) || error.status !== 401) console.error(error);
    phase.value = 'auth';
  }
}

function signedIn(next: any) {
  account.value = next;
  phase.value = 'app';
  if (location.pathname === '/admin/login' || location.pathname === '/admin/setup' || location.pathname === '/admin' || location.pathname === '/admin/') {
    history.replaceState(null, '', '/admin/content');
  }
}

async function signedOut() {
  try { await request('/v1/sessions/current', { method: 'DELETE' }); } catch { /* 退出以清本地状态为准 */ }
  setCSRF(null);
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
