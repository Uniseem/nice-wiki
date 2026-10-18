<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue';
import type { ModuleUIBootstrap } from '../../../../libs/platform/web/module-api';
import type { components } from './gen/api.gen';
const props = defineProps<{ bootstrap: ModuleUIBootstrap; parentSignal: AbortSignal }>();
const emit = defineEmits<{ selectBackup: [id: string] }>();
type Backup = components['schemas']['ReadBackupResult'] & { backupId: string };
type RequestBackupInput = { includeSecrets: boolean };
type RequestBackupResult = { backupId: string };
const backups = ref<Backup[]>([]), nextCursor = ref(''), error = ref(''), notice = ref('');
const includeSecrets = ref(false), busy = ref(false), loading = ref(false), downloading = ref(false);
const canManage = computed(() => props.bootstrap.capabilities.includes('backup.manage'));
const isSuperadmin = computed(() => props.bootstrap.capabilities.includes('superadmin'));
const controller = new AbortController();
const signal = AbortSignal.any([controller.signal, props.parentSignal]);
let operationId = '', operationInput = '';
onBeforeUnmount(() => controller.abort());
onMounted(() => { if (canManage.value) void loadBackups(false); });

function failBackup(problem: unknown): void {
  if (signal.aborted) return;
  const code = (problem as { code?: string; status?: number })?.code;
  const status = (problem as { status?: number })?.status;
  if (code === 'UNAUTHENTICATED' || status === 401) { backups.value = []; error.value = '登录已失效，请重新登录。'; }
  else if (code === 'FORBIDDEN' || status === 403) { backups.value = []; includeSecrets.value = false; error.value = '当前账号无权管理备份。'; }
  else if (code === 'STALE_RESULT') { nextCursor.value = ''; error.value = '备份目录已更新，请刷新目录。'; }
  else error.value = '备份操作未完成，请稍后重试。';
}
async function loadBackups(more: boolean): Promise<void> {
  if (loading.value) return;
  loading.value = true; error.value = '';
  try {
    const cursor = more ? nextCursor.value : '';
    const page = await props.bootstrap.apiClient.request<components['schemas']['ListBackupsResult']>(`/v1/backups?limit=20${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ''}`, { signal });
    const rows = await Promise.all(page.backupIds.map(async backupId => ({ ...await props.bootstrap.apiClient.request<components['schemas']['ReadBackupResult']>(`/v1/backups/${encodeURIComponent(backupId)}`, { signal }), backupId })));
    if (!signal.aborted) { backups.value = more ? [...backups.value, ...rows] : rows; nextCursor.value = page.nextCursor; }
  } catch (problem) { failBackup(problem); }
  finally { if (!signal.aborted) loading.value = false; }
}
async function RequestBackup(input: RequestBackupInput, requestSignal: AbortSignal): Promise<RequestBackupResult> {
  const body = JSON.stringify(input);
  if (!operationId || operationInput !== body) { operationId = crypto.randomUUID(); operationInput = body; }
  const result = await props.bootstrap.apiClient.request<components['schemas']['CreateBackupResult']>('/v1/backups', { method: 'POST', body: input, signal: requestSignal, idempotencyKey: operationId });
  return { backupId: result.backupId };
}
async function submitBackup(): Promise<void> {
  if (busy.value || !canManage.value) return;
  busy.value = true; error.value = ''; notice.value = '';
  try { const result = await RequestBackup({ includeSecrets: isSuperadmin.value && includeSecrets.value }, signal); if (!signal.aborted) { notice.value = `备份已受理：${result.backupId}。只有完整就绪后才能下载。`; operationId = ''; await loadBackups(false); } }
  catch (problem) { failBackup(problem); }
  finally { if (!signal.aborted) busy.value = false; }
}
// 便携包可以很大，只使用浏览器文件流；不退回整包Blob占用内存。
async function downloadBackup(backup: Backup): Promise<void> {
  if (downloading.value || backup.state !== 'ready' || !isSuperadmin.value) return;
  const picker = (window as Window & { showSaveFilePicker?: (options: { suggestedName: string }) => Promise<{ createWritable(): Promise<WritableStream<Uint8Array>> }> }).showSaveFilePicker;
  if (!picker) { error.value = '当前浏览器不支持流式保存大型备份，请使用支持文件流保存的浏览器。'; return; }
  downloading.value = true; error.value = '';
  let output: WritableStream<Uint8Array> | undefined;
  try {
    const handle = await picker.call(window, { suggestedName: `backup-${backup.backupId}.zip` });
    if (signal.aborted) return;
    const authorization = await props.bootstrap.apiClient.request<components['schemas']['AuthorizeBackupDownloadResult']>(`/v1/backups/${encodeURIComponent(backup.backupId)}:download`, { method: 'POST', body: {}, signal, idempotencyKey: crypto.randomUUID() });
    const response = await props.bootstrap.apiClient.request<Response>(`/v1/backups/${encodeURIComponent(backup.backupId)}/download`, { responseType: 'stream', backupDownloadToken: authorization.downloadToken, signal });
    if (!response.ok || !response.body) throw new Error('完整备份流不可用');
    output = await handle.createWritable();
    await response.body.pipeTo(output, { signal });
    output = undefined;
    if (!signal.aborted) notice.value = '完整便携备份已保存，请妥善保管。';
  } catch (problem) {
    if (output && !output.locked) await output.abort().catch(() => {});
    if ((problem as { name?: string })?.name !== 'AbortError') failBackup(problem);
  } finally { if (!signal.aborted) downloading.value = false; }
}
</script>

<template>
  <section aria-labelledby="backup-heading">
    <h2 id="backup-heading">完整备份</h2>
    <p v-if="!canManage">当前账号无权查看备份。</p>
    <template v-else>
      <form @submit.prevent="submitBackup">
        <label v-if="isSuperadmin"><input v-model="includeSecrets" type="checkbox" :disabled="busy">包含密钥和敏感配置</label>
        <button type="submit" :disabled="busy">{{ busy ? '正在提交…' : '创建完整备份' }}</button>
      </form>
      <p role="status">{{ notice }}</p><p v-if="error" role="alert">{{ error }}</p>
      <button :disabled="loading" @click="loadBackups(false)">刷新备份目录</button>
      <p v-if="loading">正在读取目录…</p><p v-else-if="!error && !backups.length">尚无可显示的备份。</p>
      <article v-for="backup in backups" :key="backup.backupId">
        <h3>{{ backup.backupId }}</h3><p>状态：{{ backup.state }}</p>
        <details v-if="backup.manifestJSON"><summary>完整清单与各域水位</summary><pre>{{ backup.manifestJSON }}</pre></details>
        <ul v-if="backup.issues.length"><li v-for="(issue, index) in backup.issues" :key="index">{{ issue.owner }}：{{ issue.reason }}</li></ul>
        <template v-if="backup.state === 'ready'"><p>完整备份已就绪。</p><button v-if="isSuperadmin" :disabled="downloading" @click="downloadBackup(backup)">下载完整便携备份</button><button @click="emit('selectBackup', backup.backupId)">选择此备份用于恢复或镜像</button></template>
      </article>
      <button v-if="nextCursor" :disabled="loading" @click="loadBackups(true)">下一页</button>
    </template>
  </section>
</template>
