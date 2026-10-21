import { findSiteLanguage, siteLanguages } from './languages';

// 后台界面多语言（gettext 风格）：源文案就是中文原文，t('中文') 按当前界面语言取译文，缺译文先用英文、再退回中文。
// 占位符写成 {0}、{1}，按参数顺序替换：t('已删除 {0} 项', count)。
// 译文写在各模块的 web/i18n.json（共用件在 libs/platform/web/i18n.json），一条中文对应各语言译文；
// 构建时按语言拆成 virtual:admin-messages/<语言>（见 modules/portal/web/vite.config.ts），只加载当前语言。
// 换语言时整页刷新：模块里预先算好的文字也跟着重算。

const storageKey = 'nw.adminLanguage';
let current = 'zh-Hans';
let messages: Record<string, string> = {};
let english: Record<string, string> = {};

const loaders: Record<string, () => Promise<{ default: Record<string, string> }>> = {
  'zh-Hant': () => import('virtual:admin-messages/zh-Hant'),
  en: () => import('virtual:admin-messages/en'),
  ja: () => import('virtual:admin-messages/ja'),
  ko: () => import('virtual:admin-messages/ko'),
  fr: () => import('virtual:admin-messages/fr'),
  de: () => import('virtual:admin-messages/de'),
  es: () => import('virtual:admin-messages/es'),
  pt: () => import('virtual:admin-messages/pt'),
  ru: () => import('virtual:admin-messages/ru'),
  it: () => import('virtual:admin-messages/it'),
  vi: () => import('virtual:admin-messages/vi'),
  th: () => import('virtual:admin-messages/th'),
  id: () => import('virtual:admin-messages/id'),
  ar: () => import('virtual:admin-messages/ar'),
};

export function t(source: string, ...values: unknown[]): string {
  const text = messages[source] ?? english[source] ?? source;
  if (!values.length) return text;
  return text.replace(/\{(\d+)\}/g, (match, index: string) => (Number(index) < values.length ? String(values[Number(index)]) : match));
}

// adminLanguage 是当前后台界面语言（站点语言标签）。
export function adminLanguage(): string {
  return current;
}

// chosenAdminLanguage 是这个浏览器里选过的后台语言；没选过时为空。
export function chosenAdminLanguage(): string {
  try {
    const stored = localStorage.getItem(storageKey) ?? '';
    return findSiteLanguage(stored) ? stored : '';
  } catch {
    return '';
  }
}

// setAdminLanguage 记下选择并整页刷新；传空字符串表示清掉选择（回到默认）。
export function setAdminLanguage(tag: string): void {
  try {
    if (tag) localStorage.setItem(storageKey, tag);
    else localStorage.removeItem(storageKey);
  } catch { /* 存不了就只对本次刷新前有效 */ }
  location.reload();
}

// loadAdminLanguage 在渲染后台之前调用一次：用这个浏览器选过的语言，没选过就用站点基准语言，站点还没初始化时用简体中文。
export async function loadAdminLanguage(): Promise<void> {
  let tag = chosenAdminLanguage();
  if (!tag) {
    try {
      const response = await fetch('/v1/home/public', { credentials: 'same-origin' });
      const home = response.ok ? ((await response.json()) as { baseLanguage?: string }) : {};
      if (home.baseLanguage && findSiteLanguage(home.baseLanguage)) tag = home.baseLanguage;
    } catch { /* 站点还没初始化 */ }
  }
  current = tag || siteLanguages[0].tag;
  if (current !== 'zh-Hans') {
    const [own, fallback] = await Promise.all([loaders[current](), current === 'en' ? null : loaders.en()]);
    messages = own.default;
    english = fallback?.default ?? {};
  }
  document.documentElement.lang = current;
  document.documentElement.dir = findSiteLanguage(current)?.direction ?? 'ltr';
}
