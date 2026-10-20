<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { errorText, request } from './api';

const tiles = ref<{ label: string; value: string; hint?: string }[]>([]);
const error = ref('');

onMounted(async () => {
  try {
    const tree = await request<{ documents: { state: string }[] }>('/v1/tree');
    const live = tree.documents.filter((d) => d.state === 'draft' || d.state === 'published');
    const published = live.filter((d) => d.state === 'published').length;
    tiles.value.push({ label: '文档总数', value: String(live.length), hint: `${published} 篇已发布，${live.length - published} 篇草稿` });
    const trash = tree.documents.filter((d) => d.state === 'trash').length;
    tiles.value.push({ label: '回收站', value: String(trash) });
  } catch (e) { error.value = errorText(e); }
  try {
    const now = new Date();
    const to = new Date(now.getTime() + 3600_000);
    const from = new Date(now.getTime() - 7 * 86400_000);
    const iso = (d: Date) => d.toISOString().slice(0, 13) + ':00:00Z';
    const stats = await request<{ rows: { visits?: number; pageViews?: number }[] }>(`/v1/statistics/pages?from=${iso(from)}&to=${iso(to)}`);
    const total = stats.rows.reduce((sum, r) => sum + (r.visits ?? r.pageViews ?? 0), 0);
    tiles.value.push({ label: '近 7 天访问', value: String(total), hint: '按小时去重的访次之和' });
  } catch { /* 没有统计权限或统计服务暂不可用时不显示 */ }
  try {
    const languages = await request<{ languages: { enabled: boolean }[] }>('/v1/languages?includeDisabled=true');
    tiles.value.push({ label: '启用的语言', value: String(languages.languages.filter((l) => l.enabled).length) });
  } catch { /* 同上 */ }
});
</script>

<template>
  <div class="dash">
    <h1 style="margin-top:0;font-size:1.3rem">概览</h1>
    <p v-if="error" class="error">{{ error }}</p>
    <div class="tiles">
      <div v-for="t in tiles" :key="t.label" class="tile"><div class="hint">{{ t.label }}</div><div class="num">{{ t.value }}</div><div v-if="t.hint" class="hint">{{ t.hint }}</div></div>
    </div>
    <p class="hint" style="margin-top:2rem">从左侧进入文档编辑、站点设置等页面。</p>
  </div>
</template>

<style scoped>
.tiles { display: grid; grid-template-columns: repeat(auto-fill, minmax(12rem, 1fr)); gap: 1rem; }
.tile { border: 1px solid var(--bd); border-radius: 10px; padding: 1rem; }
.num { font-size: 2rem; font-weight: 700; }
</style>
