<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue';
import type { ModuleUIBootstrap } from '../../../../libs/platform/web/module-api';
import type { components } from './gen/api.gen';

const props = defineProps<{ bootstrap: ModuleUIBootstrap; parentSignal: AbortSignal }>();

type Schedule = components['schemas']['Schedule'] & { revision: number };
type ListSchedulesResult = { schedules: Schedule[] };
type SaveScheduleInput = {
  scheduleId: string;
  expectedRevision: number;
  cron: string;
  timezone: string;
  enabled: boolean;
};
type SaveScheduleResult = { revision: number };

const canManage = computed(() => props.bootstrap.capabilities.includes('job.manage'));
const schedules = ref<Schedule[]>([]);
const selectedId = ref('');
const error = ref('');
const notice = ref('');
const loading = ref(false);
const saving = ref(false);
const controller = new AbortController();
const signal = AbortSignal.any([controller.signal, props.parentSignal]);
const operationKeys = new Map<string, string>();
const edit = reactive({ cron: '', timezone: '', enabled: true });

const selected = computed(() => schedules.value.find((schedule) => schedule.scheduleId === selectedId.value) ?? null);

onMounted(() => {
  if (canManage.value) void loadSchedules();
});
onBeforeUnmount(() => controller.abort());

function problemCode(value: unknown): string {
  const problem = value as { code?: unknown; problem?: { code?: unknown } } | null;
  const code = problem?.code ?? problem?.problem?.code;
  return typeof code === 'string' ? code : 'DEPENDENCY_UNAVAILABLE';
}

function fail(value: unknown): void {
  if (signal.aborted) return;
  const code = problemCode(value);
  if (code === 'UNAUTHENTICATED' || code === 'FORBIDDEN') {
    schedules.value = [];
    selectedId.value = '';
    error.value = code === 'UNAUTHENTICATED' ? '登录已失效，请重新登录。' : '当前账号无权管理调度。';
    return;
  }
  error.value = code === 'CONFLICT' || code === 'STALE_RESULT'
    ? '定时记录 revision 已变化，请刷新后重试。'
    : '调度操作未完成，请稍后重试。';
}

function operationKey(scope: string, body: unknown): string {
  const value = `${scope}:${JSON.stringify(body)}`;
  const existing = operationKeys.get(value);
  if (existing) return existing;
  const key = crypto.randomUUID();
  operationKeys.set(value, key);
  return key;
}

async function loadSchedules(): Promise<void> {
  if (loading.value || !canManage.value) return;
  loading.value = true;
  error.value = '';
  try {
    const result = await props.bootstrap.apiClient.request<ListSchedulesResult>('/v1/schedules', { signal });
    if (signal.aborted) return;
    schedules.value = result.schedules;
    if (selectedId.value && !schedules.value.some((schedule) => schedule.scheduleId === selectedId.value)) {
      selectedId.value = '';
    }
  } catch (value) {
    fail(value);
  } finally {
    if (!signal.aborted) loading.value = false;
  }
}

function choose(schedule: Schedule): void {
  selectedId.value = schedule.scheduleId;
  edit.cron = schedule.cron;
  edit.timezone = schedule.timezone;
  edit.enabled = schedule.enabled;
}

async function SaveSchedule(input: SaveScheduleInput, requestSignal: AbortSignal): Promise<SaveScheduleResult> {
  const current = selected.value;
  if (!current || current.scheduleId !== input.scheduleId || current.revision !== input.expectedRevision) throw { code: 'CONFLICT' };
  const body = {
    expectedRevision: input.expectedRevision,
    cron: input.cron,
    timezone: input.timezone,
    enabled: input.enabled,
  };
  return props.bootstrap.apiClient.request<SaveScheduleResult>(`/v1/schedules/${encodeURIComponent(input.scheduleId)}`, {
    method: 'PUT',
    body,
    signal: requestSignal,
    idempotencyKey: operationKey('update-schedule', { scheduleId: input.scheduleId, ...body }),
  });
}

async function saveSchedule(): Promise<void> {
  const current = selected.value;
  if (!current || saving.value) return;
  saving.value = true;
  error.value = '';
  notice.value = '';
  try {
    const result = await SaveSchedule({
      scheduleId: current.scheduleId,
      expectedRevision: current.revision,
      cron: edit.cron,
      timezone: edit.timezone,
      enabled: edit.enabled,
    }, signal);
    notice.value = `定时已保存；新 revision 为 ${result.revision}。`;
    await loadSchedules();
  } catch (value) {
    fail(value);
  } finally {
    if (!signal.aborted) saving.value = false;
  }
}

</script>

<template>
  <section class="schedule-editor" aria-labelledby="schedule-heading">
    <h2 id="schedule-heading">定时调度</h2>
    <p v-if="!canManage">当前账号无权查看或修改定时任务。</p>
    <template v-else>
      <p role="status">{{ notice }}</p>
      <p v-if="error" role="alert">{{ error }}</p>
      <button :disabled="loading" type="button" @click="loadSchedules">刷新定时</button>
      <p v-if="loading">正在读取定时…</p>
      <p v-else-if="schedules.length === 0">当前代次尚无定时任务。</p>

      <div class="schedule-list">
        <button v-for="schedule in schedules" :key="schedule.scheduleId" type="button" :aria-pressed="selectedId === schedule.scheduleId" @click="choose(schedule)">
          <strong>{{ schedule.owner }} · {{ schedule.type }}</strong>
          <span>{{ schedule.cron }} · {{ schedule.timezone }}</span>
          <span>{{ schedule.enabled ? '启用' : '停用' }} · revision {{ schedule.revision }} · 下次 {{ schedule.nextAt }}</span>
        </button>
      </div>

      <form v-if="selected" class="editor" @submit.prevent="saveSchedule">
        <h3>编辑 {{ selected.scheduleId }}</h3>
        <label>5 字段 cron <input v-model.trim="edit.cron" maxlength="8192" required></label>
        <label>IANA 时区 <input v-model.trim="edit.timezone" maxlength="8192" required></label>
        <label><input v-model="edit.enabled" type="checkbox">启用</label>
        <button :disabled="saving" type="submit">保存定时</button>
        <p>漏过的 tick 合并为最近一次；编辑器不会自行扩展 cron 方言。</p>
      </form>

    </template>
  </section>
</template>

<style scoped>
.schedule-editor { color: #16222c; margin-block-start: 1.5rem; }
.schedule-list { display: grid; gap: .55rem; margin-block: 1rem; }
.schedule-list button { display: grid; gap: .25rem; text-align: left; padding: .75rem; border: 1px solid #c9d2dc; border-left: .35rem solid #8b6f24; border-radius: .55rem; background: #fffaf0; color: #302716; }
.editor { display: grid; gap: .75rem; max-width: 46rem; padding: 1rem; border: 1px solid #d6c89f; border-radius: .7rem; background: #fcf8ed; }
label { display: grid; gap: .3rem; font-weight: 650; }
input { min-height: 2.25rem; padding: .35rem .5rem; border: 1px solid #cbbf9a; border-radius: .45rem; }
button { cursor: pointer; border: 1px solid #54451e; border-radius: .45rem; background: #f0e3bd; color: #352a0d; padding: .45rem .75rem; font: inherit; }
button:disabled { cursor: not-allowed; opacity: .55; }
</style>
