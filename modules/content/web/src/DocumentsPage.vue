<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { api, messageOf } from './client';
import Editor from './Editor.vue';
import { fromBody, newId, toBody, type JSONNode } from './convert';

type Meta = {
  documentId: string; title: string; parentId: string | null; orderKey: string; state: string;
  pathSegment: string | null; seoDescription: string | null; revision: number;
};

const docs = ref<Meta[]>([]);
const selected = ref<string | null>(null);
const meta = ref<Meta | null>(null);
const draft = ref<any>(null);
const lease = ref<any>(null);
const writable = ref(false);
const initialDoc = ref<JSONNode>({ type: 'doc', content: [] });
const editorKey = ref(0);
const title = ref('');
const slug = ref('');
const seo = ref('');
const status = ref('');
const error = ref('');
const busy = ref(false);
const dirty = ref(false);
const tabId = newId();
let pending: JSONNode | null = null;
let saveTimer: number | undefined;
let heartbeatTimer: number | undefined;
let heartbeat = 0;
let inputs = 0;
let lastInput = new Date().toISOString();

const rows = computed(() => {
  const children = new Map<string | null, Meta[]>();
  for (const d of docs.value) {
    if (d.state === 'trash' || d.state === 'purged') continue;
    const key = d.parentId ?? null;
    children.set(key, [...(children.get(key) ?? []), d]);
  }
  const out: { doc: Meta; depth: number }[] = [];
  const walk = (parent: string | null, depth: number) => {
    for (const d of (children.get(parent) ?? []).sort((a, b) => (a.orderKey < b.orderKey ? -1 : 1))) {
      out.push({ doc: d, depth });
      walk(d.documentId, depth + 1);
    }
  };
  walk(null, 0);
  return out;
});

async function run<T>(action: () => Promise<T>): Promise<T | undefined> {
  busy.value = true;
  error.value = '';
  try { return await action(); } catch (e) { error.value = messageOf(e); } finally { busy.value = false; }
}

async function loadTree() {
  const tree = await api<{ documents: Meta[] }>('/v1/tree');
  docs.value = tree.documents;
}

async function releaseLock() {
  window.clearInterval(heartbeatTimer);
  window.clearTimeout(saveTimer);
  const held = lease.value;
  const id = selected.value;
  lease.value = null;
  if (held && id && writable.value) {
    try { await api(`/v1/documents/${id}/lock`, { method: 'DELETE', body: { leaseId: held.leaseId, serverBootId: held.serverBootId } }); } catch { /* 锁会自行过期 */ }
  }
}

async function select(id: string) {
  if (id === selected.value) return;
  if (dirty.value) await save();
  await releaseLock();
  selected.value = id;
  await run(async () => {
    const read = await api<any>(`/v1/documents/${id}/draft`);
    applyRead(read);
    const acquired = await api<any>(`/v1/documents/${id}/lock`, { method: 'POST', body: { tabId } });
    lease.value = acquired.lease;
    writable.value = acquired.writable;
    heartbeat = acquired.lease.heartbeatSequence;
    inputs = acquired.lease.inputSequence;
    status.value = acquired.writable ? '' : `只读：${acquired.lease.holderAccountName} 正在编辑`;
    heartbeatTimer = window.setInterval(renew, 30000);
  });
}

function applyRead(read: any) {
  meta.value = read.metadata;
  draft.value = read.draft;
  title.value = read.metadata.title;
  slug.value = read.metadata.pathSegment ?? slug.value ?? '';
  seo.value = read.metadata.seoDescription ?? '';
  initialDoc.value = fromBody(read.body);
  editorKey.value++;
  dirty.value = false;
  pending = null;
}

async function renew() {
  if (!lease.value || !writable.value || !selected.value) return;
  heartbeat++;
  try {
    const result = await api<any>(`/v1/documents/${selected.value}/lock`, { method: 'PUT', body: {
      leaseId: lease.value.leaseId, fencingToken: lease.value.fencingToken, serverBootId: lease.value.serverBootId,
      lastInputAt: lastInput, heartbeatSequence: heartbeat, inputSequence: inputs,
    } });
    lease.value = result.lease;
  } catch (e) { status.value = `编辑锁续期失败：${messageOf(e)}`; }
}

function onChange(doc: JSONNode) {
  pending = doc;
  dirty.value = true;
  inputs++;
  lastInput = new Date().toISOString();
  status.value = '有未保存的修改…';
  window.clearTimeout(saveTimer);
  saveTimer = window.setTimeout(save, 1500);
}

async function save(): Promise<boolean> {
  window.clearTimeout(saveTimer);
  if (!dirty.value || !pending || !selected.value || !lease.value || !writable.value) return true;
  const body = toBody(pending);
  const sent = pending;
  status.value = '保存中…';
  try {
    const result = await api<{ draft: any }>(`/v1/documents/${selected.value}/draft`, { method: 'PUT', body: {
      body, expectedDraftVersion: draft.value.draftVersion, expectedDraftRevisionId: draft.value.revisionId,
      leaseId: lease.value.leaseId, fencingToken: lease.value.fencingToken, serverBootId: lease.value.serverBootId,
    } });
    draft.value = result.draft;
    if (pending === sent) dirty.value = false;
    status.value = `已保存 ${new Date().toLocaleTimeString()}`;
    return true;
  } catch (e) {
    error.value = messageOf(e);
    status.value = '保存失败';
    return false;
  }
}

async function refreshMeta() {
  const read = await api<any>(`/v1/documents/${selected.value}/draft`);
  meta.value = read.metadata;
  draft.value = read.draft;
  await loadTree();
}

async function create(parentId: string | null) {
  const name = window.prompt('新文档的标题');
  if (!name) return;
  const created = await run(() => api<any>('/v1/documents', { method: 'POST', body: { title: name, parentId, beforeDocumentId: null } }));
  if (created) {
    await loadTree();
    await select(created.metadata.documentId);
  }
}

async function saveTitle() {
  if (!meta.value || title.value === meta.value.title || !title.value.trim()) return;
  await run(async () => {
    await api(`/v1/documents/${meta.value!.documentId}/metadata`, { method: 'PUT', body: {
      title: title.value, pathSegment: meta.value!.pathSegment, seoDescription: meta.value!.seoDescription, shareImage: null, expectedRevision: meta.value!.revision,
    } });
    await refreshMeta();
  });
}

async function publish() {
  if (!meta.value || !(await save())) return;
  await run(async () => {
    await refreshMeta();
    const path = meta.value!.pathSegment ?? (slug.value.trim() || `page-${newId().slice(0, 6)}`);
    const result = await api<any>(`/v1/documents/${meta.value!.documentId}:publish`, { method: 'POST', body: {
      draftRevisionId: draft.value.revisionId, expectedDraftVersion: draft.value.draftVersion, expectedRevision: meta.value!.revision,
      manualPathSegment: meta.value!.pathSegment ? null : path, manualSEO: seo.value || null,
    } });
    await refreshMeta();
    status.value = result.warnings?.length ? `已发布（提示：${result.warnings.join('；')}）` : '已发布';
  });
}

async function withdraw() {
  if (!meta.value || !window.confirm('撤回后公开页将不可访问，确认撤回？')) return;
  await run(async () => {
    await api(`/v1/documents/${meta.value!.documentId}:withdraw`, { method: 'POST', body: { expectedRevision: meta.value!.revision } });
    await refreshMeta();
    status.value = '已撤回';
  });
}

onMounted(() => run(loadTree));
onBeforeUnmount(() => { void save().then(releaseLock); });
</script>

<template>
  <div class="docs">
    <aside>
      <h2>文档 <button @click="create(selected)">新建</button></h2>
      <p v-if="selected" class="hint">新建的文档会放在当前文档下；<a href="#" @click.prevent="create(null)">改为新建顶层文档</a></p>
      <div class="tree">
        <ul>
          <li v-for="r in rows" :key="r.doc.documentId" :class="{ sel: r.doc.documentId === selected }" :style="{ paddingLeft: r.depth * 14 + 'px' }">
            <div @click="select(r.doc.documentId)">
              <span style="flex:1">{{ r.doc.title }}</span>
              <span class="badge" :class="{ pub: r.doc.state === 'published' }">{{ r.doc.state === 'published' ? '已发布' : '草稿' }}</span>
            </div>
          </li>
        </ul>
        <p v-if="!rows.length" class="hint">还没有文档，点“新建”开始。</p>
      </div>
    </aside>
    <section>
      <p v-if="error" class="error">{{ error }}</p>
      <div v-if="!meta" class="empty">从左侧选择一篇文档，或新建一篇。</div>
      <template v-else>
        <div class="editor-head">
          <input class="title" v-model="title" :disabled="!writable" @blur="saveTitle" aria-label="文档标题">
          <span class="status">{{ status }}</span>
          <button :disabled="!dirty || !writable" @click="save">保存草稿</button>
          <button class="primary" :disabled="busy || !writable" @click="publish">{{ meta.state === 'published' ? '发布更新' : '发布' }}</button>
          <button v-if="meta.state === 'published'" class="danger" :disabled="busy" @click="withdraw">撤回</button>
        </div>
        <div v-if="!meta.pathSegment" class="publish-box">
          <label style="margin:0">页面路径</label>
          <input v-model="slug" placeholder="小写字母、数字和连字符，如 getting-started" pattern="[a-z0-9]+(-[a-z0-9]+)*">
          <label style="margin:0">搜索摘要</label>
          <input v-model="seo" placeholder="可留空">
        </div>
        <p v-else class="hint">公开路径：/{{ meta.pathSegment }}</p>
        <Editor :key="editorKey" :doc="initialDoc" :editable="writable" @change="onChange" />
      </template>
    </section>
  </div>
</template>
