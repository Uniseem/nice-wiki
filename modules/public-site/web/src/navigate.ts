import { enhancePage } from './enhance';
import type { Config, Translate } from './islands';

// 站内无刷新跳转：文档页之间点链接时只取新页面的 HTML，替换正文、本页目录、页面标题和交互岛配置；
// 页头、文档树（保留展开状态和滚动位置）、问答面板（对话不中断）都留在原地，新正文淡入。
// 指针移到链接上、手指按下时就开始预取，空闲时预取上一篇/下一篇；取过的页面在内存里缓存 5 分钟。
// 目标不是同语言的文档页（首页、问答页、换语言）、站点刚发了新版本或取不到时，照常整页打开。

type Page = { doc: Document; url: string };

const lifetime = 5 * 60_000;
const pages = new Map<string, { at: number; page: Promise<Page | null> }>();

const pathOf = (url: URL) => url.pathname + url.search;

function load(url: URL): Promise<Page | null> {
  const key = pathOf(url);
  const cached = pages.get(key);
  if (cached && Date.now() - cached.at < lifetime) return cached.page;
  const page = fetch(key, { headers: { Accept: 'text/html' } })
    .then(async (response) => {
      if (!response.ok || !(response.headers.get('content-type') ?? '').includes('text/html')) return null;
      return { doc: new DOMParser().parseFromString(await response.text(), 'text/html'), url: response.url };
    })
    .catch(() => null);
  pages.delete(key);
  pages.set(key, { at: Date.now(), page });
  if (pages.size > 40) pages.delete(pages.keys().next().value!);
  void page.then((result) => { if (!result) pages.delete(key); });
  return page;
}

// 能无刷新打开的链接：本站页面、不是静态文件和接口、不是新窗口/下载/换语言、不是同一页内的锚点。
function target(link: HTMLAnchorElement): URL | null {
  if ((link.target && link.target !== '_self') || link.hasAttribute('download') || link.hasAttribute('hreflang') || link.hasAttribute('data-markdown')) return null;
  const url = new URL(link.href, location.href);
  if (url.origin !== location.origin || /^\/(_site|v1|admin)(\/|$)/.test(url.pathname) || /\.[a-z0-9]{1,5}$/i.test(url.pathname)) return null;
  if (pathOf(url) === location.pathname + location.search) return null;
  return url;
}

// 新页面要和当前页同语言、同一版前端资源、同样是文档页，才能只换正文。
function compatible(doc: Document): boolean {
  const sheet = (d: Document) => d.querySelector<HTMLLinkElement>('link[rel="stylesheet"]')?.getAttribute('href');
  return doc.body.dataset.page === 'doc' && doc.body.dataset.locale === document.body.dataset.locale && sheet(doc) === sheet(document) && !!doc.getElementById('nw-main') && !!doc.getElementById('nw-toc');
}

const headSelector = 'meta[name="description"], link[rel="canonical"], link[rel="alternate"], meta[property^="og:"], meta[name^="twitter:"], script[type="application/ld+json"]';

function swap(doc: Document) {
  document.title = doc.title;
  document.head.querySelectorAll(headSelector).forEach((node) => node.remove());
  const anchor = document.head.querySelector('script');
  doc.head.querySelectorAll(headSelector).forEach((node) => document.head.insertBefore(document.importNode(node, true), anchor));
  // 只有部分页面带的样式（比如代码高亮）缺了就补上。
  const styles = new Set(Array.from(document.head.querySelectorAll('style'), (style) => style.textContent));
  doc.head.querySelectorAll('style').forEach((style) => { if (!styles.has(style.textContent)) document.head.append(document.importNode(style, true)); });

  for (const id of ['nw-main', 'nw-toc']) {
    const next = doc.getElementById(id);
    const current = document.getElementById(id);
    if (!next || !current) continue;
    const node = document.importNode(next, true) as HTMLElement;
    node.classList.add('nw-enter');
    current.replaceWith(node);
  }
  document.body.dataset.doc = doc.body.dataset.doc ?? '';

  // 文档树只改当前项：保留访客展开的分组和左栏滚动位置，当前项所在的分组展开。
  const tree = document.getElementById('nw-tree');
  const href = doc.querySelector('#nw-tree [aria-current="page"]')?.getAttribute('href');
  if (tree) {
    tree.querySelectorAll('.nw-current').forEach((node) => node.classList.remove('nw-current'));
    tree.querySelectorAll('[aria-current]').forEach((node) => node.removeAttribute('aria-current'));
    const link = href ? Array.from(tree.querySelectorAll('a')).find((node) => node.getAttribute('href') === href) : undefined;
    if (link) {
      link.setAttribute('aria-current', 'page');
      link.closest('.nw-tree-item')?.classList.add('nw-current');
      for (let group = link.closest('details'); group; group = group.parentElement?.closest('details') ?? null) group.open = true;
    }
  }

  // 交互岛（页头语言菜单、问答面板的当前页面等）换成新页面的配置。
  try {
    const config = JSON.parse(doc.getElementById('nw-islands')?.dataset.config ?? '{}') as Config;
    window.dispatchEvent(new CustomEvent<Config>('nw:navigate', { detail: config }));
  } catch { /* 配置坏了就保留原样 */ }
}

let sequence = 0;
let progress: HTMLElement | null = null;
let translate: Translate = (key) => key;
// 当前显示的是哪一页（路径加查询串）：返回、前进只改了锚点时不用换页。
let current = location.pathname + location.search;

async function go(url: URL, push: boolean, scroll?: number) {
  const id = ++sequence;
  const started = performance.now();
  const referrer = location.hostname;
  // 预取过的页面直接换；要等网络时，稍等一下仍没好才显示顶部进度条。
  const timer = window.setTimeout(() => {
    progress?.remove();
    progress = document.createElement('div');
    progress.className = 'nw-progress';
    document.body.append(progress);
  }, 120);
  const page = await load(url);
  window.clearTimeout(timer);
  progress?.remove();
  progress = null;
  if (id !== sequence) return;
  if (!page || !compatible(page.doc)) {
    if (push) location.assign(url.href);
    else location.reload();
    return;
  }
  if (push) {
    history.scrollRestoration = 'manual';
    history.replaceState({ ...history.state, scroll: window.scrollY }, '');
    const final = new URL(page.url);
    final.hash = url.hash;
    history.pushState({ scroll: 0 }, '', final.href);
  }
  swap(page.doc);
  current = location.pathname + location.search;
  const anchor = url.hash ? document.getElementById(decodeURIComponent(url.hash.slice(1))) : null;
  if (anchor) anchor.scrollIntoView({ behavior: 'instant' });
  else window.scrollTo({ top: scroll ?? 0, behavior: 'instant' });
  const heading = document.querySelector<HTMLElement>('#nw-main h1');
  if (heading) {
    heading.tabIndex = -1;
    heading.focus({ preventScroll: true });
  }
  enhancePage(translate, { referrer, elapsedMs: performance.now() - started });
}

// 用程序打开站内页面（比如搜索结果按回车）：能无刷新就无刷新。
export function visit(href: string) {
  const link = document.createElement('a');
  link.href = href;
  const url = enabled ? target(link) : null;
  if (url) void go(url, true);
  else location.href = href;
}

let enabled = false;

export function startNavigation(t: Translate) {
  if (document.body.dataset.page !== 'doc' || !('DOMParser' in window)) return;
  enabled = true;
  translate = t;

  document.addEventListener('click', (event) => {
    if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    const link = (event.target as Element | null)?.closest?.('a[href]') as HTMLAnchorElement | null;
    const url = link ? target(link) : null;
    if (!url) return;
    event.preventDefault();
    void go(url, true);
  });

  // 返回、前进：同一页里的锚点跳转交给浏览器，换页时恢复离开时的滚动位置。
  window.addEventListener('popstate', (event) => {
    if (location.pathname + location.search === current) return;
    void go(new URL(location.href), false, (event.state as { scroll?: number } | null)?.scroll);
  });

  // 预取：指针在链接上停一下、手指按下或键盘聚焦时开始取。
  let hovered: Element | null = null;
  let timer = 0;
  const intent = (delay: number) => (event: Event) => {
    const link = (event.target as Element | null)?.closest?.('a[href]') as HTMLAnchorElement | null;
    if (!link || link === hovered) return;
    hovered = link;
    window.clearTimeout(timer);
    const url = target(link);
    if (url) timer = window.setTimeout(() => { void load(url); }, delay);
  };
  document.addEventListener('pointerover', intent(60), { passive: true });
  document.addEventListener('touchstart', intent(0), { passive: true });
  document.addEventListener('focusin', intent(0));

  // 空闲时预取上一篇、下一篇。
  const idle = (run: () => void) => (typeof requestIdleCallback === 'function' ? requestIdleCallback(run) : setTimeout(run, 1500));
  const neighbours = () => idle(() => document.querySelectorAll<HTMLAnchorElement>('a[data-prefetch]').forEach((link) => { const url = target(link); if (url) void load(url); }));
  neighbours();
  window.addEventListener('nw:navigate', neighbours);
}
