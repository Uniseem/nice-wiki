<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { api, messageOf } from './client';

const options = ref<any>(null);
const config = ref<any>(null);
const builds = ref<any[]>([]);
const error = ref('');
const notice = ref('');
const busy = ref(false);
const form = ref({ profileId: '', enabled: true, chunkTokens: 512, overlapTokens: 64 });

const key = () => crypto.randomUUID().replaceAll('-', '');
const terminal = new Set(['active', 'cancelled', 'failed', 'superseded']);

async function load() {
  options.value = await api('/v1/index-options');
  try {
    config.value = (await api<{ config: any }>('/v1/index-config')).config;
    form.value = { profileId: config.value.embeddingProfileId, enabled: config.value.enabled, chunkTokens: config.value.chunkTokens, overlapTokens: config.value.overlapTokens };
  } catch { config.value = null; }
  builds.value = (await api<{ builds: any[] }>('/v1/builds?limit=50')).builds;
}

async function run(action: () => Promise<void>, done: string) {
  busy.value = true; error.value = ''; notice.value = '';
  try { await action(); await load(); notice.value = done; } catch (e) { error.value = messageOf(e); } finally { busy.value = false; }
}

const saveConfig = () => run(async () => {
  const profile = options.value.profiles.find((p: any) => p.profileId === form.value.profileId);
  if (!profile?.space) throw new Error('请选择可用的嵌入档位');
  const idempotencyKey = key();
  await api('/v1/index-config', { method: 'PUT', idempotencyKey, body: { idempotencyKey, expectedVersion: config.value?.version ?? 0, config: {
    configId: config.value?.configId ?? key(), version: config.value?.version ?? 0, enabled: form.value.enabled, embeddingProfileId: profile.profileId, space: profile.space,
    chunkTokens: Number(form.value.chunkTokens), overlapTokens: Number(form.value.overlapTokens), language: options.value.baseLanguage,
  } } });
}, '索引配置已保存');

const build = (doc: any) => {
  if (!confirm(`为「${doc.title}」建立索引会调用嵌入模型，可能产生费用。继续？`)) return;
  return run(async () => {
    const idempotencyKey = key();
    await api('/v1/builds', { method: 'POST', idempotencyKey, body: { idempotencyKey, document: doc.reference, force: false } });
  }, '已提交索引任务');
};

const cancel = (b: any) => run(async () => {
  const idempotencyKey = key();
  await api(`/v1/builds/${b.buildId}/cancel`, { method: 'POST', idempotencyKey, body: { buildId: b.buildId, reason: '管理员取消', idempotencyKey } });
}, '已取消');

const titleOf = (id: string) => options.value?.documents.find((d: any) => d.documentId === id)?.title ?? id;

onMounted(async () => { try { await load(); } catch (e) { error.value = messageOf(e); } });
</script>

<template>
  <div class="page" v-if="options">
    <h1>知识库索引</h1>
    <p v-if="error" class="err">{{ error }}</p>
    <p v-if="notice" class="ok">{{ notice }}</p>
    <p v-if="options.modelsState !== 'ready'" class="hint">模型目录暂不可用，无法配置索引（{{ options.modelErrorCode }}）。</p>
    <p v-else-if="!options.profiles.some((p: any) => p.tier === 'embedding')" class="hint">还没有嵌入档位：先到「模型接入」配置供应方与模型。</p>

    <fieldset>
      <legend>入库配置</legend>
      <form @submit.prevent="saveConfig">
        <label>嵌入档位</label>
        <select v-model="form.profileId" required>
          <option value="" disabled>请选择</option>
          <option v-for="p in options.profiles.filter((x: any) => x.tier === 'embedding')" :key="p.profileId" :value="p.profileId" :disabled="!p.usable">{{ p.label }}{{ p.usable ? '' : '（不可用）' }}</option>
        </select>
        <label>分块大小（128–1024）</label><input type="number" min="128" max="1024" v-model="form.chunkTokens">
        <label>重叠（0–128）</label><input type="number" min="0" max="128" v-model="form.overlapTokens">
        <label class="perm"><input type="checkbox" v-model="form.enabled">启用入库</label>
        <div style="margin-top:.8rem"><button class="primary" :disabled="busy">保存</button></div>
      </form>
    </fieldset>

    <h2>已发布文档</h2>
    <table>
      <thead><tr><th>文档</th><th></th></tr></thead>
      <tbody>
        <tr v-for="d in options.documents" :key="d.documentId">
          <td>{{ d.title }}</td>
          <td><button :disabled="busy || !config?.enabled" @click="build(d)">建立索引</button></td>
        </tr>
        <tr v-if="!options.documents.length"><td colspan="2" class="hint">还没有已发布文档。</td></tr>
      </tbody>
    </table>

    <h2>索引任务</h2>
    <table>
      <thead><tr><th>文档</th><th>状态</th><th>进度</th><th></th></tr></thead>
      <tbody>
        <tr v-for="b in builds" :key="b.buildId">
          <td>{{ titleOf(b.document.documentId) }}</td>
          <td>{{ b.state }}<span v-if="b.failureCode" class="err">（{{ b.failureCode }}）</span></td>
          <td>{{ b.completedChunks }} / {{ b.totalChunks }}</td>
          <td><button v-if="!terminal.has(b.state)" :disabled="busy" @click="cancel(b)">取消</button></td>
        </tr>
        <tr v-if="!builds.length"><td colspan="4" class="hint">暂无任务。</td></tr>
      </tbody>
    </table>
  </div>
  <p v-else-if="error" class="err">{{ error }}</p>
  <p v-else class="hint">加载中…</p>
</template>
