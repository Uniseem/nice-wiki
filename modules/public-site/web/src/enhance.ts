import type { Config, Language, Translate } from './islands';

// 不需要 React 的小增强：都作用在服务端已经输出的 HTML 上，脚本失败也不影响阅读。

const copyIcon = '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round"><rect x="5.25" y="5.25" width="8.5" height="8.5" rx="1.75"/><path d="M10.75 5.25V3.5a1.75 1.75 0 0 0-1.75-1.75H3.5A1.75 1.75 0 0 0 1.75 3.5V9c0 .97.78 1.75 1.75 1.75h1.75"/></svg>';
const checkIcon = '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="m3 8.5 3.2 3L13 4.5"/></svg>';

// 代码块右上角的复制按钮。
function codeCopy(t: Translate) {
  document.querySelectorAll<HTMLElement>('#nw-content .code-block').forEach((block) => {
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'nw-copy button button--tertiary button--sm button--icon-only';
    button.setAttribute('aria-label', t('复制代码'));
    button.innerHTML = copyIcon;
    button.addEventListener('click', async () => {
      try {
        await navigator.clipboard.writeText(block.querySelector('pre')?.innerText ?? '');
        button.innerHTML = checkIcon;
        button.setAttribute('data-copied', '');
        button.setAttribute('aria-label', t('已复制'));
        setTimeout(() => { button.innerHTML = copyIcon; button.removeAttribute('data-copied'); button.setAttribute('aria-label', t('复制代码')); }, 2000);
      } catch { /* 剪贴板不可用时什么也不做 */ }
    });
    block.append(button);
  });
}

// 标题旁的锚点，方便复制某一节的链接。
function headingAnchors() {
  document.querySelectorAll<HTMLElement>('#nw-content :is(h2, h3, h4)[id]').forEach((heading) => {
    const anchor = document.createElement('a');
    anchor.className = 'nw-anchor';
    anchor.href = `#${heading.id}`;
    anchor.textContent = '#';
    anchor.setAttribute('aria-hidden', 'true');
    anchor.tabIndex = -1;
    heading.append(anchor);
  });
}

// 本页目录：按滚动位置高亮当前小节，玻璃高亮块滑到当前项；目录比屏幕长时，目录自己滚动让当前项保持在中间附近
// （只滚目录这一栏，不动页面）。返回清理函数，站内跳转换页时先清掉上一页的监听。
function tocSpy(): () => void {
  const nav = document.querySelector<HTMLElement>('[data-toc]');
  const links = nav ? Array.from(nav.querySelectorAll<HTMLAnchorElement>('a[href^="#"]')) : [];
  if (!nav || !links.length) return () => {};
  const pill = nav.querySelector<HTMLElement>('.nw-toc-pill');
  const pairs = links.map((link) => ({ link, target: document.getElementById(decodeURIComponent(link.hash.slice(1))) })).filter((pair): pair is { link: HTMLAnchorElement; target: HTMLElement } => !!pair.target);
  let current: HTMLAnchorElement | undefined;
  let frame = 0;
  const update = (force = false) => {
    frame = 0;
    // 当前小节：标题已经滚到页头下方的最后一节；滚到页面底部时取最后一节。
    const atEnd = window.innerHeight + window.scrollY >= document.documentElement.scrollHeight - 4;
    let active = atEnd ? pairs[pairs.length - 1] : pairs[0];
    if (!atEnd) for (const pair of pairs) { if (pair.target.getBoundingClientRect().top <= 96) active = pair; else break; }
    if (!active || (active.link === current && !force)) return;
    current = active.link;
    links.forEach((link) => link.toggleAttribute('data-active', link === current));
    if (pill) {
      pill.style.transform = `translateY(${current.offsetTop}px)`;
      pill.style.height = `${current.offsetHeight}px`;
      pill.style.insetInlineStart = getComputedStyle(current).marginInlineStart;
      pill.style.opacity = '1';
    }
    const top = current.offsetTop;
    const bottom = top + current.offsetHeight;
    if (top < nav.scrollTop + 32 || bottom > nav.scrollTop + nav.clientHeight - 32) {
      nav.scrollTo({ top: top - (nav.clientHeight - current.offsetHeight) / 2, behavior: force ? 'instant' : 'smooth' });
    }
  };
  const schedule = () => { if (!frame) frame = requestAnimationFrame(() => update()); };
  const resize = () => update(true);
  // 第一次定位不要动画，高亮块直接出现在当前项上。
  if (pill) pill.style.transition = 'none';
  update(true);
  requestAnimationFrame(() => { if (pill) pill.style.transition = ''; });
  window.addEventListener('scroll', schedule, { passive: true });
  window.addEventListener('resize', resize);
  return () => {
    cancelAnimationFrame(frame);
    window.removeEventListener('scroll', schedule);
    window.removeEventListener('resize', resize);
  };
}

// 文档树里的当前项不在左栏可视范围内时，把左栏滚到它（只滚左栏）。
export function revealCurrent(behavior: ScrollBehavior = 'instant') {
  const item = document.querySelector<HTMLElement>('#nw-tree .nw-current');
  const box = item?.closest<HTMLElement>('.nw-sidebar');
  if (!item || !box) return;
  const top = item.getBoundingClientRect().top - box.getBoundingClientRect().top + box.scrollTop;
  if (top < box.scrollTop + 16 || top + item.offsetHeight > box.scrollTop + box.clientHeight - 16) {
    box.scrollTo({ top: top - box.clientHeight / 3, behavior });
  }
}

// 复制整页 Markdown。
function copyMarkdown(t: Translate) {
  const button = document.querySelector<HTMLButtonElement>('[data-copy-markdown]');
  const link = document.querySelector<HTMLAnchorElement>('[data-markdown]');
  if (!button || !link) return;
  const label = button.querySelector('span');
  button.addEventListener('click', async () => {
    try {
      const response = await fetch(link.href, { cache: 'no-store' });
      if (!response.ok) throw new Error();
      await navigator.clipboard.writeText(await response.text());
      if (label) label.textContent = t('已复制');
      setTimeout(() => { if (label) label.textContent = t('复制为 Markdown'); }, 2000);
    } catch {
      if (label) label.textContent = t('复制失败，请打开 Markdown 版本');
    }
  });
}

// 公告横幅的关闭按钮：关闭状态只记在本机。
function announcement() {
  document.querySelectorAll<HTMLElement>('[data-dismiss-announcement]').forEach((button) => {
    button.addEventListener('click', () => {
      const bar = button.closest<HTMLElement>('[data-announcement]');
      if (!bar) return;
      bar.hidden = true;
      try { localStorage.setItem(`announcement:${bar.dataset.announcement}`, 'closed'); } catch { /* 隐私模式 */ }
    });
  });
}

// 平台视频：点击后才加载第三方播放器。
function embeds(t: Translate) {
  document.querySelectorAll<HTMLButtonElement>('[data-embed]').forEach((button) => {
    button.className = 'button button--secondary button--sm';
    button.addEventListener('click', () => {
      const frame = document.createElement('iframe');
      frame.src = button.dataset.embed ?? '';
      frame.title = button.dataset.title || t('视频播放器');
      frame.allowFullscreen = true;
      frame.referrerPolicy = 'no-referrer';
      frame.setAttribute('sandbox', 'allow-scripts allow-same-origin allow-presentation');
      button.closest('figure')?.querySelector('img')?.remove();
      button.replaceWith(frame);
    });
  });
}

// 公式：页面里有公式时才加载 KaTeX（整个标签页只加载一次）；失败时保留 LaTeX 源码。
type Katex = { render(source: string, element: HTMLElement, options: object): void };
let katex: Promise<Katex | undefined> | undefined;
function math() {
  const nodes = document.querySelectorAll<HTMLElement>('.math-source');
  if (!nodes.length) return;
  katex ??= new Promise((resolve) => {
    const style = document.createElement('link');
    style.rel = 'stylesheet';
    style.href = '/_site/katex/katex.min.css';
    document.head.append(style);
    const script = document.createElement('script');
    script.src = '/_site/katex/katex.min.js';
    script.onload = () => resolve((window as unknown as { katex?: Katex }).katex);
    script.onerror = () => resolve(undefined);
    document.head.append(script);
  });
  void katex.then((engine) => {
    nodes.forEach((node) => { try { engine?.render(node.textContent ?? '', node, { displayMode: node.tagName === 'DIV', throwOnError: false }); } catch { /* 保留源码 */ } });
  });
}

// 访问统计信标：只报文档、语言、来源域名，不报完整地址。站内无刷新跳转时来源是本站，耗时是这次换页用的时间。
function beacon(from?: { referrer: string; elapsedMs: number }) {
  const documentId = document.body.dataset.doc;
  if (!documentId) return;
  let referrer = from?.referrer ?? '';
  if (!from) try { referrer = new URL(document.referrer).hostname; } catch { /* 无来源 */ }
  if (/^[0-9.]+$/.test(referrer) || referrer.includes(':')) referrer = '';
  const payload = {
    beaconId: crypto.randomUUID().split('-').join(''), documentId, locale: document.body.dataset.locale, eventType: 'page-view', status: 200,
    elapsedMs: Math.min(60000, Math.round(from?.elapsedMs ?? performance.now())), referrerDomain: referrer, userAgent: navigator.userAgent.slice(0, 1000),
  };
  fetch('/v1/beacons', { method: 'POST', keepalive: true, headers: { 'Content-Type': 'application/json', 'Idempotency-Key': payload.beaconId }, body: JSON.stringify(payload) }).catch(() => {});
}

// 访客自己选的语言记在 lang cookie（一年）：之后打开根路径按它跳转，不再按浏览器语言。
export function rememberLanguage(tag: string) {
  document.cookie = `lang=${encodeURIComponent(tag)}; path=/; max-age=31536000; samesite=lax`;
}

type Suggestion = { language: Language; reason: string };

// 问服务端要语言建议：访客选过语言（有 lang cookie）、本会话关过提示或站点只有一种语言时不问；结果在本会话内复用。
export async function suggestLanguage(config: Config): Promise<Suggestion | null> {
  const languages = config.languages ?? [];
  if (languages.length < 2 || /(?:^|;\s*)lang=/.test(document.cookie)) return null;
  const key = `nw-lang-suggest:${config.locale}`;
  let result: { suggestedLanguage?: string; reason: string } | null = null;
  try {
    if (sessionStorage.getItem('nw-lang-hint')) return null;
    result = JSON.parse(sessionStorage.getItem(key) ?? 'null');
  } catch { return null; }
  if (!result) {
    try {
      const response = await fetch('/v1/language-suggestions', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          browserLanguages: [...(navigator.languages ?? [navigator.language])].slice(0, 10),
          timeZone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC', currentLanguage: config.locale, previouslyRejected: false,
        }),
      });
      if (!response.ok) return null;
      result = await response.json();
      try { sessionStorage.setItem(key, JSON.stringify(result)); } catch { /* 隐私模式 */ }
    } catch { return null; }
  }
  const wanted = result?.suggestedLanguage?.toLowerCase();
  const target = languages.find((language) => !language.current && language.tag.toLowerCase() === wanted);
  return target && result ? { language: target, reason: result.reason } : null;
}

// 规则给出的建议：页首一条可关闭的提示，绝不自动跳转。
export function languageHint(target: Language) {
  let name = target.label;
  try { name = new Intl.DisplayNames([target.tag], { type: 'language' }).of(target.tag) ?? name; } catch { /* 用服务端名字 */ }
  const bar = document.createElement('div');
  bar.setAttribute('role', 'note');
  bar.className = 'flex items-center justify-center gap-3 border-b border-separator bg-surface-secondary px-4 py-2 text-sm';
  // 用目标语言自己的写法显示语言名，偏好该语言的访客一眼能认出来。
  const link = document.createElement('a');
  link.href = target.href;
  link.lang = target.tag;
  link.addEventListener('click', () => rememberLanguage(target.tag));
  link.className = 'inline-flex items-center gap-1.5 font-medium text-accent hover:underline';
  link.innerHTML = '<svg aria-hidden="true" class="size-4" fill="none" stroke="currentColor" stroke-width="1.6" viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3c2.5 2.7 3.8 5.7 3.8 9s-1.3 6.3-3.8 9c-2.5-2.7-3.8-5.7-3.8-9S9.5 5.7 12 3z"/></svg>';
  link.append(`${name} →`);
  const close = document.createElement('button');
  close.type = 'button';
  close.className = 'button button--ghost button--sm button--icon-only';
  close.setAttribute('aria-label', 'Close');
  close.textContent = '×';
  close.addEventListener('click', () => { try { sessionStorage.setItem('nw-lang-hint', '1'); } catch { /* 隐私模式 */ } bar.remove(); });
  bar.append(link, close);
  document.body.prepend(bar);
}

// 每一页正文上的增强：首次加载和站内无刷新跳转换页后都要跑一遍。
let stopTocSpy = () => {};
export function enhancePage(t: Translate, from?: { referrer: string; elapsedMs: number }) {
  stopTocSpy();
  codeCopy(t);
  headingAnchors();
  stopTocSpy = tocSpy();
  copyMarkdown(t);
  embeds(t);
  math();
  revealCurrent(from ? 'smooth' : 'instant');
  beacon(from);
}

export function enhance(t: Translate) {
  announcement();
  enhancePage(t);
}
