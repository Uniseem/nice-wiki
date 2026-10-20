<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue';
import { api, messageOf } from './client';
import Editor from './Editor.vue';
import { fromBody, newId, toBody, type JSONNode } from './convert';

type Meta = {
  documentId: string; title: string; parentId: string | null; orderKey: string; state: string;
  pathSegment: string | null; seoDescription: string | null; revision: number;
};

const docs = ref<Meta[]>([]);
const treeRevision = ref(0);
const view = ref<'docs' | 'trash'>('docs');
const trash = ref<Meta[]>([]);
const history = ref<any[] | null>(null);
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
  const tree = await api<{ documents: Meta[]; treeRevision: number }>('/v1/tree');
  docs.value = tree.documents;
  treeRevision.value = tree.treeRevision;
}

async function loadTrash() {
  const result = await api<{ documents: Meta[] }>('/v1/documents?state=trash&limit=100');
  trash.value = result.documents;
  await loadTree();
}

async function showTrash() {
  view.value = 'trash';
  await run(loadTrash);
}

async function reorder(doc: Meta, delta: number) {
  const siblings = docs.value.filter((d) => (d.parentId ?? null) === (doc.parentId ?? null) && d.state !== 'trash' && d.state !== 'purged').sort((a, b) => (a.orderKey < b.orderKey ? -1 : 1));
  const index = siblings.findIndex((d) => d.documentId === doc.documentId);
  const target = index + delta;
  if (target < 0 || target >= siblings.length) return;
  const ids = siblings.map((d) => d.documentId);
  [ids[index], ids[target]] = [ids[target], ids[index]];
  await run(async () => {
    await api('/v1/tree:reorder', { method: 'POST', body: { parentId: doc.parentId ?? null, orderedDocumentIds: ids, expectedTreeRevision: treeRevision.value } });
    await loadTree();
  });
}

// 可选的新父节点：除自己和自己的后代之外的所有活动文档。
const moveTargets = computed(() => {
  if (!meta.value) return [];
  const banned = new Set<string>([meta.value.documentId]);
  let grew = true;
  while (grew) {
    grew = false;
    for (const d of docs.value) if (d.parentId && banned.has(d.parentId) && !banned.has(d.documentId)) { banned.add(d.documentId); grew = true; }
  }
  return rows.value.filter((r) => !banned.has(r.doc.documentId));
});

async function moveTo(event: Event) {
  const value = (event.target as HTMLSelectElement).value;
  if (!meta.value || value === '__keep') return;
  await save();
  const id = meta.value.documentId;
  await run(async () => {
    await api('/v1/tree:move', { method: 'POST', body: { documentId: id, newParentId: value === '__root' ? null : value, beforeDocumentId: null, expectedTreeRevision: treeRevision.value } });
    await refreshMeta();
  });
  (event.target as HTMLSelectElement).value = '__keep';
}

async function remove() {
  if (!meta.value) return;
  const hasChildren = docs.value.some((d) => d.parentId === meta.value!.documentId && d.state !== 'trash');
  const text = hasChildren ? '该文档含子文档，将一起移入回收站，确认删除？' : '确认把该文档移入回收站？已发布的文档公开页会立即失效。';
  if (!window.confirm(text)) return;
  const id = meta.value.documentId;
  const revision = meta.value.revision;
  await save();
  await releaseLock();
  await run(async () => {
    await api(`/v1/documents/${id}:trash`, { method: 'POST', body: { expectedRevision: revision, expectedTreeRevision: treeRevision.value, includeSubtree: hasChildren, replacementDocumentId: null } });
    selected.value = null;
    meta.value = null;
    await loadTree();
  });
}

async function restore(doc: Meta) {
  const path = window.prompt('恢复后使用的页面路径（小写字母、数字和连字符）', doc.pathSegment ?? `restored-${newId().slice(0, 6)}`);
  if (!path) return;
  await run(async () => {
    await api(`/v1/documents/${doc.documentId}:restore`, { method: 'POST', body: { expectedRevision: doc.revision, parentId: null, pathSegment: path } });
    await loadTrash();
  });
}

async function purge(doc: Meta) {
  if (!window.confirm(`彻底删除“${doc.title}”后无法恢复，确认？（仅超级管理员）`)) return;
  const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(doc.documentId));
  const hash = Array.from(new Uint8Array(digest), (b) => b.toString(16).padStart(2, '0')).join('');
  await run(async () => {
    await api('/v1/trash:purge', { method: 'POST', body: { documentIds: [doc.documentId], expectedTrashRevision: treeRevision.value, confirmationHash: hash } });
    await loadTrash();
  });
}

async function rollback(entry: any) {
  if (!meta.value || !window.confirm('把正文回滚到这个版本并作为新的发布？当前草稿会被覆盖。')) return;
  const id = meta.value.documentId;
  await releaseLock();
  await run(async () => {
    await api(`/v1/documents/${id}/history:rollback`, { method: 'POST', body: { historyId: entry.historyId, expectedRevision: meta.value!.revision, acknowledgeMissingAssets: false } });
    const keep = selected.value;
    selected.value = null;
    await loadTree();
    if (keep) await select(keep);
    history.value = null;
    status.value = '已回滚并重新发布';
  });
}

async function toggleHistory() {
  if (history.value) { history.value = null; return; }
  await run(async () => {
    const result = await api<{ history: any[] }>(`/v1/documents/${selected.value}/history?limit=20`);
    history.value = result.history;
  });
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

// 新建文档：在左栏里就地填标题（不用浏览器弹窗）。
const creating = ref<{ parentId: string | null } | null>(null);
const newTitle = ref('');
const titleInput = ref<HTMLInputElement | null>(null);
function create(parentId: string | null) { creating.value = { parentId }; newTitle.value = ''; void nextTick(() => titleInput.value?.focus()); }
async function submitCreate() {
  const name = newTitle.value.trim();
  const parentId = creating.value?.parentId ?? null;
  creating.value = null;
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

async function preview() {
  if (!meta.value || !(await save())) return;
  await run(async () => {
    const grant = await api<{ previewId: string; token: string }>('/v1/preview/tokens', { method: 'POST', body: { documentId: meta.value!.documentId, draftRevisionId: draft.value.revisionId } });
    window.open(`/v1/preview/${encodeURIComponent(grant.token)}?previewId=${encodeURIComponent(grant.previewId)}`, '_blank', 'noopener');
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
      <p style="margin:.2rem 0 .6rem">
        <a href="#" @click.prevent="view = 'docs'" :style="{ fontWeight: view === 'docs' ? 700 : 400 }">全部文档</a> ·
        <a href="#" @click.prevent="showTrash" :style="{ fontWeight: view === 'trash' ? 700 : 400 }">回收站</a>
      </p>
      <form v-if="creating" class="row" @submit.prevent="submitCreate" style="margin-bottom:.6rem">
        <input ref="titleInput" type="text" v-model="newTitle" placeholder="新文档的标题" required style="flex:1;min-width:0">
        <button class="primary">创建</button>
        <button type="button" @click="creating = null">取消</button>
      </form>
      <p v-if="selected" class="hint">新建的文档会放在当前文档下；<a href="#" @click.prevent="create(null)">改为新建顶层文档</a></p>
      <div v-if="view === 'trash'" class="tree">
        <ul>
          <li v-for="d in trash" :key="d.documentId">
            <div style="flex-wrap:wrap"><span style="flex:1">{{ d.title }}</span>
              <button @click="restore(d)">恢复</button> <button class="danger" @click="purge(d)">彻底删除</button></div>
          </li>
        </ul>
        <p v-if="!trash.length" class="hint">回收站是空的。</p>
      </div>
      <div v-else class="tree">
        <ul>
          <li v-for="r in rows" :key="r.doc.documentId" :class="{ sel: r.doc.documentId === selected }" :style="{ paddingLeft: r.depth * 14 + 'px' }">
            <div @click="select(r.doc.documentId)">
              <span style="flex:1">{{ r.doc.title }}</span>
              <span class="move"><button title="上移" @click.stop="reorder(r.doc, -1)">↑</button><button title="下移" @click.stop="reorder(r.doc, 1)">↓</button></span>
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
          <select :disabled="busy || !writable" style="width:auto" aria-label="移动到" @change="moveTo">
            <option value="__keep">移动到…</option><option value="__root">顶层</option>
            <option v-for="r in moveTargets" :key="r.doc.documentId" :value="r.doc.documentId">{{ '　'.repeat(r.depth) + r.doc.title }}</option>
          </select>
          <button :disabled="busy" @click="preview">预览</button>
          <button :disabled="busy" @click="toggleHistory">发布历史</button>
          <button class="danger" :disabled="busy" @click="remove">删除</button>
        </div>
        <div v-if="!meta.pathSegment" class="publish-box">
          <label style="margin:0">页面路径</label>
          <input v-model="slug" placeholder="小写字母、数字和连字符，如 getting-started" pattern="[a-z0-9]+(-[a-z0-9]+)*">
          <label style="margin:0">搜索摘要</label>
          <input v-model="seo" placeholder="可留空">
        </div>
        <p v-else class="hint">公开路径：/{{ meta.pathSegment }}</p>
        <ul v-if="history" class="history">
          <li v-for="h in history" :key="h.historyId">{{ new Date(h.createdAt).toLocaleString() }} · {{ h.authorAccountName }} · {{ ({ publish: '发布', withdraw: '撤回', rollback: '回滚' } as any)[h.operation] ?? h.operation }}
            <button v-if="h.operation !== 'withdraw'" :disabled="busy || !writable" @click="rollback(h)">回滚到此版本</button></li>
          <li v-if="!history.length" class="hint">还没有发布记录。</li>
        </ul>
        <Editor :key="editorKey" :doc="initialDoc" :editable="writable" @change="onChange" />
      </template>
    </section>
  </div>
</template>
