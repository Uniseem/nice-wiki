import { ClockArrowRotateLeft } from '@gravity-ui/icons';
import { Button, Chip, Drawer, Form, Modal } from '@heroui/react';
import { api, confirmAction, EmptyBlock, ErrorBlock, fmtDateTime, fmtRelative, LoadingBlock, notify, errorText, SelectInput, TextAreaInput, TextInput, useLoad } from '@ui';
import { useMemo, useState } from 'react';
import { flatten, operationLabel, pathPattern, subtree, suggestPath, type HistoryEntry, type Meta } from './types';

const pathHint = '只用小写字母、数字和连字符，例如 getting-started。发布后仍可在页面设置里修改，旧地址会自动跳转。';

// 首次发布：确定页面路径和搜索摘要。
export function PublishDialog({ meta, onPublish, onClose }: { meta: Meta; onPublish: (path: string, seo: string | null) => Promise<void>; onClose: () => void }) {
  const [path, setPath] = useState(() => suggestPath(meta.title));
  const [seo, setSeo] = useState(meta.seoDescription ?? '');
  const [busy, setBusy] = useState(false);
  const invalid = path !== '' && !pathPattern.test(path);
  return (
    <Modal.Backdrop isOpen onOpenChange={(open) => { if (!open) onClose(); }}>
      <Modal.Container size="md">
        <Modal.Dialog>
          <Modal.CloseTrigger aria-label="关闭" />
          <Modal.Header><Modal.Heading>发布“{meta.title}”</Modal.Heading></Modal.Header>
          <Form className="flex flex-col" onSubmit={async (event) => { event.preventDefault(); if (invalid || !path) return; setBusy(true); await onPublish(path, seo.trim() || null); setBusy(false); }}>
            <Modal.Body className="flex flex-col gap-4">
              <TextInput isRequired description={pathHint} error={invalid ? '格式不对：只能用小写字母、数字和连字符' : null} label="页面路径" value={path} onChange={(value) => setPath(value.trim().toLowerCase())} />
              <TextAreaInput description="显示在搜索结果和分享卡片里，可以留空。" label="搜索摘要" maxLength={300} rows={3} value={seo} onChange={setSeo} />
            </Modal.Body>
            <Modal.Footer>
              <Button variant="tertiary" onPress={onClose}>取消</Button>
              <Button isDisabled={invalid || !path} isPending={busy} type="submit">发布</Button>
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
      notify.success('页面设置已保存');
      await onSaved();
    } catch (problem) {
      notify.error('保存失败', errorText(problem));
    } finally { setBusy(false); }
  };
  return (
    <Modal.Backdrop isOpen onOpenChange={(open) => { if (!open) onClose(); }}>
      <Modal.Container size="md">
        <Modal.Dialog>
          <Modal.CloseTrigger aria-label="关闭" />
          <Modal.Header><Modal.Heading>页面设置</Modal.Heading></Modal.Header>
          <Form className="flex flex-col" onSubmit={(event) => { event.preventDefault(); if (!invalid) void save(); }}>
            <Modal.Body className="flex flex-col gap-4">
              <TextInput
                description={meta.pathSegment ? pathHint : '还没有发布过：路径会在首次发布时确定，也可以现在先填。'}
                error={invalid ? '格式不对：只能用小写字母、数字和连字符' : null}
                label="页面路径"
                placeholder="getting-started"
                value={path}
                onChange={(value) => setPath(value.trim().toLowerCase())}
              />
              <TextAreaInput description="显示在搜索结果和分享卡片里，可以留空。" label="搜索摘要" maxLength={300} rows={3} value={seo} onChange={setSeo} />
            </Modal.Body>
            <Modal.Footer>
              <Button variant="tertiary" onPress={onClose}>取消</Button>
              <Button isDisabled={invalid} isPending={busy} type="submit">保存</Button>
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
    return [{ value: '__root', label: '顶层（不放在任何文档下）' }, ...flatten(docs).filter((row) => !banned.has(row.doc.documentId)).map((row) => ({
      value: row.doc.documentId, label: `${'　'.repeat(row.depth)}${row.doc.title}`, text: row.doc.title,
    }))];
  }, [docs, meta.documentId]);
  const [target, setTarget] = useState<string>(meta.parentId ?? '__root');
  const [busy, setBusy] = useState(false);
  return (
    <Modal.Backdrop isOpen onOpenChange={(open) => { if (!open) onClose(); }}>
      <Modal.Container size="sm">
        <Modal.Dialog>
          <Modal.CloseTrigger aria-label="关闭" />
          <Modal.Header><Modal.Heading>移动“{meta.title}”</Modal.Heading></Modal.Header>
          <Modal.Body>
            <SelectInput description="子文档会跟着一起移动；已发布的页面地址会变化，旧地址自动跳转。" label="放到" options={options} value={target} onChange={setTarget} />
          </Modal.Body>
          <Modal.Footer>
            <Button variant="tertiary" onPress={onClose}>取消</Button>
            <Button
              isDisabled={target === (meta.parentId ?? '__root')}
              isPending={busy}
              onPress={async () => { setBusy(true); await onMove(target === '__root' ? null : target); setBusy(false); }}
            >
              移动
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
    const ok = await confirmAction({ title: '回滚到这个版本？', body: `正文会回到 ${fmtDateTime(entry.createdAt)} 的发布内容，并作为新的发布上线；当前草稿会被覆盖。`, confirmLabel: '回滚并发布', tone: 'warning' });
    if (ok) await onRollback(entry.historyId);
  };
  return (
    <Drawer.Backdrop isOpen onOpenChange={(open) => { if (!open) onClose(); }}>
      <Drawer.Content placement="right">
        <Drawer.Dialog className="w-full max-w-md">
          <Drawer.CloseTrigger aria-label="关闭" />
          <Drawer.Header><Drawer.Heading>发布历史</Drawer.Heading></Drawer.Header>
          <Drawer.Body>
            {history.error ? <ErrorBlock error={history.error} onRetry={history.reload} /> : history.loading ? <LoadingBlock /> : !history.data?.length ? (
              <EmptyBlock icon={<ClockArrowRotateLeft />} title="还没有发布记录" description="发布之后，每个版本都会出现在这里。" />
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
                    {entry.operation !== 'withdraw' && (index === 0 ? <p className="mt-2 text-xs text-muted">当前线上版本</p> : (
                      <Button className="mt-2" isDisabled={!writable} size="sm" variant="secondary" onPress={() => rollback(entry)}>回滚到此版本</Button>
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
