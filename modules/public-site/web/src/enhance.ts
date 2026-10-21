import type { Config, Translate } from './islands';

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

// 本页目录：按滚动位置高亮当前小节。
function tocSpy() {
  const links = Array.from(document.querySelectorAll<HTMLAnchorElement>('[data-toc] a[href^="#"]'));
  if (!links.length || !('IntersectionObserver' in window)) return;
  const targets = links.map((link) => document.getElementById(decodeURIComponent(link.hash.slice(1)))).filter((el): el is HTMLElement => !!el);
  const visible = new Set<Element>();
  const update = () => {
    const first = targets.find((target) => visible.has(target)) ?? [...targets].reverse().find((target) => target.getBoundingClientRect().top < 120);
    links.forEach((link) => link.toggleAttribute('data-active', !!first && decodeURIComponent(link.hash.slice(1)) === first.id));
  };
  const observer = new IntersectionObserver((entries) => {
    for (const entry of entries) {
      if (entry.isIntersecting) visible.add(entry.target);
      else visible.delete(entry.target);
    }
    update();
  }, { rootMargin: '-72px 0px -60% 0px' });
  targets.forEach((target) => observer.observe(target));
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

// 公式：页面里有公式时才加载 KaTeX；失败时保留 LaTeX 源码。
function math() {
  const nodes = document.querySelectorAll<HTMLElement>('.math-source');
  if (!nodes.length) return;
  const style = document.createElement('link');
  style.rel = 'stylesheet';
  style.href = '/_site/katex/katex.min.css';
  document.head.append(style);
  const script = document.createElement('script');
  script.src = '/_site/katex/katex.min.js';
  script.onload = () => {
    const katex = (window as unknown as { katex?: { render(source: string, element: HTMLElement, options: object): void } }).katex;
    nodes.forEach((node) => { try { katex?.render(node.textContent ?? '', node, { displayMode: node.tagName === 'DIV', throwOnError: false }); } catch { /* 保留源码 */ } });
  };
  document.head.append(script);
}

// 访问统计信标：只报文档、语言、来源域名，不报完整地址。
function beacon() {
  const documentId = document.body.dataset.doc;
  if (!documentId) return;
  let referrer = '';
  try { referrer = new URL(document.referrer).hostname; } catch { /* 无来源 */ }
  if (/^[0-9.]+$/.test(referrer) || referrer.includes(':')) referrer = '';
  const payload = {
    beaconId: crypto.randomUUID().split('-').join(''), documentId, locale: document.body.dataset.locale, eventType: 'page-view', status: 200,
    elapsedMs: Math.min(60000, Math.round(performance.now())), referrerDomain: referrer, userAgent: navigator.userAgent.slice(0, 1000),
  };
  fetch('/v1/beacons', { method: 'POST', keepalive: true, headers: { 'Content-Type': 'application/json', 'Idempotency-Key': payload.beaconId }, body: JSON.stringify(payload) }).catch(() => {});
}

// 语言建议（规则兜底）：浏览器偏好语言与本页不同、且本站启用了它时，页首给一个可关闭的提示，绝不自动跳转。
function languageHint(config: Config) {
  const languages = config.languages ?? [];
  if (languages.length < 2) return;
  try { if (sessionStorage.getItem('nw-lang-hint')) return; } catch { return; }
  const current = config.locale.toLowerCase();
  for (const preferred of (navigator.languages ?? [navigator.language]).map((item) => item.toLowerCase())) {
    const base = preferred.split('-')[0];
    if (current === preferred || current.split('-')[0] === base) return;
    const hit = languages.find((language) => language.tag.toLowerCase() === preferred || language.tag.toLowerCase().split('-')[0] === base);
    if (!hit) continue;
    let name = hit.label;
    try { name = new Intl.DisplayNames([hit.tag], { type: 'language' }).of(hit.tag) ?? name; } catch { /* 用服务端名字 */ }
    const bar = document.createElement('div');
    bar.setAttribute('role', 'note');
    bar.className = 'flex items-center justify-center gap-3 border-b border-separator bg-surface-secondary px-4 py-2 text-sm';
    const link = document.createElement('a');
    link.href = hit.href;
    link.className = 'link';
    link.textContent = `🌐 ${name}`;
    const close = document.createElement('button');
    close.type = 'button';
    close.className = 'button button--ghost button--sm button--icon-only';
    close.setAttribute('aria-label', 'Close');
    close.textContent = '×';
    close.addEventListener('click', () => { try { sessionStorage.setItem('nw-lang-hint', '1'); } catch { /* 隐私模式 */ } bar.remove(); });
    bar.append(link, close);
    document.body.prepend(bar);
    return;
  }
}

export function enhance(config: Config, t: Translate) {
  codeCopy(t);
  headingAnchors();
  tocSpy();
  copyMarkdown(t);
  announcement();
  embeds(t);
  math();
  beacon();
  languageHint(config);
}
