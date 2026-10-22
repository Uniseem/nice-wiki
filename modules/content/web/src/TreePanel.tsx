import { ArrowDown, ArrowLeft, ArrowUp, ArrowUpRightFromSquare, ChevronDown, ChevronRight, Ellipsis, FileText, FolderArrowRight, Plus, TrashBin } from '@gravity-ui/icons';
import { Button, cn, Dropdown, Label, SearchField, Spinner } from '@heroui/react';
import type { Key } from '@heroui/react';
import { api, ApiError, askText, confirmAction, EmptyBlock, errorText, fmtRelative, notify, useHasGrant, useLoad, useStoredState, t } from '@ui';
import { useEffect, useMemo, useRef, useState, type DragEvent } from 'react';
import { MoveDialog } from './dialogs';
import { ancestors, byOrder, flatten, liveDocs, pathPattern, sha256Hex, subtree, suggestPath, type Meta } from './types';

type Drop = { id: string | null; position: 'before' | 'after' | 'inside' };

// 左侧文档树：搜索、展开/收起（记在本浏览器）、拖动调整层级和顺序、新建子文档、移动；底部进入回收站。
export function TreePanel({ docs, treeRevision, selected, publicPath, onSelect, onCreate, onChanged }: {
  docs: Meta[];
  treeRevision: number;
  selected: string | null;
  publicPath: (doc: Meta) => string | null;
  onSelect: (id: string) => void;
  onCreate: (parentId: string | null) => void;
  onChanged: () => Promise<void>;
}) {
  const [query, setQuery] = useState('');
  const [expandedList, setExpandedList] = useStoredState<string[]>('nw-content-tree-expanded', []);
  const expanded = useMemo(() => new Set(expandedList), [expandedList]);
  const [view, setView] = useState<'docs' | 'trash'>('docs');
  const [moving, setMoving] = useState<Meta | null>(null);
  const [dragging, setDragging] = useState<string | null>(null);
  const [drop, setDrop] = useState<Drop | null>(null);
  const nav = useRef<HTMLElement>(null);
  const canEdit = useHasGrant('content.edit');
  const canTrash = useHasGrant('content.trash', 'content.publish');
  const live = useMemo(() => liveDocs(docs), [docs]);

  const expand = (ids: string[]) => setExpandedList((current) => {
    const missing = ids.filter((id) => !current.includes(id));
    return missing.length ? [...current, ...missing] : current;
  });
  const toggle = (id: string) => setExpandedList((current) => current.includes(id) ? current.filter((item) => item !== id) : [...current, id]);

  // 新打开的文档所在的分支自动展开，并滚到可见处（之后用户收起也不再强行展开）。
  const revealed = useRef<string | null>(null);
  useEffect(() => {
    if (!selected || revealed.current === selected || !docs.some((doc) => doc.documentId === selected)) return;
    revealed.current = selected;
    const chain = ancestors(docs, selected).map((doc) => doc.documentId);
    if (chain.length) expand(chain);
    requestAnimationFrame(() => nav.current?.querySelector(`[data-doc="${selected}"]`)?.scrollIntoView({ block: 'nearest' }));
  }, [selected, docs]);

  const rows = useMemo(() => {
    const all = flatten(docs);
    const q = query.trim().toLowerCase();
    // 搜索结果平铺显示，下面附上所在位置，同名文档也能分清。
    if (q) return all.filter((row) => row.doc.title.toLowerCase().includes(q)).map((row) => ({ ...row, depth: 0, hasChildren: false, trail: ancestors(docs, row.doc.documentId).map((doc) => doc.title).join(' / ') }));
    // 只显示祖先全部展开的节点。
    const visible = new Set<string>();
    return all.filter((row) => {
      const show = !row.doc.parentId || (visible.has(row.doc.parentId) && expanded.has(row.doc.parentId));
      if (show) visible.add(row.doc.documentId);
      return show;
    }).map((row) => ({ ...row, trail: '' }));
  }, [docs, query, expanded]);

  const siblings = (parentId: string | null, without?: string) => live.filter((item) => (item.parentId ?? null) === parentId && item.documentId !== without).sort(byOrder);

  // 同级第一篇不能上移、最后一篇不能下移：菜单里对应项置灰。
  const edgeKeys = (doc: Meta): string[] => {
    const group = siblings(doc.parentId ?? null);
    const index = group.findIndex((item) => item.documentId === doc.documentId);
    return [...(index === 0 ? ['up'] : []), ...(index === group.length - 1 ? ['down'] : [])];
  };

  const reorder = async (doc: Meta, delta: number) => {
    const group = siblings(doc.parentId ?? null);
    const index = group.findIndex((item) => item.documentId === doc.documentId);
    const target = index + delta;
    if (target < 0 || target >= group.length) return;
    const ids = group.map((item) => item.documentId);
    [ids[index], ids[target]] = [ids[target], ids[index]];
    try {
      await api('/v1/tree:reorder', { method: 'POST', body: { parentId: doc.parentId ?? null, orderedDocumentIds: ids, expectedTreeRevision: treeRevision } });
      await onChanged();
    } catch (problem) { notify.error(t('调整顺序失败'), errorText(problem)); }
  };

  const move = async (documentId: string, parentId: string | null, beforeId: string | null) => {
    const doc = live.find((item) => item.documentId === documentId);
    if (!doc) return;
    // 位置没变就不发请求。
    if ((doc.parentId ?? null) === parentId) {
      const group = siblings(parentId);
      const next = group[group.findIndex((item) => item.documentId === documentId) + 1]?.documentId ?? null;
      if (next === beforeId) return;
    }
    try {
      await api('/v1/tree:move', { method: 'POST', body: { documentId, newParentId: parentId, beforeDocumentId: beforeId, expectedTreeRevision: treeRevision } });
      if (parentId) expand([parentId]);
      await onChanged();
    } catch (problem) { notify.error(t('移动失败'), errorText(problem)); }
  };

  // 拖动：落在一行的上沿/下沿表示放到它前面/后面，落在中间表示成为它的最后一个子文档；不能拖进自己的子树。
  const banned = useMemo(() => dragging ? subtree(docs, dragging) : new Set<string>(), [docs, dragging]);
  const dragOver = (event: DragEvent<HTMLElement>, doc: Meta | null) => {
    if (!dragging || (doc && banned.has(doc.documentId))) return;
    event.preventDefault();
    event.stopPropagation();
    event.dataTransfer.dropEffect = 'move';
    let position: Drop['position'] = 'inside';
    if (doc) {
      const rect = event.currentTarget.getBoundingClientRect();
      const offset = (event.clientY - rect.top) / rect.height;
      position = offset < 0.28 ? 'before' : offset > 0.72 ? 'after' : 'inside';
    }
    const id = doc?.documentId ?? null;
    if (drop?.id !== id || drop?.position !== position) setDrop({ id, position });
  };
  const dropOn = (event: DragEvent<HTMLElement>, doc: Meta | null) => {
    event.preventDefault();
    event.stopPropagation();
    const source = dragging;
    const target = drop;
    setDragging(null);
    setDrop(null);
    if (!source || !target) return;
    if (!doc) { void move(source, null, null); return; }
    if (target.position === 'inside') { void move(source, doc.documentId, null); return; }
    const parentId = doc.parentId ?? null;
    if (target.position === 'before') { void move(source, parentId, doc.documentId); return; }
    const group = siblings(parentId, source);
    void move(source, parentId, group[group.findIndex((item) => item.documentId === doc.documentId) + 1]?.documentId ?? null);
  };
  const dragEnd = () => { setDragging(null); setDrop(null); };

  if (view === 'trash') return <TrashList docs={live} treeRevision={treeRevision} onBack={() => setView('docs')} onChanged={onChanged} />;

  const draggable = canEdit && !query;
  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex items-center justify-between gap-2 px-4 pb-2 pt-3">
        <h2 className="text-sm font-semibold text-foreground">{t('文档')}<span className="ms-1.5 text-xs font-normal text-muted">{live.length}</span></h2>
        {canEdit && <Button isIconOnly aria-label={t('新建顶层文档')} size="sm" variant="ghost" onPress={() => onCreate(null)}><Plus /></Button>}
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
      <nav
        ref={nav}
        aria-label={t('文档树')}
        className={cn('min-h-0 flex-1 overflow-y-auto px-2 pb-6', dragging && drop?.id === null && 'bg-accent-soft/40')}
        onDragOver={(event) => dragOver(event, null)}
        onDrop={(event) => dropOn(event, null)}
      >
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
            {rows.map(({ doc, depth, hasChildren, trail }) => {
              const active = doc.documentId === selected;
              const open = expanded.has(doc.documentId);
              const target = drop?.id === doc.documentId ? drop.position : null;
              const link = doc.state === 'published' ? publicPath(doc) : null;
              return (
                <li key={doc.documentId}>
                  <div
                    className={cn(
                      'group relative flex min-h-8 items-center gap-0.5 rounded-lg pe-1 text-sm transition-colors',
                      active ? 'bg-accent-soft text-accent-soft-foreground' : 'text-foreground hover:bg-default',
                      dragging === doc.documentId && 'opacity-40',
                      target === 'inside' && 'bg-accent-soft ring-2 ring-inset ring-accent',
                    )}
                    data-doc={doc.documentId}
                    draggable={draggable}
                    style={{ paddingInlineStart: `${depth * 16 + 2}px` }}
                    onDragEnd={dragEnd}
                    onDragOver={(event) => dragOver(event, doc)}
                    onDragStart={(event) => { event.dataTransfer.effectAllowed = 'move'; event.dataTransfer.setData('text/plain', doc.title); setDragging(doc.documentId); }}
                    onDrop={(event) => dropOn(event, doc)}
                  >
                    {target === 'before' && <span className="pointer-events-none absolute -top-px end-1 h-0.5 rounded-full bg-accent" style={{ insetInlineStart: `${depth * 16 + 8}px` }} />}
                    {target === 'after' && <span className="pointer-events-none absolute -bottom-px end-1 h-0.5 rounded-full bg-accent" style={{ insetInlineStart: `${depth * 16 + 8}px` }} />}
                    <button
                      aria-label={open ? t('收起') : t('展开')}
                      className={cn('flex size-6 shrink-0 items-center justify-center rounded-md text-muted hover:bg-default-hover hover:text-foreground', !hasChildren && 'invisible')}
                      tabIndex={-1}
                      type="button"
                      onClick={() => toggle(doc.documentId)}
                    >
                      {open ? <ChevronDown className="size-3.5" /> : <ChevronRight className="size-3.5 rtl:-scale-x-100" />}
                    </button>
                    <button className="flex min-h-8 min-w-0 flex-1 items-center gap-2 py-1 text-start" title={doc.title} type="button" onClick={() => onSelect(doc.documentId)}>
                      <FileText className={cn('size-4 shrink-0', active ? 'text-accent' : 'text-muted')} />
                      <span className="min-w-0">
                        <span className={cn('block truncate', active && 'font-medium')}>{doc.title || t('无标题')}</span>
                        {trail && <span className="block truncate text-xs text-muted">{trail}</span>}
                      </span>
                      {doc.state === 'draft' && <span className="shrink-0 rounded border border-border px-1 text-[10px] leading-4 text-muted">{t('草稿')}</span>}
                    </button>
                    {canEdit && !query && (
                      <span className="flex shrink-0 items-center opacity-0 transition-opacity focus-within:opacity-100 group-hover:opacity-100">
                        <Dropdown>
                          <Button isIconOnly aria-label={t('更多')} className="size-6 min-w-6" size="sm" variant="ghost"><Ellipsis className="size-3.5" /></Button>
                          <Dropdown.Popover placement="bottom end">
                            <Dropdown.Menu
                              disabledKeys={edgeKeys(doc)}
                              onAction={(key: Key) => {
                                if (key === 'child') onCreate(doc.documentId);
                                if (key === 'move') setMoving(doc);
                                if (key === 'up') void reorder(doc, -1);
                                if (key === 'down') void reorder(doc, 1);
                                if (key === 'open' && link) window.open(link, '_blank', 'noopener');
                              }}
                            >
                              <Dropdown.Item id="child" textValue={t('新建子文档')}><Plus className="size-4 text-muted" /><Label>{t('新建子文档')}</Label></Dropdown.Item>
                              <Dropdown.Item id="move" textValue={t('移动到')}><FolderArrowRight className="size-4 text-muted" /><Label>{t('移动到…')}</Label></Dropdown.Item>
                              <Dropdown.Item id="up" textValue={t('上移')}><ArrowUp className="size-4 text-muted" /><Label>{t('上移')}</Label></Dropdown.Item>
                              <Dropdown.Item id="down" textValue={t('下移')}><ArrowDown className="size-4 text-muted" /><Label>{t('下移')}</Label></Dropdown.Item>
                              {link ? <Dropdown.Item id="open" textValue={t('查看公开页')}><ArrowUpRightFromSquare className="size-4 text-muted" /><Label>{t('查看公开页')}</Label></Dropdown.Item> : null}
                            </Dropdown.Menu>
                          </Dropdown.Popover>
                        </Dropdown>
                        <Button isIconOnly aria-label={t('新建子文档')} className="size-6 min-w-6" size="sm" variant="ghost" onPress={() => { expand([doc.documentId]); onCreate(doc.documentId); }}><Plus className="size-3.5" /></Button>
                      </span>
                    )}
                  </div>
                </li>
              );
            })}
          </ul>
        )}
        {draggable && rows.length > 0 && <p className="px-2 pt-3 text-xs text-muted/80">{t('拖动文档可以调整顺序和层级。')}</p>}
      </nav>
      {canTrash && (
        <div className="border-t border-separator p-2">
          <Button className="w-full justify-start" size="sm" variant="ghost" onPress={() => setView('trash')}>
            <TrashBin />{t('回收站')}
          </Button>
        </div>
      )}
      {moving && (
        <MoveDialog
          docs={docs}
          meta={moving}
          onClose={() => setMoving(null)}
          onMove={async (parentId) => { await move(moving.documentId, parentId, null); setMoving(null); }}
        />
      )}
    </div>
  );
}

type Trashed = Meta & { trashedAt?: string | null };

// 回收站：一键恢复到原位置（原上级已不在时回到顶层），沿用原路径；当时一起删除的子文档跟着恢复。
function TrashList({ docs, treeRevision, onBack, onChanged }: { docs: Meta[]; treeRevision: number; onBack: () => void; onChanged: () => Promise<void> }) {
  const isSuperadmin = useHasGrant('superadmin');
  const trash = useLoad((signal) => api<{ documents: Trashed[] }>('/v1/documents?state=trash&limit=100', { signal }).then((result) => result.documents), []);
  const [busy, setBusy] = useState<string | null>(null);
  const items = trash.data ?? [];
  // 一起删除的子文档不单独列出，跟着上级恢复。
  const roots = items.filter((doc) => !items.some((other) => other.documentId === doc.parentId && other.trashedAt === doc.trashedAt));
  const origin = (doc: Trashed) => ancestors([...docs, ...items], doc.documentId).map((item) => item.title).join(' / ');
  const together = (root: Trashed): Trashed[] => {
    const out: Trashed[] = [];
    const walk = (parentId: string) => {
      for (const child of items.filter((item) => item.parentId === parentId && item.trashedAt === root.trashedAt).sort(byOrder)) { out.push(child); walk(child.documentId); }
    };
    walk(root.documentId);
    return out;
  };

  const restoreOne = async (doc: Trashed, parentId: string | null, pathSegment: string | null) => {
    await api(`/v1/documents/${doc.documentId}:restore`, { method: 'POST', body: { expectedRevision: doc.revision, parentId, pathSegment } });
  };

  const restore = async (doc: Trashed) => {
    const parentId = doc.parentId && docs.some((item) => item.documentId === doc.parentId) ? doc.parentId : null;
    setBusy(doc.documentId);
    try {
      try {
        await restoreOne(doc, parentId, null);
      } catch (problem) {
        // 原路径已被同级占用：换一个路径再恢复。
        if (!(problem instanceof ApiError) || problem.code !== 'CONFLICT') throw problem;
        const path = await askText({
          title: t('恢复“{0}”', doc.title), label: t('页面路径'), defaultValue: suggestPath(doc.title),
          description: t('原来的路径已被其他文档使用，换一个再恢复。路径只能用小写字母、数字和连字符。'), confirmLabel: t('恢复'),
          validate: (value) => pathPattern.test(value.trim()) ? null : t('格式不对：只能用小写字母、数字和连字符'),
        });
        if (!path) return;
        await restoreOne(doc, parentId, path.trim());
      }
      const children = together(doc);
      for (const child of children) await restoreOne(child, child.parentId, null);
      const placed = parentId || !doc.parentId ? t('文档已回到原来的位置。') : t('原来的上级已不在，文档回到了顶层。');
      notify.success(t('已恢复为草稿'), [placed, children.length ? t('一起删除的 {0} 篇子文档也已恢复。', children.length) : '', t('需要公开时重新发布。')].join(''));
      trash.reload();
      await onChanged();
    } catch (problem) { notify.error(t('恢复失败'), errorText(problem)); trash.reload(); } finally { setBusy(null); }
  };

  // 彻底删除时连同一起删除的子文档；确认摘要是排序后 ID 列表的 SHA-256。
  const purge = async (doc: Trashed) => {
    const children = together(doc);
    const ok = await confirmAction({
      title: t('彻底删除“{0}”？', doc.title),
      body: children.length ? t('连同 {0} 篇子文档一起彻底删除，无法恢复，公开地址会返回“已删除”。', children.length) : t('彻底删除后无法恢复，公开地址会返回“已删除”。'),
      confirmLabel: t('彻底删除'), tone: 'danger',
    });
    if (!ok) return;
    setBusy(doc.documentId);
    try {
      const ids = [doc, ...children].map((item) => item.documentId).sort();
      await api('/v1/trash:purge', { method: 'POST', body: { documentIds: ids, expectedTrashRevision: treeRevision, confirmationHash: await sha256Hex(ids.join(',')) } });
      notify.success(t('已彻底删除'));
      trash.reload();
      await onChanged();
    } catch (problem) { notify.error(t('删除失败'), errorText(problem)); } finally { setBusy(null); }
  };

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex items-center gap-2 px-3 pb-2 pt-3">
        <Button isIconOnly aria-label={t('返回文档')} size="sm" variant="ghost" onPress={onBack}><ArrowLeft className="rtl:-scale-x-100" /></Button>
        <h2 className="text-sm font-semibold text-foreground">{t('回收站')}</h2>
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto px-3 pb-3">
        {trash.loading && !trash.data ? <div className="flex justify-center py-8"><Spinner size="sm" /></div> : !roots.length ? (
          <EmptyBlock className="mt-2" icon={<TrashBin />} title={t('回收站是空的')} description={t('删除的文档会先放在这里。')} />
        ) : (
          <ul className="flex flex-col gap-2">
            {roots.map((doc) => {
              const where = origin(doc);
              const count = together(doc).length;
              return (
              <li key={doc.documentId} className="rounded-xl border border-border bg-surface p-3">
                <div className="truncate text-sm font-medium text-foreground">{doc.title || t('无标题')}</div>
                <div className="truncate text-xs text-muted">{where ? t('原位置：{0}', where) : t('原位置：顶层')}</div>
                {doc.trashedAt && <div className="text-xs text-muted">{t('删除于 {0}', fmtRelative(doc.trashedAt))}{count ? t('，含 {0} 篇子文档', count) : ''}</div>}
                <div className="mt-2 flex gap-2">
                  <Button isPending={busy === doc.documentId} size="sm" variant="secondary" onPress={() => restore(doc)}>{t('恢复')}</Button>
                  {isSuperadmin && <Button isDisabled={busy === doc.documentId} size="sm" variant="danger-soft" onPress={() => purge(doc)}>{t('彻底删除')}</Button>}
                </div>
              </li>
              );
            })}
          </ul>
        )}
      </div>
    </div>
  );
}
