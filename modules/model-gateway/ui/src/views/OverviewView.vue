<script lang="ts">
import type { ModuleAPIClient, ModuleUIBootstrap } from "../../../../../libs/platform/web/module-api";
import type { components } from "../gen/api.gen";

export type AdminCatalog = components["schemas"]["ReadAdminCatalogResult"];
export async function loadAdminOverview(apiClient: ModuleAPIClient, signal: AbortSignal): Promise<AdminCatalog> {
  signal.throwIfAborted();
  return apiClient.request<AdminCatalog>("/v1/admin/catalog", { signal });
}
</script>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from "vue";
import EditView from "./EditView.vue";

type Model = components["schemas"]["Model"];
type Profile = components["schemas"]["Profile"];
type Provider = components["schemas"]["ProviderSummary"];
type Assignment = components["schemas"]["TaskAssignment"];
type Money = components["schemas"]["Money"];
type FeePolicy = components["schemas"]["FeeBudgetPolicy"];
type Ledger = components["schemas"]["GetLedgerResult"];
const props = defineProps<{ bootstrap: ModuleUIBootstrap; parentSignal: AbortSignal }>();
const catalog = ref<AdminCatalog>();
const error = ref("");
const message = ref("");
const busy = ref(false);
const denied = ref(false);
const controller = new AbortController();
let navigation: AbortController | undefined;
const selectedProvider = ref<Provider>();
const credentials = reactive({ provider: "", secret: "" });
const model = reactive({ id: "", provider: "", version: 0, providerVersion: "", capabilities: ["chat"], dimension: 0, preprocessing: "none", maxInput: 0, maxOutput: 0, enabled: true, manualOverride: true, manualPrice: false, inputPrice: "", outputPrice: "", fixedPrice: "" });
const profile = reactive({ id: "", version: 0, costClass: "qa", tier: "fast", models: [] as string[], maximum: "0", daily: "0", concurrency: 1 });
const task = reactive({ id: "", version: 0, tier: "fast", override: "" });
const policy = reactive({ version: 0, hourly: false, session: "0", multiplier: 5, daily: "0" });
const policyConfigured = ref(false);
const pending = new Map<string, { hash: string; key: string }>();
const ledger = ref<Ledger>();
const ledgerFilter = reactive({ from: new Date(Date.now() - 86400000).toISOString().slice(0, 16), until: new Date().toISOString().slice(0, 16), costClass: "", cursor: "" });
const superadmin = computed(() => !denied.value && props.bootstrap.capabilities.includes("superadmin"));
const canRead = computed(() => !denied.value && (superadmin.value || props.bootstrap.capabilities.includes("model.manage")));
const modelsForProvider = computed(() => (catalog.value?.models ?? []).filter(value => value.providerId === credentials.provider));
const probeModel = ref("");
const probe = ref<components["schemas"]["Invocation"]>();
let probeTimer: ReturnType<typeof setTimeout> | undefined;

function clearSensitive(): void {
  denied.value = true; catalog.value = undefined; ledger.value = undefined; probe.value = undefined;
  credentials.secret = ""; selectedProvider.value = undefined; pending.clear(); navigation?.abort();
  if (probeTimer) clearTimeout(probeTimer);
}
function failure(problem: unknown): void {
  const value = problem as { code?: string; message?: string };
  if (value.code === "CANCELLED" || (problem instanceof DOMException && problem.name === "AbortError")) return;
  if (value.code === "UNAUTHENTICATED" || value.code === "FORBIDDEN") clearSensitive();
  error.value = ["VERSION_MISMATCH", "CONFLICT"].includes(value.code ?? "") ? "配置版本已变化，输入已保留。请重新读取当前配置后再提交，不会自动覆盖。" : value.message ?? "请求未完成，请查看服务状态后重试。";
}
function micros(value: string): Money {
  if (!/^(0|[1-9]\d*)(\.\d{1,6})?$/.test(value)) throw new Error("美元金额须非负，最多六位小数");
  const [whole, fraction = ""] = value.split(".");
  const amount = BigInt(whole) * 1000000n + BigInt(fraction.padEnd(6, "0"));
  if (amount > 9007199254740991n) throw new Error("金额超过可保存范围");
  return { currency: "USD", micros: Number(amount) };
}
function dollars(value?: Money): string {
  if (!value) return "费用未知";
  const integer = BigInt(value.micros);
  return `$${integer / 1000000n}.${String(integer % 1000000n).padStart(6, "0")}`;
}
function decimal(value: Money): string { return dollars(value).slice(1); }
async function write<T>(path: string, method: "PUT" | "POST", body: Record<string, unknown>): Promise<T> {
  controller.signal.throwIfAborted();
  const hash = JSON.stringify(body);
  let command = pending.get(path);
  if (!command || command.hash !== hash) { command = { hash, key: crypto.randomUUID() }; pending.set(path, command); }
  const result = await props.bootstrap.apiClient.request<T>(path, { method, body: { ...body, idempotencyKey: command.key }, idempotencyKey: command.key, signal: controller.signal });
  pending.delete(path); return result;
}
async function perform(work: () => Promise<void>): Promise<void> {
  if (busy.value) return;
  busy.value = true; error.value = ""; message.value = "";
  try { await work(); } catch (problem) { failure(problem); } finally { busy.value = false; }
}
async function reload(): Promise<void> {
  navigation?.abort(); navigation = new AbortController();
  const stop = () => navigation?.abort(); controller.signal.addEventListener("abort", stop, { once: true });
  try {
    const next = await loadAdminOverview(props.bootstrap.apiClient, navigation.signal);
    navigation.signal.throwIfAborted(); catalog.value = next;
    try {
      const result = await props.bootstrap.apiClient.request<components["schemas"]["ReadFeeBudgetPolicyResult"]>("/v1/budget-policy", { signal: navigation.signal });
      navigation.signal.throwIfAborted(); const current = result.policy;
      Object.assign(policy, { version: current.version, hourly: current.hourlyLimitsEnabled, session: decimal(current.sessionHourlyLimit), multiplier: current.networkMultiplier, daily: decimal(current.dailyQaLimit) }); policyConfigured.value = true;
    } catch (problem) { if ((problem as { code?: string }).code === "NOT_FOUND") policyConfigured.value = false; else throw problem; }
  } finally { controller.signal.removeEventListener("abort", stop); }
}
function editModel(value?: Model): void {
  Object.assign(model, value ? { id: value.modelId, provider: value.providerId, version: value.version, providerVersion: value.providerModelVersion, capabilities: [...value.capabilities], dimension: value.dimension, preprocessing: value.preprocessorVersion, maxInput: value.maxInputTokens, maxOutput: value.maxOutputTokens, enabled: value.enabled, manualOverride: value.manualOverride, manualPrice: false, inputPrice: "", outputPrice: "", fixedPrice: "" } : { id: "", provider: "", version: 0, providerVersion: "", capabilities: ["chat"], dimension: 0, preprocessing: "none", maxInput: 0, maxOutput: 0, enabled: true, manualOverride: true, manualPrice: false, inputPrice: "", outputPrice: "", fixedPrice: "" });
}
async function saveModel(): Promise<void> {
  await perform(async () => {
    const value: Model = { modelId: model.id, version: model.version + 1, providerId: model.provider, providerModelVersion: model.providerVersion, capabilities: [...model.capabilities], dimension: model.dimension, preprocessorVersion: model.preprocessing, maxInputTokens: model.maxInput, maxOutputTokens: model.maxOutput, catalogOrigin: "manual", manualOverride: model.manualOverride, enabled: model.enabled };
    const body: Record<string, unknown> = { model: value, expectedVersion: model.version };
    if (model.manualPrice) body.manualPrice = { ...(model.inputPrice ? { inputPerMillionCost: micros(model.inputPrice) } : {}), ...(model.outputPrice ? { outputPerMillionCost: micros(model.outputPrice) } : {}), ...(model.fixedPrice ? { fixedCost: micros(model.fixedPrice) } : {}) };
    await write(`/v1/admin/models/${encodeURIComponent(model.id)}`, "PUT", body); model.version++; message.value = "模型已保存；空间身份变化须使用新模型标识。"; await reload();
  });
}
function editProfile(value?: Profile): void {
  Object.assign(profile, value ? { id: value.profileId, version: value.version, costClass: value.costClass, tier: value.tier, models: [...value.orderedModelIds], maximum: decimal(value.maximumCost), daily: decimal(value.dailyLimitCost), concurrency: value.concurrency } : { id: "", version: 0, costClass: "qa", tier: "fast", models: [], maximum: "0", daily: "0", concurrency: 1 });
}
function moveModel(index: number, direction: number): void {
  const target = index + direction; if (target < 0 || target >= profile.models.length) return;
  [profile.models[index], profile.models[target]] = [profile.models[target]!, profile.models[index]!];
}
async function saveProfile(): Promise<void> {
  await perform(async () => {
    const value: Profile = { profileId: profile.id, version: profile.version + 1, costClass: profile.costClass as Profile["costClass"], tier: profile.tier as Profile["tier"], orderedModelIds: [...profile.models], maximumCost: micros(profile.maximum), dailyLimitCost: micros(profile.daily), concurrency: profile.concurrency };
    await write(`/v1/admin/profiles/${encodeURIComponent(profile.id)}`, "PUT", { profile: value, expectedVersion: profile.version }); profile.version++; await reload(); message.value = "档位已保存。";
  });
}
function editTask(value: Assignment): void { Object.assign(task, { id: value.taskId, version: value.version, tier: value.tier, override: value.overrideModelId ?? "" }); }
async function saveTask(): Promise<void> {
  await perform(async () => { const assignment: Assignment = { taskId: task.id, version: task.version + 1, tier: task.tier as Assignment["tier"], ...(task.override ? { overrideModelId: task.override } : {}) }; await write(`/v1/task-assignments/${encodeURIComponent(task.id)}`, "PUT", { assignment, expectedVersion: task.version }); task.version++; await reload(); message.value = "任务分配已保存。"; });
}
async function savePolicy(): Promise<void> {
  await perform(async () => { const value: FeePolicy = { version: policy.version + 1, hourlyLimitsEnabled: policy.hourly, sessionHourlyLimit: micros(policy.session), networkMultiplier: policy.multiplier, dailyQaLimit: micros(policy.daily) }; await write("/v1/budget-policy", "PUT", { policy: value, expectedVersion: policy.version }); await reload(); message.value = "费用策略已保存。"; });
}
async function saveCredential(): Promise<void> {
  await perform(async () => { const current = catalog.value?.providers.find(value => value.providerId === credentials.provider); if (!current || !credentials.secret) throw new Error("请选择渠道并输入新密钥"); const path = `/v1/admin/providers/${encodeURIComponent(current.providerId)}/credentials${current.credentialVersion ? '/rotate' : ''}`; await write(path, "POST", { providerId: current.providerId, ...(current.credentialVersion ? { newSecret: credentials.secret } : { secret: credentials.secret }), expectedVersion: current.credentialVersion }); credentials.secret = ""; await reload(); message.value = "密钥已保存；已在途调用继续使用原版本。"; });
}
async function refreshCatalog(): Promise<void> { await perform(async () => { await write("/v1/catalog:refresh", "POST", { sourceUrl: "https://models.dev/api.json" }); await reload(); message.value = "已更新公开模型目录，手工覆盖已保留。"; }); }
async function loadLedger(next = false): Promise<void> {
  await perform(async () => { const query = new URLSearchParams({ from: new Date(`${ledgerFilter.from}Z`).toISOString(), until: new Date(`${ledgerFilter.until}Z`).toISOString(), limit: "50" }); if (ledgerFilter.costClass) query.set("costClass", ledgerFilter.costClass); if (next && ledger.value?.nextCursor) query.set("cursor", ledger.value.nextCursor); ledger.value = await props.bootstrap.apiClient.request<Ledger>(`/v1/admin/ledger?${query}`, { signal: controller.signal }); });
}
async function pollProbe(id: string): Promise<void> {
  try { const result = await props.bootstrap.apiClient.request<components["schemas"]["GetInvocationResult"]>(`/v1/inferences/${encodeURIComponent(id)}`, { signal: controller.signal }); controller.signal.throwIfAborted(); probe.value = result.invocation; if (!["succeeded", "failed", "cancelled", "reconciled", "charge_unknown"].includes(result.invocation.state)) probeTimer = setTimeout(() => { void pollProbe(id); }, 1000); }
  catch (problem) { failure(problem); }
}
async function runProbe(): Promise<void> {
  await perform(async () => { if (!credentials.provider || !probeModel.value) throw new Error("请选择渠道与诊断模型"); const result = await write<components["schemas"]["CreateConnectionProbeResult"]>(`/v1/providers/${encodeURIComponent(credentials.provider)}:probe`, "POST", { providerId: credentials.provider, modelId: probeModel.value }); probe.value = result.invocation; if (probeTimer) clearTimeout(probeTimer); void pollProbe(result.invocation.invocationId); message.value = "诊断已接受，将按真实调用记录费用。"; });
}
function stop(): void { controller.abort(); navigation?.abort(); credentials.secret = ""; pending.clear(); if (probeTimer) clearTimeout(probeTimer); }
props.parentSignal.addEventListener("abort", stop, { once: true });
onMounted(() => { if (canRead.value) void perform(reload); });
onUnmounted(() => { props.parentSignal.removeEventListener("abort", stop); stop(); });
</script>

<template>
  <main class="gateway-ui">
    <header><div><h1>模型与费用</h1><p>管理调用渠道、功能档位和真实费用记录。</p></div><button v-if="canRead" :disabled="busy" @click="perform(reload)">刷新状态</button></header>
    <p v-if="!canRead" role="status">当前账号无权查看模型管理。请返回后台重新登录或联系管理员。</p>
    <p v-if="error" class="error" role="alert">{{ error }}</p><p v-if="message" role="status">{{ message }}</p>
    <template v-if="canRead && catalog">
      <section><h2>调用渠道</h2><table><thead><tr><th>名称</th><th>协议</th><th>状态</th><th>密钥</th><th>版本</th><th v-if="superadmin">操作</th></tr></thead><tbody><tr v-for="value in catalog.providers" :key="value.providerId"><td>{{ value.displayName }}</td><td>{{ value.adapter }}</td><td>{{ value.enabled ? '启用' : '停用' }}</td><td>{{ value.credentialConfigured ? `已配置（版本 ${value.credentialVersion}）` : '未配置' }}</td><td>{{ value.version }}</td><td v-if="superadmin"><button @click="selectedProvider = value">编辑</button></td></tr></tbody></table><p v-if="!catalog.providers.length">尚未配置模型渠道，文档浏览与编辑不受影响。</p>
        <template v-if="superadmin"><button @click="selectedProvider = undefined">添加渠道</button><EditView :api-client="bootstrap.apiClient" :provider="selectedProvider" :signal="controller.signal" @saved="perform(reload)" @forbidden="clearSensitive" /></template>
      </section>
      <section v-if="superadmin"><h2>密钥与连接诊断</h2><form @submit.prevent="saveCredential"><label>渠道<select v-model="credentials.provider" required><option disabled value="">请选择</option><option v-for="value in catalog.providers" :key="value.providerId" :value="value.providerId">{{ value.displayName }}</option></select></label><label>新密钥<input v-model="credentials.secret" type="password" autocomplete="new-password" required></label><button :disabled="busy">保存或轮换密钥</button></form><form @submit.prevent="runProbe"><label>诊断模型<select v-model="probeModel" required><option disabled value="">请选择</option><option v-for="value in modelsForProvider" :key="value.modelId" :value="value.modelId">{{ value.modelId }}</option></select></label><button :disabled="busy">发起付费连接诊断</button></form><p v-if="probe">诊断状态：{{ probe.state }}；调用 {{ probe.invocationId }}<span v-if="probe.attempts.some(value => value.state === 'charge_unknown')">，费用待对账</span></p></section>
      <section><h2>模型</h2><table><thead><tr><th>标识</th><th>能力</th><th>状态</th><th>来源</th><th v-if="superadmin">操作</th></tr></thead><tbody><tr v-for="value in catalog.models" :key="value.modelId"><td>{{ value.modelId }}</td><td>{{ value.capabilities.join('、') }}</td><td>{{ value.enabled ? '启用' : '停用' }}</td><td>{{ value.catalogOrigin }}{{ value.manualOverride ? '（手工覆盖）' : '' }}</td><td v-if="superadmin"><button @click="editModel(value)">编辑</button></td></tr></tbody></table>
        <template v-if="superadmin"><button @click="editModel()">添加模型</button><button :disabled="busy" @click="refreshCatalog">刷新公开模型目录</button><form class="editor" @submit.prevent="saveModel"><label>模型标识<input v-model="model.id" :disabled="model.version > 0" required></label><label>渠道<select v-model="model.provider" :disabled="model.version > 0" required><option disabled value="">请选择</option><option v-for="value in catalog.providers" :key="value.providerId" :value="value.providerId">{{ value.displayName }}</option></select></label><label>供应方不可变版本<input v-model="model.providerVersion" :disabled="model.version > 0" required></label><fieldset><legend>能力</legend><label v-for="capability in ['chat', 'vision', 'file', 'structured', 'reasoning', 'embedding', 'rerank', 'translation']" :key="capability"><input v-model="model.capabilities" :value="capability" type="checkbox">{{ capability }}</label></fieldset><label>嵌入维度<input v-model.number="model.dimension" :disabled="model.version > 0" type="number" min="0" step="1" required></label><label>预处理与分词版本<select v-model="model.preprocessing" :disabled="model.version > 0"><option value="none">不预处理（未配置精确分词）</option><option>none;tiktoken-go@v0.7.0;cl100k_base</option><option>none;tiktoken-go@v0.7.0;o200k_base</option></select></label><label>最大输入 token<input v-model.number="model.maxInput" type="number" min="0" step="1" required></label><label>最大输出 token<input v-model.number="model.maxOutput" type="number" min="0" step="1" required></label><label><input v-model="model.enabled" type="checkbox">启用</label><label><input v-model="model.manualOverride" type="checkbox">保留人工覆盖</label><label><input v-model="model.manualPrice" type="checkbox">替换人工价格</label><fieldset v-if="model.manualPrice"><legend>美元价格，空项保持未知</legend><label>每百万输入 token<input v-model="model.inputPrice" inputmode="decimal"></label><label>每百万输出 token<input v-model="model.outputPrice" inputmode="decimal"></label><label>每次固定费用<input v-model="model.fixedPrice" inputmode="decimal"></label></fieldset><button :disabled="busy">保存模型</button></form></template>
      </section>
      <section><h2>功能档位</h2><ul><li v-for="value in catalog.profiles" :key="value.profileId">{{ value.profileId }} · {{ value.costClass }} / {{ value.tier }} · {{ value.orderedModelIds.join(' → ') }}<button v-if="superadmin" @click="editProfile(value)">编辑</button></li></ul><template v-if="superadmin"><button @click="editProfile()">添加档位</button><form class="editor" @submit.prevent="saveProfile"><label>档位标识<input v-model="profile.id" :disabled="profile.version > 0" required></label><label>费用类别<select v-model="profile.costClass"><option>qa</option><option>background</option><option>admin-probe</option></select></label><label>功能<select v-model="profile.tier"><option v-for="tier in ['fast', 'accurate', 'embedding', 'vision', 'rerank']" :key="tier">{{ tier }}</option></select></label><fieldset><legend>主模型与有序备用</legend><label v-for="value in catalog.models" :key="value.modelId"><input v-model="profile.models" type="checkbox" :value="value.modelId">{{ value.modelId }}</label><ol><li v-for="(id, index) in profile.models" :key="id">{{ id }} <button type="button" :disabled="index === 0" @click="moveModel(index, -1)">上移</button><button type="button" :disabled="index === profile.models.length - 1" @click="moveModel(index, 1)">下移</button></li></ol></fieldset><label>单次上限（美元，0 不限）<input v-model="profile.maximum" inputmode="decimal" required></label><label>每日上限（美元，0 不限）<input v-model="profile.daily" inputmode="decimal" required></label><label>并发上限<input v-model.number="profile.concurrency" type="number" min="0" step="1" required></label><button :disabled="busy">保存档位</button></form></template></section>
      <section><h2>任务分配</h2><ul><li v-for="value in catalog.taskAssignments" :key="value.taskId">{{ value.taskId }} · {{ value.tier }} · {{ value.overrideModelId ?? '档位主模型' }}<button v-if="superadmin" @click="editTask(value)">编辑</button></li></ul><form v-if="superadmin" class="editor" @submit.prevent="saveTask"><label>任务标识<input v-model="task.id" required></label><label>固定默认档位<select v-model="task.tier"><option v-for="tier in ['fast', 'accurate', 'embedding', 'vision', 'rerank']" :key="tier">{{ tier }}</option></select></label><label>覆盖主模型<select v-model="task.override"><option value="">使用档位主模型</option><option v-for="value in catalog.models" :key="value.modelId" :value="value.modelId">{{ value.modelId }}</option></select></label><button :disabled="busy">保存任务分配</button></form></section>
      <section><h2>问答费用策略</h2><p v-if="!policyConfigured">尚未明确保存费用策略，问答付费调用会失败关闭。</p><form class="editor" @submit.prevent="savePolicy"><label><input v-model="policy.hourly" type="checkbox" :disabled="!superadmin">启用小时额度</label><label>会话每小时（美元）<input v-model="policy.session" :readonly="!superadmin" inputmode="decimal" required></label><label>网络额度倍数<input v-model.number="policy.multiplier" :readonly="!superadmin" type="number" min="1" max="100" step="1" required></label><label>全站问答每日（美元，0 不限）<input v-model="policy.daily" :readonly="!superadmin" inputmode="decimal" required></label><button v-if="superadmin" :disabled="busy">保存费用策略</button></form><p>后台费用独立统计；未知费用不会按零计算，已有预留不会因跨日释放。</p></section>
      <section><h2>费用账本</h2><form @submit.prevent="loadLedger()"><label>开始（UTC）<input v-model="ledgerFilter.from" type="datetime-local" required></label><label>结束（UTC）<input v-model="ledgerFilter.until" type="datetime-local" required></label><label>类别<select v-model="ledgerFilter.costClass"><option value="">全部</option><option>qa</option><option>background</option><option>admin-probe</option></select></label><button :disabled="busy">查询</button></form><template v-if="ledger"><table><thead><tr><th>时间</th><th>调用 / 尝试</th><th>真实 token</th><th>费用</th><th>状态</th></tr></thead><tbody><tr v-for="entry in ledger.entries" :key="entry.entryId"><td>{{ entry.createdAt }}</td><td>{{ entry.invocationId }} / {{ entry.attemptId }}</td><td>{{ entry.inputTokens }} / {{ entry.outputTokens }}</td><td>{{ dollars(entry.actualCost) }}</td><td>{{ entry.status }}</td></tr></tbody></table><p v-if="!ledger.entries.length">查询范围内没有已登记账单。</p><button v-if="ledger.nextCursor" :disabled="busy" @click="loadLedger(true)">下一页</button></template></section>
    </template>
  </main>
</template>

<style scoped>
.gateway-ui{font:15px/1.6 system-ui,sans-serif;color:#18252c;max-width:1240px;margin:auto;padding:28px}header{display:flex;align-items:center;justify-content:space-between;gap:20px}h1{margin:0;font-size:28px}h2{font-size:21px}header p{color:#52636b;margin:5px 0}section{border:1px solid #d9e1e5;border-radius:12px;margin-top:22px;padding:22px;background:#fff}table{width:100%;border-collapse:collapse;text-align:left;overflow-wrap:anywhere}th,td{padding:10px 8px;border-bottom:1px solid #e7ecef}th{color:#52636b}button{border:1px solid #b7c9ce;border-radius:7px;background:#f2f8f7;color:#183f3d;padding:7px 12px;cursor:pointer;margin:4px}button:disabled{opacity:.55;cursor:wait}.editor{display:grid;grid-template-columns:repeat(auto-fit,minmax(240px,1fr));gap:14px;background:#f8fafb;padding:18px;margin-top:18px;border-radius:9px}.editor h3,.editor fieldset,.editor p{grid-column:1/-1}label{display:flex;gap:7px;flex-direction:column;margin:6px 0}label:has(input[type=checkbox]){flex-direction:row;align-items:center}input,select{border:1px solid #b7c9ce;border-radius:6px;padding:8px;font:inherit;min-width:0}fieldset{border:1px solid #d9e1e5;border-radius:6px}fieldset label{display:inline-flex;margin-right:14px}.error{color:#a42626;background:#fff0ed;padding:12px;border-radius:8px}@media(max-width:700px){.gateway-ui{padding:12px}section{padding:14px;overflow:auto}header{align-items:flex-start}.editor{grid-template-columns:1fr}table{min-width:580px}}
</style>
