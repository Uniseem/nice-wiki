import { createApp, h, type App } from 'vue';
import type { ModuleAPIClient, ModuleUIBootstrap } from '../../../../libs/platform/web/module-api';
import OverviewPanel from './OverviewPanel.vue';
import AuditExplorer from './AuditExplorer.vue';
import { createAnalyticsClient } from './client';
import './style.css';
const mounts = new WeakMap<HTMLElement, { revision: string; capabilities: string; api: ModuleAPIClient; app: App; unmount(): void }>();

// mountModuleUI只接portal同源端口；同DOM重挂先停旧请求，权限变化由统一unmount事件清理。
export function mountModuleUI(rootElement: HTMLElement, bootstrap: ModuleUIBootstrap): void {
  if (!(rootElement instanceof HTMLElement) || rootElement.ownerDocument !== document || !rootElement.isConnected || bootstrap.module !== 'analytics' || !bootstrap.buildRevision || !Array.isArray(bootstrap.capabilities) || !bootstrap.apiClient?.request) throw new Error('统计模块启动信息无效');
  const capabilities = JSON.stringify([...bootstrap.capabilities].sort()); const current = mounts.get(rootElement);
  if (current?.revision === bootstrap.buildRevision && current.capabilities === capabilities && current.api === bootstrap.apiClient) return;
  current?.unmount(); const controller = new AbortController(); const client = createAnalyticsClient(bootstrap.apiClient);
  const app = createApp({ setup: () => () => h('section', { class: 'analytics-module', lang: bootstrap.locale }, [h('header', [h('h1', '统计与审计'), h('p', '查看已保存的访问统计、费用投影与管理审计。')]), ...(bootstrap.capabilities.includes('stats.read') ? [h(OverviewPanel, { client, parentSignal: controller.signal })] : []), ...(bootstrap.capabilities.includes('audit.read') ? [h(AuditExplorer, { client, parentSignal: controller.signal })] : []), ...(!bootstrap.capabilities.some(value => value === 'stats.read' || value === 'audit.read') ? [h('p', { role: 'status' }, '当前没有统计或审计读取权限。')] : [])]) });
  function unmountAnalytics(): void { controller.abort(); app.unmount(); mounts.delete(rootElement); }
  rootElement.addEventListener('nice-wiki:unmount', unmountAnalytics, { once: true, signal: controller.signal }); mounts.set(rootElement, { revision: bootstrap.buildRevision, capabilities, api: bootstrap.apiClient, app, unmount: unmountAnalytics }); app.mount(rootElement);
}
