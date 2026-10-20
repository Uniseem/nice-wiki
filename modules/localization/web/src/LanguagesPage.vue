<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { api, messageOf } from './client';

type Language = { tag: string; version: number; enabled: boolean; isDefault: boolean; automaticTranslation: boolean; direction: 'ltr' | 'rtl'; reflectionEnabled: boolean; batchOutputTokens: number; translationMemoryMinimumCharacters: number };

const languages = ref<Language[]>([]);
const error = ref('');
const notice = ref('');
const busy = ref(false);
const tag = ref('');
const nodes = ref<any[]>([]);
const progress = ref<any[]>([]);
const target = ref('');
const rtlPrefixes = ['ar', 'he', 'fa', 'ur'];

async function load() {
  languages.value = (await api<{ languages: Language[] }>('/v1/languages?includeDisabled=true')).languages;
  const targets = languages.value.filter((l) => l.enabled && !l.isDefault);
  if (!targets.some((l) => l.tag === target.value)) target.value = targets[0]?.tag ?? '';
  const base = languages.value.find((l) => l.isDefault)?.tag;
  try { nodes.value = base ? await (await fetch(`/${base}/navigation`)).json() : []; } catch { nodes.value = []; }
  try { progress.value = (await api<{ languages: any[] }>('/v1/translations/progress')).languages; } catch { progress.value = []; }
}
async function run(action: () => Promise<void>, done = '') {
  busy.value = true; error.value = ''; notice.value = '';
  try { await action(); await load(); if (done) notice.value = done; } catch (e) { error.value = messageOf(e); } finally { busy.value = false; }
}
const key = () => crypto.randomUUID().replaceAll('-', '');

const add = () => run(async () => {
  const value = tag.value.trim();
  const direction = rtlPrefixes.includes(value.split('-')[0]) ? 'rtl' : 'ltr';
  const idempotencyKey = key();
  await api(`/v1/languages/${encodeURIComponent(value)}`, { method: 'PUT', idempotencyKey, body: { expectedVersion: 0, idempotencyKey, language: {
    tag: value, version: 1, enabled: false, isDefault: false, automaticTranslation: false, direction, reflectionEnabled: false, batchOutputTokens: 1000, translationMemoryMinimumCharacters: 20 } } });
  tag.value = '';
}, '语言已添加（默认未启用；启用后访客才能看到该语言的页面）');

const setEnabled = (language: Language, enabled: boolean) => run(async () => {
  const idempotencyKey = key();
  await api(`/v1/languages/${encodeURIComponent(language.tag)}/enabled`, { method: 'POST', idempotencyKey, body: { tag: language.tag, enabled, expectedVersion: language.version, idempotencyKey } });
});

const translate = (node: any) => {
  if (!confirm(`翻译「${node.title ?? node.documentId}」到 ${target.value} 会调用模型，可能产生费用。继续？`)) return;
  return run(async () => {
    const idempotencyKey = key();
    await api('/v1/translations', { method: 'POST', idempotencyKey, body: { document: node.publication, targetLanguage: target.value, idempotencyKey } });
  }, '已开始翻译，完成后访客就能看到该语言的页面；稍后刷新查看进度。');
};

const setAuto = (language: Language, on: boolean) => {
  if (on && !confirm('开启后，每篇文档发布新版本时都会自动调用模型翻译到该语言，可能产生费用。确认开启？')) return;
  return run(async () => {
    const idempotencyKey = key();
    await api(`/v1/languages/${encodeURIComponent(language.tag)}`, { method: 'PUT', idempotencyKey, body: { expectedVersion: language.version, idempotencyKey, language: { ...language, version: language.version + 1, automaticTranslation: on } } });
  });
};

onMounted(() => run(async () => {}));
</script>

<template>
  <div class="page">
    <h1>语言</h1>
    <p v-if="error" class="err">{{ error }}</p>
    <p v-if="notice" class="ok">{{ notice }}</p>
    <p class="hint">基准语言的页面直接来自文档正文。其他语言需要翻译：翻译会调用模型并产生费用，必须先在“模型接入”里配置并由你批准。</p>
    <table>
      <thead><tr><th>语言标签</th><th>方向</th><th>状态</th><th>自动翻译</th><th></th></tr></thead>
      <tbody>
        <tr v-for="l in languages" :key="l.tag">
          <td>{{ l.tag }}<span v-if="l.isDefault" class="hint">（基准语言）</span></td>
          <td>{{ l.direction === 'rtl' ? '从右到左' : '从左到右' }}</td>
          <td>{{ l.enabled ? '已启用' : '未启用' }}</td>
          <td>{{ l.automaticTranslation ? '发布时自动翻译' : '关闭' }}
            <button v-if="!l.isDefault && l.enabled" :disabled="busy" @click="setAuto(l, !l.automaticTranslation)">{{ l.automaticTranslation ? '关闭' : '开启' }}</button></td>
          <td>
            <button v-if="!l.isDefault && !l.enabled" :disabled="busy" @click="setEnabled(l, true)">启用</button>
            <button v-else-if="!l.isDefault" :disabled="busy" @click="setEnabled(l, false)">停用</button>
          </td>
        </tr>
      </tbody>
    </table>
    <h2>文档译文</h2>
    <p v-if="!languages.some((l) => l.enabled && !l.isDefault)" class="hint">先启用一个非基准语言，才能翻译文档。</p>
    <template v-else>
      <div class="row">
        <label>目标语言</label>
        <select v-model="target" style="width:auto"><option v-for="l in languages.filter((x) => x.enabled && !x.isDefault)" :key="l.tag" :value="l.tag">{{ l.tag }}</option></select>
        <span v-for="p in progress.filter((x) => x.language === target)" :key="p.language" class="hint">已完成 {{ p.completedUnits }} 块，进行中 {{ p.queuedUnits }}，待复核 {{ p.reviewUnits }}</span>
      </div>
      <table>
        <thead><tr><th>已发布文档</th><th></th></tr></thead>
        <tbody>
          <tr v-for="n in nodes" :key="n.documentId"><td>{{ n.title ?? n.documentId }}</td><td><button :disabled="busy" @click="translate(n)">翻译</button></td></tr>
          <tr v-if="!nodes.length"><td colspan="2" class="hint">还没有已发布文档。</td></tr>
        </tbody>
      </table>
    </template>

    <h2>添加语言</h2>
    <form class="row" @submit.prevent="add">
      <input type="text" v-model="tag" placeholder="语言标签，如 en、ja、zh-Hant" required pattern="[A-Za-z]{2,8}(-[A-Za-z0-9]{1,8})*" style="max-width:16rem">
      <button class="primary" :disabled="busy">添加语言</button>
    </form>
  </div>
</template>
