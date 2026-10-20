<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { errorText, request, setCSRF } from './api';

const emit = defineEmits<{ (e: 'signed-in', account: any): void }>();

// 向导：未有账号 → 注册首个超管；已有账号 → 登录；登录后若站点未初始化 → 填站点名与基准语言。
type Step = 'loading' | 'register' | 'login' | 'site';
const step = ref<Step>('loading');
const accountName = ref('admin');
const password = ref('');
const confirmation = ref('');
const siteName = ref('');
const baseLanguage = ref('zh-Hans');
const error = ref('');
const busy = ref(false);
let account: any = null;

onMounted(async () => {
  try {
    const state = await request<{ registrationOpen: boolean }>('/v1/registration');
    step.value = state.registrationOpen ? 'register' : 'login';
  } catch (e) {
    error.value = errorText(e);
    step.value = 'login';
  }
});

async function run(action: () => Promise<void>) {
  busy.value = true;
  error.value = '';
  try { await action(); } catch (e) { error.value = errorText(e); } finally { busy.value = false; }
}

async function afterLogin(result: any) {
  setCSRF(result.session.csrfToken, result.session.sessionId);
  account = result.account ?? (await request<{ account: any }>('/v1/self')).account;
  const init = await request<{ initialized: boolean }>('/v1/initialization');
  if (init.initialized) emit('signed-in', account);
  else step.value = 'site';
}

const register = () => run(async () => {
  if (password.value !== confirmation.value) throw new Error('两次输入的口令不一致');
  await afterLogin(await request('/v1/registration', { method: 'POST', body: { accountName: accountName.value, password: password.value, passwordConfirmation: confirmation.value } }));
});

const login = () => run(async () => {
  await afterLogin(await request('/v1/sessions', { method: 'POST', body: { accountName: accountName.value, password: password.value } }));
});

const initialize = () => run(async () => {
  await request('/v1/initialization', { method: 'POST', body: { name: siteName.value, baseLanguage: baseLanguage.value } });
  // 基准语言要在本地化域登记为默认语言，公开页才能按该语言渲染。
  const languages = await request<{ languages: any[] }>('/v1/languages?includeDisabled=true');
  if (!languages.languages.some((l) => l.tag === baseLanguage.value)) {
    await request(`/v1/languages/${baseLanguage.value}`, { method: 'PUT', body: {
      expectedVersion: 0, idempotencyKey: crypto.randomUUID().replaceAll('-', ''),
      language: { tag: baseLanguage.value, version: 1, enabled: true, isDefault: true, automaticTranslation: false, direction: 'ltr', reflectionEnabled: false, batchOutputTokens: 1000, translationMemoryMinimumCharacters: 20 },
    } });
  }
  emit('signed-in', account);
});
</script>

<template>
  <form class="card" @submit.prevent="step === 'register' ? register() : step === 'login' ? login() : initialize()">
    <template v-if="step === 'loading'"><p class="hint">加载中…</p></template>
    <template v-else-if="step === 'site'">
      <h1>初始化站点</h1>
      <label for="site-name">站点名称</label>
      <input id="site-name" v-model="siteName" required maxlength="120" autofocus>
      <label for="site-lang">基准语言</label>
      <select id="site-lang" v-model="baseLanguage">
        <option value="zh-Hans">简体中文 (zh-Hans)</option>
        <option value="en">English (en)</option>
      </select>
      <p class="hint">初始化不调用模型，不产生费用。</p>
    </template>
    <template v-else>
      <h1>{{ step === 'register' ? '创建管理员账号' : '登录后台' }}</h1>
      <p v-if="step === 'register'" class="hint">这是第一个账号，将成为唯一的超级管理员。</p>
      <label for="acct">账号名</label>
      <input id="acct" v-model="accountName" required autocomplete="username">
      <label for="pw">口令</label>
      <input id="pw" v-model="password" type="password" required autocomplete="current-password">
      <template v-if="step === 'register'">
        <label for="pw2">确认口令</label>
        <input id="pw2" v-model="confirmation" type="password" required autocomplete="new-password">
      </template>
    </template>
    <p v-if="error" class="error">{{ error }}</p>
    <button class="primary" :disabled="busy || step === 'loading'" style="width:100%;margin-top:1rem">
      {{ busy ? '处理中…' : step === 'register' ? '创建并继续' : step === 'login' ? '登录' : '完成初始化' }}
    </button>
  </form>
</template>
