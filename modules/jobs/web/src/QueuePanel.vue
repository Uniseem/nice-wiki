<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import type { ModuleUIBootstrap } from '../../../../libs/platform/web/module-api';
import type { components } from './gen/api.gen';
import { ReconnectProgress, type ProgressEvent } from './progress';

const props = defineProps<{ bootstrap: ModuleUIBootstrap; parentSignal: AbortSignal }>();

type JobSummary = components['schemas']['JobSummary'];
type ListJobsResult = components['schemas']['ListJobsResult'];
type ReadJobResult = components['schemas']['ReadJobResult'];
type DeliverySummary = components['schemas']['DeliverySummary'];
type ListDeliveriesResult = components['schemas']['ListDeliveriesResult'];
type JobStateFilter = 'active' | 'failed' | 'dead-letter' | 'all';
type RequestJobCancellationInput = { jobId: string };
type RequestJobCancellationResult = { state: string };

const canManage = computed(() => props.bootstrap.capabilities.includes('job.manage'));
const jobs = ref<JobSummary[]>([]);
const deliveries = ref<DeliverySummary[]>([]);
const selected = ref<JobSummary | null>(null);
const detail = ref<ReadJobResult | null>(null);
const progressEvents = ref<ProgressEvent[]>([]);
const stateFilter = ref<JobStateFilter>('active');
const ownerFilter = ref('');
const nextCursor = ref('');
const error = ref('');
const notice = ref('');
const loading = ref(false);
const loadingDeliveries = ref(false);
const busy = ref(false);
const reconnectState = ref('');
const controller = new AbortController();
const signal = AbortSignal.any([controller.signal, props.parentSignal]);
const actionKeys = new Map<string, string>();
let progressController: AbortController | null = null;

onMounted(() => {
  if (canManage.value) {
    void loadJobs(false);
    void loadDeliveries();
  }
});
onBeforeUnmount(() => {
  progressController?.abort();
  controller.abort();
});

function problemCode(value: unknown): string {
  const problem = value as { code?: unknown; problem?: { code?: unknown } } | null;
  const code = problem?.code ?? problem?.problem?.code;
  return typeof code === 'string' ? code : 'DEPENDENCY_UNAVAILABLE';
}

function fail(value: unknown): void {
  if (signal.aborted) return;
  const code = problemCode(value);
  if (code === 'UNAUTHENTICATED' || code === 'FORBIDDEN') {
    jobs.value = [];
    deliveries.value = [];
    selected.value = null;
    detail.value = null;
    progressEvents.value = [];
    error.value = code === 'UNAUTHENTICATED' ? '登录已失效，请重新登录。' : '当前账号无权管理任务，已清空敏感界面。';
    return;
  }
  if (code === 'VERSION_MISMATCH' || code === 'STALE_RESULT' || code === 'CONFLICT') {
    error.value = '任务或权限版本已变化，请重新载入。';
    return;
  }
  error.value = '任务操作未完成，请稍后重试。';
}

function operationKey(scope: string, input: unknown): string {
  const value = `${scope}:${JSON.stringify(input)}`;
  const existing = actionKeys.get(value);
  if (existing) return existing;
  const key = crypto.randomUUID();
  actionKeys.set(value, key);
  return key;
}

async function RequestJobCancellation(input: RequestJobCancellationInput, requestSignal: AbortSignal): Promise<RequestJobCancellationResult> {
  return props.bootstrap.apiClient.request<RequestJobCancellationResult>(
    `/v1/jobs/${encodeURIComponent(input.jobId)}:cancel`,
    { method: 'POST', body: {}, signal: requestSignal, idempotencyKey: operationKey('cancel', input) },
  );
}

async function loadJobs(more: boolean): Promise<void> {
  if (loading.value || !canManage.value) return;
  loading.value = true;
  error.value = '';
  try {
    const query = new URLSearchParams({ state: stateFilter.value, limit: '50' });
    if (ownerFilter.value) query.set('owner', ownerFilter.value);
    if (more && nextCursor.value) query.set('cursor', nextCursor.value);
    const page = await props.bootstrap.apiClient.request<ListJobsResult>(`/v1/jobs?${query}`, { signal });
    if (signal.aborted) return;
    jobs.value = more ? [...jobs.value, ...page.items] : page.items;
    nextCursor.value = page.pageInfo.hasMore ? page.pageInfo.nextCursor ?? '' : '';
  } catch (value) {
    fail(value);
  } finally {
    if (!signal.aborted) loading.value = false;
  }
}

async function loadDeliveries(): Promise<void> {
  if (loadingDeliveries.value || !canManage.value) return;
  loadingDeliveries.value = true;
  try {
    const page = await props.bootstrap.apiClient.request<ListDeliveriesResult>('/v1/deliveries?limit=50', { signal });
    if (!signal.aborted) deliveries.value = page.items;
  } catch (value) {
    fail(value);
  } finally {
    if (!signal.aborted) loadingDeliveries.value = false;
  }
}

async function selectJob(job: JobSummary): Promise<void> {
  progressController?.abort();
  progressController = new AbortController();
  const currentController = progressController;
  selected.value = job;
  detail.value = null;
  progressEvents.value = [];
  reconnectState.value = '';
  try {
    const loaded = await props.bootstrap.apiClient.request<ReadJobResult>(`/v1/jobs/${encodeURIComponent(job.jobId)}`, { signal });
    if (currentController.signal.aborted || selected.value?.jobId !== job.jobId) return;
    detail.value = loaded;
    const combined = AbortSignal.any([signal, currentController.signal]);
    reconnectState.value = '正在连接进度…';
    void ReconnectProgress(
      { jobId: job.jobId, lastEventId: job.progress?.seq ?? 0 },
      combined,
      (event) => {
        if (combined.aborted || selected.value?.jobId !== job.jobId) return;
        progressEvents.value = [...progressEvents.value.slice(-199), event];
        reconnectState.value = '进度已连接';
      },
    ).catch((value: unknown) => {
      if (!combined.aborted && (value as { name?: string })?.name !== 'AbortError') {
        const code = problemCode(value);
        if (code === 'UNAUTHENTICATED' || code === 'FORBIDDEN') fail(value);
        else reconnectState.value = `进度已停止：${code}`;
      }
    });
  } catch (value) {
    if (currentController.signal.aborted || selected.value?.jobId !== job.jobId) return;
    fail(value);
  }
}

async function cancelSelected(): Promise<void> {
  const job = selected.value;
  if (!job || busy.value) return;
  busy.value = true;
  error.value = '';
  notice.value = '';
  try {
    const result = await RequestJobCancellation({ jobId: job.jobId }, signal);
    notice.value = `已记录取消请求，当前状态：${result.state}。需等待任务所属服务确认安全终局。`;
    await loadJobs(false);
  } catch (value) {
    fail(value);
  } finally {
    if (!signal.aborted) busy.value = false;
  }
}

async function retryJob(job: JobSummary): Promise<void> {
  if (busy.value) return;
  busy.value = true;
  error.value = '';
  try {
    await props.bootstrap.apiClient.request('/v1/jobs/' + encodeURIComponent(job.jobId) + ':retry', {
      method: 'POST',
      body: { jobId: job.jobId },
      signal,
      idempotencyKey: operationKey('retry', { jobId: job.jobId }),
    });
    notice.value = '任务已按原安全根重新排队或复用既有结果。';
    await loadJobs(false);
  } catch (value) {
    fail(value);
  } finally {
    if (!signal.aborted) busy.value = false;
  }
}

async function replayDelivery(delivery: DeliverySummary): Promise<void> {
  if (busy.value) return;
  busy.value = true;
  error.value = '';
  try {
    await props.bootstrap.apiClient.request('/v1/deliveries/' + encodeURIComponent(delivery.deliveryId) + ':replay', {
      method: 'POST',
      body: {},
      signal,
      idempotencyKey: operationKey('replay', { deliveryId: delivery.deliveryId }),
    });
    notice.value = '死信投递已重新排队。';
    await loadDeliveries();
  } catch (value) {
    fail(value);
  } finally {
    if (!signal.aborted) busy.value = false;
  }
}

function units(event: ProgressEvent): string {
  return event.totalUnits > 0 ? `${event.completedUnits}/${event.totalUnits}` : String(event.completedUnits);
}
</script>

<template>
  <section class="queue-panel" aria-labelledby="queue-heading">
    <h2 id="queue-heading">任务队列</h2>
    <p v-if="!canManage">当前账号无权查看任务队列。</p>
    <template v-else>
      <form class="filters" @submit.prevent="loadJobs(false)">
        <label>状态
          <select v-model="stateFilter">
            <option value="active">活动</option>
            <option value="failed">失败</option>
            <option value="dead-letter">死信</option>
            <option value="all">全部</option>
          </select>
        </label>
        <label>所属服务 <input v-model.trim="ownerFilter" maxlength="64" placeholder="例如 knowledge"></label>
        <button :disabled="loading" type="submit">刷新</button>
      </form>

      <p role="status">{{ notice }}</p>
      <p v-if="error" role="alert">{{ error }}</p>
      <p v-if="loading">正在读取任务…</p>
      <p v-else-if="jobs.length === 0">当前筛选下没有任务。</p>

      <div class="job-grid">
        <article v-for="job in jobs" :key="job.jobId" :aria-current="selected?.jobId === job.jobId ? 'true' : undefined">
          <button class="job-title" type="button" @click="selectJob(job)">{{ job.owner }} · {{ job.type }}</button>
          <dl>
            <dt>状态</dt><dd>{{ selected?.jobId === job.jobId && detail ? detail.state : job.state }}</dd>
            <dt>尝试</dt><dd>{{ job.attempt }}</dd>
            <dt>修订</dt><dd>{{ job.revision }}</dd>
            <dt>任务</dt><dd><code>{{ job.jobId }}</code></dd>
          </dl>
          <p v-if="job.progress">{{ job.progress.phase }} · {{ job.progress.completedUnits }}/{{ job.progress.totalUnits }}</p>
          <p v-if="job.lastErrorCode">最后错误：{{ job.lastErrorCode }}</p>
          <button v-if="job.state === 'dead-letter'" :disabled="busy" type="button" @click="retryJob(job)">重试死信任务</button>
        </article>
      </div>
      <button v-if="nextCursor" :disabled="loading" type="button" @click="loadJobs(true)">加载更多</button>

      <aside v-if="selected" class="job-detail">
        <h3>任务详情</h3>
        <p><code>{{ selected.jobId }}</code></p>
        <p v-if="detail">当前状态：{{ detail.state }}；修订：{{ detail.revision }}</p>
        <p v-if="detail?.resultRefs.length">结果引用：{{ detail.resultRefs.join('、') }}</p>
        <button v-if="!['succeeded', 'cancelled', 'dead-letter'].includes(detail?.state ?? selected.state)" :disabled="busy" type="button" @click="cancelSelected">请求取消</button>
        <p>{{ reconnectState }}</p>
        <ol v-if="progressEvents.length" class="progress-list">
          <li v-for="event in progressEvents" :key="event.seq"><span>{{ event.phase }}</span><strong>{{ units(event) }}</strong></li>
        </ol>
        <p v-else>尚无已保留的进度检查点。</p>
      </aside>

      <section class="deliveries" aria-labelledby="deliveries-heading">
        <h3 id="deliveries-heading">死信投递</h3>
        <button :disabled="loadingDeliveries" type="button" @click="loadDeliveries">刷新投递</button>
        <p v-if="deliveries.length === 0">当前没有可见死信投递。</p>
        <article v-for="delivery in deliveries" :key="delivery.deliveryId">
          <p>{{ delivery.producer }} → {{ delivery.consumer }} · {{ delivery.eventType }}@{{ delivery.eventVersion }}</p>
          <p>尝试 {{ delivery.attempts }} 次；错误：{{ delivery.lastErrorCode }}</p>
          <button :disabled="busy" type="button" @click="replayDelivery(delivery)">重放</button>
        </article>
      </section>
    </template>
  </section>
</template>

<style scoped>
.queue-panel { --jobs-border: #c9d2dc; --jobs-accent: #0b6b5d; color: #16222c; }
.filters { display: flex; flex-wrap: wrap; gap: 1rem; align-items: end; margin-block: 1rem; }
label { display: grid; gap: .35rem; font-weight: 650; }
input, select { min-height: 2.25rem; padding: .35rem .5rem; border: 1px solid var(--jobs-border); border-radius: .45rem; background: #fff; }
button { cursor: pointer; border: 1px solid #173f3a; border-radius: .45rem; background: #eaf5f1; color: #123b35; padding: .45rem .75rem; font: inherit; }
button:disabled { cursor: not-allowed; opacity: .55; }
.job-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(17rem, 1fr)); gap: .75rem; margin-block: 1rem; }
article, aside, .deliveries { border: 1px solid var(--jobs-border); border-radius: .7rem; background: #f8fafb; padding: 1rem; }
.job-title { width: 100%; text-align: left; background: transparent; border: 0; padding: 0; font-weight: 750; color: #0b5148; }
dl { display: grid; grid-template-columns: 4rem 1fr; gap: .25rem .5rem; }
dt { color: #53636f; } dd { margin: 0; overflow-wrap: anywhere; }
code { overflow-wrap: anywhere; }
.job-detail { margin-block: 1rem; border-left: .35rem solid var(--jobs-accent); }
.progress-list { max-height: 18rem; overflow: auto; }
.progress-list li { display: flex; justify-content: space-between; padding-block: .3rem; border-bottom: 1px dashed #cbd6df; }
.deliveries { margin-block-start: 1rem; }
.deliveries article { margin-block-start: .75rem; background: #fff; }
</style>
