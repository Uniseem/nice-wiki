<script lang="ts">
import type { ModuleAPIClient } from "../../../../../libs/platform/web/module-api";
import type { components } from "../gen/api.gen";

export type ProviderEdit = components["schemas"]["PutProviderInput"];
export type ProviderEditResult = components["schemas"]["PutProviderResult"];

export async function submitPrimaryEdit(apiClient: ModuleAPIClient, input: ProviderEdit, signal: AbortSignal): Promise<ProviderEditResult> {
  signal.throwIfAborted();
  if (!Number.isSafeInteger(input.expectedVersion) || input.expectedVersion < 0 || input.provider.version !== input.expectedVersion + 1 || !input.idempotencyKey) throw new Error("VALIDATION");
  const url = new URL(input.provider.baseUrl);
  if (url.protocol !== "https:" || url.username || url.password || url.search || url.hash) throw new Error("供应方地址须为不含认证信息的 HTTPS 根地址");
  return apiClient.request<ProviderEditResult>(`/v1/admin/providers/${encodeURIComponent(input.provider.providerId)}`, { method: "PUT", body: input, signal, idempotencyKey: input.idempotencyKey });
}
</script>

<script setup lang="ts">
import { computed, onUnmounted, reactive, ref, watch } from "vue";


type Summary = components["schemas"]["ProviderSummary"];
const props = defineProps<{ apiClient: ModuleAPIClient; provider?: Summary; signal: AbortSignal }>();
const emit = defineEmits<{ saved: []; forbidden: [] }>();
const form = reactive({ id: "", name: "", base: "", adapter: "openai_chat", enabled: true, connect: 10000, timeout: 180000, idle: 30000, concurrency: 1 });
const replaceHeaders = ref(false);
const headers = ref<Array<{ name: string; value: string }>>([]);
const busy = ref(false);
const error = ref("");
const retryKey = ref("");
const version = computed(() => props.provider?.version ?? 0);
watch(() => props.provider, (provider) => {
  Object.assign(form, provider ? { id: provider.providerId, name: provider.displayName, base: provider.baseUrl, adapter: provider.adapter, enabled: provider.enabled, connect: provider.connectTimeoutMs, timeout: provider.requestTimeoutMs, idle: provider.streamIdleTimeoutMs, concurrency: provider.concurrencyLimit } : { id: "", name: "", base: "", adapter: "openai_chat", enabled: true, connect: 10000, timeout: 180000, idle: 30000, concurrency: 1 });
  replaceHeaders.value = false; headers.value = []; error.value = ""; retryKey.value = "";
}, { immediate: true });
watch([form, replaceHeaders, headers], () => { retryKey.value = ""; }, { deep: true });
function clearSecrets(): void { headers.value.forEach(header => { header.value = ""; }); headers.value = []; }
props.signal.addEventListener("abort", clearSecrets, { once: true });
onUnmounted(() => { props.signal.removeEventListener("abort", clearSecrets); clearSecrets(); });
async function save(): Promise<void> {
  if (busy.value) return;
  busy.value = true; error.value = ""; retryKey.value ||= crypto.randomUUID();
  const input: ProviderEdit = { provider: { providerId: form.id, version: version.value + 1, adapter: form.adapter as Summary["adapter"], baseUrl: form.base, enabled: form.enabled, credentialVersion: props.provider?.credentialVersion ?? 0, connectTimeoutMs: form.connect, requestTimeoutMs: form.timeout, displayName: form.name, extraHeaderNames: props.provider?.extraHeaderNames ?? [], extraHeaderSecretRefs: [], concurrencyLimit: form.concurrency, streamIdleTimeoutMs: form.idle }, expectedVersion: version.value, idempotencyKey: retryKey.value };
  if (replaceHeaders.value) input.secretHeaders = headers.value.map(header => ({ ...header }));
  try { await submitPrimaryEdit(props.apiClient, input, props.signal); headers.value.forEach(header => { header.value = ""; }); headers.value = []; emit("saved"); }
  catch (failure) { const problem = failure as { code?: string; message?: string }; if (["UNAUTHENTICATED", "FORBIDDEN"].includes(problem.code ?? "")) emit("forbidden"); error.value = problem.code === "VERSION_MISMATCH" || problem.code === "CONFLICT" ? "配置已变化，输入已保留。请先刷新并核对当前版本，再重新提交。" : problem.message ?? "保存失败，请保留当前输入后重试。"; }
  finally { busy.value = false; }
}
</script>

<template>
  <form class="editor" @submit.prevent="save">
    <h3>{{ provider ? `编辑渠道：${provider.displayName}` : "添加渠道" }}</h3>
    <label>渠道标识<input v-model="form.id" :disabled="!!provider" required maxlength="128"></label>
    <label>显示名称<input v-model="form.name" required maxlength="128"></label>
    <label>协议<select v-model="form.adapter"><option v-for="adapter in ['openai_chat', 'anthropic_messages', 'gemini', 'openai_embeddings', 'gemini_embeddings', 'rerank']" :key="adapter">{{ adapter }}</option></select></label>
    <label>HTTPS API 根地址<input v-model="form.base" type="url" placeholder="https://api.example.com/v1" required></label>
    <label>并发上限<input v-model.number="form.concurrency" type="number" min="0" step="1" required></label>
    <label>连接超时（毫秒）<input v-model.number="form.connect" type="number" min="0" max="10000" step="1" required></label>
    <label>请求超时（毫秒）<input v-model.number="form.timeout" type="number" min="0" max="180000" step="1" required></label>
    <label>流空闲超时（毫秒）<input v-model.number="form.idle" type="number" min="0" max="180000" step="1" required></label>
    <label><input v-model="form.enabled" type="checkbox">启用渠道</label>
    <label><input v-model="replaceHeaders" type="checkbox">替换全部额外请求头</label>
    <fieldset v-if="replaceHeaders"><legend>秘密值只写入，不回显</legend><p>空列表保存会清空全部额外请求头。不勾选替换则保留原配置。</p>
      <div v-for="(header, index) in headers" :key="index"><input v-model="header.name" aria-label="请求头名称" required><input v-model="header.value" type="password" autocomplete="new-password" aria-label="秘密值" required><button type="button" @click="headers.splice(index, 1)">移除</button></div>
      <button type="button" @click="headers.push({ name: '', value: '' })">添加请求头</button>
    </fieldset>
    <p v-if="error" role="alert">{{ error }}</p><button :disabled="busy">{{ busy ? "保存中…" : "保存渠道" }}</button>
  </form>
</template>
