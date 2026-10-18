<script setup lang="ts">
import { ref, computed, watch, onBeforeUnmount } from 'vue';
import type { ModuleUIBootstrap } from '../../../../libs/platform/web/module-api';
import type { components } from './gen/api.gen';
const props = defineProps<{ bootstrap: ModuleUIBootstrap; parentSignal: AbortSignal; initialBackup?: string }>();
type ApproveFullRestoreInput = { backupId: string; targetEpoch: string };
type ApproveFullRestoreResult = { restoreId: string };
const backupId = ref(props.initialBackup ?? ''), targetEpoch = ref(crypto.randomUUID());
const validation = ref<components['schemas']['ValidateRestoreResult']>(), approved = ref(false);
const progress = ref<components['schemas']['ReadRestoreProgressResult']>(), restoreId = ref('');
const error = ref(''), busy = ref(false), isSuperadmin = computed(() => props.bootstrap.capabilities.includes('superadmin'));
const controller = new AbortController(), signal = AbortSignal.any([controller.signal, props.parentSignal]);
let validatedInput = '', operationId = '', operationInput = '', progressToken = '', progressExpiresAt = '', timer: ReturnType<typeof setTimeout> | undefined;
watch(() => props.initialBackup, value => { if (!restoreId.value) backupId.value = value ?? ''; });
watch(backupId, () => { validation.value = undefined; approved.value = false; targetEpoch.value = crypto.randomUUID(); operationId = ''; validatedInput = ''; });
onBeforeUnmount(() => { controller.abort(); if (timer) clearTimeout(timer); progressToken = ''; });
function failRestore(problem: unknown): void {
  if (signal.aborted) return;
  const { code, status } = (problem ?? {}) as { code?: string; status?: number };
  if (code === 'UNAUTHENTICATED' || status === 401) { validation.value = undefined; approved.value = false; error.value = '后台会话已失效；恢复进度仍可凭本页只读令牌查询。管理操作需重新登录。'; }
  else if (code === 'FORBIDDEN' || status === 403) { validation.value = undefined; approved.value = false; progress.value = undefined; progressToken = ''; error.value = '当前账号或令牌无权访问此恢复。'; }
  else error.value = '恢复操作未完成，请检查进度后重试。';
}
async function validateRestore(): Promise<void> {
  if (busy.value || !isSuperadmin.value || !backupId.value || restoreId.value) return;
  busy.value = true; error.value = ''; validation.value = undefined; approved.value = false;
  const input = { backupId: backupId.value, targetEpoch: targetEpoch.value };
  try { const result = await props.bootstrap.apiClient.request<components['schemas']['ValidateRestoreResult']>('/v1/restores:validate', { method: 'POST', body: input, signal, idempotencyKey: crypto.randomUUID() }); if (!signal.aborted && JSON.stringify(input) === JSON.stringify({ backupId: backupId.value, targetEpoch: targetEpoch.value })) { validation.value = result; validatedInput = JSON.stringify(input); } }
  catch (problem) { failRestore(problem); }
  finally { if (!signal.aborted) busy.value = false; }
}
async function ApproveFullRestore(input: ApproveFullRestoreInput, requestSignal: AbortSignal): Promise<ApproveFullRestoreResult> {
  const body = JSON.stringify(input);
  if (!validation.value?.valid || !approved.value || body !== validatedInput) throw new Error('必须批准已校验的完整备份');
  if (!operationId || operationInput !== body) { operationId = crypto.randomUUID(); operationInput = body; }
  const result = await props.bootstrap.apiClient.request<components['schemas']['CreateRestoreResult']>('/v1/restores', { method: 'POST', body: input, signal: requestSignal, idempotencyKey: operationId });
  if (!requestSignal.aborted) { restoreId.value = result.restoreId; progressToken = result.progressToken ?? ''; progressExpiresAt = result.progressExpiresAt ?? ''; }
  return { restoreId: result.restoreId };
}
async function refreshRestoreProgress(): Promise<void> {
  if (!restoreId.value || signal.aborted) return;
  if (timer) clearTimeout(timer);
  if (!progressToken || !progressExpiresAt || Date.parse(progressExpiresAt) <= Date.now()) { progressToken = ''; error.value = '只读进度令牌不可用或已过期，请重新登录后查看恢复进度。'; return; }
  try {
    const result = await props.bootstrap.apiClient.request<components['schemas']['ReadRestoreProgressResult']>(`/v1/restores/${encodeURIComponent(restoreId.value)}/progress`, { signal, restoreProgressToken: progressToken });
    if (signal.aborted) return;
    progress.value = result;
    if (!['completed', 'rolled-back', 'aborted', 'blocked'].includes(result.phase)) timer = setTimeout(() => { void refreshRestoreProgress(); }, 3000);
    if (['completed', 'rolled-back', 'aborted'].includes(result.phase)) progressToken = '';
  } catch (problem) { failRestore(problem); }
}
async function submitRestore(): Promise<void> {
  if (busy.value || !isSuperadmin.value || restoreId.value) return;
  busy.value = true; error.value = '';
  try { await ApproveFullRestore({ backupId: backupId.value, targetEpoch: targetEpoch.value }, signal); if (!signal.aborted) await refreshRestoreProgress(); }
  catch (problem) { failRestore(problem); }
  finally { if (!signal.aborted) busy.value = false; }
}
</script>

<template>
  <section aria-labelledby="restore-heading">
    <h2 id="restore-heading">恢复整个站点</h2>
    <p v-if="!isSuperadmin">恢复需要超级管理员权限。</p>
    <template v-else>
      <p>恢复会替换全站业务数据，不能选择部分域。站点进入维护，后台会话可能失效；完成后请使用备份中的账号重新登录。</p>
      <label>完整备份ID <input v-model="backupId" :disabled="busy || !!restoreId" autocomplete="off"></label>
      <button :disabled="busy || !backupId || !!restoreId" @click="validateRestore">校验全部域</button>
      <p>本次目标代次：{{ targetEpoch }}</p>
      <template v-if="validation">
        <p>{{ validation.valid ? '全部预检通过，可批准本次完整恢复。' : '预检未通过，无法发起恢复。' }}</p>
        <ul><li v-for="(issue, index) in validation.issues" :key="index">{{ issue.owner }}：{{ issue.reason }}</li></ul>
        <form v-if="validation.valid && !restoreId" @submit.prevent="submitRestore">
          <label><input v-model="approved" type="checkbox" :disabled="busy">我批准将这份完整备份恢复到上述目标代次，并接受维护影响。</label>
          <button type="submit" :disabled="busy || !approved">{{ busy ? '正在提交…' : '批准完整恢复' }}</button>
        </form>
      </template>
      <p v-if="error" role="alert">{{ error }}</p>
      <template v-if="restoreId">
        <p>恢复ID：{{ restoreId }}。请保留本页；只读令牌仅在本页内存中保存。</p>
        <button :disabled="busy" @click="refreshRestoreProgress">刷新恢复进度</button>
        <template v-if="progress">
          <p role="status">阶段：{{ progress.phase }}</p>
          <ul><li v-for="domain in progress.domains" :key="domain.owner">{{ domain.owner }}：{{ domain.state }}</li></ul>
          <p v-if="progress.failedOwners.length" role="alert">失败域：{{ progress.failedOwners.join('、') }}</p>
          <p v-if="progress.phase === 'completed'">恢复已完成，请用备份中的账号重新登录。</p>
          <p v-if="progress.phase === 'blocked'">恢复尚未完成，站点保持维护。请使用管理恢复入口处理失败域。</p>
        </template>
      </template>
    </template>
  </section>
</template>
