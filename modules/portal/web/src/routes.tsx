import { Archive, BookOpen, ChartColumn, Comments, Database, FaceRobot, FileText, Gear, Globe, House, ListCheck, Person, Persons, Picture, Shield } from '@gravity-ui/icons';
import { lazy, type ComponentType, type LazyExoticComponent } from 'react';
import groups from '../../../../contracts/permission-groups.json';
import { t } from '@ui';

// 后台页面的路径、所需权限与中文名只在 contracts/permission-groups.json 的 adminRoutes 登记；
// 这里只补界面需要的分组、图标和页面组件（各域页面源码在 modules/<域>/web/src）。

type Contract = { key: string; path: string; permissions: string[]; labelZH: string; navigation: boolean };

export type Page = {
  key: string;
  path: string;
  label: string;
  permissions: string[];
  navigation: boolean;
  section: string;
  icon: ComponentType<{ className?: string }>;
  component: LazyExoticComponent<ComponentType>;
  // 整屏布局（如文档编辑）：外壳不加内边距，由页面自己排版。
  bleed?: boolean;
};

const ui: Record<string, Omit<Page, 'key' | 'path' | 'label' | 'permissions' | 'navigation'>> = {
  dashboard: { section: '', icon: House, component: lazy(() => import('./Dashboard')) },
  me: { section: '', icon: Person, component: lazy(() => import('./Profile')) },
  content: { section: t('内容'), icon: FileText, component: lazy(() => import('@modules/content/web/src')), bleed: true },
  media: { section: t('内容'), icon: Picture, component: lazy(() => import('@modules/media/web/src')) },
  settings: { section: t('站点'), icon: Gear, component: lazy(() => import('@modules/site-config/web/src')) },
  localization: { section: t('站点'), icon: Globe, component: lazy(() => import('@modules/localization/web/src')) },
  models: { section: 'AI', icon: FaceRobot, component: lazy(() => import('@modules/model-gateway/web/src')) },
  knowledge: { section: 'AI', icon: Database, component: lazy(() => import('@modules/knowledge/web/src')) },
  memory: { section: 'AI', icon: BookOpen, component: lazy(() => import('@modules/memory/web/src')) },
  qa: { section: 'AI', icon: Comments, component: lazy(() => import('@modules/qa/web/src')) },
  accounts: { section: t('系统'), icon: Persons, component: lazy(() => import('@modules/identity/web/src')) },
  analytics: { section: t('系统'), icon: ChartColumn, component: lazy(() => import('@modules/analytics/web/src')) },
  jobs: { section: t('系统'), icon: ListCheck, component: lazy(() => import('@modules/jobs/web/src')) },
  guard: { section: t('系统'), icon: Shield, component: lazy(() => import('@modules/traffic-guard/web/src')) },
  backup: { section: t('系统'), icon: Archive, component: lazy(() => import('@modules/data-protection/web/src')) },
};

const order = ['dashboard', 'content', 'media', 'settings', 'localization', 'models', 'knowledge', 'memory', 'qa', 'accounts', 'analytics', 'jobs', 'guard', 'backup', 'me'];

export const pages: Page[] = order.map((key) => {
  const route = (groups.adminRoutes as Contract[]).find((item) => item.key === key)!;
  return { key, path: route.path, label: t(route.labelZH), permissions: route.permissions, navigation: route.navigation, ...ui[key] };
});

export const sections = ['', t('内容'), t('站点'), 'AI', t('系统')];

// 旧版后台按服务名的地址，打开时换成登记的新地址。
const legacy: Record<string, string> = {
  '/admin/site-config': '/admin/site', '/admin/identity': '/admin/accounts', '/admin/model-gateway': '/admin/models',
  '/admin/traffic-guard': '/admin/guard', '/admin/data-protection': '/admin/backup',
};

export function legacyPath(path: string): string | null {
  return legacy[path] ?? null;
}

export function canOpen(page: Page, grants: string[]): boolean {
  return page.permissions.some((permission) => grants.includes(permission));
}

export function pageFor(path: string): Page | undefined {
  return pages.find((page) => path === page.path || path.startsWith(page.path + '/'));
}
