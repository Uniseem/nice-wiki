<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { api, messageOf } from './client';

const catalog = ref<any>({ providers: [], models: [], profiles: [], taskAssignments: [] });
const error = ref('');
const notice = ref('');
const busy = ref(false);

const provider = ref({ providerId: '', adapter: 'openai_chat', baseUrl: 'https://', displayName: '' });
const secretFor = ref<Record<string, string>>({});
const model = ref({ modelId: '', providerId: '', providerModelVersion: '', capabilities: 'chat', dimension: 0, maxInputTokens: 128000, maxOutputTokens: 4096 });

const profile = ref({ profileId: '', costClass: 'background', tier: 'embedding', modelIds: [] as string[], maximumUsd: 0, concurrency: 4, dailyUsd: 0 });
const tiers: Record<string, string> = { fast: '快速', accurate: '准确', vision: '视觉', embedding: '向量', rerank: '重排' };
const costClasses: Record<string, string> = { qa: '问答', background: '后台任务', 'admin-probe': '管理员探测' };
// 任务目录来自 contracts/ai-tasks.json；这里只列要手动指定档位的常用任务。
const tasks = [
  ['qa.preflight', 'fast', '问答：前置分类'], ['qa.inspect', 'fast', '问答：检索核查'], ['qa.answer', 'accurate', '问答：最终回答'], ['qa.cache-match', 'fast', '问答：缓存判断'],
  ['qa.cache-translate', 'fast', '问答：缓存翻译'], ['qa.compact', 'fast', '问答：对话压缩'], ['knowledge.embed', 'embedding', '知识库：生成向量'], ['knowledge.rerank', 'rerank', '知识库：重排'],
  ['knowledge.context', 'fast', '知识库：检索上下文'], ['localization.document', 'accurate', '翻译：文档'], ['localization.ui', 'accurate', '翻译：界面文字'],
  ['localization.suggest', 'fast', '翻译：语言建议'], ['memory.overview', 'fast', '记忆：项目概述'], ['memory.assistant', 'accurate', '记忆：助手'], ['content.slug', 'fast', '发布：路径名'], ['content.seo', 'fast', '发布：SEO 描述'],
];
const overrides = ref<Record<string, string>>({});

const adapters: Record<string, string> = {
  openai_chat: 'OpenAI 兼容对话', anthropic_messages: 'Anthropic 消息', gemini: 'Gemini 对话',
  openai_embeddings: 'OpenAI 兼容向量', gemini_embeddings: 'Gemini 向量', rerank: '重排序',
};

async function load() {
  catalog.value = await api('/v1/admin/catalog');
}
async function run(action: () => Promise<void>, done = '') {
  busy.value = true; error.value = ''; notice.value = '';
  try { await action(); await load(); if (done) notice.value = done; } catch (e) { error.value = messageOf(e); } finally { busy.value = false; }
}
const key = () => crypto.randomUUID().replaceAll('-', '');

const addProvider = () => run(async () => {
  const idempotencyKey = key();
  const p = provider.value;
  await api(`/v1/admin/providers/${p.providerId}`, { method: 'PUT', idempotencyKey, body: { expectedVersion: 0, idempotencyKey, provider: {
    providerId: p.providerId, version: 1, adapter: p.adapter, baseUrl: p.baseUrl, enabled: true, credentialVersion: 0, connectTimeoutMs: 5000, requestTimeoutMs: 60000,
    displayName: p.displayName || p.providerId, extraHeaderNames: [], extraHeaderSecretRefs: [], concurrencyLimit: 4, streamIdleTimeoutMs: 30000 } } });
  provider.value = { providerId: '', adapter: 'openai_chat', baseUrl: 'https://', displayName: '' };
}, '供应方已添加；请接着填写密钥。');

const saveSecret = (p: any) => run(async () => {
  const idempotencyKey = key();
  await api(`/v1/admin/providers/${p.providerId}/credentials${p.credentialConfigured ? '/rotate' : ''}`, { method: 'POST', idempotencyKey,
    body: { providerId: p.providerId, secret: secretFor.value[p.providerId], expectedVersion: p.credentialVersion, idempotencyKey } });
  secretFor.value[p.providerId] = '';
}, '密钥已加密保存，不会再显示。');

const addModel = () => run(async () => {
  const idempotencyKey = key();
  const m = model.value;
  await api(`/v1/admin/models/${m.modelId}`, { method: 'PUT', idempotencyKey, body: { expectedVersion: 0, idempotencyKey, model: {
    modelId: m.modelId, version: 1, providerId: m.providerId, providerModelVersion: m.providerModelVersion,
    capabilities: m.capabilities.split(',').map((c) => c.trim()).filter(Boolean), dimension: m.dimension, preprocessorVersion: 'none',
    maxInputTokens: m.maxInputTokens, maxOutputTokens: m.maxOutputTokens, catalogOrigin: 'manual', manualOverride: true, enabled: true } } });
}, '模型已添加。');

const micros = (usd: number) => Math.round(Number(usd) * 1_000_000);
const addProfile = () => run(async () => {
  const idempotencyKey = key();
  const p = profile.value;
  const existing = catalog.value.profiles.find((x: any) => x.profileId === p.profileId);
  await api(`/v1/admin/profiles/${p.profileId}`, { method: 'PUT', idempotencyKey, body: { expectedVersion: existing?.version ?? 0, idempotencyKey, profile: {
    profileId: p.profileId, version: (existing?.version ?? 0) + 1, costClass: p.costClass, tier: p.tier, orderedModelIds: p.modelIds,
    maximumCost: { currency: 'USD', micros: micros(p.maximumUsd) }, concurrency: Number(p.concurrency), dailyLimitCost: { currency: 'USD', micros: micros(p.dailyUsd) } } } });
}, '档位已保存。');

const assignTask = (task: string[]) => run(async () => {
  const idempotencyKey = key();
  const [taskId, tier] = task;
  const existing = catalog.value.taskAssignments.find((x: any) => x.taskId === taskId);
  const model = overrides.value[taskId];
  await api(`/v1/task-assignments/${taskId}`, { method: 'PUT', idempotencyKey, body: { expectedVersion: existing?.version ?? 0, idempotencyKey, assignment: {
    taskId, tier, version: (existing?.version ?? 0) + 1, ...(model ? { overrideModelId: model } : {}) } } });
}, '任务分配已保存。');
const assigned = (taskId: string) => catalog.value.taskAssignments.find((x: any) => x.taskId === taskId)?.overrideModelId ?? '';

onMounted(() => run(async () => {}));
</script>

<template>
  <div class="page">
    <h1>模型接入</h1>
    <p v-if="error" class="err">{{ error }}</p>
    <p v-if="notice" class="ok">{{ notice }}</p>
    <p class="hint">这里只配置供应方、密钥和模型。真正调用模型（索引、问答、翻译）会产生费用，每次都需要你批准。密钥只在提交时出现一次，保存后加密存放。</p>

    <fieldset>
      <legend>供应方</legend>
      <table>
        <thead><tr><th>名称</th><th>协议</th><th>地址</th><th>密钥</th></tr></thead>
        <tbody>
          <tr v-for="p in catalog.providers" :key="p.providerId">
            <td>{{ p.displayName }}<br><span class="hint">{{ p.providerId }}</span></td><td>{{ adapters[p.adapter] ?? p.adapter }}</td><td>{{ p.baseUrl }}</td>
            <td>
              <span :class="p.credentialConfigured ? 'ok' : 'err'">{{ p.credentialConfigured ? `已配置（版本 ${p.credentialVersion}）` : '未配置' }}</span>
              <form class="row" @submit.prevent="saveSecret(p)">
                <input type="password" v-model="secretFor[p.providerId]" placeholder="输入密钥" autocomplete="off" style="max-width:14rem">
                <button :disabled="busy || !secretFor[p.providerId]">{{ p.credentialConfigured ? '轮换' : '保存' }}</button>
              </form>
            </td>
          </tr>
        </tbody>
      </table>
      <p v-if="!catalog.providers.length" class="hint">还没有供应方。</p>
      <form class="row" @submit.prevent="addProvider">
        <input type="text" v-model="provider.providerId" placeholder="供应方 ID（字母数字）" required pattern="[A-Za-z0-9._:-]+" style="max-width:12rem">
        <select v-model="provider.adapter" style="width:auto"><option v-for="(label, k) in adapters" :key="k" :value="k">{{ label }}</option></select>
        <input type="text" v-model="provider.baseUrl" placeholder="https://api.example.com/v1" required pattern="https?://.*" style="max-width:18rem">
        <input type="text" v-model="provider.displayName" placeholder="显示名" style="max-width:10rem">
        <button class="primary" :disabled="busy">添加供应方</button>
      </form>
    </fieldset>

    <fieldset>
      <legend>模型</legend>
      <table>
        <thead><tr><th>模型</th><th>供应方</th><th>能力</th><th>窗口</th><th>状态</th></tr></thead>
        <tbody>
          <tr v-for="m in catalog.models" :key="m.modelId">
            <td>{{ m.providerModelVersion }}<br><span class="hint">{{ m.modelId }}</span></td><td>{{ m.providerId }}</td><td>{{ m.capabilities.join('、') }}</td>
            <td>{{ m.maxInputTokens }} / {{ m.maxOutputTokens }}</td><td>{{ m.enabled ? '启用' : '停用' }}</td>
          </tr>
        </tbody>
      </table>
      <p v-if="!catalog.models.length" class="hint">还没有模型。</p>
      <form class="row" @submit.prevent="addModel">
        <input type="text" v-model="model.modelId" placeholder="网关模型 ID" required pattern="[A-Za-z0-9._:-]+" style="max-width:10rem">
        <select v-model="model.providerId" required style="width:auto"><option value="" disabled>选择供应方</option><option v-for="p in catalog.providers" :key="p.providerId" :value="p.providerId">{{ p.displayName }}</option></select>
        <input type="text" v-model="model.providerModelVersion" placeholder="供应方模型名" required style="max-width:12rem">
        <input type="text" v-model="model.capabilities" placeholder="能力，逗号分隔：chat,vision,embedding…" style="max-width:16rem">
        <button class="primary" :disabled="busy">添加模型</button>
      </form>
    </fieldset>

    <fieldset>
      <legend>档位</legend>
      <p class="hint">档位把模型按“快速/准确/视觉/向量/重排”分级并绑定费用类别；同一级里排第一的模型为主，其余是备用。</p>
      <table v-if="catalog.profiles.length">
        <thead><tr><th>档位</th><th>类别</th><th>级别</th><th>模型（按顺序）</th><th>单次上限 / 日上限</th></tr></thead>
        <tbody>
          <tr v-for="p in catalog.profiles" :key="p.profileId">
            <td>{{ p.profileId }}</td><td>{{ costClasses[p.costClass] }}</td><td>{{ tiers[p.tier] }}</td><td>{{ p.orderedModelIds.join(' → ') }}</td>
            <td>{{ p.maximumCost.micros ? '$' + p.maximumCost.micros / 1e6 : '不限' }} / {{ p.dailyLimitCost.micros ? '$' + p.dailyLimitCost.micros / 1e6 : '不限' }}</td>
          </tr>
        </tbody>
      </table>
      <form @submit.prevent="addProfile">
        <div class="row">
          <input type="text" v-model="profile.profileId" placeholder="档位 ID" required pattern="[A-Za-z0-9._:-]+" style="max-width:10rem">
          <select v-model="profile.costClass" style="width:auto"><option v-for="(l, k) in costClasses" :key="k" :value="k">{{ l }}</option></select>
          <select v-model="profile.tier" style="width:auto"><option v-for="(l, k) in tiers" :key="k" :value="k">{{ l }}</option></select>
        </div>
        <label>模型（可多选，勾选顺序即优先顺序）</label>
        <div><label v-for="m in catalog.models" :key="m.modelId" class="perm"><input type="checkbox" :value="m.modelId" v-model="profile.modelIds">{{ m.providerModelVersion }}</label></div>
        <div class="row">
          <label>单次费用上限（美元，0 不限）</label><input type="number" min="0" step="0.01" v-model="profile.maximumUsd" style="max-width:7rem">
          <label>每日上限（美元，0 不限）</label><input type="number" min="0" step="0.01" v-model="profile.dailyUsd" style="max-width:7rem">
          <label>并发</label><input type="number" min="0" v-model="profile.concurrency" style="max-width:5rem">
        </div>
        <div style="margin-top:.6rem"><button class="primary" :disabled="busy || !profile.modelIds.length">保存档位</button></div>
      </form>
    </fieldset>

    <fieldset>
      <legend>任务分配</legend>
      <p class="hint">每个 AI 任务默认走对应级别的档位；需要时可以为单个任务指定某个模型。</p>
      <table>
        <thead><tr><th>任务</th><th>默认级别</th><th>指定模型</th><th></th></tr></thead>
        <tbody>
          <tr v-for="t in tasks" :key="t[0]">
            <td>{{ t[2] }}<br><span class="hint">{{ t[0] }}</span></td><td>{{ tiers[t[1]] }}</td>
            <td><select v-model="overrides[t[0]]" style="width:auto"><option value="">（按档位）</option><option v-for="m in catalog.models" :key="m.modelId" :value="m.modelId">{{ m.providerModelVersion }}</option></select>
              <span v-if="assigned(t[0])" class="hint">当前：{{ assigned(t[0]) }}</span></td>
            <td><button :disabled="busy" @click="assignTask(t)">保存</button></td>
          </tr>
        </tbody>
      </table>
    </fieldset>
  </div>
</template>
