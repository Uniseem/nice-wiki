import { FileText, Magnifier } from '@gravity-ui/icons';
import { Modal, Spinner } from '@heroui/react';
import { searchDocuments, transient, type SearchHit } from '@modules/qa/web/src/island/client';
import { useEffect, useMemo, useRef, useState } from 'react';
import type { Translate } from './islands';
import { visit } from './navigate';

type NavNode = { documentId: string; parentId: string | null; title: string; canonicalPath: string; seoDescription?: string | null };
type Hit = { node: NavNode; trail: string; score: number };
// 列表里的一行：检索全文的入口、全文命中的段落、标题匹配的文档。
type Row = { kind: 'deep' } | { kind: 'passage'; hit: SearchHit } | { kind: 'title'; hit: Hit };
type Deep = { query: string; status: 'loading' | 'done' | 'error'; hits: SearchHit[]; message?: string };

let cache: Promise<NavNode[]> | null = null;
// 当前语言的文档导航（标题已按语言翻译），整页只取一次。
function loadNav(locale: string): Promise<NavNode[]> {
  cache ??= fetch(`/${encodeURIComponent(locale)}/navigation`).then((response) => (response.ok ? response.json() as Promise<NavNode[]> : [])).catch(() => []);
  return cache;
}

// 快速搜索（⌘K）：输入时按标题和搜索摘要即时匹配；按回车（或选中“检索全文”）在知识索引上做关键词 + 向量混合检索，
// 列出命中的段落，回车打开对应段落。全文检索要用 AI 问答的额度，问答没开启时只有标题匹配。
export default function Search({ open, locale, t, deep: deepEnabled, onOpenChange }: { open: boolean; locale: string; t: Translate; deep: boolean; onOpenChange: (open: boolean) => void }) {
  const [nodes, setNodes] = useState<NavNode[] | null>(null);
  const [query, setQuery] = useState('');
  const [index, setIndex] = useState(0);
  const [deep, setDeep] = useState<Deep | null>(null);
  const list = useRef<HTMLUListElement>(null);
  useEffect(() => { if (open) void loadNav(locale).then(setNodes); else { setQuery(''); setIndex(0); setDeep(null); } }, [open, locale]);

  const hits = useMemo((): Hit[] => {
    const all = nodes ?? [];
    const byId = new Map(all.map((node) => [node.documentId, node]));
    const trailOf = (node: NavNode) => {
      const names: string[] = [];
      for (let parent = node.parentId ? byId.get(node.parentId) : undefined; parent; parent = parent.parentId ? byId.get(parent.parentId) : undefined) names.unshift(parent.title);
      return names.join(' / ');
    };
    const q = query.trim().toLowerCase();
    if (!q) return all.slice(0, 8).map((node) => ({ node, trail: trailOf(node), score: 0 }));
    return all.flatMap((node) => {
      const title = node.title.toLowerCase();
      const score = title.startsWith(q) ? 3 : title.includes(q) ? 2 : (node.seoDescription ?? '').toLowerCase().includes(q) ? 1 : 0;
      return score ? [{ node, trail: trailOf(node), score }] : [];
    }).sort((a, b) => b.score - a.score).slice(0, 20);
  }, [nodes, query]);

  const trimmed = query.trim();
  const current = deep && deep.query === trimmed ? deep : null;
  const rows = useMemo((): Row[] => [
    ...(deepEnabled && trimmed && current?.status !== 'done' ? [{ kind: 'deep' } as Row] : []),
    ...(current?.status === 'done' ? current.hits.map((hit): Row => ({ kind: 'passage', hit })) : []),
    ...hits.map((hit): Row => ({ kind: 'title', hit })),
  ], [deepEnabled, trimmed, current, hits]);

  useEffect(() => { list.current?.querySelector('[aria-selected="true"]')?.scrollIntoView({ block: 'nearest' }); }, [index]);

  async function searchAll() {
    if (!trimmed || current?.status === 'loading') return;
    setDeep({ query: trimmed, status: 'loading', hits: [] });
    try {
      const found = await searchDocuments(trimmed, locale);
      setDeep((value) => (value?.query === trimmed ? { query: trimmed, status: 'done', hits: found } : value));
      setIndex(0);
    } catch (problem) {
      const message = transient(problem) ? t('服务正在更新，请稍后再试。') : t('全文检索暂时不可用，请稍后再试。');
      setDeep((value) => (value?.query === trimmed ? { query: trimmed, status: 'error', hits: [], message } : value));
    }
  }

  const choose = (row?: Row) => {
    if (!row) return;
    if (row.kind === 'deep') void searchAll();
    else visit(row.kind === 'passage' ? row.hit.href : row.hit.node.canonicalPath);
  };

  const firstPassage = rows.findIndex((row) => row.kind === 'passage');
  const firstTitle = rows.findIndex((row) => row.kind === 'title');
  const empty = nodes && !rows.length;

  return (
    <Modal.Backdrop isOpen={open} onOpenChange={onOpenChange}>
      <Modal.Container placement="top" size="lg">
        <Modal.Dialog aria-label={t('搜索文档')} className="mt-[10vh] gap-0 overflow-hidden rounded-3xl p-0">
          <div className="flex items-center gap-3 border-b border-separator px-4">
            <Magnifier className="size-4 shrink-0 text-muted" />
            <input
              autoFocus
              aria-label={t('搜索文档')}
              className="h-14 min-w-0 flex-1 bg-transparent text-base text-foreground outline-none placeholder:text-muted"
              placeholder={t('搜索文档标题…')}
              value={query}
              onChange={(event) => { setQuery(event.target.value); setIndex(0); }}
              onKeyDown={(event) => {
                if (event.key === 'ArrowDown' && rows.length) { event.preventDefault(); setIndex((value) => (value + 1) % rows.length); }
                if (event.key === 'ArrowUp' && rows.length) { event.preventDefault(); setIndex((value) => (value + rows.length - 1) % rows.length); }
                if (event.key === 'Enter' && !event.nativeEvent.isComposing) { event.preventDefault(); choose(rows[index]); }
              }}
            />
            <kbd className="hidden rounded-md border border-border px-1.5 font-sans text-xs leading-5 text-muted sm:block">Esc</kbd>
          </div>
          {empty ? (
            <p className="px-4 py-10 text-center text-sm text-muted">{t('没有找到相关文档')}</p>
          ) : (
            <ul ref={list} aria-label={t('搜索文档')} className="max-h-[60vh] overflow-y-auto p-2" role="listbox">
              {current?.status === 'done' && !current.hits.length && <li aria-hidden="true" className="px-3 py-2 text-xs text-muted">{t('全文中没有找到相关内容')}</li>}
              {rows.map((row, position) => {
                const selected = position === index;
                const rowClass = `flex w-full items-start gap-3 rounded-xl px-3 py-2.5 text-start transition-colors ${selected ? 'bg-default' : 'hover:bg-default/60'}`;
                const heading = (key: string, label: string) => <li key={key} aria-hidden="true" className="px-3 pb-1 pt-2 text-xs font-medium text-muted">{label}</li>;
                if (row.kind === 'deep') {
                  return (
                    <li key="deep" aria-selected={selected} role="option">
                      <button className={rowClass} type="button" onClick={() => choose(row)} onMouseEnter={() => setIndex(position)}>
                        {current?.status === 'loading' ? <Spinner className="mt-0.5 shrink-0" color="current" size="sm" /> : <Magnifier className="mt-0.5 size-4 shrink-0" />}
                        <span className="min-w-0">
                          <span className="block truncate text-sm font-medium text-foreground">{current?.status === 'loading' ? t('正在检索全文…') : `${t('检索全文')}：${trimmed}`}</span>
                          {current?.status === 'error' && <span className="block text-xs text-danger">{current.message}</span>}
                        </span>
                        {current?.status !== 'loading' && <kbd className="ms-auto hidden shrink-0 rounded-md border border-border px-1.5 font-sans text-xs leading-5 text-muted sm:block">↵</kbd>}
                      </button>
                    </li>
                  );
                }
                if (row.kind === 'passage') {
                  return [
                    position === firstPassage && heading('passages', t('全文检索')),
                    <li key={`p${position}`} aria-selected={selected} role="option">
                      <a className={rowClass} href={row.hit.href} onClick={(event) => { event.preventDefault(); choose(row); }} onMouseEnter={() => setIndex(position)}>
                        <FileText className="mt-0.5 size-4 shrink-0" />
                        <span className="min-w-0">
                          <span className="block truncate text-sm font-medium text-foreground">{row.hit.title}</span>
                          {row.hit.excerpt && <span className="mt-0.5 line-clamp-2 text-xs leading-5 text-muted">{row.hit.excerpt}</span>}
                        </span>
                      </a>
                    </li>,
                  ];
                }
                const hit = row.hit;
                return [
                  position === firstTitle && firstPassage >= 0 && heading('titles', t('标题匹配')),
                  <li key={hit.node.documentId} aria-selected={selected} role="option">
                    <a className={rowClass} href={hit.node.canonicalPath} onClick={(event) => { event.preventDefault(); choose(row); }} onMouseEnter={() => setIndex(position)}>
                      <FileText className="mt-0.5 size-4 shrink-0 text-muted" />
                      <span className="min-w-0">
                        <span className="block truncate text-sm font-medium text-foreground">{hit.node.title}</span>
                        {hit.trail && <span className="block truncate text-xs text-muted">{hit.trail}</span>}
                        {hit.node.seoDescription && <span className="mt-0.5 line-clamp-1 text-xs text-muted">{hit.node.seoDescription}</span>}
                      </span>
                    </a>
                  </li>,
                ];
              })}
            </ul>
          )}
        </Modal.Dialog>
      </Modal.Container>
    </Modal.Backdrop>
  );
}
