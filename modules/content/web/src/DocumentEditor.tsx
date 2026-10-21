import { ArrowUpRightFromSquare, CircleCheck, CircleExclamation, ClockArrowRotateLeft, Ellipsis, Eye, FolderArrowRight, Gear, Lock, PaperPlane, TrashBin, ArrowUturnCcwLeft as Withdraw } from '@gravity-ui/icons';
import { Button, Chip, Dropdown, Label, Spinner } from '@heroui/react';
import type { Key } from '@heroui/react';
import { api, confirmAction, errorText, fmtTime, newKey, notify, Notice, useHasGrant, t } from '@ui';
import { useCallback, useEffect, useRef, useState } from 'react';
import { fromBody, toBody, type JSONNode } from './convert';
import { HistoryDrawer, MoveDialog, PublishDialog, SettingsDialog } from './dialogs';
import { Editor } from './editor/Editor';
import type { Draft, DraftRead, Lease, Meta } from './types';

type SaveState = { kind: 'idle' } | { kind: 'dirty' } | { kind: 'saving' } | { kind: 'saved'; at: Date } | { kind: 'error'; message: string };
const tabId = newKey();
const warningText: Record<string, string> = { 'publication.seo_missing': t('没有填写搜索摘要，可以在“页面设置”里补充') };

// 一篇文档的编辑会话：读草稿 → 取编辑锁（别人持有时只读）→ 自动保存草稿 → 发布等操作；卸载时保存并释放锁。
export function DocumentEditor({ documentId, docs, treeRevision, publicPath, onTreeChanged, onClosed, onReopen }: {
  documentId: string;
  docs: Meta[];
  treeRevision: number;
  publicPath: (doc: Meta) => string | null;
  onTreeChanged: () => Promise<void>;
  onClosed: () => void;
  onReopen: () => void;
}) {
  const canPublish = useHasGrant('content.publish');
  const isSuperadmin = useHasGrant('superadmin');
  const [meta, setMeta] = useState<Meta | null>(null);
  const [initial, setInitial] = useState<JSONNode | null>(null);
  const [writable, setWritable] = useState(false);
  const [holder, setHolder] = useState('');
  const [title, setTitle] = useState('');
  const [save, setSave] = useState<SaveState>({ kind: 'idle' });
  const [draftHash, setDraftHash] = useState('');
  const [loadError, setLoadError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const [dialog, setDialog] = useState<'publish' | 'settings' | 'move' | 'history' | null>(null);

  // 计时器和最新状态放在 ref 里，供自动保存与卸载时使用。
  const live = useRef({ draft: null as Draft | null, lease: null as Lease | null, writable: false, pending: null as JSONNode | null, dirty: false, heartbeat: 0, inputs: 0, lastInput: new Date().toISOString(), meta: null as Meta | null });
  const timers = useRef<{ save?: number; heartbeat?: number }>({});
  const saving = useRef<Promise<boolean> | null>(null);

  const flush = useCallback(async (): Promise<boolean> => {
    window.clearTimeout(timers.current.save);
    if (saving.current) await saving.current;
    const state = live.current;
    if (!state.dirty || !state.pending || !state.lease || !state.writable || !state.draft) return true;
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
    setTitle(next.title);
  }, []);

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
        setWritable(acquired.writable);
        setHolder(acquired.writable ? '' : acquired.lease.holderAccountName ?? t('其他人'));
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
    return () => {
      cancelled = true;
      removeEventListener('beforeunload', beforeUnload);
      void flush().then(releaseLock);
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

  const act = async (action: () => Promise<void>) => {
    setBusy(true);
    try { await action(); } catch (problem) { notify.error(errorText(problem)); } finally { setBusy(false); }
  };

  const saveTitle = () => {
    const current = live.current.meta;
    const next = title.trim();
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
    if (!meta.pathSegment) setDialog('publish');
    else void publish(null, null);
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
      await flush();
      await releaseLock();
      await api(`/v1/documents/${documentId}:trash`, { method: 'POST', body: { expectedRevision: meta.revision, expectedTreeRevision: treeRevision, includeSubtree: hasChildren, replacementDocumentId: null } });
      notify.success(t('已移入回收站'));
      await onTreeChanged();
      onClosed();
    });
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
  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex flex-col gap-3 border-b border-separator px-5 pb-3 pt-4 sm:px-6">
        <div className="flex flex-wrap items-start gap-3">
          <input
            aria-label={t('文档标题')}
            className="min-w-0 flex-1 bg-transparent text-2xl font-semibold tracking-tight text-foreground outline-none placeholder:text-muted disabled:opacity-80"
            disabled={!writable}
            maxLength={200}
            placeholder={t('无标题')}
            value={title}
            onBlur={saveTitle}
            onChange={(event) => setTitle(event.target.value)}
            onKeyDown={(event) => { if (event.key === 'Enter') (event.target as HTMLInputElement).blur(); if (event.key === 'Escape') { setTitle(meta.title); (event.target as HTMLInputElement).blur(); } }}
          />
          <div className="flex items-center gap-2">
            <Button isDisabled={busy} size="sm" variant="secondary" onPress={preview}><Eye />{t('预览')}</Button>
            {canPublish && (
              <Button isDisabled={!writable || upToDate} isPending={busy} size="sm" variant={upToDate ? 'secondary' : 'primary'} onPress={startPublish}>
                <PaperPlane />{upToDate ? t('已是最新') : meta.state === 'published' ? t('发布更新') : t('发布')}
              </Button>
            )}
            <Dropdown>
              <Button isIconOnly aria-label={t('更多操作')} size="sm" variant="tertiary"><Ellipsis /></Button>
              <Dropdown.Popover placement="bottom end">
                <Dropdown.Menu
                  disabledKeys={writable ? [] : ['settings', 'move', 'withdraw', 'delete']}
                  onAction={(key: Key) => {
                    if (key === 'settings') setDialog('settings');
                    if (key === 'move') setDialog('move');
                    if (key === 'history') setDialog('history');
                    if (key === 'withdraw') void withdraw();
                    if (key === 'delete') void remove();
                  }}
                >
                  <Dropdown.Item id="settings" textValue={t('页面设置')}><Gear className="size-4 text-muted" /><Label>{t('页面设置')}</Label></Dropdown.Item>
                  <Dropdown.Item id="move" textValue={t('移动到')}><FolderArrowRight className="size-4 text-muted" /><Label>{t('移动到…')}</Label></Dropdown.Item>
                  {canPublish ? <Dropdown.Item id="history" textValue={t('发布历史')}><ClockArrowRotateLeft className="size-4 text-muted" /><Label>{t('发布历史')}</Label></Dropdown.Item> : null}
                  {canPublish && meta.state === 'published' ? <Dropdown.Item id="withdraw" textValue={t('撤回发布')}><Withdraw className="size-4 text-muted" /><Label>{t('撤回发布')}</Label></Dropdown.Item> : null}
                  {canPublish ? <Dropdown.Item id="delete" textValue={t('删除')} variant="danger"><TrashBin className="size-4" /><Label>{t('移入回收站')}</Label></Dropdown.Item> : null}
                </Dropdown.Menu>
              </Dropdown.Popover>
            </Dropdown>
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted">
          <Chip color={meta.state === 'published' ? 'success' : 'default'} size="sm" variant="soft">{meta.state === 'published' ? t('已发布') : t('草稿')}</Chip>
          {meta.state === 'published' && !upToDate && <Chip color="warning" size="sm" variant="soft">{t('有未发布的修改')}</Chip>}
          {meta.pathSegment ? <span>{t('路径 /{0}', meta.pathSegment)}</span> : <span>{t('发布时设置页面路径')}</span>}
          {link && <a className="inline-flex items-center gap-1 text-accent hover:underline" href={link} rel="noopener" target="_blank">{t('查看公开页')}<ArrowUpRightFromSquare className="size-3" /></a>}
          <SaveIndicator state={save} />
        </div>
        {!writable && (
          <Notice status="warning" title={t('只读：{0} 正在编辑这篇文档', holder)}>
            <span className="flex flex-wrap items-center gap-2">{t('对方离开或保存后锁会自动释放，届时重新打开即可编辑。')}
              <Button size="sm" variant="secondary" onPress={onReopen}><Lock />{t('重新尝试')}</Button>
              {isSuperadmin && <Button size="sm" variant="danger-soft" onPress={forceUnlock}>{t('强制解锁')}</Button>}
            </span>
          </Notice>
        )}
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto">
        <Editor doc={initial} editable={writable} onChange={onChange} />
      </div>
      {dialog === 'publish' && <PublishDialog meta={meta} onClose={() => setDialog(null)} onPublish={async (path, seo) => { if (await publish(path, seo)) setDialog(null); }} />}
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
          onRollback={async (historyId) => {
            await act(async () => {
              await releaseLock();
              await api(`/v1/documents/${documentId}/history:rollback`, { method: 'POST', body: { historyId, expectedRevision: live.current.meta!.revision, acknowledgeMissingAssets: false } });
              notify.success(t('已回滚并重新发布'));
              await onTreeChanged();
              onReopen();
            });
          }}
        />
      )}
    </div>
  );
}

function SaveIndicator({ state }: { state: SaveState }) {
  switch (state.kind) {
    case 'dirty': return <span className="inline-flex items-center gap-1">{t('有未保存的修改…')}</span>;
    case 'saving': return <span className="inline-flex items-center gap-1.5"><Spinner color="current" size="sm" />{t('保存中')}</span>;
    case 'saved': return <span className="inline-flex items-center gap-1 text-success"><CircleCheck className="size-3.5" />{t('已保存 {0}', fmtTime(state.at))}</span>;
    case 'error': return <span className="inline-flex items-center gap-1 text-danger" title={state.message}><CircleExclamation className="size-3.5" />{t('保存失败')}</span>;
    default: return null;
  }
}
