<script lang="ts">
import type { ModuleAPIClient } from "../../../../../libs/platform/web/module-api";
import type { operations } from "../gen/api.gen";
type ConfigResult = operations["readIndexConfig"]["responses"][200]["content"]["application/json"];
export type IndexConfig = ConfigResult["config"];
export type IndexOptions = operations["readIndexOptions"]["responses"][200]["content"]["application/json"];
export type BuildsResult = operations["listBuilds"]["responses"][200]["content"]["application/json"];
export type LoadAdminOverviewInput = { limit: number; cursor?: string };
export type LoadAdminOverviewResult = { config?: IndexConfig; options: IndexOptions; builds: BuildsResult };
export function safeCode(error: unknown): string {
 const code = (error as { code?: unknown })?.code;
 return typeof code === "string" && /^[A-Z][A-Z0-9_]{0,63}$/.test(code) ? code : "DEPENDENCY_UNAVAILABLE";
}
export async function loadAdminOverview(client: ModuleAPIClient, input: LoadAdminOverviewInput, signal: AbortSignal): Promise<LoadAdminOverviewResult> {
 const query = new URLSearchParams({ limit: String(input.limit) });
 if (input.cursor) query.set("cursor", input.cursor);
 const config = client.request<ConfigResult>("/v1/index-config", { signal }).catch(error => { if (safeCode(error) === "NOT_FOUND") return undefined; throw error; });
 const [current, options, builds] = await Promise.all([config, client.request<IndexOptions>("/v1/index-options", { signal }), client.request<BuildsResult>(`/v1/builds?${query}`, { signal })]);
 return { config: current?.config, options, builds };
}
</script>
<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref } from "vue";
import EditView from "./EditView.vue";
const props = defineProps<{ client: ModuleAPIClient }>();
const overview = ref<LoadAdminOverviewResult>();
const loading = ref(false); const failure = ref(""); const notice = ref(""); const editing = ref(false);
const documentID = ref(""); const force = ref(false); const consent = ref(false); const busy = ref(false);
let controller: AbortController | undefined;
let disposed = false;
let pendingBuild: { serialized: string; key: string } | undefined;
const stateLabels: Record<string, string> = { queued: "已排队", fetching: "读取发布正文", extracting: "提取资源", chunking: "生成分块", embedding: "生成向量", validating: "核验索引", active: "已启用", cancelled: "已取消", failed: "失败", superseded: "已被新版本替代" };
async function refresh(cursor?: string): Promise<void> {
 controller?.abort(); const current = new AbortController(); controller = current;
 loading.value = true; failure.value = "";
 try {
  const next = await loadAdminOverview(props.client, { limit: 20, cursor }, current.signal);
  if (current.signal.aborted) return;
  if (cursor && overview.value) next.builds.builds = [...overview.value.builds.builds, ...next.builds.builds];
  overview.value = next;
 } catch (error) { if (!current.signal.aborted) failure.value = safeCode(error); }
 finally { if (!current.signal.aborted) loading.value = false; }
}
async function createBuild(): Promise<void> {
 const selected = overview.value?.options.documents.find(item => item.documentId === documentID.value);
 if (!selected || !consent.value || !overview.value?.config?.enabled || busy.value) return;
 const body = { document: selected.reference, force: force.value };
 const serialized = JSON.stringify(body);
 if (pendingBuild?.serialized !== serialized) pendingBuild = { serialized, key: crypto.randomUUID() };
 busy.value = true; failure.value = "";
 try {
  await props.client.request("/v1/builds", { method: "POST", body: { ...body, idempotencyKey: pendingBuild.key }, idempotencyKey: pendingBuild.key, signal: controller?.signal });
  if (disposed) return;
  pendingBuild = undefined; consent.value = false; notice.value = "构建请求已接收，请查看服务端状态。"; await refresh();
 } catch (error) { if (!disposed) failure.value = safeCode(error); }
 finally { if (!disposed) busy.value = false; }
}
async function cancelBuild(buildId: string): Promise<void> {
 if (busy.value) return; busy.value = true; failure.value = "";
 try {
  const key = `knowledge.cancel.${buildId}`;
  await props.client.request(`/v1/builds/${encodeURIComponent(buildId)}/cancel`, { method: "POST", body: { buildId, reason: "管理员取消索引构建", idempotencyKey: key }, idempotencyKey: key, signal: controller?.signal });
  if (disposed) return;
  notice.value = "取消请求已提交，已发生费用以服务端记录为准。"; await refresh();
 } catch (error) { if (!disposed) failure.value = safeCode(error); }
 finally { if (!disposed) busy.value = false; }
}
function afterSave(): void { editing.value = false; notice.value = "索引配置已保存。已有索引保持服务，重建需单独提交。"; void refresh(); }
onMounted(() => { void refresh(); });
onBeforeUnmount(() => { disposed = true; controller?.abort(); });
</script>
<template>
 <section class="knowledge-admin" aria-labelledby="knowledge-title">
  <header class="knowledge-header"><div><p class="knowledge-eyebrow">知识管理</p><h1 id="knowledge-title">发布内容索引</h1><p>管理检索配置与入库进度，索引只来自已发布内容。</p></div><button :disabled="loading || busy" @click="refresh()">{{ loading ? '加载中…' : '刷新状态' }}</button></header>
  <p v-if="failure" role="alert" class="knowledge-error">操作未完成：{{ failure }}。请保留当前输入，确认服务状态后重试。</p>
  <p v-if="notice" role="status" class="knowledge-notice">{{ notice }}</p>
  <template v-if="overview">
   <section class="knowledge-card"><div class="knowledge-row"><h2>索引配置</h2><button :disabled="loading || busy" @click="editing = !editing">{{ editing ? '收起编辑' : '编辑配置' }}</button></div>
    <p v-if="overview.config">{{ overview.config.enabled ? '已开启入库' : '已暂停入库' }} · 配置版本 {{ overview.config.version }} · {{ overview.config.language }}</p><p v-else>尚未配置索引，请选择可用模型并保存。</p>
    <dl v-if="overview.config"><dt>分块预算</dt><dd>{{ overview.config.chunkTokens }} tokens，重叠 {{ overview.config.overlapTokens }} tokens</dd><dt>嵌入空间</dt><dd class="knowledge-mono">{{ overview.config.space.spaceId }}</dd></dl>
    <EditView v-if="editing" :client="client" :config="overview.config" :options="overview.options" @saved="afterSave" />
   </section>
   <section class="knowledge-card"><h2>手工构建</h2>
    <p v-if="overview.options.documentsState !== 'ready'" role="status">发布文档目录暂不可用：{{ overview.options.documentErrorCode }}</p>
    <form v-else @submit.prevent="createBuild"><label>已发布文档<select v-model="documentID" required :disabled="busy"><option value="" disabled>选择文档</option><option v-for="item in overview.options.documents" :key="item.documentId" :value="item.documentId">{{ item.title }}</option></select></label>
     <label class="knowledge-check"><input v-model="force" type="checkbox" :disabled="busy">重新构建同一发布版本</label><label class="knowledge-check"><input v-model="consent" type="checkbox" :disabled="busy">我确认提交此构建，可能产生后台模型费用</label>
     <button type="submit" :disabled="busy || !consent || !documentID || !overview.config?.enabled">{{ busy ? '提交中…' : '提交构建' }}</button>
    </form>
   </section>
   <section class="knowledge-card"><h2>构建记录</h2><p v-if="overview.builds.builds.length === 0">尚无构建记录。</p>
    <div v-else class="knowledge-table-wrap"><table><thead><tr><th>文档</th><th>代数</th><th>状态</th><th>分块进度</th><th>操作</th></tr></thead><tbody><tr v-for="build in overview.builds.builds" :key="build.buildId"><td>{{ overview.options.documents.find(item => item.documentId === build.document.documentId)?.title || build.document.documentId }}</td><td>{{ build.generation }}</td><td>{{ stateLabels[build.state] || build.state }}<span v-if="build.failureCode"> · {{ build.failureCode }}</span></td><td>{{ build.completedChunks }} / {{ build.totalChunks }}</td><td><button v-if="!['active','cancelled','failed','superseded'].includes(build.state)" :disabled="busy" @click="cancelBuild(build.buildId)">取消</button></td></tr></tbody></table></div>
    <button v-if="overview.builds.nextCursor" :disabled="loading || busy" @click="refresh(overview.builds.nextCursor)">加载更多</button>
   </section>
  </template>
 </section>
</template>
