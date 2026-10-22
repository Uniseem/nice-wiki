import { FileText, Magnifier } from '@gravity-ui/icons';
import { Modal } from '@heroui/react';
import { useEffect, useMemo, useRef, useState } from 'react';
import type { Translate } from './islands';

type NavNode = { documentId: string; parentId: string | null; title: string; canonicalPath: string; seoDescription?: string | null };
type Hit = { node: NavNode; trail: string; score: number };

let cache: Promise<NavNode[]> | null = null;
// 当前语言的文档导航（标题已按语言翻译），整页只取一次。
function loadNav(locale: string): Promise<NavNode[]> {
  cache ??= fetch(`/${encodeURIComponent(locale)}/navigation`).then((response) => (response.ok ? response.json() as Promise<NavNode[]> : [])).catch(() => []);
  return cache;
}

// 快速搜索（⌘K）：按标题和搜索摘要查找已发布文档，结果附所在位置，回车直接打开。
export default function Search({ open, locale, t, onOpenChange }: { open: boolean; locale: string; t: Translate; onOpenChange: (open: boolean) => void }) {
  const [nodes, setNodes] = useState<NavNode[] | null>(null);
  const [query, setQuery] = useState('');
  const [index, setIndex] = useState(0);
  const list = useRef<HTMLUListElement>(null);
  useEffect(() => { if (open) void loadNav(locale).then(setNodes); else { setQuery(''); setIndex(0); } }, [open, locale]);

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

  useEffect(() => { list.current?.querySelector('[aria-selected="true"]')?.scrollIntoView({ block: 'nearest' }); }, [index]);
  const go = (hit?: Hit) => { if (hit) location.href = hit.node.canonicalPath; };

  return (
    <Modal.Backdrop isOpen={open} onOpenChange={onOpenChange}>
      <Modal.Container placement="top" size="lg">
        <Modal.Dialog aria-label={t('搜索文档')} className="mt-[10vh] gap-0 overflow-hidden p-0">
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
                if (event.key === 'ArrowDown' && hits.length) { event.preventDefault(); setIndex((value) => (value + 1) % hits.length); }
                if (event.key === 'ArrowUp' && hits.length) { event.preventDefault(); setIndex((value) => (value + hits.length - 1) % hits.length); }
                if (event.key === 'Enter' && !event.nativeEvent.isComposing) { event.preventDefault(); go(hits[index]); }
              }}
            />
            <kbd className="hidden rounded-md border border-border px-1.5 font-sans text-xs leading-5 text-muted sm:block">Esc</kbd>
          </div>
          {nodes && !hits.length ? (
            <p className="px-4 py-10 text-center text-sm text-muted">{t('没有找到相关文档')}</p>
          ) : (
            <ul ref={list} aria-label={t('搜索文档')} className="max-h-[60vh] overflow-y-auto p-2" role="listbox">
              {hits.map((hit, position) => (
                <li key={hit.node.documentId} aria-selected={position === index} role="option">
                  <a
                    className={`flex items-start gap-3 rounded-xl px-3 py-2.5 transition-colors ${position === index ? 'bg-default' : 'hover:bg-default/60'}`}
                    href={hit.node.canonicalPath}
                    onMouseEnter={() => setIndex(position)}
                  >
                    <FileText className="mt-0.5 size-4 shrink-0 text-muted" />
                    <span className="min-w-0">
                      <span className="block truncate text-sm font-medium text-foreground">{hit.node.title}</span>
                      {hit.trail && <span className="block truncate text-xs text-muted">{hit.trail}</span>}
                      {hit.node.seoDescription && <span className="mt-0.5 line-clamp-1 block text-xs text-muted">{hit.node.seoDescription}</span>}
                    </span>
                  </a>
                </li>
              ))}
            </ul>
          )}
        </Modal.Dialog>
      </Modal.Container>
    </Modal.Backdrop>
  );
}
