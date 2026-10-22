import { ArrowRotateLeft, ArrowUpRightFromSquare, ChevronRight, CircleCheck, CircleExclamation, ClockArrowRotateLeft, Ellipsis, Eye, FolderArrowRight, Gear, Lock, PaperPlane, TrashBin, ArrowUturnCcwLeft as Withdraw } from '@gravity-ui/icons';
import { Button, Chip, Dropdown, Label, Separator, Spinner } from '@heroui/react';
import type { Key } from '@heroui/react';
import type { Editor as TiptapEditor } from '@tiptap/react';
import { api, confirmAction, errorText, fmtDateTime, fmtRelative, fmtTime, newKey, notify, Notice, setNavigationGuard, useHasGrant, t } from '@ui';
import { Fragment, useCallback, useEffect, useLayoutEffect, useRef, useState, type ReactNode } from 'react';
import { fromBody, toBody, type JSONNode } from './convert';
import { HistoryDrawer, MoveDialog, PublishDialog, SettingsDialog } from './dialogs';
import { Editor } from './editor/Editor';
import { ancestors, type Draft, type DraftRead, type HistoryEntry, type Lease, type Meta } from './types';

type SaveState = { kind: 'idle' } | { kind: 'dirty' } | { kind: 'saving' } | { kind: 'saved'; at: Date } | { kind: 'error'; message: string };
const tabId = newKey();
const warningText: Record<string, string> = { 'publication.seo_missing': t('没有填写搜索摘要，可以在“页面设置”里补充') };

// 一篇文档的编辑会话：读草稿 → 取编辑锁（别人持有时只读）→ 自动保存草稿 → 发布等操作；卸载时保存并释放锁。
// 版式参照常见 wiki：顶栏是位置（面包屑）、保存状态和发布操作，标题作为正文的第一行，宽屏右侧显示大纲。
export function DocumentEditor({ documentId, docs, treeRevision, fresh, leading, publicPath, onTreeChanged, onSelect, onClosed, onReopen }: {
  documentId: string;
  docs: Meta[];
  treeRevision: number;
  fresh: boolean;
  leading: ReactNode;
  publicPath: (doc: Meta) => string | null;
  onTreeChanged: () => Promise<void>;
  onSelect: (id: string) => void;
  onClosed: () => void;
  onReopen: () => void;
}) {
  const canPublish = useHasGrant('content.publish');
  const isSuperadmin = useHasGrant('superadmin');
  const [meta, setMeta] = useState<Meta | null>(null);
  const [initial, setInitial] = useState<JSONNode | null>(null);
  // 取锁结果回来之前是 pending：不能编辑，但也不要提示“别人在编辑”。
  const [lock, setLock] = useState<'pending' | 'writable' | 'readonly'>('pending');
  const writable = lock === 'writable';
  const [holder, setHolder] = useState('');
  const [title, setTitle] = useState('');
  const [save, setSave] = useState<SaveState>({ kind: 'idle' });
  const [draftHash, setDraftHash] = useState('');
  const [loadError, setLoadError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const [dialog, setDialog] = useState<'publish' | 'settings' | 'move' | 'history' | null>(null);
  const [publishDraft, setPublishDraft] = useState('');
  const [titleFocus, setTitleFocus] = useState(false);

  // 计时器和最新状态放在 ref 里，供自动保存与卸载时使用。
  const live = useRef({ draft: null as Draft | null, lease: null as Lease | null, writable: false, pending: null as JSONNode | null, dirty: false, heartbeat: 0, inputs: 0, lastInput: new Date().toISOString(), meta: null as Meta | null });
  const timers = useRef<{ save?: number; heartbeat?: number }>({});
  const saving = useRef<Promise<boolean> | null>(null);
  const editorRef = useRef<TiptapEditor | null>(null);
  const titleRef = useRef<HTMLTextAreaElement>(null);
  const editingTitle = useRef(false);
  const cancelTitle = useRef(false);

  const flush = useCallback(async (): Promise<boolean> => {
    window.clearTimeout(timers.current.save);
    if (saving.current) await saving.current;
    const state = live.current;
    if (!state.dirty || !state.pending) return true;
    // 有没存的修改却已没有编辑锁（锁已释放或从未拿到）：存不上，不能当作已保存。
    if (!state.lease || !state.writable || !state.draft) {
      setSave({ kind: 'error', message: t('编辑锁已失效，修改没能保存') });
      return false;
    }
    const sent = state.pending;
    setSave({ kind: 'saving' });
    saving.current = (async () => {
      try {
        const body = toBody(sent);
        const result = await api<{ draft: Draft }>(`/v1/documents/${documentId}/draft`, {
          method: 'PUT', body: {
            body, expectedDraftVersion: state.draft!.draftVersion, expectedDraftRevisionId: state.draft!.revisionId,
            leaseId: state.lease!.leaseId, fencingToken: state.lease!.fencingToken, serverBootId: state.lease!.serverBootId,
          },
        });
        state.draft = result.draft;
        setDraftHash(result.draft.bodyHash);
        if (state.pending === sent) { state.dirty = false; setSave({ kind: 'saved', at: new Date() }); } else setSave({ kind: 'dirty' });
        return true;
      } catch (problem) {
        const message = errorText(problem);
        setSave({ kind: 'error', message });
        notify.error(t('草稿保存失败'), message);
        return false;
      } finally {
        saving.current = null;
      }
    })();
    return saving.current;
  }, [documentId]);

  const releaseLock = useCallback(async () => {
    window.clearInterval(timers.current.heartbeat);
    window.clearTimeout(timers.current.save);
    const state = live.current;
    const held = state.lease;
    state.lease = null;
    if (held && state.writable) {
      try { await api(`/v1/documents/${documentId}/lock`, { method: 'DELETE', body: { leaseId: held.leaseId, serverBootId: held.serverBootId } }); } catch { /* 锁会自行过期 */ }
    }
  }, [documentId]);

  const applyMeta = useCallback((next: Meta, draft?: Draft) => {
    live.current.meta = next;
    if (draft) { live.current.draft = draft; setDraftHash(draft.bodyHash); }
    setMeta(next);
    if (!editingTitle.current) setTitle(next.title);
  }, []);

  // 在文档树里移动、改顺序后元数据版本会变：跟上树里的最新元数据，避免之后改标题/设置时版本冲突。
  useEffect(() => {
    const fromTree = docs.find((doc) => doc.documentId === documentId);
    const current = live.current.meta;
    if (fromTree && current && fromTree.revision > current.revision) applyMeta({ ...current, ...fromTree });
  }, [docs, documentId, applyMeta]);

  const refreshMeta = useCallback(async () => {
    const read = await api<DraftRead>(`/v1/documents/${documentId}/draft`);
    applyMeta(read.metadata, read.draft);
    await onTreeChanged();
    return read;
  }, [documentId, applyMeta, onTreeChanged]);

  useEffect(() => {
    let cancelled = false;
    const state = live.current;
    void (async () => {
      try {
        const read = await api<DraftRead>(`/v1/documents/${documentId}/draft`);
        if (cancelled) return;
        applyMeta(read.metadata, read.draft);
        setInitial(fromBody(read.body as { content: JSONNode[] }));
        const acquired = await api<{ lease: Lease; writable: boolean }>(`/v1/documents/${documentId}/lock`, { method: 'POST', body: { tabId } });
        if (cancelled) {
          if (acquired.writable) void api(`/v1/documents/${documentId}/lock`, { method: 'DELETE', body: { leaseId: acquired.lease.leaseId, serverBootId: acquired.lease.serverBootId } }).catch(() => {});
          return;
        }
        state.lease = acquired.lease;
        state.writable = acquired.writable;
        state.heartbeat = acquired.lease.heartbeatSequence;
        state.inputs = acquired.lease.inputSequence;
        setLock(acquired.writable ? 'writable' : 'readonly');
        setHolder(acquired.writable ? '' : acquired.lease.holderAccountName || t('其他人'));
        timers.current.heartbeat = window.setInterval(async () => {
          if (!state.lease || !state.writable) return;
          state.heartbeat++;
          try {
            const result = await api<{ lease: Lease }>(`/v1/documents/${documentId}/lock`, {
              method: 'PUT', body: {
                leaseId: state.lease.leaseId, fencingToken: state.lease.fencingToken, serverBootId: state.lease.serverBootId,
                lastInputAt: state.lastInput, heartbeatSequence: state.heartbeat, inputSequence: state.inputs,
              },
            });
            state.lease = result.lease;
          } catch (problem) {
            notify.warning(t('编辑锁续期失败'), t('{0}。请保存后刷新页面。', errorText(problem)));
          }
        }, 30000);
      } catch (problem) {
        if (!cancelled) setLoadError(problem);
      }
    })();
    const beforeUnload = (event: BeforeUnloadEvent) => { if (live.current.dirty) event.preventDefault(); };
    addEventListener('beforeunload', beforeUnload);
    // ⌘S / Ctrl+S：立即保存草稿，而不是弹出浏览器的“保存网页”。
    const saveKey = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && !event.altKey && event.key.toLowerCase() === 's') { event.preventDefault(); void flush(); }
    };
    addEventListener('keydown', saveKey);
    // 切换文档或离开页面前先把草稿存好；存不上就留在原文档，除非用户明确放弃这些修改。
    const unguard = setNavigationGuard(async () => {
      if (await flush()) return true;
      const discard = await confirmAction({
        title: t('最近的修改没能保存'), body: t('离开会丢掉这些修改。可以留在这里复制内容，或稍后再试一次。'),
        confirmLabel: t('放弃修改并离开'), cancelLabel: t('留在这里'), tone: 'danger',
      });
      if (discard) state.dirty = false;
      return discard;
    });
    return () => {
      cancelled = true;
      unguard();
      removeEventListener('beforeunload', beforeUnload);
      removeEventListener('keydown', saveKey);
      // 守卫放行后才会卸载；万一仍没存上就不主动放锁，让锁自然过期。
      void flush().then((saved) => { if (saved) void releaseLock(); });
    };
  }, [documentId, applyMeta, flush, releaseLock]);

  const onChange = useCallback((doc: JSONNode) => {
    const state = live.current;
    state.pending = doc;
    state.dirty = true;
    state.inputs++;
    state.lastInput = new Date().toISOString();
    setSave({ kind: 'dirty' });
    window.clearTimeout(timers.current.save);
    timers.current.save = window.setTimeout(() => void flush(), 1500);
  }, [flush]);

  // 标题框随内容自动增高（长标题换行显示，不截断）。
  useLayoutEffect(() => {
    const box = titleRef.current;
    if (!box) return;
    box.style.height = 'auto';
    box.style.height = `${box.scrollHeight}px`;
  }, [title, meta]);

  // 刚新建的文档：拿到编辑锁后聚焦并选中标题，直接输入就是改名。
  useEffect(() => {
    if (!fresh || lock !== 'writable') return;
    titleRef.current?.focus();
    titleRef.current?.select();
  }, [fresh, lock]);

  const act = async (action: () => Promise<void>) => {
    setBusy(true);
    try { await action(); } catch (problem) { notify.error(errorText(problem)); } finally { setBusy(false); }
  };

  const saveTitle = () => {
    editingTitle.current = false;
    if (cancelTitle.current) { cancelTitle.current = false; return; }
    const current = live.current.meta;
    const next = title.replace(/\s+/g, ' ').trim();
    if (!current || !next || next === current.title) { if (current) setTitle(current.title); return; }
    void act(async () => {
      await api(`/v1/documents/${documentId}/metadata`, { method: 'PUT', body: { title: next, pathSegment: current.pathSegment, seoDescription: current.seoDescription, shareImage: current.shareImage ?? null, expectedRevision: current.revision } });
      await refreshMeta();
    });
  };

  const publish = async (path: string | null, seo: string | null) => {
    if (!(await flush())) return false;
    let ok = false;
    await act(async () => {
      const read = await refreshMeta();
      const result = await api<{ warnings?: string[] }>(`/v1/documents/${documentId}:publish`, {
        method: 'POST', body: {
          draftRevisionId: read.draft.revisionId, expectedDraftVersion: read.draft.draftVersion, expectedRevision: read.metadata.revision,
          manualPathSegment: read.metadata.pathSegment ? null : path, manualSEO: seo || null,
        },
      });
      await refreshMeta();
      const warnings = (result.warnings ?? []).map((warning) => warningText[warning] ?? warning);
      notify.success(t('已发布'), warnings.length ? t('提示：{0}', warnings.join(t('；'))) : t('访客现在可以看到最新内容。'));
      ok = true;
    });
    return ok;
  };

  const startPublish = () => {
    if (!meta) return;
    // 自己和上级都有路径时直接发布；否则先在对话框里补齐路径。
    if (meta.pathSegment && ancestors(docs, documentId).every((doc) => doc.pathSegment)) { void publish(null, null); return; }
    // 首次发布：先存好草稿，对话框按这一版正文请 AI 建议路径和摘要。
    void act(async () => {
      if (!(await flush())) return;
      const read = await refreshMeta();
      setPublishDraft(read.draft.revisionId);
      setDialog('publish');
    });
  };

  const preview = () => act(async () => {
    if (!(await flush())) return;
    const draft = live.current.draft!;
    const grant = await api<{ previewId: string; token: string }>('/v1/preview/tokens', { method: 'POST', body: { documentId, draftRevisionId: draft.revisionId } });
    window.open(`/v1/preview/${encodeURIComponent(grant.token)}?previewId=${encodeURIComponent(grant.previewId)}`, '_blank', 'noopener');
  });

  const withdraw = async () => {
    const ok = await confirmAction({ title: t('撤回发布？'), body: t('撤回后公开页将不可访问，草稿保留，可以随时重新发布。'), confirmLabel: t('撤回'), tone: 'warning' });
    if (!ok || !meta) return;
    await act(async () => {
      await api(`/v1/documents/${documentId}:withdraw`, { method: 'POST', body: { expectedRevision: meta.revision } });
      await refreshMeta();
      notify.success(t('已撤回'));
    });
  };

  const remove = async () => {
    if (!meta) return;
    const hasChildren = docs.some((doc) => doc.parentId === documentId && doc.state !== 'trash' && doc.state !== 'purged');
    const ok = await confirmAction({
      title: t('删除“{0}”？', meta.title),
      body: hasChildren ? t('这篇文档含有子文档，会一起移入回收站。已发布的页面会立即失效，可以在回收站恢复。') : t('文档会移入回收站，可以恢复。已发布的页面会立即失效。'),
      confirmLabel: t('移入回收站'), tone: 'danger',
    });
    if (!ok) return;
    await act(async () => {
      if (!(await flush())) return;
      await releaseLock();
      try {
        await api(`/v1/documents/${documentId}:trash`, { method: 'POST', body: { expectedRevision: meta.revision, expectedTreeRevision: treeRevision, includeSubtree: hasChildren, replacementDocumentId: null } });
      } catch (problem) {
        // 锁已经放了：重新打开文档，重新取锁后才能继续编辑和保存。正文刚存过，不会丢。
        onReopen();
        throw problem;
      }
      notify.success(t('已移入回收站'));
      await onTreeChanged();
      onClosed();
    });
  };

  // 回滚到某次发布：草稿换成那次的正文并重新发布。锁要先放掉，之后重新打开文档取新锁。
  const rollback = (historyId: string, done: string) => act(async () => {
    if (!(await flush())) return;
    await releaseLock();
    try {
      await api(`/v1/documents/${documentId}/history:rollback`, { method: 'POST', body: { historyId, expectedRevision: live.current.meta!.revision, acknowledgeMissingAssets: false } });
    } catch (problem) {
      onReopen();
      throw problem;
    }
    notify.success(done);
    await onTreeChanged();
    onReopen();
  });

  // 放弃未发布的修改：回滚到当前线上版本。正文与线上相同，下游按哈希复用，不会重新付费处理。
  const discard = async () => {
    const ok = await confirmAction({ title: t('放弃未发布的修改？'), body: t('草稿会恢复成线上版本的内容，之后的修改会丢失，发布历史里会多一条回滚记录。'), confirmLabel: t('放弃修改'), tone: 'danger' });
    if (!ok) return;
    const history = await api<{ history: HistoryEntry[] }>(`/v1/documents/${documentId}/history?limit=1`).catch((problem) => { notify.error(errorText(problem)); return null; });
    const current = history?.history[0];
    if (!current || current.operation === 'withdraw') { notify.error(t('没有找到线上版本')); return; }
    await rollback(current.historyId, t('已恢复成线上版本'));
  };

  const forceUnlock = async () => {
    const lease = live.current.lease;
    if (!lease) return;
    const ok = await confirmAction({ title: t('强制解除编辑锁？'), body: t('{0} 尚未保存的修改可能会丢失。只在对方确实已经离开时使用。', holder), confirmLabel: t('强制解锁'), tone: 'danger' });
    if (!ok) return;
    await act(async () => {
      await api(`/v1/documents/${documentId}/lock:force-release`, { method: 'POST', body: { expectedLeaseId: lease.leaseId } });
      notify.success(t('已解除编辑锁'));
      onReopen();
    });
  };

  if (loadError) return <div className="p-6"><Notice status="danger" title={t('文档打开失败')}>{errorText(loadError)}</Notice></div>;
  if (!meta || !initial) return <div className="flex flex-1 items-center justify-center"><Spinner /></div>;

  const link = meta.state === 'published' ? publicPath(meta) : null;
  // 草稿正文与线上版本一致（且没有待保存的输入）时，无需再发布。
  const upToDate = meta.state === 'published' && (save.kind === 'idle' || save.kind === 'saved') && draftHash === meta.publishedReference?.contentHash;
  const trail = ancestors(docs, documentId);
  const header = (
    <div className="mb-8">
      {lock === 'readonly' && (
        <Notice className="mb-6" status="warning" title={t('只读：{0} 正在编辑这篇文档', holder)}>
          <span className="flex flex-wrap items-center gap-2">{t('对方离开或保存后锁会自动释放，届时重新打开即可编辑。')}
            <Button size="sm" variant="secondary" onPress={onReopen}><Lock />{t('重新尝试')}</Button>
            {isSuperadmin && <Button size="sm" variant="danger-soft" onPress={forceUnlock}>{t('强制解锁')}</Button>}
          </span>
        </Notice>
      )}
      <textarea
        ref={titleRef}
        aria-label={t('文档标题')}
        className="block w-full resize-none overflow-hidden bg-transparent text-[2rem] font-bold leading-tight tracking-tight text-foreground outline-none placeholder:text-muted/50 disabled:cursor-default"
        disabled={!writable}
        maxLength={200}
        placeholder={t('无标题')}
        rows={1}
        value={title}
        onBlur={() => { setTitleFocus(false); saveTitle(); }}
        onChange={(event) => setTitle(event.target.value.replace(/\n/g, ''))}
        onFocus={() => { editingTitle.current = true; setTitleFocus(true); }}
        onKeyDown={(event) => {
          // 回车确认标题并跳到正文开头；Esc 放弃这次修改。
          if (event.key === 'Enter' && !event.nativeEvent.isComposing) { event.preventDefault(); event.currentTarget.blur(); editorRef.current?.commands.focus('start'); }
          if (event.key === 'Escape') { cancelTitle.current = true; setTitle(meta.title); event.currentTarget.blur(); }
        }}
      />
      <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted">
        {link ? (
          <a className="inline-flex min-w-0 items-center gap-1 hover:text-accent" href={link} rel="noopener" target="_blank" title={t('查看公开页')}>
            <span className="truncate font-mono">{link}</span><ArrowUpRightFromSquare className="size-3 shrink-0" />
          </a>
        ) : <span>{meta.pathSegment ? t('页面路径 /{0}，发布后生效', meta.pathSegment) : t('首次发布时设置页面路径')}</span>}
        {meta.lastPublishedAt && <span title={fmtDateTime(meta.lastPublishedAt)}>{t('最后发布于 {0}', fmtRelative(meta.lastPublishedAt))}</span>}
        {/* 标题属于元数据，不走草稿：已发布文档改标题后公开页立即更新。 */}
        {titleFocus && meta.state === 'published' && <span className="text-warning">{t('已发布文档的标题修改后会立即更新公开页')}</span>}
      </div>
    </div>
  );
  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <header className="flex h-12 shrink-0 items-center gap-2 border-b border-separator px-3">
        {leading}
        <nav aria-label={t('文档位置')} className="flex min-w-0 flex-1 items-center gap-0.5 text-sm">
          {trail.map((doc) => (
            <Fragment key={doc.documentId}>
              <button className="hidden max-w-40 shrink truncate rounded-md px-1.5 py-0.5 text-muted transition-colors hover:bg-default hover:text-foreground @3xl:block" title={doc.title} type="button" onClick={() => onSelect(doc.documentId)}>{doc.title || t('无标题')}</button>
              <ChevronRight className="hidden size-3.5 shrink-0 text-muted @3xl:block rtl:-scale-x-100" />
            </Fragment>
          ))}
          <span className="min-w-0 truncate px-1.5 font-medium text-foreground">{title.trim() || t('无标题')}</span>
        </nav>
        <div className="flex shrink-0 items-center gap-1.5">
          <SaveIndicator state={save} />
          {meta.state !== 'published' ? <Chip size="sm" variant="soft">{t('草稿')}</Chip>
            : upToDate ? <Chip color="success" size="sm" variant="soft">{t('已发布')}</Chip>
              : <Chip color="warning" size="sm" variant="soft">{t('有未发布的修改')}</Chip>}
          <Separator className="mx-1 hidden h-5 @2xl:block" orientation="vertical" />
          <Button aria-label={t('预览')} isDisabled={busy} size="sm" variant="ghost" onPress={preview}><Eye /><span className="hidden @3xl:inline">{t('预览')}</span></Button>
          {canPublish && (
            <Button isDisabled={!writable || upToDate} isPending={busy} size="sm" variant={upToDate ? 'secondary' : 'primary'} onPress={startPublish}>
              <PaperPlane />{upToDate ? t('已是最新') : meta.state === 'published' ? t('发布更新') : t('发布')}
            </Button>
          )}
          <Dropdown>
            <Button isIconOnly aria-label={t('更多操作')} size="sm" variant="ghost"><Ellipsis /></Button>
            <Dropdown.Popover className="min-w-48" placement="bottom end">
              <Dropdown.Menu
                disabledKeys={writable ? [] : ['settings', 'move', 'discard', 'withdraw', 'delete']}
                onAction={(key: Key) => {
                  if (key === 'open' && link) window.open(link, '_blank', 'noopener');
                  if (key === 'settings') setDialog('settings');
                  if (key === 'move') setDialog('move');
                  if (key === 'history') setDialog('history');
                  if (key === 'withdraw') void withdraw();
                  if (key === 'discard') void discard();
                  if (key === 'delete') void remove();
                }}
              >
                {link ? <Dropdown.Item id="open" textValue={t('查看公开页')}><ArrowUpRightFromSquare className="size-4 text-muted" /><Label>{t('查看公开页')}</Label></Dropdown.Item> : null}
                <Dropdown.Item id="settings" textValue={t('页面设置')}><Gear className="size-4 text-muted" /><Label>{t('页面设置')}</Label></Dropdown.Item>
                <Dropdown.Item id="move" textValue={t('移动到')}><FolderArrowRight className="size-4 text-muted" /><Label>{t('移动到…')}</Label></Dropdown.Item>
                {canPublish ? <Dropdown.Item id="history" textValue={t('发布历史')}><ClockArrowRotateLeft className="size-4 text-muted" /><Label>{t('发布历史')}</Label></Dropdown.Item> : null}
                {canPublish && meta.state === 'published' && !upToDate ? <Dropdown.Item id="discard" textValue={t('放弃未发布的修改')}><ArrowRotateLeft className="size-4 text-muted" /><Label>{t('放弃未发布的修改')}</Label></Dropdown.Item> : null}
                {canPublish && meta.state === 'published' ? <Dropdown.Item id="withdraw" textValue={t('撤回发布')}><Withdraw className="size-4 text-muted" /><Label>{t('撤回发布')}</Label></Dropdown.Item> : null}
                {canPublish ? <Dropdown.Item id="delete" textValue={t('删除')} variant="danger"><TrashBin className="size-4" /><Label>{t('移入回收站')}</Label></Dropdown.Item> : null}
              </Dropdown.Menu>
            </Dropdown.Popover>
          </Dropdown>
        </div>
      </header>
      <Editor doc={initial} docs={docs} documentId={documentId} editable={writable} editorRef={editorRef} header={header} toolbar={lock !== 'readonly'} onChange={onChange} />
      {dialog === 'publish' && <PublishDialog docs={docs} meta={meta} draftRevisionId={publishDraft} onClose={() => setDialog(null)} onPublish={async (path, seo) => { if (await publish(path, seo)) setDialog(null); }} />}
      {dialog === 'settings' && <SettingsDialog meta={meta} onClose={() => setDialog(null)} onSaved={async () => { await refreshMeta(); setDialog(null); }} />}
      {dialog === 'move' && (
        <MoveDialog
          docs={docs}
          meta={meta}
          onClose={() => setDialog(null)}
          onMove={(parentId) => act(async () => {
            await flush();
            await api('/v1/tree:move', { method: 'POST', body: { documentId, newParentId: parentId, beforeDocumentId: null, expectedTreeRevision: treeRevision } });
            await refreshMeta();
            setDialog(null);
            notify.success(t('已移动'));
          })}
        />
      )}
      {dialog === 'history' && (
        <HistoryDrawer
          documentId={documentId}
          writable={writable}
          onClose={() => setDialog(null)}
          onRollback={(historyId) => rollback(historyId, t('已回滚并重新发布'))}
        />
      )}
    </div>
  );
}

function SaveIndicator({ state }: { state: SaveState }) {
  const base = 'hidden items-center gap-1 whitespace-nowrap text-xs text-muted @xl:inline-flex';
  switch (state.kind) {
    case 'dirty': return <span className={base}>{t('未保存')}</span>;
    case 'saving': return <span className={base}><Spinner color="current" size="sm" />{t('保存中')}</span>;
    case 'saved': return <span className={base} title={t('已保存 {0}', fmtTime(state.at))}><CircleCheck className="size-3.5 text-success" />{t('已保存')}</span>;
    case 'error': return <span className="inline-flex items-center gap-1 whitespace-nowrap text-xs text-danger" title={state.message}><CircleExclamation className="size-3.5" />{t('保存失败')}</span>;
    default: return null;
  }
}
