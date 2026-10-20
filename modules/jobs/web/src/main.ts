import { createApp, h, onBeforeUnmount, type App } from 'vue';
import type { ModuleUIBootstrap } from '../../../../libs/platform/web/module-api';
import QueuePanel from './QueuePanel.vue';
import ScheduleEditor from './ScheduleEditor.vue';
import { ConfigureProgressAPI } from './progress';

const mounts = new WeakMap<HTMLElement, { app: App; revision: string }>();

// portal 只注入同源 API 客户端；本模块不读取 service token 或跨域地址。
export function mountModuleUI(rootElement: HTMLElement, bootstrap: ModuleUIBootstrap): void {
  if (!(rootElement instanceof HTMLElement) || rootElement.ownerDocument !== document || bootstrap.module !== 'jobs' || !bootstrap.buildRevision || !Array.isArray(bootstrap.capabilities) || !bootstrap.apiClient?.request) {
    throw new Error('任务管理启动信息无效');
  }
  const current = mounts.get(rootElement);
  if (current?.revision === bootstrap.buildRevision) return;
  current?.app.unmount();
  ConfigureProgressAPI(bootstrap.apiClient);
  const controller = new AbortController();
  const app = createApp({
    setup() {
      onBeforeUnmount(() => controller.abort());
      return () => h('section', { class: 'jobs-workspace', lang: bootstrap.locale }, [
        h('header', [
          h('h1', '任务与调度'),
          h('p', '查看任务、死信投递和进度；取消只记录意图，需由任务所属服务确认安全终态。'),
        ]),
        h(QueuePanel, { bootstrap, parentSignal: controller.signal }),
        h(ScheduleEditor, { bootstrap, parentSignal: controller.signal }),
      ]);
    },
  });
  mounts.set(rootElement, { app, revision: bootstrap.buildRevision });
  rootElement.addEventListener('nice-wiki:unmount', () => {
    app.unmount();
    mounts.delete(rootElement);
  }, { once: true, signal: controller.signal });
  app.mount(rootElement);
}
