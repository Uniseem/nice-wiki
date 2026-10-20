<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { api, messageOf } from './client';

type Language = { tag: string; version: number; enabled: boolean; isDefault: boolean; automaticTranslation: boolean; direction: 'ltr' | 'rtl'; reflectionEnabled: boolean; batchOutputTokens: number; translationMemoryMinimumCharacters: number };

const languages = ref<Language[]>([]);
const error = ref('');
const notice = ref('');
const busy = ref(false);
const tag = ref('');
const rtlPrefixes = ['ar', 'he', 'fa', 'ur'];

async function load() {
  languages.value = (await api<{ languages: Language[] }>('/v1/languages?includeDisabled=true')).languages;
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
          <td>{{ l.automaticTranslation ? '发布时自动翻译' : '关闭' }}</td>
          <td>
            <button v-if="!l.isDefault && !l.enabled" :disabled="busy" @click="setEnabled(l, true)">启用</button>
            <button v-else-if="!l.isDefault" :disabled="busy" @click="setEnabled(l, false)">停用</button>
          </td>
        </tr>
      </tbody>
    </table>
    <form class="row" @submit.prevent="add">
      <input type="text" v-model="tag" placeholder="语言标签，如 en、ja、zh-Hant" required pattern="[A-Za-z]{2,8}(-[A-Za-z0-9]{1,8})*" style="max-width:16rem">
      <button class="primary" :disabled="busy">添加语言</button>
    </form>
  </div>
</template>
