import type { ReactNode } from 'react';
import type { Citation } from './client';

// 回答里常见的 Markdown（段落、标题、列表、代码块、行内代码、加粗、链接）转成 React 元素。
// 不解析 HTML、不用 innerHTML：所有文字都按纯文本渲染，链接只放行 http(s) 与站内相对地址。

// 引用标识：模型按约定写 [[citation.…]]，也可能写成 [1](citation.…)、<sup>…</sup> 或裸标识，统一换成 ⟦id⟧ 再渲染成上标编号。
const citationId = 'citation\\.[0-9a-f]{8,64}';
function markCitations(text: string): string {
  return text
    .replace(/<\/?sup>/gi, '')
    .replace(new RegExp(`\\[\\[?[^\\]\\n]*\\]?\\]\\((${citationId})\\)`, 'g'), '⟦$1⟧')
    .replace(new RegExp(`\\[\\[?(${citationId})\\]?\\]`, 'g'), '⟦$1⟧')
    .replace(new RegExp(`[(（](${citationId})[)）]`, 'g'), '⟦$1⟧')
    // 流式输出时末尾可能是写了一半的引用（标识是 64 位十六进制，不足的就是没写完），先不显示。
    .replace(/(\[\[?[^\]\n]*\]?\]?\(?)?citation\.[0-9a-f]{0,63}$/, '')
    .replace(new RegExp(`(?<!⟦)(${citationId})(?!⟧)`, 'g'), '⟦$1⟧');
}

type Cites = { refs: Map<string, Citation>; order: Map<string, number> };

function safeHref(href: string): string | null {
  return /^(https?:\/\/|\/)/i.test(href) ? href : null;
}

function inline(text: string, keyPrefix: string, cites: Cites): ReactNode[] {
  const out: ReactNode[] = [];
  const pattern = /(⟦citation\.[0-9a-f]+⟧)|(`[^`]+`)|(\*\*[^*]+\*\*)|(\[[^\]]+\]\([^)\s]+\))/g;
  let last = 0;
  let index = 0;
  for (const match of text.matchAll(pattern)) {
    const start = match.index ?? 0;
    if (start > last) out.push(text.slice(last, start));
    const token = match[0];
    const key = `${keyPrefix}-${index++}`;
    if (token.startsWith('⟦')) {
      // 编号按在回答里第一次出现的顺序；不在本轮引用里的标识直接丢掉，不显示原文。
      const id = token.slice(1, -1);
      const ref = cites.refs.get(id);
      if (ref) {
        if (!cites.order.has(id)) cites.order.set(id, cites.order.size + 1);
        const target = ref.href ? safeHref(ref.href) : null;
        const number = cites.order.get(id);
        out.push(<sup key={key}>{target ? <a className="nw-cite" href={target} title={ref.title}>{number}</a> : <span className="nw-cite">{number}</span>}</sup>);
      }
    } else if (token.startsWith('`')) out.push(<code key={key}>{token.slice(1, -1)}</code>);
    else if (token.startsWith('**')) out.push(<strong key={key}>{token.slice(2, -2)}</strong>);
    else {
      const [, label, href] = /^\[([^\]]+)\]\(([^)\s]+)\)$/.exec(token) ?? [];
      const target = href ? safeHref(href) : null;
      out.push(target ? <a key={key} href={target} rel="noopener">{label}</a> : token);
    }
    last = start + token.length;
  }
  if (last < text.length) out.push(text.slice(last));
  return out;
}

export function Markdown({ text, citations = [] }: { text: string; citations?: Citation[] }) {
  const cites: Cites = { refs: new Map(citations.filter((item) => item.citationId).map((item) => [item.citationId!, item])), order: new Map() };
  const lines = markCitations(text.replace(/\r\n/g, '\n')).split('\n');
  const blocks: ReactNode[] = [];
  let i = 0;
  while (i < lines.length) {
    const line = lines[i];
    if (line.startsWith('```')) {
      const code: string[] = [];
      i++;
      while (i < lines.length && !lines[i].startsWith('```')) code.push(lines[i++]);
      i++;
      blocks.push(<pre key={blocks.length}><code>{code.join('\n')}</code></pre>);
      continue;
    }
    const heading = /^(#{1,4})\s+(.*)$/.exec(line);
    if (heading) {
      blocks.push(<p key={blocks.length} className="font-semibold">{inline(heading[2], `h${i}`, cites)}</p>);
      i++;
      continue;
    }
    if (/^\s*([-*]|\d+\.)\s+/.test(line)) {
      const ordered = /^\s*\d+\./.test(line);
      const items: ReactNode[] = [];
      while (i < lines.length && /^\s*([-*]|\d+\.)\s+/.test(lines[i])) {
        items.push(<li key={i}>{inline(lines[i].replace(/^\s*([-*]|\d+\.)\s+/, ''), `l${i}`, cites)}</li>);
        i++;
      }
      blocks.push(ordered ? <ol key={blocks.length}>{items}</ol> : <ul key={blocks.length}>{items}</ul>);
      continue;
    }
    if (!line.trim()) { i++; continue; }
    const paragraph: string[] = [];
    while (i < lines.length && lines[i].trim() && !lines[i].startsWith('```') && !/^(#{1,4})\s+/.test(lines[i]) && !/^\s*([-*]|\d+\.)\s+/.test(lines[i])) paragraph.push(lines[i++]);
    blocks.push(<p key={blocks.length}>{paragraph.flatMap((part, index) => index ? [<br key={`b${index}`} />, ...inline(part, `p${i}-${index}`, cites)] : inline(part, `p${i}-${index}`, cites))}</p>);
  }
  return <div className="nw-prose nw-answer">{blocks}</div>;
}
