import ReactMarkdown, { type Components } from 'react-markdown';
import remarkGfm from 'remark-gfm';
import type { Citation } from './client';

// 回答用 react-markdown + remark-gfm 渲染（列表嵌套、列表里的代码块、表格、删除线等都按 CommonMark/GFM 处理）。
// 不渲染 HTML、不用 innerHTML；链接只放行 http(s) 与站内相对地址，引用标识渲染成上标编号。

// 引用标识：模型按约定写 [[citation.…]]，也可能写成 [1](citation.…)、<sup>…</sup> 或裸标识，统一换成 [](cite:标识) 链接，由 a 组件渲染成编号。
const citationId = 'citation\\.[0-9a-f]{8,64}';
function markCitations(text: string): string {
  return text
    .replace(/<\/?sup>/gi, '')
    .replace(new RegExp(`\\[\\[?[^\\]\\n]*\\]?\\]\\((${citationId})\\)`, 'g'), '⟦$1⟧')
    .replace(new RegExp(`\\[\\[?(${citationId})\\]?\\]`, 'g'), '⟦$1⟧')
    .replace(new RegExp(`[(（](${citationId})[)）]`, 'g'), '⟦$1⟧')
    // 流式输出时末尾可能是写了一半的引用（标识是 64 位十六进制，不足的就是没写完），先不显示。
    .replace(/(\[\[?[^\]\n]*\]?\]?\(?)?citation\.[0-9a-f]{0,63}$/, '')
    .replace(new RegExp(`(?<!⟦)(${citationId})(?!⟧)`, 'g'), '⟦$1⟧')
    .replace(/⟦(citation\.[0-9a-f]+)⟧/g, '[](cite:$1)');
}

function safeHref(href: string): string | null {
  return /^(https?:\/\/|\/)/i.test(href) ? href : null;
}

// 只保留引用和安全链接，其余地址一律丢掉（链接文字照常显示）。
function urlTransform(url: string): string {
  return url.startsWith('cite:') ? url : safeHref(url) ?? '';
}

export function Markdown({ text, citations = [] }: { text: string; citations?: Citation[] }) {
  const refs = new Map(citations.filter((item) => item.citationId).map((item) => [item.citationId!, item]));
  // 编号按在回答里第一次出现的顺序；不在本轮引用里的标识直接丢掉，不显示原文。
  const order = new Map<string, number>();
  const components: Components = {
    a({ href, children }) {
      if (href?.startsWith('cite:')) {
        const id = href.slice(5);
        const ref = refs.get(id);
        if (!ref) return null;
        if (!order.has(id)) order.set(id, order.size + 1);
        const target = ref.href ? safeHref(ref.href) : null;
        const number = order.get(id);
        return <sup>{target ? <a className="nw-cite" href={target} title={ref.title}>{number}</a> : <span className="nw-cite">{number}</span>}</sup>;
      }
      return href ? <a href={href} rel="noopener">{children}</a> : <>{children}</>;
    },
    // 对话气泡里标题不放大，只加粗。
    h1: ({ children }) => <p className="font-semibold">{children}</p>,
    h2: ({ children }) => <p className="font-semibold">{children}</p>,
    h3: ({ children }) => <p className="font-semibold">{children}</p>,
    h4: ({ children }) => <p className="font-semibold">{children}</p>,
    table: ({ children }) => <div className="overflow-x-auto"><table>{children}</table></div>,
    img: () => null,
  };
  return (
    <div className="nw-prose nw-answer">
      <ReactMarkdown components={components} remarkPlugins={[remarkGfm]} skipHtml urlTransform={urlTransform}>
        {markCitations(text.replace(/\r\n/g, '\n'))}
      </ReactMarkdown>
    </div>
  );
}
