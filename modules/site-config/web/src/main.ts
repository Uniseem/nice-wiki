import { createApp, type App } from 'vue';
import type { ModuleUIBootstrap } from '../../../../libs/platform/web/module-api';
import SitePage from './SitePage.vue';
import { configureAPI } from './client';
import './style.css';

const mounts = new WeakMap<HTMLElement, { app: App; revision: string }>();

// portal 只注入同源 API 客户端；本模块不接触凭据，也不请求其他域的路径。
export function mountModuleUI(rootElement: HTMLElement, bootstrap: ModuleUIBootstrap): void {
  if (bootstrap.module !== 'site-config' || !bootstrap.apiClient?.request) throw new Error('界面启动信息无效');
  const current = mounts.get(rootElement);
  if (current?.revision === bootstrap.buildRevision) return;
  current?.app.unmount();
  configureAPI(bootstrap.apiClient);
  const app = createApp(SitePage);
  mounts.set(rootElement, { app, revision: bootstrap.buildRevision });
  rootElement.addEventListener('nice-wiki:unmount', () => { app.unmount(); mounts.delete(rootElement); }, { once: true });
  app.mount(rootElement);
}
