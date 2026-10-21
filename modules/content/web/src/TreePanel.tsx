import { ArrowDown, ArrowLeft, ArrowUp, ChevronDown, ChevronRight, Ellipsis, FileText, Plus, TrashBin } from '@gravity-ui/icons';
import { Button, Chip, cn, Dropdown, Label, SearchField, Spinner } from '@heroui/react';
import type { Key } from '@heroui/react';
import { api, askText, confirmAction, EmptyBlock, errorText, fmtRelative, notify, useHasGrant, useLoad, t } from '@ui';
import { useMemo, useState } from 'react';
import { byOrder, flatten, liveDocs, pathPattern, sha256Hex, suggestPath, type Meta } from './types';

// 左侧文档树：搜索、展开/收起、新建（顶层或子文档）、上下移动；底部进入回收站。
export function TreePanel({ docs, treeRevision, selected, onSelect, onCreate, onChanged }: {
  docs: Meta[];
  treeRevision: number;
  selected: string | null;
  onSelect: (id: string) => void;
  onCreate: (parentId: string | null) => void;
  onChanged: () => Promise<void>;
}) {
  const [query, setQuery] = useState('');
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());
  const [view, setView] = useState<'docs' | 'trash'>('docs');
  const canEdit = useHasGrant('content.edit');
  const canTrash = useHasGrant('content.trash', 'content.publish');
  const trashCount = docs.filter((doc) => doc.state === 'trash').length;

  const rows = useMemo(() => {
    const all = flatten(docs);
    const q = query.trim().toLowerCase();
    if (q) return all.filter((row) => row.doc.title.toLowerCase().includes(q)).map((row) => ({ ...row, depth: 0, hasChildren: false }));
    // 收起的节点隐藏其所有后代。
    const hidden = new Set<string>();
    return all.filter((row) => {
      if (row.doc.parentId && (hidden.has(row.doc.parentId) || collapsed.has(row.doc.parentId))) { hidden.add(row.doc.documentId); return false; }
      return true;
    });
  }, [docs, query, collapsed]);

  // 同级第一篇不能上移、最后一篇不能下移：菜单里对应项置灰。
  const edgeKeys = useMemo(() => {
    const groups = new Map<string | null, Meta[]>();
    for (const doc of liveDocs(docs)) groups.set(doc.parentId ?? null, [...(groups.get(doc.parentId ?? null) ?? []), doc]);
    const keys = new Map<string, string[]>();
    for (const group of groups.values()) {
      const sorted = [...group].sort(byOrder);
      sorted.forEach((doc, index) => keys.set(doc.documentId, [...(index === 0 ? ['up'] : []), ...(index === sorted.length - 1 ? ['down'] : [])]));
    }
    return keys;
  }, [docs]);

  const toggle = (id: string) => setCollapsed((current) => { const next = new Set(current); if (next.has(id)) next.delete(id); else next.add(id); return next; });

  const reorder = async (doc: Meta, delta: number) => {
    const siblings = liveDocs(docs).filter((item) => (item.parentId ?? null) === (doc.parentId ?? null)).sort(byOrder);
    const index = siblings.findIndex((item) => item.documentId === doc.documentId);
    const target = index + delta;
    if (target < 0 || target >= siblings.length) return;
    const ids = siblings.map((item) => item.documentId);
    [ids[index], ids[target]] = [ids[target], ids[index]];
    try {
      await api('/v1/tree:reorder', { method: 'POST', body: { parentId: doc.parentId ?? null, orderedDocumentIds: ids, expectedTreeRevision: treeRevision } });
      await onChanged();
    } catch (problem) { notify.error(t('调整顺序失败'), errorText(problem)); }
  };

  if (view === 'trash') return <TrashList treeRevision={treeRevision} onBack={() => setView('docs')} onChanged={onChanged} />;

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex items-center justify-between gap-2 px-4 pb-2 pt-4">
        <h2 className="text-sm font-semibold text-foreground">{t('文档')}</h2>
        {canEdit && <Button isIconOnly aria-label={t('新建顶层文档')} size="sm" variant="tertiary" onPress={() => onCreate(null)}><Plus /></Button>}
      </div>
      <div className="px-3 pb-2">
        <SearchField aria-label={t('搜索文档')} value={query} variant="secondary" onChange={setQuery}>
          <SearchField.Group>
            <SearchField.SearchIcon />
            <SearchField.Input placeholder={t('搜索标题')} />
            <SearchField.ClearButton />
          </SearchField.Group>
        </SearchField>
      </div>
      <nav aria-label={t('文档树')} className="min-h-0 flex-1 overflow-y-auto px-2 pb-3">
        {!rows.length ? (
          <EmptyBlock
            className="mx-1 mt-2"
            icon={<FileText />}
            title={query ? t('没有匹配的文档') : t('还没有文档')}
            description={query ? t('换个关键词试试。') : t('新建第一篇文档，写完点“发布”，访客就能看到。')}
            action={!query && canEdit ? <Button size="sm" onPress={() => onCreate(null)}><Plus />{t('新建文档')}</Button> : undefined}
          />
        ) : (
          <ul className="flex flex-col gap-px">
            {rows.map(({ doc, depth, hasChildren }) => {
              const active = doc.documentId === selected;
              const open = !collapsed.has(doc.documentId);
              return (
                <li key={doc.documentId}>
                  <div
                    className={cn('group flex h-9 items-center gap-1 rounded-lg pe-1 text-sm transition-colors', active ? 'bg-accent-soft text-accent-soft-foreground' : 'text-foreground hover:bg-default')}
                    style={{ paddingInlineStart: `${depth * 14 + 4}px` }}
                  >
                    <button
                      aria-label={open ? t('收起') : t('展开')}
                      className={cn('flex size-6 shrink-0 items-center justify-center rounded-md text-muted hover:bg-default', !hasChildren && 'invisible')}
                      tabIndex={-1}
                      type="button"
                      onClick={() => toggle(doc.documentId)}
                    >
                      {open ? <ChevronDown className="size-3.5" /> : <ChevronRight className="size-3.5 rtl:-scale-x-100" />}
                    </button>
                    <button className="flex min-w-0 flex-1 items-center gap-2 py-1 text-start" type="button" onClick={() => onSelect(doc.documentId)}>
                      <span className={cn('size-1.5 shrink-0 rounded-full', doc.state === 'published' ? 'bg-success' : 'bg-border')} title={doc.state === 'published' ? t('已发布') : t('草稿')} />
                      <span className={cn('truncate', active && 'font-medium')}>{doc.title || t('无标题')}</span>
                    </button>
                    {canEdit && !query && (
                      <span className="flex shrink-0 items-center opacity-0 transition-opacity focus-within:opacity-100 group-hover:opacity-100">
                        <Button isIconOnly aria-label={t('新建子文档')} className="size-7 min-w-7" size="sm" variant="ghost" onPress={() => onCreate(doc.documentId)}><Plus className="size-3.5" /></Button>
                        <Dropdown>
                          <Button isIconOnly aria-label={t('更多')} className="size-7 min-w-7" size="sm" variant="ghost"><Ellipsis className="size-3.5" /></Button>
                          <Dropdown.Popover placement="bottom end">
                            <Dropdown.Menu disabledKeys={edgeKeys.get(doc.documentId)} onAction={(key: Key) => { if (key === 'up') void reorder(doc, -1); if (key === 'down') void reorder(doc, 1); if (key === 'child') onCreate(doc.documentId); }}>
                              <Dropdown.Item id="child" textValue={t('新建子文档')}><Plus className="size-4 text-muted" /><Label>{t('新建子文档')}</Label></Dropdown.Item>
                              <Dropdown.Item id="up" textValue={t('上移')}><ArrowUp className="size-4 text-muted" /><Label>{t('上移')}</Label></Dropdown.Item>
                              <Dropdown.Item id="down" textValue={t('下移')}><ArrowDown className="size-4 text-muted" /><Label>{t('下移')}</Label></Dropdown.Item>
                            </Dropdown.Menu>
                          </Dropdown.Popover>
                        </Dropdown>
                      </span>
                    )}
                  </div>
                </li>
              );
            })}
          </ul>
        )}
      </nav>
      {canTrash && (
        <div className="border-t border-separator p-2">
          <Button className="w-full justify-start" size="sm" variant="ghost" onPress={() => setView('trash')}>
            <TrashBin />{t('回收站')}
            {trashCount > 0 && <Chip className="ms-auto" size="sm" variant="soft">{trashCount}</Chip>}
          </Button>
        </div>
      )}
    </div>
  );
}

function TrashList({ treeRevision, onBack, onChanged }: { treeRevision: number; onBack: () => void; onChanged: () => Promise<void> }) {
  const isSuperadmin = useHasGrant('superadmin');
  const trash = useLoad((signal) => api<{ documents: (Meta & { trashedAt?: string | null })[] }>('/v1/documents?state=trash&limit=100', { signal }).then((result) => result.documents), []);
  const [busy, setBusy] = useState<string | null>(null);

  const restore = async (doc: Meta) => {
    const path = await askText({
      title: t('恢复“{0}”', doc.title), label: t('页面路径'), defaultValue: doc.pathSegment ?? suggestPath(doc.title),
      description: t('恢复到顶层。路径只能用小写字母、数字和连字符，不能和现有页面重复。'), confirmLabel: t('恢复'),
      validate: (value) => pathPattern.test(value.trim()) ? null : t('格式不对：只能用小写字母、数字和连字符'),
    });
    if (!path) return;
    setBusy(doc.documentId);
    try {
      await api(`/v1/documents/${doc.documentId}:restore`, { method: 'POST', body: { expectedRevision: doc.revision, parentId: null, pathSegment: path.trim() } });
      notify.success(t('已恢复'), t('文档已回到顶层，需要的话再移动到合适的位置。'));
      trash.reload();
      await onChanged();
    } catch (problem) { notify.error(t('恢复失败'), errorText(problem)); } finally { setBusy(null); }
  };

  const purge = async (doc: Meta) => {
    const ok = await confirmAction({ title: t('彻底删除“{0}”？', doc.title), body: t('彻底删除后无法恢复，公开地址会返回“已删除”。'), confirmLabel: t('彻底删除'), tone: 'danger' });
    if (!ok) return;
    setBusy(doc.documentId);
    try {
      await api('/v1/trash:purge', { method: 'POST', body: { documentIds: [doc.documentId], expectedTrashRevision: treeRevision, confirmationHash: await sha256Hex(doc.documentId) } });
      notify.success(t('已彻底删除'));
      trash.reload();
      await onChanged();
    } catch (problem) { notify.error(t('删除失败'), errorText(problem)); } finally { setBusy(null); }
  };

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex items-center gap-2 px-3 pb-2 pt-4">
        <Button isIconOnly aria-label={t('返回文档')} size="sm" variant="ghost" onPress={onBack}><ArrowLeft className="rtl:-scale-x-100" /></Button>
        <h2 className="text-sm font-semibold text-foreground">{t('回收站')}</h2>
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto px-3 pb-3">
        {trash.loading && !trash.data ? <div className="flex justify-center py-8"><Spinner size="sm" /></div> : !trash.data?.length ? (
          <EmptyBlock className="mt-2" icon={<TrashBin />} title={t('回收站是空的')} description={t('删除的文档会先放在这里。')} />
        ) : (
          <ul className="flex flex-col gap-2">
            {trash.data.map((doc) => (
              <li key={doc.documentId} className="rounded-xl border border-border bg-surface p-3">
                <div className="truncate text-sm font-medium text-foreground">{doc.title || t('无标题')}</div>
                {doc.trashedAt && <div className="text-xs text-muted">{t('删除于 {0}', fmtRelative(doc.trashedAt))}</div>}
                <div className="mt-2 flex gap-2">
                  <Button isPending={busy === doc.documentId} size="sm" variant="secondary" onPress={() => restore(doc)}>{t('恢复')}</Button>
                  {isSuperadmin && <Button isDisabled={busy === doc.documentId} size="sm" variant="danger-soft" onPress={() => purge(doc)}>{t('彻底删除')}</Button>}
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}
