<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { api, messageOf } from './client';

const labels: Record<string, string> = {
  qaPerSubjectPerMinute: '每位访客每分钟提问上限', qaBurst: '每位访客突发次数', qaPerSitePerMinute: '全站每分钟提问上限',
  qaSubjectConcurrency: '每位访客并发', qaSiteConcurrency: '全站并发', qaPerNetworkPerMinute: '每个网络每分钟上限',
  challengeEnabled: '启用工作量证明挑战', externalChallengeEnabled: '启用外部挑战（需要另行配置密钥）',
};

const policy = ref<Record<string, any> | null>(null);
const revision = ref(0);
const siteId = ref('');
const bans = ref<any[]>([]);
const events = ref<any[]>([]);
const error = ref('');
const notice = ref('');
const busy = ref(false);
const banRef = ref('');
const banTtl = ref(3600);
const banReason = ref('manual');

async function load() {
  const [self, current, banList, riskList] = await Promise.all([
    api<{ account: { siteId: string } }>('/v1/self'),
    // 还没保存过策略时给出默认值，保存后版本从 0 起。
    api<{ policyJSON: Record<string, any>; revision: number }>('/v1/policies').catch((e: any) => {
      if (e?.status !== 404) throw e;
      return { revision: 0, policyJSON: { qaPerSubjectPerMinute: 30, qaBurst: 5, qaPerSitePerMinute: 600, qaSubjectConcurrency: 1, qaSiteConcurrency: 32, challengeEnabled: true, externalChallengeEnabled: false, qaPerNetworkPerMinute: 60 } };
    }),
    api<{ items: any[] }>('/v1/bans?limit=50'),
    api<{ items: any[] }>('/v1/risk-events?limit=50'),
  ]);
  siteId.value = self.account.siteId;
  policy.value = current.policyJSON;
  revision.value = current.revision;
  bans.value = banList.items;
  events.value = riskList.items;
}

async function run(action: () => Promise<void>, done = '') {
  busy.value = true; error.value = ''; notice.value = '';
  try { await action(); await load(); if (done) notice.value = done; } catch (e) { error.value = messageOf(e); } finally { busy.value = false; }
}

const savePolicy = () => run(async () => {
  await api('/v1/policies', { method: 'PUT', body: { siteId: siteId.value, expectedRevision: revision.value, policyJSON: policy.value } });
}, '策略已保存');
const createBan = () => run(async () => {
  await api('/v1/bans', { method: 'POST', body: { subjectRef: banRef.value.trim(), scope: 'ai', ttlSeconds: banTtl.value, reasonCode: banReason.value } });
  banRef.value = '';
}, '禁令已建立');
const revoke = (ban: any) => run(async () => { await api(`/v1/bans/${ban.banId}`, { method: 'DELETE', body: { expectedRevision: ban.revision } }); }, '禁令已撤销');

onMounted(() => run(async () => {}));
</script>

<template>
  <div class="page">
    <h1>防滥用</h1>
    <p v-if="error" class="err">{{ error }}</p>
    <p v-if="notice" class="ok">{{ notice }}</p>

    <fieldset v-if="policy">
      <legend>访客提问准入策略（版本 {{ revision }}）</legend>
      <form @submit.prevent="savePolicy">
        <template v-for="(value, key) in policy" :key="key">
          <label v-if="typeof value === 'boolean'" class="perm"><input type="checkbox" v-model="policy[key]">{{ labels[key] ?? key }}</label>
          <template v-else><label>{{ labels[key] ?? key }}</label><input type="number" v-model.number="policy[key]" min="1"></template>
        </template>
        <div style="margin-top:.8rem"><button class="primary" :disabled="busy">保存策略</button></div>
      </form>
    </fieldset>

    <fieldset>
      <legend>禁令</legend>
      <table>
        <thead><tr><th>对象引用</th><th>原因</th><th>状态</th><th>到期</th><th></th></tr></thead>
        <tbody>
          <tr v-for="b in bans" :key="b.banId">
            <td><code>{{ b.subjectRef.slice(0, 16) }}…</code></td><td>{{ b.reasonCode === 'abuse' ? '滥用' : '人工' }}</td>
            <td>{{ ({ active: '生效中', expired: '已过期', revoked: '已撤销' } as any)[b.state] }}</td><td>{{ new Date(b.expiresAt).toLocaleString() }}</td>
            <td><button v-if="b.state === 'active'" :disabled="busy" @click="revoke(b)">撤销</button></td>
          </tr>
        </tbody>
      </table>
      <p v-if="!bans.length" class="hint">没有禁令。</p>
      <form class="row" @submit.prevent="createBan">
        <input type="text" v-model="banRef" placeholder="不可逆网络引用（来自下方可疑事件）" required pattern="[0-9a-f]{64}" style="max-width:28rem">
        <input type="number" v-model.number="banTtl" min="60" max="2592000" style="width:8rem" title="持续秒数">
        <select v-model="banReason" style="width:auto"><option value="manual">人工</option><option value="abuse">滥用</option></select>
        <button class="primary" :disabled="busy">建立禁令</button>
      </form>
    </fieldset>

    <fieldset>
      <legend>可疑事件</legend>
      <table>
        <thead><tr><th>时间</th><th>动作</th><th>处置</th><th>网络标识</th></tr></thead>
        <tbody><tr v-for="e in events" :key="e.eventId"><td>{{ new Date(e.occurredAt).toLocaleString() }}</td><td>{{ e.action }}</td><td>{{ e.decision }}</td><td><code>{{ e.networkKey }}</code></td></tr></tbody>
      </table>
      <p v-if="!events.length" class="hint">暂无记录。</p>
    </fieldset>
  </div>
</template>
