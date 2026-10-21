<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { api, messageOf } from './client';

const labels: Record<string, string> = {
  enabled: '公开问答', cacheEnabled: '答案缓存', compactionEnabled: '长对话压缩', profileId: '答案模型档位',
  maximumOutputTokens: '回答最大输出词元', maximumHistoryTokens: '历史最大词元', maximumContextTokens: '上下文最大词元',
  maximumQuestionCharacters: '问题最大字符数', preflightOutputTokens: '预检输出词元', inspectionOutputTokens: '核查输出词元',
  maximumSubqueries: '最多子问题数', maximumRetrievalRounds: '最多检索轮数', maximumToolCalls: '最多工具调用', cacheTtlDays: '缓存保留天数',
  recordRetentionDays: '问答记录保留天数', retainedRecentTokens: '压缩时保留的近期词元', contextReservePercent: '上下文预留比例（%）',
  semanticThreshold: '语义相似阈值（0–1）', semanticMode: '语义判定方式',
};
const skip = new Set(['version', 'retrievalSpace']);

const policy = ref<Record<string, any> | null>(null);
const version = ref(0);
const turns = ref<any[]>([]);
const cacheEntries = ref<any[]>([]);
// 调试台：不经访客准入直接试问一个问题，看回答、引用和失败原因；费用按后台任务类别计。
const dbg = ref({ question: '', profileId: 'background-accurate', running: false, answer: '', citations: [] as any[], status: '', error: '' });
async function runDebug() {
  if (!dbg.value.question.trim() || dbg.value.running) return;
  Object.assign(dbg.value, { running: true, answer: '', citations: [], status: '正在提问…', error: '' });
  try {
    const idempotencyKey = crypto.randomUUID().replaceAll('-', '');
    const created = await api<{ debug: { debugId: string } }>('/v1/admin/debug-turns', { method: 'POST', idempotencyKey, body: { question: dbg.value.question, profileId: dbg.value.profileId, language: 'zh-Hans', idempotencyKey } });
    for (let i = 0; i < 90; i++) {
      await new Promise((resolve) => setTimeout(resolve, 2000));
      const read = await api<{ debug: any; events: any[] }>(`/v1/admin/debug-turns/${created.debug.debugId}?afterSequence=0`);
      dbg.value.answer = read.events.filter((e) => e.event === 'delta').map((e) => e.delta).join('');
      dbg.value.citations = read.events.filter((e) => e.event === 'citation').map((e) => e.citation);
      const last = read.events[read.events.length - 1];
      if (last?.event === 'completed') { dbg.value.status = '完成'; break; }
      if (last?.event === 'failed') { dbg.value.status = '失败'; dbg.value.error = `失败原因：${last.code ?? '未知'}（常见原因：诊断档位不存在或不属于“后台任务”类别、知识库没有索引、模型没有配置）`; break; }
    }
  } catch (e) { dbg.value.status = '失败'; dbg.value.error = messageOf(e); }
  finally { dbg.value.running = false; }
}
const error = ref('');
const notice = ref('');
const busy = ref(false);

async function load() {
  const current = await api<{ policy: Record<string, any>; version: number }>('/v1/admin/policy');
  policy.value = current.policy;
  version.value = current.version;
  try { turns.value = (await api<{ items: any[] }>('/v1/admin/turns?limit=20')).items ?? []; } catch { turns.value = []; }
  try { cacheEntries.value = (await api<{ entries: any[] }>('/v1/admin/cache?limit=30')).entries ?? []; } catch { cacheEntries.value = []; }
}
async function run(action: () => Promise<void>, done = '') {
  busy.value = true; error.value = ''; notice.value = '';
  try { await action(); await load(); if (done) notice.value = done; } catch (e) { error.value = messageOf(e); } finally { busy.value = false; }
}
// 精选：把一轮已完成的问答清理成公开的问题和答案（待批准），批准后长期有效、优先命中，仍随来源撤回/改动失效。
const curate = ref<{ turnId: string; question: string; answer: string } | null>(null);
async function openCurate(turn: any) {
  try {
    const record = await api<{ question: string; answer: string }>(`/v1/admin/turns/${turn.turnId}`);
    curate.value = { turnId: turn.turnId, question: record.question, answer: record.answer };
  } catch (e) { error.value = messageOf(e); }
}
const submitCurate = () => run(async () => {
  if (!curate.value) return;
  const idempotencyKey = crypto.randomUUID().replaceAll('-', '');
  await api('/v1/admin/cache', { method: 'POST', idempotencyKey, body: { turnId: curate.value.turnId, publicQuestion: curate.value.question, publicAnswer: curate.value.answer, idempotencyKey } });
  curate.value = null;
}, '已提交为待批准的精选条目，请在下方“答案缓存条目”里批准。');
const approveCache = (entry: any) => run(async () => {
  const idempotencyKey = crypto.randomUUID().replaceAll('-', '');
  await api(`/v1/admin/cache/${entry.cacheId}/approve`, { method: 'POST', idempotencyKey, body: { cacheId: entry.cacheId, expectedVersion: entry.version, idempotencyKey } });
}, '精选条目已批准。');
const disableCache = (entry: any) => {
  if (!confirm(`禁用后这条缓存答案不会再被复用：\n「${entry.question}」`)) return;
  return run(async () => {
    const idempotencyKey = crypto.randomUUID().replaceAll('-', '');
    await api(`/v1/admin/cache/${entry.cacheId}/disable`, { method: 'POST', idempotencyKey, body: { cacheId: entry.cacheId, expectedVersion: entry.version, idempotencyKey } });
  }, '已禁用该缓存条目');
};
const save = () => run(async () => {
  const idempotencyKey = crypto.randomUUID().replaceAll('-', '');
  await api('/v1/admin/policy', { method: 'PUT', idempotencyKey, body: { policy: policy.value, expectedVersion: version.value, idempotencyKey } });
}, '策略已保存');
onMounted(() => run(async () => {}));
</script>

<template>
  <div class="page">
    <h1>问答管理</h1>
    <p v-if="error" class="err">{{ error }}</p>
    <p v-if="notice" class="ok">{{ notice }}</p>
    <p class="hint">公开问答默认关闭。开启前需要在“模型接入”里配置好模型，并在“防滥用”里确认访客限额；每次回答都会调用模型并产生费用。</p>
    <fieldset v-if="policy">
      <legend>问答策略（版本 {{ version }}）</legend>
      <form @submit.prevent="save">
        <template v-for="(value, key) in policy" :key="key">
          <template v-if="!skip.has(String(key))">
            <label v-if="typeof value === 'boolean'" class="perm"><input type="checkbox" v-model="policy[key]">{{ labels[key] ?? key }}</label>
            <template v-else-if="typeof value === 'number'"><label>{{ labels[key] ?? key }}</label><input type="number" step="any" v-model.number="policy[key]"></template>
            <template v-else><label>{{ labels[key] ?? key }}</label><input type="text" v-model="policy[key]"></template>
          </template>
        </template>
        <div style="margin-top:.8rem"><button class="primary" :disabled="busy">保存策略</button></div>
      </form>
    </fieldset>
    <fieldset>
      <legend>最近的问答记录</legend>
      <p v-if="!turns.length" class="hint">还没有问答记录。</p>
      <table v-else><thead><tr><th>时间</th><th>问题</th><th>状态</th></tr></thead>
        <tbody><tr v-for="t in turns" :key="t.turnId"><td>{{ t.createdAt }}</td><td>{{ t.questionPreview }}</td><td>{{ t.answerKind || '未完成' }}{{ t.feedback ? '（反馈：' + t.feedback + '）' : '' }} <button v-if="t.answerKind === 'answered'" :disabled="busy" @click="openCurate(t)">设为精选</button></td></tr></tbody></table>
      <form v-if="curate" @submit.prevent="submitCurate" style="margin-top:.6rem">
        <label>公开问题（去掉个人信息后的通用问法）</label><input type="text" v-model="curate.question" required>
        <label>公开答案</label><textarea v-model="curate.answer" rows="4" required></textarea>
        <div class="row" style="margin-top:.4rem"><button class="primary" :disabled="busy">提交精选</button><button type="button" @click="curate = null">取消</button></div>
      </form>
    </fieldset>
    <fieldset>
      <legend>调试台</legend>
      <p class="hint">管理员直接试问一个问题：检索知识库并调用模型回答，结果只保存在隔离的诊断记录里（7 天），不影响访客对话和缓存。会产生模型费用，按“后台任务”类别计。</p>
      <form class="row" @submit.prevent="runDebug">
        <input type="text" v-model="dbg.question" placeholder="要试问的问题" maxlength="500" style="min-width:18rem" required>
        <input type="text" v-model="dbg.profileId" placeholder="诊断档位 ID（后台任务类别）" style="max-width:14rem" required>
        <button class="primary" :disabled="dbg.running">{{ dbg.running ? '诊断中…' : '试问' }}</button>
      </form>
      <p v-if="dbg.status" class="hint">{{ dbg.status }}</p>
      <p v-if="dbg.error" class="err">{{ dbg.error }}</p>
      <div v-if="dbg.answer"><strong>回答</strong><pre style="white-space:pre-wrap">{{ dbg.answer }}</pre></div>
      <ul v-if="dbg.citations.length"><li v-for="(c, i) in dbg.citations" :key="i">来源：{{ c.title || c.href }} <span class="hint">{{ c.href }}</span></li></ul>
    </fieldset>
    <fieldset>
      <legend>答案缓存条目</legend>
      <p v-if="!cacheEntries.length" class="hint">还没有缓存条目。打开“答案缓存”后，完整且可共享的问题回答完会自动写入；撤回或改动来源文档后，相关缓存会因引用核验失败而不再命中。</p>
      <table v-else><thead><tr><th>问题</th><th>答案</th><th>语言</th><th>过期</th><th></th></tr></thead>
        <tbody><tr v-for="c in cacheEntries" :key="c.cacheId"><td>{{ c.question }}</td><td>{{ (c.answer || '').slice(0, 80) }}</td><td>{{ c.language }}</td><td>{{ c.curated ? '精选（长期）' : (c.expiresAt ?? '') }} {{ c.state === 'pending' ? '· 待批准' : c.state === 'disabled' ? '· 已禁用' : '' }}</td><td><button v-if="c.state === 'pending'" :disabled="busy" @click="approveCache(c)">批准</button> <button v-if="c.state !== 'disabled'" :disabled="busy" @click="disableCache(c)">禁用</button></td></tr></tbody></table>
    </fieldset>
  </div>
</template>
