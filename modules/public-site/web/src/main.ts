import { createApp, h, nextTick, reactive, shallowRef } from 'vue';
import PreviewPanel from './PreviewPanel.vue';
import type { LoadPreviewInput, LoadPreviewResult } from './client';

interface ModuleUIBootstrap {
  module: 'public-site';
  apiBaseURL: string;
  locale: string;
  mountSelector: string;
  capabilities: string[];
  buildRevision: string;
}
interface PreviewHandle { LoadPreview(input: LoadPreviewInput): Promise<LoadPreviewResult> }
const mounted = new WeakMap<HTMLElement, { revision: string; unmount(): void }>();

// mountModuleUI 挂载本域预览岛；编辑器以事件在当前页内存传入精确草稿与CSRF。
export function mountModuleUI(rootElement: HTMLElement, bootstrap: ModuleUIBootstrap): void {
  if (!rootElement || rootElement.ownerDocument !== document || !rootElement.isConnected || bootstrap.module !== 'public-site' || !Array.isArray(bootstrap.capabilities)) {
    throw new Error('公开交付模块启动元数据不合法。');
  }
  const previous = mounted.get(rootElement);
  if (previous?.revision === bootstrap.buildRevision) return;
  previous?.unmount();
  const current = reactive({ documentId: '', draftRevisionId: '', apiBaseURL: bootstrap.apiBaseURL, csrfToken: '' });
  const preview = shallowRef<PreviewHandle>();
  const controller = new AbortController();
  const app = createApp({ setup: () => () => h(PreviewPanel, { ...current, ref: preview }) });
  app.mount(rootElement);
  const request = async (event: Event): Promise<void> => {
    const input = (event as CustomEvent<Pick<LoadPreviewInput, 'documentId' | 'draftRevisionId' | 'csrfToken' | 'operationId'>>).detail;
    if (!input || !bootstrap.capabilities.includes('content.preview')) return;
    Object.assign(current, { documentId: input.documentId, draftRevisionId: input.draftRevisionId, csrfToken: input.csrfToken });
    await nextTick();
    try {
      await preview.value?.LoadPreview({ ...current, operationId: input.operationId, signal: controller.signal });
    } catch { /* 组件负责安全错误与401/403清理。 */ }
  };
  const unmount = (): void => {
    controller.abort();
    rootElement.removeEventListener('public-site:preview-request', request);
    rootElement.removeEventListener('public-site:unmount', unmount);
    app.unmount();
    current.csrfToken = '';
    mounted.delete(rootElement);
  };
  rootElement.addEventListener('public-site:preview-request', request);
  rootElement.addEventListener('public-site:unmount', unmount, { once: true });
  mounted.set(rootElement, { revision: bootstrap.buildRevision, unmount });
}
