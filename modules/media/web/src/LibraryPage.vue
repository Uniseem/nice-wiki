<script setup lang="ts">
import { onMounted, ref, watch } from 'vue';
import { api, messageOf } from './client';

type Asset = { ref: { assetId: string; version: number; sha256: string; mediaType: string; byteSize: number }; kind: string; originalName: string; createdAt: string; state: string };

const tab = ref<'all' | 'unused'>('all');
const kind = ref('');
const assets = ref<Asset[]>([]);
const historyRisk = ref<string[]>([]);
const cursor = ref<string | null>(null);
const error = ref('');
const notice = ref('');
const busy = ref(false);
const references = ref<{ asset: Asset; snapshot: any } | null>(null);
const selected = ref<Set<string>>(new Set());
const plan = ref<any>(null);

const kinds: Record<string, string> = { image: '图片', attachment: '附件', video: '视频', drawing_scene: '绘图', drawing_svg: '绘图导出', platform_cover: '平台封面' };

async function run(action: () => Promise<void>) {
  busy.value = true; error.value = ''; notice.value = '';
  try { await action(); } catch (e) { error.value = messageOf(e); } finally { busy.value = false; }
}

async function load(more = false) {
  const query = new URLSearchParams({ limit: '30' });
  if (kind.value) query.set('kind', kind.value);
  if (more && cursor.value) query.set('cursor', cursor.value);
  const result = await api<any>(`/v1/assets${tab.value === 'unused' ? '/unused' : ''}?${query}`);
  assets.value = more ? [...assets.value, ...result.assets] : result.assets;
  historyRisk.value = result.historyAffectedDocumentIds ?? [];
  cursor.value = result.pageInfo?.nextCursor ?? null;
}

const reload = () => run(async () => { selected.value = new Set(); plan.value = null; await load(); });

async function showReferences(asset: Asset) {
  await run(async () => {
    const result = await api<{ snapshot: any }>(`/v1/assets/${asset.ref.assetId}/references`);
    references.value = { asset, snapshot: result.snapshot };
  });
}

function toggle(id: string) {
  const next = new Set(selected.value);
  if (next.has(id)) next.delete(id); else next.add(id);
  selected.value = next;
}

const propose = () => run(async () => {
  const chosen = assets.value.filter((a) => selected.value.has(a.ref.assetId)).map((a) => a.ref);
  const result = await api<{ plan: any }>('/v1/cleanup-plans', { method: 'POST', body: { assets: chosen } });
  plan.value = result.plan;
});

const confirmPlan = () => run(async () => {
  if (!window.confirm('确认删除这些资源？删除会在 24 小时的迟到引用保护窗口之后执行。')) return;
  const risky = plan.value.historyAffectedDocumentIds?.length > 0;
  if (risky && !window.confirm('部分历史版本会因此缺少文件，确认接受？')) return;
  await api(`/v1/cleanup-plans/${plan.value.planId}:confirm`, { method: 'POST', body: { planId: plan.value.planId, confirmationHash: plan.value.confirmationHash, acknowledgeHistoryLoss: risky, expectedRevision: plan.value.revision } });
  notice.value = '已确认，将在保护窗口结束后删除。';
  plan.value = null;
  await load();
});

const cancelPlan = () => run(async () => {
  await api(`/v1/cleanup-plans/${plan.value.planId}`, { method: 'DELETE', body: {} });
  plan.value = null;
});

const size = (n: number) => (n > 1 << 20 ? (n / (1 << 20)).toFixed(1) + ' MiB' : (n / 1024).toFixed(1) + ' KiB');
watch([tab, kind], reload);
onMounted(reload);
</script>

<template>
  <div class="page">
    <h1>资源库</h1>
    <p v-if="error" class="err">{{ error }}</p>
    <p v-if="notice" class="ok">{{ notice }}</p>
    <div class="row">
      <button :class="{ primary: tab === 'all' }" @click="tab = 'all'">全部资源</button>
      <button :class="{ primary: tab === 'unused' }" @click="tab = 'unused'">未被引用</button>
      <select v-model="kind" style="width:auto"><option value="">全部类别</option><option v-for="(label, k) in kinds" :key="k" :value="k">{{ label }}</option></select>
    </div>
    <p v-if="tab === 'unused'" class="hint">这里只列当前没有任何文档或站点设置引用的资源。选中后可生成清理计划；历史版本里仍引用的资源会被标出风险。</p>
    <table>
      <thead><tr><th v-if="tab === 'unused'"></th><th>预览</th><th>名称</th><th>类别</th><th>大小</th><th>上传时间</th><th></th></tr></thead>
      <tbody>
        <tr v-for="a in assets" :key="a.ref.assetId">
          <td v-if="tab === 'unused'"><input type="checkbox" :checked="selected.has(a.ref.assetId)" @change="toggle(a.ref.assetId)" style="width:auto"></td>
          <td><img v-if="a.ref.mediaType.startsWith('image/')" :src="`/v1/assets/${a.ref.assetId}/bytes`" alt="" style="max-width:64px;max-height:48px"></td>
          <td>{{ a.originalName }}</td><td>{{ kinds[a.kind] ?? a.kind }}</td><td>{{ size(a.ref.byteSize) }}</td>
          <td>{{ new Date(a.createdAt).toLocaleString() }}</td>
          <td><button @click="showReferences(a)">引用</button></td>
        </tr>
      </tbody>
    </table>
    <p v-if="!assets.length" class="hint">没有资源。</p>
    <button v-if="cursor" :disabled="busy" @click="run(() => load(true))">加载更多</button>

    <fieldset v-if="tab === 'unused' && assets.length">
      <legend>清理</legend>
      <button :disabled="busy || !selected.size" @click="propose">为选中的 {{ selected.size }} 项生成清理计划</button>
      <div v-if="plan" style="margin-top:.8rem">
        <p>计划 {{ plan.planId }}：共 {{ plan.assets.length }} 个资源<span v-if="plan.historyAffectedDocumentIds.length">，历史版本涉及 {{ plan.historyAffectedDocumentIds.length }} 篇文档</span>。</p>
        <div class="row"><button class="danger" :disabled="busy" @click="confirmPlan">确认清理</button><button :disabled="busy" @click="cancelPlan">放弃计划</button></div>
      </div>
    </fieldset>

    <fieldset v-if="references">
      <legend>“{{ references.asset.originalName }}”的引用</legend>
      <p>当前引用 {{ references.snapshot.currentReferences.length }} 处，历史引用 {{ references.snapshot.historyReferences.length }} 处。</p>
      <ul>
        <li v-for="r in references.snapshot.currentReferences" :key="r.sourceRevisionId + r.documentId">文档 {{ r.documentId ?? r.sourceObjectId }}（{{ r.sourceService }}）</li>
      </ul>
      <button @click="references = null">关闭</button>
    </fieldset>
  </div>
</template>
