import { ClockArrowRotateLeft } from '@gravity-ui/icons';
import { Button, Chip, Drawer, Form, Modal } from '@heroui/react';
import { api, ApiError, confirmAction, EmptyBlock, ErrorBlock, fmtDateTime, fmtRelative, LoadingBlock, notify, errorText, SelectInput, TextAreaInput, TextInput, useLoad, t } from '@ui';
import { useEffect, useMemo, useRef, useState } from 'react';
import { flatten, operationLabel, pathPattern, subtree, suggestPath, type HistoryEntry, type Meta } from './types';

const pathHint = t('只用小写字母、数字和连字符，例如 getting-started。发布后仍可在页面设置里修改，旧地址会自动跳转。');

// 首次发布：确定页面路径和搜索摘要。打开时请 AI 按标题和正文建议，用户改过的字段不再覆盖；没有 AI 时手填。
export function PublishDialog({ meta, draftRevisionId, onPublish, onClose }: { meta: Meta; draftRevisionId: string; onPublish: (path: string, seo: string | null) => Promise<void>; onClose: () => void }) {
  const [path, setPath] = useState('');
  const [seo, setSeo] = useState(meta.seoDescription ?? '');
  const [busy, setBusy] = useState(false);
  const [suggesting, setSuggesting] = useState(true);
  const [aiNote, setAiNote] = useState<string | null>(null);
  const touched = useRef({ path: false, seo: false });
  const invalid = path !== '' && !pathPattern.test(path);
  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const suggested = await api<{ pathSegment: string | null; seoDescription: string | null }>(`/v1/documents/${encodeURIComponent(meta.documentId)}:suggest-publication`, { method: 'POST', body: { draftRevisionId } });
        if (cancelled) return;
        if (!touched.current.path) setPath(suggested.pathSegment ?? suggestPath(meta.title));
        if (!touched.current.seo && suggested.seoDescription) setSeo(suggested.seoDescription);
      } catch (problem) {
        if (cancelled) return;
        if (!touched.current.path) setPath(suggestPath(meta.title));
        setAiNote(problem instanceof ApiError && problem.code === 'UNSUPPORTED_CAPABILITY' ? t('还没有为路径名和摘要配置 AI 模型，请手动填写。') : t('AI 暂时没能给出建议，请手动填写。'));
      } finally {
        if (!cancelled) setSuggesting(false);
      }
    })();
    return () => { cancelled = true; };
  }, [meta.documentId, meta.title, draftRevisionId]);
  return (
    <Modal.Backdrop isOpen onOpenChange={(open) => { if (!open) onClose(); }}>
      <Modal.Container size="md">
        <Modal.Dialog>
          <Modal.CloseTrigger aria-label={t('关闭')} />
          <Modal.Header><Modal.Heading>{t('发布“{0}”', meta.title)}</Modal.Heading></Modal.Header>
          <Form className="flex flex-col" onSubmit={async (event) => { event.preventDefault(); if (invalid || !path) return; setBusy(true); await onPublish(path, seo.trim() || null); setBusy(false); }}>
            <Modal.Body className="flex flex-col gap-4">
              {suggesting && <p className="text-sm text-muted">{t('正在用 AI 生成路径和搜索摘要…')}</p>}
              {aiNote && <p className="text-sm text-muted">{aiNote}</p>}
              <TextInput isRequired description={pathHint} error={invalid ? t('格式不对：只能用小写字母、数字和连字符') : null} label={t('页面路径')} value={path} onChange={(value) => { touched.current.path = true; setPath(value.trim().toLowerCase()); }} />
              <TextAreaInput description={t('显示在搜索结果和分享卡片里，可以留空。')} label={t('搜索摘要')} maxLength={300} rows={3} value={seo} onChange={(value) => { touched.current.seo = true; setSeo(value); }} />
            </Modal.Body>
            <Modal.Footer>
              <Button variant="tertiary" onPress={onClose}>{t('取消')}</Button>
              <Button isDisabled={invalid || !path} isPending={busy} type="submit">{t('发布')}</Button>
            </Modal.Footer>
          </Form>
        </Modal.Dialog>
      </Modal.Container>
    </Modal.Backdrop>
  );
}

// 页面设置：标题以外的元数据（路径、搜索摘要）。
export function SettingsDialog({ meta, onSaved, onClose }: { meta: Meta; onSaved: () => Promise<void>; onClose: () => void }) {
  const [path, setPath] = useState(meta.pathSegment ?? '');
  const [seo, setSeo] = useState(meta.seoDescription ?? '');
  const [busy, setBusy] = useState(false);
  const invalid = path !== '' && !pathPattern.test(path);
  const save = async () => {
    setBusy(true);
    try {
      await api(`/v1/documents/${meta.documentId}/metadata`, {
        method: 'PUT', body: { title: meta.title, pathSegment: path || meta.pathSegment, seoDescription: seo.trim() || null, shareImage: meta.shareImage ?? null, expectedRevision: meta.revision },
      });
      notify.success(t('页面设置已保存'));
      await onSaved();
    } catch (problem) {
      notify.error(t('保存失败'), errorText(problem));
    } finally { setBusy(false); }
  };
  return (
    <Modal.Backdrop isOpen onOpenChange={(open) => { if (!open) onClose(); }}>
      <Modal.Container size="md">
        <Modal.Dialog>
          <Modal.CloseTrigger aria-label={t('关闭')} />
          <Modal.Header><Modal.Heading>{t('页面设置')}</Modal.Heading></Modal.Header>
          <Form className="flex flex-col" onSubmit={(event) => { event.preventDefault(); if (!invalid) void save(); }}>
            <Modal.Body className="flex flex-col gap-4">
              <TextInput
                description={meta.pathSegment ? pathHint : t('还没有发布过：路径会在首次发布时确定，也可以现在先填。')}
                error={invalid ? t('格式不对：只能用小写字母、数字和连字符') : null}
                label={t('页面路径')}
                placeholder="getting-started"
                value={path}
                onChange={(value) => setPath(value.trim().toLowerCase())}
              />
              <TextAreaInput description={t('显示在搜索结果和分享卡片里，可以留空。')} label={t('搜索摘要')} maxLength={300} rows={3} value={seo} onChange={setSeo} />
            </Modal.Body>
            <Modal.Footer>
              <Button variant="tertiary" onPress={onClose}>{t('取消')}</Button>
              <Button isDisabled={invalid} isPending={busy} type="submit">{t('保存')}</Button>
            </Modal.Footer>
          </Form>
        </Modal.Dialog>
      </Modal.Container>
    </Modal.Backdrop>
  );
}

// 移动到：可选的新父级是顶层或除自己与后代之外的任一文档。
export function MoveDialog({ meta, docs, onMove, onClose }: { meta: Meta; docs: Meta[]; onMove: (parentId: string | null) => Promise<void>; onClose: () => void }) {
  const options = useMemo(() => {
    const banned = subtree(docs, meta.documentId);
    return [{ value: '__root', label: t('顶层（不放在任何文档下）') }, ...flatten(docs).filter((row) => !banned.has(row.doc.documentId)).map((row) => ({
      value: row.doc.documentId, label: `${'\u3000'.repeat(row.depth)}${row.doc.title}`, text: row.doc.title,
    }))];
  }, [docs, meta.documentId]);
  const [target, setTarget] = useState<string>(meta.parentId ?? '__root');
  const [busy, setBusy] = useState(false);
  return (
    <Modal.Backdrop isOpen onOpenChange={(open) => { if (!open) onClose(); }}>
      <Modal.Container size="sm">
        <Modal.Dialog>
          <Modal.CloseTrigger aria-label={t('关闭')} />
          <Modal.Header><Modal.Heading>{t('移动“{0}”', meta.title)}</Modal.Heading></Modal.Header>
          <Modal.Body>
            <SelectInput description={t('子文档会跟着一起移动；已发布的页面地址会变化，旧地址自动跳转。')} label={t('放到')} options={options} value={target} onChange={setTarget} />
          </Modal.Body>
          <Modal.Footer>
            <Button variant="tertiary" onPress={onClose}>{t('取消')}</Button>
            <Button
              isDisabled={target === (meta.parentId ?? '__root')}
              isPending={busy}
              onPress={async () => { setBusy(true); await onMove(target === '__root' ? null : target); setBusy(false); }}
            >
              {t('移动')}
            </Button>
          </Modal.Footer>
        </Modal.Dialog>
      </Modal.Container>
    </Modal.Backdrop>
  );
}

// 发布历史：每次发布、撤回、回滚各一条；可以把正文回滚到某次发布并重新发布。
export function HistoryDrawer({ documentId, writable, onRollback, onClose }: { documentId: string; writable: boolean; onRollback: (historyId: string) => Promise<void>; onClose: () => void }) {
  const history = useLoad((signal) => api<{ history: HistoryEntry[] }>(`/v1/documents/${documentId}/history?limit=50`, { signal }).then((result) => result.history), [documentId]);
  const rollback = async (entry: HistoryEntry) => {
    const ok = await confirmAction({ title: t('回滚到这个版本？'), body: t('正文会回到 {0} 的发布内容，并作为新的发布上线；当前草稿会被覆盖。', fmtDateTime(entry.createdAt)), confirmLabel: t('回滚并发布'), tone: 'warning' });
    if (ok) await onRollback(entry.historyId);
  };
  return (
    <Drawer.Backdrop isOpen onOpenChange={(open) => { if (!open) onClose(); }}>
      <Drawer.Content placement="right">
        <Drawer.Dialog className="w-full max-w-md">
          <Drawer.CloseTrigger aria-label={t('关闭')} />
          <Drawer.Header><Drawer.Heading>{t('发布历史')}</Drawer.Heading></Drawer.Header>
          <Drawer.Body>
            {history.error ? <ErrorBlock error={history.error} onRetry={history.reload} /> : history.loading ? <LoadingBlock /> : !history.data?.length ? (
              <EmptyBlock icon={<ClockArrowRotateLeft />} title={t('还没有发布记录')} description={t('发布之后，每个版本都会出现在这里。')} />
            ) : (
              <ol className="relative flex flex-col gap-1 border-s border-separator ps-5">
                {history.data.map((entry, index) => (
                  <li key={entry.historyId} className="relative rounded-xl p-3 transition-colors hover:bg-default/60">
                    <span className={`absolute -start-[26px] top-4 size-2.5 rounded-full ${index === 0 ? 'bg-accent' : 'bg-border'}`} />
                    <div className="flex flex-wrap items-center gap-2">
                      <Chip color={entry.operation === 'withdraw' ? 'warning' : entry.operation === 'rollback' ? 'accent' : 'success'} size="sm" variant="soft">{operationLabel[entry.operation] ?? entry.operation}</Chip>
                      <span className="text-sm text-foreground">{entry.authorAccountName}</span>
                      <span className="text-xs text-muted" title={fmtDateTime(entry.createdAt)}>{fmtRelative(entry.createdAt)}</span>
                    </div>
                    {/* 最新一条（没被撤回时）就是线上版本，不必回滚到它。 */}
                    {entry.operation !== 'withdraw' && (index === 0 ? <p className="mt-2 text-xs text-muted">{t('当前线上版本')}</p> : (
                      <Button className="mt-2" isDisabled={!writable} size="sm" variant="secondary" onPress={() => rollback(entry)}>{t('回滚到此版本')}</Button>
                    ))}
                  </li>
                ))}
              </ol>
            )}
          </Drawer.Body>
        </Drawer.Dialog>
      </Drawer.Content>
    </Drawer.Backdrop>
  );
}
