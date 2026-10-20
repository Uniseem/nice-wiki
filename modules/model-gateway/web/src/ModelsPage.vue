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
        <input type="text" v-model="provider.baseUrl" placeholder="https://api.example.com/v1" required pattern="https://.*" style="max-width:18rem">
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
      <legend>档位与任务分配</legend>
      <p class="hint">档位 {{ catalog.profiles.length }} 个，任务分配 {{ catalog.taskAssignments.length }} 项。档位和任务的编辑后续补充。</p>
    </fieldset>
  </div>
</template>
