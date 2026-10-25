import { I18nProvider } from '@heroui/react';
import { installExternalLinks } from '@ui/links';
import { preloadQa, QaFloating, QaHome, QaInline } from '@modules/qa/web/src/island';
import { lazy, Suspense, useEffect, useState } from 'react';
import { createPortal } from 'react-dom';
import { createRoot } from 'react-dom/client';
import { enhance, languageHint, suggestLanguage } from './enhance';
import { HeaderActions, LanguageSuggestion, type Config, type Language } from './islands';
import { startNavigation } from './navigate';
import './site.css';

// 公开页的交互岛：页面由 Go 完整输出，这里用一个 React 根，通过 portal 接管页头操作区与问答区，
// 页头、文档树抽屉、问答面板之间因此能共享状态（比如页头按钮直接打开问答面板）。
// 抽屉类部件（问答面板、窄屏文档树）第一次打开时才下载，首屏只带页头按钮和菜单。
// 文档页之间站内跳转不刷新页面（navigate.ts），换页后新页面的配置经 nw:navigate 事件传进来。

const initialConfig: Config = (() => {
  try { return JSON.parse(document.getElementById('nw-islands')?.dataset.config ?? '{}') as Config; } catch { return {} as Config; }
})();
// 站内跳转只在同语言的页面之间发生，界面文字不随换页变化。
const t = (key: string) => initialConfig.ui?.[key] ?? key;
// 当前文档页的标题（问答窗口里提示 AI 正在读哪一页）；首页、问答页等没有文档时为空。
const pageTitle = (config: Config) => (config.documentId ? document.querySelector('main h1, article h1, h1')?.textContent?.trim() || undefined : undefined);
const loadNav = () => import('./nav-drawer');
const MobileNav = lazy(loadNav);
const loadSearch = () => import('./search');
const Search = lazy(loadSearch);

function Islands({ header, ask, homeAsk }: { header: HTMLElement | null; ask: HTMLElement | null; homeAsk: HTMLElement | null }) {
  const [config, setConfig] = useState(initialConfig);
  const [askOpen, setAskOpen] = useState(false);
  const [navOpen, setNavOpen] = useState(false);
  const [navUsed, setNavUsed] = useState(false);
  const [suggested, setSuggested] = useState<Language | null>(null);
  const [searchOpen, setSearchOpen] = useState(false);
  const [searchUsed, setSearchUsed] = useState(false);
  if (navOpen && !navUsed) setNavUsed(true);
  if (searchOpen && !searchUsed) setSearchUsed(true);

  // 站内换页：换成新页面的配置（当前文档、语言菜单的链接），关掉文档树抽屉和搜索；问答面板保持打开。
  useEffect(() => {
    const onNavigate = (event: Event) => {
      setConfig({ ...initialConfig, ...(event as CustomEvent<Config>).detail });
      setNavOpen(false);
      setSearchOpen(false);
    };
    window.addEventListener('nw:navigate', onNavigate);
    return () => window.removeEventListener('nw:navigate', onNavigate);
  }, []);

  // ⌘K / Ctrl+K 打开快速搜索（常见文档站的习惯）。
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && !event.altKey && event.key.toLowerCase() === 'k') { event.preventDefault(); setSearchOpen(true); }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);

  // 语言建议：决策模型判断的用居中弹窗，规则判断的用页首提示条。
  useEffect(() => {
    void suggestLanguage(initialConfig).then((result) => {
      if (result?.reason === 'model') setSuggested(result.language);
      else if (result) languageHint(result.language);
    });
  }, []);

  // 页头的“文档树”按钮由服务端输出；指针移上去或按下时就开始预取抽屉代码。
  useEffect(() => {
    const open = () => setNavOpen(true);
    const preload = () => { void loadNav(); };
    const intents = ['pointerenter', 'pointerdown', 'focus'];
    const buttons = [...document.querySelectorAll('[data-open-nav]')];
    for (const button of buttons) {
      button.addEventListener('click', open);
      intents.forEach((name) => button.addEventListener(name, preload));
    }
    return () => {
      for (const button of buttons) {
        button.removeEventListener('click', open);
        intents.forEach((name) => button.removeEventListener(name, preload));
      }
    };
  }, []);

  // 首页主视觉在问答没开启时放一个搜索框（服务端输出），点开 ⌘K 搜索，指针移上去就预取搜索代码；非 Mac 上快捷键提示改成 Ctrl K。
  useEffect(() => {
    if (!/Mac|iPhone|iPad/.test(navigator.platform)) document.querySelectorAll('[data-shortcut]').forEach((node) => { node.textContent = 'Ctrl K'; });
    const search = () => setSearchOpen(true);
    const preloadSearch = () => { void loadSearch(); };
    const bindings: [Element, string, EventListener][] = [];
    for (const node of document.querySelectorAll('[data-open-search]')) bindings.push([node, 'click', search], [node, 'pointerenter', preloadSearch], [node, 'focus', preloadSearch]);
    bindings.forEach(([node, name, listener]) => node.addEventListener(name, listener));
    return () => bindings.forEach(([node, name, listener]) => node.removeEventListener(name, listener));
  }, []);

  const onAsk = () => {
    if (ask) ask.querySelector('textarea')?.focus();
    else setAskOpen(true);
  };

  return (
    <I18nProvider locale={config.locale || document.documentElement.lang}>
      {header && createPortal(<HeaderActions config={config} t={t} onAsk={onAsk} onAskIntent={ask ? undefined : preloadQa} onSearch={() => setSearchOpen(true)} onSearchIntent={() => { void loadSearch(); }} />, header)}
      {searchUsed && <Suspense fallback={null}><Search deep={config.aiEnabled} locale={config.locale || document.documentElement.lang} open={searchOpen} t={t} onOpenChange={setSearchOpen} /></Suspense>}
      {navUsed && <Suspense fallback={null}><MobileNav open={navOpen} t={t} onOpenChange={setNavOpen} /></Suspense>}
      {config.aiEnabled && !ask && config.kind !== 'home' && <QaFloating documentId={config.documentId || undefined} locale={config.locale} open={askOpen} pageTitle={pageTitle(config)} t={t} onOpenChange={setAskOpen} />}
      {config.aiEnabled && ask && createPortal(<QaInline locale={config.locale} t={t} />, ask)}
      {config.aiEnabled && homeAsk && createPortal(<QaHome locale={config.locale} t={t} welcome={config.welcome} />, homeAsk)}
      {suggested && <LanguageSuggestion config={config} t={t} target={suggested} onClose={() => setSuggested(null)} />}
    </I18nProvider>
  );
}

const header = document.getElementById('nw-header');
const ask = document.getElementById('nw-ask');
const homeAsk = document.getElementById('nw-home-ask');
// 服务端放在这几处的是无脚本时的退路内容，由 React 接管后清掉。
header?.replaceChildren();
ask?.replaceChildren();
homeAsk?.replaceChildren();
const host = document.createElement('div');
host.id = 'nw-qa-root';
document.body.append(host);
createRoot(host).render(<Islands ask={ask} header={header} homeAsk={homeAsk} />);
enhance(t);
installExternalLinks();
startNavigation(t);
