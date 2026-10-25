import ReactMarkdown, { type Components } from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { useEffect, useId, useMemo, useRef } from 'react';
import type { Citation } from './client';

// 回答用 react-markdown + remark-gfm 渲染（列表嵌套、列表里的代码块、表格、删除线等都按 CommonMark/GFM 处理）。
// 不渲染 HTML、不用 innerHTML；链接只放行 http(s) 与站内相对地址，引用标识渲染成上标编号。

// 引用标识：模型按约定写 [[citation.…]]，也可能写成 [1](citation.…)、<sup>…</sup> 或裸标识，统一换成 [](cite:标识) 链接，由 a 组件渲染成编号。
const citationId = 'citation\\.[0-9a-f]{8,64}';
function markCitations(text: string): string {
  // 模型偶尔把点写成冒号；只统一标识写法，是否有真实来源仍由refs白名单决定。
  text = text.replace(/\bcitation[.:]([0-9a-f]{8,64})\b/gi, (_, hash: string) => `citation.${hash.toLowerCase()}`);
  // 逐字输出时，引用前缀也可能尚未写完，避免先露出[[citation再跳成编号。
  const start = text.lastIndexOf('[[');
  if (start >= 0) {
    const pending = text.slice(start + 2).toLowerCase();
    if ('citation.'.startsWith(pending) || 'citation:'.startsWith(pending) || /^citation[.:][0-9a-f]*$/.test(pending)) text = text.slice(0, start);
  }
  return text
    .replace(/<\/?sup>/gi, '')
    .replace(new RegExp(`\\[\\[?[^\\]\\n]*\\]?\\]\\((${citationId})\\)`, 'g'), '⟦$1⟧')
    .replace(new RegExp(`\\[\\[?(${citationId})\\]?\\]`, 'g'), '⟦$1⟧')
    .replace(new RegExp(`[(（](${citationId})[)）]`, 'g'), '⟦$1⟧')
    // 流式输出时末尾可能是写了一半的引用（标识是 64 位十六进制，不足的就是没写完），先不显示。
    .replace(/(\[\[?[^\]\n]*\]?\]?\(?)?citation[.:][0-9a-f]{0,63}$/, '')
    .replace(new RegExp(`(?<!⟦)(${citationId})(?!⟧)`, 'g'), '⟦$1⟧')
    .replace(/⟦(citation\.[0-9a-f]+)⟧/g, '[](cite:$1)');
}

function safeHref(href: string): string | null {
  return /^(https?:\/\/|\/(?!\/))/i.test(href) ? href : null;
}

// 只保留引用和安全链接，其余地址一律丢掉（链接文字照常显示）。
function urlTransform(url: string): string {
  return url.startsWith('#nw-reference-') ? url : safeHref(url) ?? '';
}

type MarkdownNode = {
  type: string;
  url?: string;
  value?: string;
  children?: MarkdownNode[];
  data?: { hProperties: Record<string, unknown> };
  ordered?: boolean;
  start?: number;
  spread?: boolean;
};

// 在 Markdown 语法树里统一正文与文末编号，代码块和行内代码中的示例不参与引用。
// 同一精确链接复用编号；不同段落锚点保留独立条目，避免把不同证据合并掉。
function referencePlugin(citations: Citation[], scope: string, label: string) {
  return () => (tree: MarkdownNode) => {
    const refs = new Map(citations.filter(item => item.citationId).map(item => [item.citationId!, item]));
    const entries = new Map<string, { number: number; citation: Citation }>();
    let hasMarkers = false;
    const entry = (citation: Citation) => {
      const href = citation.href && safeHref(citation.href);
      if (!href) return;
      if (!entries.has(href)) entries.set(href, { number: entries.size + 1, citation });
      return entries.get(href);
    };
    const prepare = (parent: MarkdownNode) => {
      parent.children = parent.children?.flatMap(node => {
        if (node.type === 'text') {
          return markCitations(node.value ?? '').split(/(\[\]\(cite:citation\.[0-9a-f]+\))/g).filter(Boolean).map(value => {
            const match = /^\[\]\((cite:citation\.[0-9a-f]+)\)$/.exec(value);
            return match ? { type: 'link', url: match[1], children: [] } : { type: 'text', value };
          });
        }
        if (node.type === 'link' && /^citation[.:][0-9a-f]{8,64}$/i.test(node.url ?? '')) {
          node.url = 'cite:' + node.url!.toLowerCase().replace('citation:', 'citation.');
        } else if (node.type !== 'link' && node.children) prepare(node);
        return [node];
      });
    };
    prepare(tree);
    const walk = (parent: MarkdownNode) => {
      parent.children = parent.children?.flatMap(node => {
        if (node.type === 'link' && node.url?.startsWith('cite:')) {
          hasMarkers = true;
          const citation = refs.get(node.url.slice(5));
          const ref = citation && entry(citation);
          if (!ref) return [];
          node.url = `#${scope}-${ref.number}`;
          node.children = [{ type: 'text', value: `[${ref.number}]` }];
          node.data = { hProperties: { 'data-citation-number': ref.number, title: ref.citation.title } };
        } else if (node.children) walk(node);
        return [node];
      });
    };
    walk(tree);
    // 兼容已有的只附来源列表、正文没有引用标记的历史回答。
    if (!hasMarkers) citations.forEach(entry);
    if (!entries.size) return;
    tree.children!.push({
      type: 'list', ordered: true, start: 1, spread: false,
      data: { hProperties: { className: ['nw-references'], 'aria-label': label } },
      children: [...entries].map(([href, { number, citation }]) => ({
        type: 'listItem', spread: false,
        data: { hProperties: { id: `${scope}-${number}`, className: ['nw-reference'], tabIndex: -1 } },
        children: [{ type: 'paragraph', children: [{
          type: 'link', url: href,
          data: { hProperties: { 'data-reference-link': true } },
          children: [{ type: 'text', value: citation.title || href }],
        }] }],
      })),
    });
  };
}

// 只滚动当前对话或弹窗的阅读区域，缓入缓出后再框出对应文献；滚轮、触摸和再次点击可中断。
function revealReference(target: HTMLElement, signal: AbortSignal) {
  let box = target.parentElement;
  while (box && !(/auto|scroll/.test(getComputedStyle(box).overflowY) && box.scrollHeight > box.clientHeight)) box = box.parentElement;
  const scrolling = box ?? document.scrollingElement as HTMLElement;
  const root = scrolling === document.scrollingElement;
  const start = scrolling.scrollTop;
  const destination = () => Math.max(0, Math.min(scrolling.scrollHeight - scrolling.clientHeight,
    scrolling.scrollTop + target.getBoundingClientRect().top - (root ? 0 : scrolling.getBoundingClientRect().top) - (root ? 88 : 20)));
  const duration = Math.min(900, 420 + Math.abs(destination() - start) * .25);
  let frame = 0;
  let highlight: Animation | undefined;
  const cancel = () => { cancelAnimationFrame(frame); highlight?.cancel(); clean(); };
  const clean = () => {
    scrolling.removeEventListener('wheel', cancel);
    scrolling.removeEventListener('touchstart', cancel);
    signal.removeEventListener('abort', cancel);
  };
  const finish = () => {
    clean();
    target.focus({ preventScroll: true });
    highlight = target.animate([
      { outlineColor: 'transparent', offset: 0 },
      { outlineColor: 'currentColor', offset: .15 },
      { outlineColor: 'currentColor', offset: .6 },
      { outlineColor: 'transparent', offset: 1 },
    ], { duration: 1600, easing: 'ease-out' });
    signal.addEventListener('abort', () => highlight?.cancel(), { once: true });
  };
  if (matchMedia('(prefers-reduced-motion: reduce)').matches) {
    scrolling.scrollTo({ top: destination(), behavior: 'instant' });
    finish();
    return;
  }
  scrolling.addEventListener('wheel', cancel, { passive: true, once: true });
  scrolling.addEventListener('touchstart', cancel, { passive: true, once: true });
  signal.addEventListener('abort', cancel, { once: true });
  const started = performance.now();
  const tick = (now: number) => {
    const progress = Math.min(1, (now - started) / duration);
    const eased = progress < .5 ? 4 * progress ** 3 : 1 - (-2 * progress + 2) ** 3 / 2;
    scrolling.scrollTo({ top: start + (destination() - start) * eased, behavior: 'instant' });
    if (progress < 1) frame = requestAnimationFrame(tick);
    else finish();
  };
  frame = requestAnimationFrame(tick);
}

export function Markdown({ text, citations = [], referenceLabel = '参考文献' }: { text: string; citations?: Citation[]; referenceLabel?: string }) {
  const scope = `nw-reference-${useId()}`;
  const navigation = useRef<AbortController | null>(null);
  useEffect(() => () => navigation.current?.abort(), []);
  const components = useMemo<Components>(() => ({
    a({ href, children, node, title }) {
      if (node?.properties['data-citation-number']) {
        return <sup><a className="nw-cite" href={href} title={title} aria-label={`${referenceLabel} ${node.properties['data-citation-number']}`} onClick={event => {
          if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
          const target = href && document.getElementById(href.slice(1));
          if (!target) return;
          event.preventDefault();
          navigation.current?.abort();
          navigation.current = new AbortController();
          revealReference(target, navigation.current.signal);
        }}>{children}</a></sup>;
      }
      if (node?.properties['data-reference-link']) return <a href={href} target="_blank" rel="noopener noreferrer">{children}</a>;
      return href ? <a href={href} rel="noopener">{children}</a> : <>{children}</>;
    },
    // 对话气泡里标题不放大，只加粗。
    h1: ({ children }) => <p className="font-semibold">{children}</p>,
    h2: ({ children }) => <p className="font-semibold">{children}</p>,
    h3: ({ children }) => <p className="font-semibold">{children}</p>,
    h4: ({ children }) => <p className="font-semibold">{children}</p>,
    table: ({ children }) => <div className="overflow-x-auto"><table>{children}</table></div>,
    img: () => null,
  }), [referenceLabel]);
  return (
    <div className="nw-prose nw-answer">
      <ReactMarkdown components={components} remarkPlugins={[remarkGfm, referencePlugin(citations, scope, referenceLabel)]} skipHtml urlTransform={urlTransform}>
        {text.replace(/\r\n/g, '\n')}
      </ReactMarkdown>
    </div>
  );
}
