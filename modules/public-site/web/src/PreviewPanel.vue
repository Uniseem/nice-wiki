<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue';
import { requestPreview, type LoadPreviewInput, type LoadPreviewResult, type Problem } from './client';

const props = defineProps<{
  documentId: string;
  draftRevisionId: string;
  apiBaseURL: string;
  csrfToken: string;
}>();
const previewHTML = ref('');
const message = ref('预览仅对当前登录账号开放。');
const busy = ref(false);
let controller: AbortController | undefined;
let operationId = '';
let operationSource = '';
let expiresAt = 0;

// LoadPreview 由编辑器传入明确草稿ID和当前页内存CSRF；刷新重新验证本人grant。
async function LoadPreview(input: LoadPreviewInput): Promise<LoadPreviewResult> {
  controller?.abort();
  const active = new AbortController();
  controller = active;
  const cancel = (): void => active.abort();
  input.signal.addEventListener('abort', cancel, { once: true });
  if (input.signal.aborted) active.abort();
  previewHTML.value = '';
  busy.value = true;
  try {
    const result = await requestPreview({ ...input, signal: active.signal });
    if (controller === active) {
      previewHTML.value = result.html;
      expiresAt = Date.parse(result.expiresAt);
      message.value = '当前草稿的受保护预览；刷新仍需当前账号权限。';
    }
    return result;
  } catch (error) {
    if (controller === active && !active.signal.aborted) {
      previewHTML.value = '';
      const code = (error as Partial<Problem>).code;
      message.value = code === 'UNAUTHENTICATED' ? '登录已失效，请重新登录后领取预览。'
        : code === 'FORBIDDEN' ? '当前账号无预览权限，已清空预览内容。'
        : code === 'STALE_RESULT' || code === 'NOT_FOUND' ? '该草稿或预览已变化，请重新领取。'
        : '预览依赖暂不可用，请稍后重试。';
      if (code === 'FORBIDDEN' || code === 'UNAUTHENTICATED' || code === 'STALE_RESULT' || code === 'NOT_FOUND') operationId = '';
    }
    throw error;
  } finally {
    input.signal.removeEventListener('abort', cancel);
    if (controller === active) busy.value = false;
  }
}

async function load(): Promise<void> {
  const source = props.documentId + ':' + props.draftRevisionId;
  if (!operationId || source !== operationSource || Date.now() >= expiresAt && expiresAt > 0) {
    operationId = crypto.randomUUID();
    operationSource = source;
    expiresAt = 0;
  }
  try {
    await LoadPreview({ ...props, operationId, signal: new AbortController().signal });
  } catch { /* LoadPreview 已提供安全中文错误，不把token或正文写日志。 */ }
}

watch([() => props.documentId, () => props.draftRevisionId, () => props.csrfToken], () => {
  controller?.abort();
  previewHTML.value = '';
  operationId = '';
  expiresAt = 0;
});

onBeforeUnmount(() => {
  controller?.abort();
  previewHTML.value = '';
  operationId = '';
});
defineExpose({ LoadPreview });
</script>

<template>
  <section aria-label="草稿预览">
    <p role="status">{{ message }}</p>
    <button type="button" :disabled="busy || !documentId || !draftRevisionId || !csrfToken" @click="load">
      {{ busy ? '正在领取预览…' : '领取或刷新预览' }}
    </button>
    <iframe v-if="previewHTML" title="当前草稿的受保护预览" :srcdoc="previewHTML"
      sandbox="allow-same-origin" referrerpolicy="no-referrer" />
  </section>
</template>

<style scoped>
section { display: grid; gap: 1rem; }
button { justify-self: start; }
iframe { width: 100%; min-height: 70vh; border: 1px solid #8885; }
</style>
