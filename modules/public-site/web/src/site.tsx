import { I18nProvider } from '@heroui/react';
import { preloadQa, QaFloating, QaInline } from '@modules/qa/web/src/island';
import { lazy, Suspense, useEffect, useState } from 'react';
import { createPortal } from 'react-dom';
import { createRoot } from 'react-dom/client';
import { enhance } from './enhance';
import { HeaderActions, type Config } from './islands';
import './site.css';

// 公开页的交互岛：页面由 Go 完整输出，这里用一个 React 根，通过 portal 接管页头操作区与问答区，
// 页头、文档树抽屉、问答面板之间因此能共享状态（比如页头按钮直接打开问答面板）。
// 抽屉类部件（问答面板、窄屏文档树）第一次打开时才下载，首屏只带页头按钮和菜单。

const config: Config = (() => {
  try { return JSON.parse(document.getElementById('nw-islands')?.dataset.config ?? '{}') as Config; } catch { return {} as Config; }
})();
const t = (key: string) => config.ui?.[key] ?? key;
const loadNav = () => import('./nav-drawer');
const MobileNav = lazy(loadNav);

function Islands({ header, ask }: { header: HTMLElement | null; ask: HTMLElement | null }) {
  const [askOpen, setAskOpen] = useState(false);
  const [navOpen, setNavOpen] = useState(false);
  const [navUsed, setNavUsed] = useState(false);
  if (navOpen && !navUsed) setNavUsed(true);

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

  const onAsk = () => {
    if (ask) ask.querySelector('textarea')?.focus();
    else setAskOpen(true);
  };

  return (
    <I18nProvider locale={config.locale || document.documentElement.lang}>
      {header && createPortal(<HeaderActions config={config} t={t} onAsk={onAsk} onAskIntent={ask ? undefined : preloadQa} />, header)}
      {navUsed && <Suspense fallback={null}><MobileNav open={navOpen} t={t} onOpenChange={setNavOpen} /></Suspense>}
      {config.aiEnabled && !ask && <QaFloating locale={config.locale} open={askOpen} t={t} onOpenChange={setAskOpen} />}
      {config.aiEnabled && ask && createPortal(<QaInline locale={config.locale} t={t} />, ask)}
    </I18nProvider>
  );
}

const header = document.getElementById('nw-header');
const ask = document.getElementById('nw-ask');
// 服务端放在这两处的是无脚本时的退路内容，由 React 接管后清掉。
header?.replaceChildren();
ask?.replaceChildren();
const host = document.createElement('div');
host.id = 'nw-qa-root';
document.body.append(host);
createRoot(host).render(<Islands ask={ask} header={header} />);
enhance(config, t);
