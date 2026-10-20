<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { api, messageOf } from './client';

type Entry = { entryId: string; version: number; title: string; body: string; bodyHash: string; language: string; tags: string[]; priority: number; state: string; updatedAt: string; description: string; maximumBytes: number; readonly: boolean; source: string; autoUpdateEnabled: boolean; fileRef: any };

const entries = ref<Entry[]>([]);
const stateFilter = ref('active');
const error = ref('');
const notice = ref('');
const busy = ref(false);
const editing = ref<Entry | null>(null);
const draft = ref({ title: '', description: '', body: '', language: 'zh-Hans', priority: 50, tags: '' });

async function sha256(text: string): Promise<string> {
  const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(text));
  return Array.from(new Uint8Array(digest), (b) => b.toString(16).padStart(2, '0')).join('');
}
const key = () => crypto.randomUUID().replaceAll('-', '');

async function load() {
  entries.value = (await api<{ entries: Entry[] }>(`/v1/entries?state=${stateFilter.value}&limit=100`)).entries;
}
async function run(action: () => Promise<void>, done = '') {
  busy.value = true; error.value = ''; notice.value = '';
  try { await action(); await load(); if (done) notice.value = done; } catch (e) { error.value = messageOf(e); } finally { busy.value = false; }
}

function startNew() {
  editing.value = null;
  draft.value = { title: '', description: '', body: '', language: 'zh-Hans', priority: 50, tags: '' };
}
function startEdit(entry: Entry) {
  editing.value = entry;
  draft.value = { title: entry.title, description: entry.description, body: entry.body, language: entry.language, priority: entry.priority, tags: entry.tags.join(', ') };
}

const save = () => run(async () => {
  const d = draft.value;
  const tags = d.tags.split(',').map((t) => t.trim()).filter(Boolean);
  const bodyHash = await sha256(d.body);
  const idempotencyKey = key();
  const now = new Date().toISOString().replace(/\.\d+Z$/, 'Z');
  if (editing.value) {
    const e = editing.value;
    await api(`/v1/entries/${e.entryId}`, { method: 'PUT', idempotencyKey, body: { expectedVersion: e.version, idempotencyKey, entry: { ...e, title: d.title, description: d.description, body: d.body, bodyHash, language: d.language, priority: d.priority, tags, updatedAt: now } } });
  } else {
    await api('/v1/entries', { method: 'POST', idempotencyKey, body: { idempotencyKey, entry: {
      entryId: key(), version: 0, title: d.title, body: d.body, bodyHash, language: d.language, tags, priority: d.priority, state: 'active', updatedAt: now,
      description: d.description, maximumBytes: 4096, readonly: false, source: 'manual', autoUpdateEnabled: false,
      fileRef: { owner: 'memory', objectId: key(), version: 1, sha256: bodyHash, byteSize: new TextEncoder().encode(d.body).length, mediaType: 'text/markdown' } } } });
  }
  startNew();
}, '已保存');

const archive = (e: Entry) => run(async () => {
  const idempotencyKey = key();
  await api(`/v1/entries/${e.entryId}/archive`, { method: 'POST', idempotencyKey, body: { entryId: e.entryId, expectedVersion: e.version, idempotencyKey } });
}, '已归档');
const restore = (e: Entry) => run(async () => {
  const idempotencyKey = key();
  await api(`/v1/entries/${e.entryId}/restore`, { method: 'POST', idempotencyKey, body: { entryId: e.entryId, expectedVersion: e.version, idempotencyKey } });
}, '已恢复');

onMounted(() => run(async () => {}));
</script>

<template>
  <div class="page">
    <h1>长期记忆</h1>
    <p v-if="error" class="err">{{ error }}</p>
    <p v-if="notice" class="ok">{{ notice }}</p>
    <p class="hint">记忆是你写给助手和问答的长期背景（术语、语气、禁忌等），按优先级进入上下文；改动不会自动调用模型。</p>
    <div class="row">
      <select v-model="stateFilter" style="width:auto" @change="run(async () => {})"><option value="active">使用中</option><option value="archived">已归档</option></select>
      <button @click="startNew">新建记忆</button>
    </div>
    <table>
      <thead><tr><th>标题</th><th>说明</th><th>语言</th><th>优先级</th><th>更新</th><th></th></tr></thead>
      <tbody>
        <tr v-for="e in entries" :key="e.entryId">
          <td>{{ e.title }}</td><td>{{ e.description }}</td><td>{{ e.language }}</td><td>{{ e.priority }}</td><td>{{ new Date(e.updatedAt).toLocaleString() }}</td>
          <td class="row">
            <button v-if="!e.readonly && e.state === 'active'" @click="startEdit(e)">编辑</button>
            <button v-if="e.state === 'active'" :disabled="busy" @click="archive(e)">归档</button>
            <button v-else :disabled="busy" @click="restore(e)">恢复</button>
          </td>
        </tr>
      </tbody>
    </table>
    <p v-if="!entries.length" class="hint">没有记忆。</p>
    <fieldset>
      <legend>{{ editing ? '编辑记忆' : '新建记忆' }}</legend>
      <form @submit.prevent="save">
        <label>标题</label><input type="text" v-model="draft.title" required maxlength="200">
        <label>说明</label><input type="text" v-model="draft.description">
        <label>正文（Markdown，最多 4096 字节）</label><textarea v-model="draft.body" rows="8" required></textarea>
        <div class="row"><div><label>语言</label><input type="text" v-model="draft.language" style="width:8rem"></div>
          <div><label>优先级（1–100）</label><input type="number" v-model.number="draft.priority" min="1" max="100" style="width:8rem"></div>
          <div><label>标签（逗号分隔）</label><input type="text" v-model="draft.tags"></div></div>
        <div style="margin-top:.8rem"><button class="primary" :disabled="busy">保存</button></div>
      </form>
    </fieldset>
  </div>
</template>
