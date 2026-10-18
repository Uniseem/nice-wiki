import { createApp, h, onBeforeUnmount, ref, type App } from 'vue';
import type { ModuleUIBootstrap } from '../../../../libs/platform/web/module-api';
import BackupPanel from './BackupPanel.vue';
import RestoreWizard from './RestoreWizard.vue';
import GitSyncPanel from './GitSyncPanel.vue';

const mounts = new WeakMap<HTMLElement, { app: App; revision: string }>();

// mountModuleUI 只接受portal注入的同源端口，不在浏览器构造服务凭据。
export function mountModuleUI(rootElement: HTMLElement, bootstrap: ModuleUIBootstrap): void {
  if (!(rootElement instanceof HTMLElement) || rootElement.ownerDocument !== document || bootstrap.module !== 'data-protection' || !bootstrap.buildRevision || !Array.isArray(bootstrap.capabilities) || !bootstrap.apiClient?.request) {
    throw new Error('数据保护启动信息无效');
  }
  const current = mounts.get(rootElement);
  if (current?.revision === bootstrap.buildRevision) return;
  current?.app.unmount();
  const controller = new AbortController();
  const app = createApp({
    setup() {
      const selectedBackup = ref('');
      onBeforeUnmount(() => controller.abort());
      return () => h('section', { class: 'data-protection', lang: bootstrap.locale }, [
        h('h1', '数据保护'),
        h('p', '完整备份、全站恢复与版本镜像。备份和恢复期间站点会进入维护。'),
        h(BackupPanel, { bootstrap, parentSignal: controller.signal, onSelectBackup: (id: string) => { selectedBackup.value = id; } }),
        h(RestoreWizard, { bootstrap, parentSignal: controller.signal, initialBackup: selectedBackup.value }),
        h(GitSyncPanel, { bootstrap, parentSignal: controller.signal, initialBackup: selectedBackup.value }),
      ]);
    },
  });
  mounts.set(rootElement, { app, revision: bootstrap.buildRevision });
  rootElement.addEventListener('nice-wiki:unmount', () => { app.unmount(); mounts.delete(rootElement); }, { once: true, signal: controller.signal });
  app.mount(rootElement);
}
