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
const error = ref('');
const notice = ref('');
const busy = ref(false);

async function load() {
  const current = await api<{ policy: Record<string, any>; version: number }>('/v1/admin/policy');
  policy.value = current.policy;
  version.value = current.version;
  try { turns.value = (await api<{ turns: any[] }>('/v1/admin/turns?limit=20')).turns ?? []; } catch { turns.value = []; }
}
async function run(action: () => Promise<void>, done = '') {
  busy.value = true; error.value = ''; notice.value = '';
  try { await action(); await load(); if (done) notice.value = done; } catch (e) { error.value = messageOf(e); } finally { busy.value = false; }
}
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
        <tbody><tr v-for="t in turns" :key="t.turnId"><td>{{ t.createdAt }}</td><td>{{ t.question }}</td><td>{{ t.state }}</td></tr></tbody></table>
    </fieldset>
  </div>
</template>
