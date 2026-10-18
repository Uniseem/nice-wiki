<script lang="ts">
import type { ModuleAPIClient } from "../../../../../libs/platform/web/module-api";
import type { operations } from "../gen/api.gen";
import type { IndexConfig, IndexOptions } from "./OverviewView.vue";
import { safeCode } from "./OverviewView.vue";
export type SubmitPrimaryEditInput = operations["putIndexConfig"]["requestBody"]["content"]["application/json"];
export type SubmitPrimaryEditResult = operations["putIndexConfig"]["responses"][200]["content"]["application/json"];
export async function SubmitPrimaryEdit(client: ModuleAPIClient, input: SubmitPrimaryEditInput, signal: AbortSignal): Promise<SubmitPrimaryEditResult> {
 const config = input.config;
 if (!Number.isSafeInteger(input.expectedVersion) || input.expectedVersion < 0 || config.version !== input.expectedVersion || !Number.isInteger(config.chunkTokens) || config.chunkTokens < 128 || config.chunkTokens > 1024 || !Number.isInteger(config.overlapTokens) || config.overlapTokens < 0 || config.overlapTokens > 128 || config.overlapTokens >= config.chunkTokens || !input.idempotencyKey) throw { code: "VALIDATION" };
 return client.request<SubmitPrimaryEditResult>("/v1/index-config", { method: "PUT", body: input, idempotencyKey: input.idempotencyKey, signal });
}
</script>
<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from "vue";
const props = defineProps<{ client: ModuleAPIClient; config?: IndexConfig; options: IndexOptions }>();
const emit = defineEmits<{ saved: [config: IndexConfig] }>();
const enabled = ref(props.config?.enabled ?? false);
const profileID = ref(props.config?.embeddingProfileId ?? "");
const rerankID = ref(props.config?.rerankProfileId ?? "");
const chunkTokens = ref(props.config?.chunkTokens ?? 512);
const overlapTokens = ref(props.config?.overlapTokens ?? 64);
const expectedVersion = props.config?.version ?? 0;
const configID = props.config?.configId ?? `knowledge.config.${crypto.randomUUID()}`;
const failure = ref(""); const busy = ref(false); const conflict = ref(false); const currentVersion = ref<number>();
const controller = new AbortController();
const embeddingOptions = computed(() => props.options.profiles.filter(item => item.tier === "embedding" && item.costClass === "background"));
const rerankOptions = computed(() => props.options.profiles.filter(item => item.tier === "rerank" && item.costClass === "qa"));
let pending: { serialized: string; key: string } | undefined;
async function save(): Promise<void> {
 if (busy.value || conflict.value) return;
 const selected = embeddingOptions.value.find(item => item.profileId === profileID.value);
 const space = selected?.space ?? (profileID.value === props.config?.embeddingProfileId ? props.config?.space : undefined);
 if (!space || (enabled.value && (!selected?.usable || props.options.modelsState !== "ready"))) { failure.value = "请选择可用的后台嵌入档位。"; return; }
 const config: IndexConfig = { configId: configID, version: expectedVersion, enabled: enabled.value, embeddingProfileId: profileID.value, space, chunkTokens: Number(chunkTokens.value), overlapTokens: Number(overlapTokens.value), language: props.options.baseLanguage };
 if (rerankID.value) config.rerankProfileId = rerankID.value;
 const serialized = JSON.stringify(config);
 if (pending?.serialized !== serialized) pending = { serialized, key: crypto.randomUUID() };
 busy.value = true; failure.value = "";
 try {
  const result = await SubmitPrimaryEdit(props.client, { config, expectedVersion, idempotencyKey: pending.key }, controller.signal);
  if (!controller.signal.aborted) emit("saved", result.config);
 } catch (error) {
  if (controller.signal.aborted) return;
  const code = safeCode(error);
  if (code === "VERSION_MISMATCH" || code === "CONFLICT") { conflict.value = true; failure.value = "配置版本已变化，当前输入已保留。请记录修改，关闭编辑后刷新最新版本，再重新编辑。";
   try { const latest = await props.client.request<{ config: IndexConfig; version: number }>("/v1/index-config", { signal: controller.signal }); if (!controller.signal.aborted) currentVersion.value = latest.version; } catch { /* 权威不可用仍保留输入和冲突状态，不推测版本。 */ } }
  else failure.value = `保存未完成：${code}。当前输入已保留。`;
 } finally { if (!controller.signal.aborted) busy.value = false; }
}
onBeforeUnmount(() => { controller.abort(); });
</script>
<template>
 <form class="knowledge-edit" @submit.prevent="save">
  <p>保存参数不会触发模型或重建。切换模型后，已有索引按原空间保留，需单独构建新代数。</p>
  <p v-if="options.modelsState !== 'ready'" role="status">模型目录暂不可用：{{ options.modelErrorCode }}。可暂停已有配置的入库。</p>
  <p v-if="failure" role="alert" class="knowledge-error">{{ failure }}<span v-if="currentVersion !== undefined"> 服务端当前版本：{{ currentVersion }}。</span></p>
  <fieldset :disabled="busy || conflict"><legend>入库参数 · 当前版本 {{ expectedVersion }}</legend>
   <label class="knowledge-check"><input v-model="enabled" type="checkbox">开启发布内容入库</label>
   <label>后台嵌入档位<select v-model="profileID" required><option value="" disabled>选择档位</option><option v-if="config && !embeddingOptions.some(item => item.profileId === config?.embeddingProfileId)" :value="config.embeddingProfileId">当前档位（目录不可用或已移除）</option><option v-for="item in embeddingOptions" :key="item.profileId" :value="item.profileId" :disabled="!item.usable && item.profileId !== config?.embeddingProfileId">{{ item.label }}{{ item.usable ? '' : `（${item.reasonCode}）` }}</option></select></label>
   <label>检索重排档位<select v-model="rerankID"><option value="">不启用重排</option><option v-if="config?.rerankProfileId && !rerankOptions.some(item => item.profileId === config?.rerankProfileId)" :value="config.rerankProfileId">当前档位（目录不可用或已移除）</option><option v-for="item in rerankOptions" :key="item.profileId" :value="item.profileId" :disabled="!item.usable">{{ item.label }}</option></select></label>
   <div class="knowledge-grid"><label>每块 token 预算<input v-model.number="chunkTokens" type="number" min="128" max="1024" step="1" required></label><label>重叠 token 预算<input v-model.number="overlapTokens" type="number" min="0" max="128" step="1" required></label></div>
   <p>源语言：{{ options.baseLanguage }}</p><button type="submit">{{ busy ? '保存中…' : '保存配置' }}</button>
  </fieldset>
 </form>
</template>
