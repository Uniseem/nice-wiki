import { createApp, h, onBeforeUnmount, type App } from 'vue';
import type { ModuleUIBootstrap } from '../../../../libs/platform/web/module-api';
import BackupPanel from './BackupPanel.vue';

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
      onBeforeUnmount(() => controller.abort());
      return () => h('section', { class: 'data-protection', lang: bootstrap.locale }, [
        h('h1', '数据保护'),
        h('p', '完整备份：创建、列表、下载（可设口令加密，含密钥的备份必须设口令），可开每日自动备份。备份期间站点照常运行。'),
        h(BackupPanel, { bootstrap, parentSignal: controller.signal }),
        h('section', { 'aria-labelledby': 'restore-heading' }, [
          h('h2', { id: 'restore-heading' }, '如何恢复'),
          h('p', '恢复会替换全站数据，需要在服务器上停服务后进行，目前不能在后台页面里直接恢复；版本镜像与远程同步尚未提供。'),
          h('p', '把下载的备份放到服务器上，在部署目录运行：'),
          h('pre', 'deploy/restore-archive.sh <备份文件的绝对路径>'),
          h('p', '加密备份（.enc）会先提示输入口令，也可设环境变量 NW_BACKUP_PASSPHRASE 免交互。脚本会先校验归档、停服务、逐个数据卷替换并重新启动；含密钥的备份会把密钥解到 deploy/secrets-restored，确认后再覆盖 deploy/secrets。恢复完成后请用备份中的账号重新登录。'),
        ]),
      ]);
    },
  });
  mounts.set(rootElement, { app, revision: bootstrap.buildRevision });
  rootElement.addEventListener('nice-wiki:unmount', () => { app.unmount(); mounts.delete(rootElement); }, { once: true, signal: controller.signal });
  app.mount(rootElement);
}
