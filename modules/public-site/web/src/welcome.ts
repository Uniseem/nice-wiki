import { preloadQa } from '@modules/qa/web/src/island';
import { bindBackHome, showAskBackdrop } from './backdrop';
import { enhancePage } from './enhance';
import { forgetEntry, startLetterField, type LetterField } from './field';
import type { Translate } from './islands';
import { swapHead } from './navigate';

// 欢迎字按实际译文宽度缩放。首页本身不铺背景；两种入口各有退场：
// 进 AI 问答时字母点阵从点击处向外铺开、波前扫过处字母快速翻动，首页内容放大虚化散去，然后页内换成问答页并升起磨砂玻璃——
// 不整页跳转，点阵画布和绘制线程一直不停，所以整段动画不会因为换页卡一下；
// 进文档时内容轻轻上移淡出，稳重利落，再整页打开。普通链接行为（新标签打开等）保持可用。

// 预取问答页：指针移上去、按下或获得焦点时就开始取，点下去时通常已经到手。
let askPage: Promise<Document | null> | null = null;
const fetchAsk = (href: string) => askPage ??= fetch(href, { headers: { Accept: 'text/html' } })
  .then(async (response) => response.ok && (response.headers.get('content-type') ?? '').includes('text/html') ? new DOMParser().parseFromString(await response.text(), 'text/html') : null)
  .catch(() => null);

// 页内换成问答页：标题和元信息、页头、主体、交互岛配置都换成问答页的，地址改成问答页。
// 不是同一版前端或不是问答页时返回 false，退回整页跳转。
function swapToAsk(doc: Document, href: string, t: Translate, started: number): boolean {
  const sheet = (page: Document) => page.querySelector('link[rel="stylesheet"]')?.getAttribute('href');
  if (doc.body.dataset.page !== 'ask' || sheet(doc) !== sheet(document) || !doc.getElementById('nw-ask')) return false;
  const referrer = location.hostname;
  swapHead(doc);
  for (const selector of ['header.nw-site-header', '#nw-main', '#nw-islands']) document.querySelector(selector)?.replaceWith(document.importNode(doc.querySelector(selector)!, true));
  document.body.dataset.page = 'ask';
  document.body.dataset.doc = doc.body.dataset.doc ?? '';
  history.pushState({}, '', href);
  // 在问答页按浏览器返回：整页加载首页，最稳。
  window.addEventListener('popstate', () => location.reload(), { once: true });
  forgetEntry();
  try { window.dispatchEvent(new CustomEvent('nw:navigate', { detail: JSON.parse(document.getElementById('nw-islands')?.dataset.config ?? '{}') })); } catch { /* 配置坏了就保留原样 */ }
  // 交互岛接管新的页头和问答区（见 site.tsx）。
  window.dispatchEvent(new Event('nw:page'));
  // 站标原地不动，页头其余部分和站名淡入。
  document.querySelectorAll('.nw-header-inner > :not(.nw-brand), .nw-brand > span').forEach((node, index) => node.animate([{ opacity: 0 }, { opacity: 1 }], { duration: 420, delay: 60 + index * 40, easing: 'cubic-bezier(.2,.8,.2,1)', fill: 'backwards' }));
  showAskBackdrop(true);
  bindBackHome();
  enhancePage(t, { referrer, elapsedMs: performance.now() - started });
  return true;
}

export function enhanceWelcome(t: Translate) {
  const welcome = document.querySelector<HTMLElement>('.nw-welcome');
  const outline = welcome?.querySelector<HTMLElement>('.nw-welcome-outline');
  const text = outline?.querySelector('span');
  if (!welcome || !outline || !text) return;
  const fit = () => {
    text.style.fontSize = '100px';
    text.style.fontSize = `${Math.min(260, welcome.clientHeight * .52, Math.max(32, outline.clientWidth / Math.max(1, text.scrollWidth) * 100))}px`;
  };
  const observer = new ResizeObserver(fit);
  observer.observe(outline);
  void document.fonts.ready.then(fit);
  let field: LetterField | null = null;
  let leaving = false;
  welcome.querySelectorAll<HTMLAnchorElement>('[data-home-choice]').forEach((link) => {
    const agent = link.dataset.homeChoice === 'agent';
    if (agent) {
      const prefetch = () => { preloadQa(); void fetchAsk(link.href); };
      for (const name of ['pointerenter', 'pointerdown', 'focus']) link.addEventListener(name, prefetch, { once: true });
    }
    link.addEventListener('click', (event) => {
      if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey || event.defaultPrevented) return;
      event.preventDefault();
      if (leaving) return;
      leaving = true;
      if (matchMedia('(prefers-reduced-motion: reduce)').matches) { location.assign(link.href); return; }
      welcome.dataset.leaving = link.dataset.homeChoice;
      // 键盘回车触发时没有指针位置，从入口中心铺开。
      const box = link.getBoundingClientRect();
      const x = event.detail ? event.clientX : box.left + box.width / 2, y = event.detail ? event.clientY : box.top + box.height / 2;
      const started = performance.now();
      const page = agent ? fetchAsk(link.href) : Promise.resolve(null);
      if (agent) {
        field ??= startLetterField(0);
        field.enter(x, y);
      }
      const animations = [...welcome.children].map((node, index) => node.animate(agent
        ? [{ opacity: 1, filter: 'blur(0)', scale: '1' }, { opacity: 0, filter: 'blur(10px)', scale: '1.06' }]
        : [{ opacity: 1, translate: '0 0' }, { opacity: 0, translate: '0 -10px' }],
      { duration: agent ? 520 : 300, delay: index * (agent ? 50 : 25), easing: 'cubic-bezier(.4,0,1,1)', fill: 'forwards' }));
      void Promise.all(animations.map((animation) => animation.finished)).then(() => page).then((doc) => {
        if (!doc || !swapToAsk(doc, link.href, t, started)) location.assign(link.href);
      });
    });
  });
  // 浏览器返回命中页面缓存时，清掉已经完成的退场动画和点阵。
  window.addEventListener('pageshow', (event) => {
    if (!event.persisted) return;
    leaving = false;
    delete welcome.dataset.leaving;
    welcome.getAnimations({ subtree: true }).forEach((animation) => animation.cancel());
    void field?.fade(0, 300);
  });
}
