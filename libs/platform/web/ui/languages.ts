// 预设的 15 种站点语言（与 libs/platform/languages.go 一致）：初始化时的基准语言、之后添加的语言和后台界面语言都只能从这里选。
export type SiteLanguage = { tag: string; native: string; direction: 'ltr' | 'rtl' };

export const siteLanguages: SiteLanguage[] = [
  { tag: 'zh-Hans', native: '简体中文', direction: 'ltr' },
  { tag: 'zh-Hant', native: '繁體中文', direction: 'ltr' },
  { tag: 'en', native: 'English', direction: 'ltr' },
  { tag: 'ja', native: '日本語', direction: 'ltr' },
  { tag: 'ko', native: '한국어', direction: 'ltr' },
  { tag: 'fr', native: 'Français', direction: 'ltr' },
  { tag: 'de', native: 'Deutsch', direction: 'ltr' },
  { tag: 'es', native: 'Español', direction: 'ltr' },
  { tag: 'pt', native: 'Português', direction: 'ltr' },
  { tag: 'ru', native: 'Русский', direction: 'ltr' },
  { tag: 'it', native: 'Italiano', direction: 'ltr' },
  { tag: 'vi', native: 'Tiếng Việt', direction: 'ltr' },
  { tag: 'th', native: 'ไทย', direction: 'ltr' },
  { tag: 'id', native: 'Bahasa Indonesia', direction: 'ltr' },
  { tag: 'ar', native: 'العربية', direction: 'rtl' },
];

export const findSiteLanguage = (tag: string) => siteLanguages.find((language) => language.tag === tag);
