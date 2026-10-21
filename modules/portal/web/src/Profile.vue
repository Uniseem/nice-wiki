<script setup lang="ts">
import { ref } from 'vue';
import { ApiError, request, setCSRF } from './api';

const props = defineProps<{ account: any }>();
const oldPassword = ref(''), newPassword = ref(''), confirmation = ref('');
const busy = ref(false), error = ref(''), done = ref(false);

async function submit() {
  error.value = '';
  if (newPassword.value !== confirmation.value) { error.value = '两次输入的新口令不一致。'; return; }
  if (newPassword.value === oldPassword.value) { error.value = '新口令不能与旧口令相同。'; return; }
  busy.value = true;
  try {
    const self = await request<{ credentialRevision: number }>('/v1/self');
    const changed = await request<{ session?: { csrfToken: string; sessionId: string; auth?: { grants?: string[] } } }>('/v1/self/password:change', { method: 'POST', body: { oldPassword: oldPassword.value, newPassword: newPassword.value, confirmation: confirmation.value, expectedCredentialRevision: self.credentialRevision } });
    // 改口令换了新会话（带完整授权），先存下来，否则重载后还是改口令前的受限授权。
    if (changed.session) setCSRF(changed.session.csrfToken, changed.session.sessionId, changed.session.auth?.grants);
    done.value = true;
    // 口令改完后其他设备的会话会失效，本会话换了新令牌；整页重载以取回新的授权。
    setTimeout(() => location.assign('/admin/dashboard'), 1200);
  } catch (e) {
    error.value = e instanceof ApiError && e.status === 400 ? '口令不符合要求（旧口令不对，或新口令太简单/太短）。' : (e as Error).message;
  } finally { busy.value = false; }
}
</script>

<template>
  <div class="page">
    <h1>个人设置</h1>
    <p class="hint">账号：{{ account?.accountName }}</p>
    <p v-if="account?.mustChangePassword" class="error">这是超级管理员给你设置的初始口令，请先修改口令，之后才能使用后台的其他功能。</p>
    <p v-if="done" class="ok">口令已修改，正在进入后台…</p>
    <form @submit.prevent="submit">
      <label>旧口令</label><input v-model="oldPassword" type="password" autocomplete="current-password" required>
      <label>新口令</label><input v-model="newPassword" type="password" autocomplete="new-password" required minlength="12">
      <label>确认新口令</label><input v-model="confirmation" type="password" autocomplete="new-password" required minlength="12">
      <p v-if="error" class="error">{{ error }}</p>
      <div style="margin-top:.8rem"><button class="primary" :disabled="busy || done">修改口令</button></div>
    </form>
  </div>
</template>
