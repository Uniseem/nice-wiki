<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue';
import type { ModuleUIBootstrap } from '../../../../libs/platform/web/module-api';
import type { components } from './gen/api.gen';
const props = defineProps<{ bootstrap: ModuleUIBootstrap; parentSignal: AbortSignal; initialBackup?: string }>();
type RequestMirrorSyncInput = { mirrorId: string; expectedRemoteHead: string };
type RequestMirrorSyncResult = { state: string };
const canManage = computed(() => props.bootstrap.capabilities.includes('backup.manage'));
const savedRemoteURL = ref(''), savedBranch = ref('');
const remoteURL = ref(''), branch = ref('refs/heads/main'), credentialRef = ref(''), revision = ref(0);
const backupId = ref(props.initialBackup ?? ''), mirrorId = ref(''), status = ref<components['schemas']['ReadMirrorStatusResult']>();
const busy = ref(false), error = ref(''), notice = ref(''), remoteConfigured = ref(false);
const controller = new AbortController(), signal = AbortSignal.any([controller.signal, props.parentSignal]);
let syncOperation = '', syncInput = '', saveOperation = '', saveInput = '', buildOperation = '', buildInput = '';
onBeforeUnmount(() => controller.abort());
watch(() => props.initialBackup, value => { backupId.value = value ?? ''; });
onMounted(() => { if (canManage.value) void loadMirrorSettings(); });
function failMirror(problem: unknown): void {
  if (signal.aborted) return;
  const { code, status: responseStatus } = (problem ?? {}) as { code?: string; status?: number };
  if (code === 'UNAUTHENTICATED' || responseStatus === 401 || code === 'FORBIDDEN' || responseStatus === 403) { remoteURL.value = ''; credentialRef.value = ''; status.value = undefined; mirrorId.value = ''; error.value = responseStatus === 401 || code === 'UNAUTHENTICATED' ? '登录已失效，请重新登录。' : '当前账号无权管理镜像。'; }
  else if (code === 'CONFLICT' || responseStatus === 409) error.value = '远程分支或配置已变化。请刷新后核对，不会强制推送。';
  else error.value = '镜像操作未完成，请稍后重试。';
}
async function loadMirrorSettings(): Promise<void> {
  error.value = '';
  try {
    const settings = await props.bootstrap.apiClient.request<components['schemas']['ReadProtectionSettingsResult']>('/v1/settings', { signal });
    if (signal.aborted) return;
    remoteConfigured.value = !!settings.remote; savedRemoteURL.value = settings.remote?.url ?? ''; savedBranch.value = settings.remote?.branch ?? '';
    remoteURL.value = settings.remote?.url ?? ''; branch.value = settings.remote?.branch ?? 'refs/heads/main'; revision.value = settings.remote?.expectedRevision ?? 0;
    try { const result = await props.bootstrap.apiClient.request<components['schemas']['ReadMirrorStatusResult']>('/v1/mirrors/status', { signal }); if (!signal.aborted) { status.value = result; mirrorId.value = result.mirrorId ?? ''; } }
    catch (problem) { const { code, status } = (problem ?? {}) as { code?: string; status?: number }; if (code !== 'NOT_FOUND' && status !== 404) throw problem; }
  } catch (problem) { failMirror(problem); }
}
async function saveRemote(): Promise<void> {
  if (busy.value || !canManage.value) return;
  busy.value = true; error.value = ''; notice.value = '';
  const input = { url: remoteURL.value, branch: branch.value, credentialRef: credentialRef.value, expectedRevision: revision.value }, body = JSON.stringify(input);
  if (!saveOperation || saveInput !== body) { saveOperation = crypto.randomUUID(); saveInput = body; }
  try { const result = await props.bootstrap.apiClient.request<components['schemas']['ConfigureRemoteResult']>('/v1/git-remote', { method: 'PUT', body: input, signal, idempotencyKey: saveOperation }); if (!signal.aborted) { revision.value = result.revision; remoteConfigured.value = true; savedRemoteURL.value = input.url; savedBranch.value = input.branch; credentialRef.value = ''; saveOperation = ''; notice.value = '远程配置已保存，尚未发起同步。'; } }
  catch (problem) { failMirror(problem); }
  finally { if (!signal.aborted) busy.value = false; }
}
async function buildMirror(): Promise<void> {
  if (busy.value || !canManage.value || !backupId.value) return;
  busy.value = true; error.value = ''; notice.value = '';
  if (!buildOperation || buildInput !== backupId.value) { buildOperation = crypto.randomUUID(); buildInput = backupId.value; }
  try { const result = await props.bootstrap.apiClient.request<components['schemas']['BuildMirrorResult']>('/v1/mirrors', { method: 'POST', body: { backupId: backupId.value }, signal, idempotencyKey: buildOperation }); if (!signal.aborted) { mirrorId.value = result.mirrorId; buildOperation = ''; notice.value = `本地镜像已建立：${result.commitHash}，尚未同步远程。`; await loadMirrorSettings(); } }
  catch (problem) { failMirror(problem); }
  finally { if (!signal.aborted) busy.value = false; }
}
async function RequestMirrorSync(input: RequestMirrorSyncInput, requestSignal: AbortSignal): Promise<RequestMirrorSyncResult> {
  const body = JSON.stringify(input);
  if (!syncOperation || syncInput !== body) { syncOperation = crypto.randomUUID(); syncInput = body; }
  const result = await props.bootstrap.apiClient.request<components['schemas']['SyncMirrorResult']>(`/v1/mirrors/${encodeURIComponent(input.mirrorId)}:sync`, { method: 'POST', body: { expectedRemoteHead: input.expectedRemoteHead }, signal: requestSignal, idempotencyKey: syncOperation });
  return { state: result.state };
}
async function submitSync(): Promise<void> {
  if (busy.value || !canManage.value || !mirrorId.value || !status.value || !remoteConfigured.value) return;
  busy.value = true; error.value = ''; notice.value = '';
  try { const result = await RequestMirrorSync({ mirrorId: mirrorId.value, expectedRemoteHead: status.value.remoteHead }, signal); if (!signal.aborted) { syncOperation = ''; notice.value = `远程同步状态：${result.state}`; await loadMirrorSettings(); } }
  catch (problem) { failMirror(problem); }
  finally { if (!signal.aborted) busy.value = false; }
}
</script>

<template>
  <section aria-labelledby="mirror-heading">
    <h2 id="mirror-heading">版本镜像与远程同步</h2>
    <p v-if="!canManage">当前账号无权管理镜像。</p>
    <template v-else>
      <p>镜像只包含允许的公开投影。同步需明确提交；远程分叉时停止，不提供强制推送。</p>
      <form @submit.prevent="saveRemote">
        <label>远程地址 <input v-model="remoteURL" type="url" :disabled="busy" placeholder="部署白名单中的HTTPS或SSH仓库"></label>
        <label>完整分支 <input v-model="branch" :disabled="busy" placeholder="refs/heads/main"></label>
        <label>部署凭据ID（空为无凭据） <input v-model="credentialRef" :disabled="busy" autocomplete="off"></label>
        <button :disabled="busy" type="submit">保存远程配置</button>
      </form>
      <label>完整就绪备份ID <input v-model="backupId" :disabled="busy"></label><button :disabled="busy || !backupId" @click="buildMirror">建立本地镜像</button>
      <p v-if="status">状态：{{ status.state }}；本地提交：{{ status.commitHash }}；已知远程提交：{{ status.remoteHead || '空分支' }}</p>
      <p v-if="remoteConfigured">同步目标：{{ savedRemoteURL }}，分支：{{ savedBranch }}</p>
      <button :disabled="busy || !mirrorId || !status || !remoteConfigured" @click="submitSync">明确同步到远程</button>
      <button :disabled="busy" @click="loadMirrorSettings">刷新配置与状态</button>
      <p v-if="notice" role="status">{{ notice }}</p><p v-if="error" role="alert">{{ error }}</p>
    </template>
  </section>
</template>
