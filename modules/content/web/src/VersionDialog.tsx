import { Button, Chip, Modal } from '@heroui/react';
import { EditorContent, useEditor } from '@tiptap/react';
import { api, ErrorBlock, fmtDateTime, LoadingBlock, useLoad, t } from '@ui';
import { fromBody, type JSONNode } from './convert';
import { DiffHighlight } from './editor/diff';
import { documentExtensions } from './editor/Editor';
import { operationLabel, type HistoryEntry } from './types';

type Diff = { addedBlockIds: string[]; removedBlockIds: string[]; changedBlockIds: string[]; movedBlockIds: string[] };

// 查看某次发布的正文：只读显示，并按块标出这次发布相对上一次新增（绿）和修改（黄）的内容；删除的块不在这个版本里，只给数量。
export function VersionDialog({ documentId, entry, previous, first, onRollback, onClose }: {
  documentId: string;
  entry: HistoryEntry;
  previous: HistoryEntry | null;
  // 是否文档的首次发布（上一次发布不在列表里但确实存在时为 false，此时不做对比）。
  first: boolean;
  onRollback: (() => void) | null;
  onClose: () => void;
}) {
  const data = useLoad(async (signal) => {
    const read = await api<{ body: { content: JSONNode[] } }>(`/v1/documents/${documentId}/history/${entry.historyId}`, { signal });
    const diff = previous
      ? await api<Diff>(`/v1/documents/${documentId}/history:diff`, { method: 'POST', body: { leftRevisionId: previous.reference.publishedRevisionId, rightRevisionId: entry.reference.publishedRevisionId }, signal }).catch(() => null)
      : null;
    return { doc: fromBody(read.body), diff };
  }, [documentId, entry.historyId]);
  const diff = data.data?.diff;
  return (
    <Modal.Backdrop isOpen onOpenChange={(open) => { if (!open) onClose(); }}>
      <Modal.Container scroll="inside" size="lg">
        <Modal.Dialog className="max-h-[90dvh]">
          <Modal.CloseTrigger aria-label={t('关闭')} />
          <Modal.Header className="flex-col items-start gap-2">
            <Modal.Heading>{t('{0}于 {1}', operationLabel[entry.operation] ?? entry.operation, fmtDateTime(entry.createdAt))}</Modal.Heading>
            <div className="flex flex-wrap items-center gap-2 text-xs text-muted">
              <span>{entry.authorAccountName}</span>
              {first ? <span>{t('首次发布，以下是全部内容。')}</span> : !previous ? null : diff ? (
                <>
                  <Chip color="success" size="sm" variant="soft">{t('新增 {0} 块', diff.addedBlockIds.length)}</Chip>
                  <Chip color="warning" size="sm" variant="soft">{t('修改 {0} 块', diff.changedBlockIds.length)}</Chip>
                  {diff.removedBlockIds.length > 0 && <Chip color="danger" size="sm" variant="soft">{t('删除 {0} 块', diff.removedBlockIds.length)}</Chip>}
                  <span>{t('与上一次发布相比')}</span>
                </>
              ) : data.loading ? null : <span>{t('没能取到与上一次发布的对比。')}</span>}
            </div>
          </Modal.Header>
          <Modal.Body>
            {data.error ? <ErrorBlock error={data.error} onRetry={data.reload} /> : !data.data ? <LoadingBlock lines={6} /> : <VersionBody diff={diff ?? null} doc={data.data.doc} />}
          </Modal.Body>
          <Modal.Footer>
            <Button variant="tertiary" onPress={onClose}>{t('关闭')}</Button>
            {onRollback && <Button isDisabled={!data.data} onPress={onRollback}>{t('回滚到此版本')}</Button>}
          </Modal.Footer>
        </Modal.Dialog>
      </Modal.Container>
    </Modal.Backdrop>
  );
}

function VersionBody({ doc, diff }: { doc: JSONNode; diff: Diff | null }) {
  const editor = useEditor({
    editable: false,
    content: doc,
    extensions: [...documentExtensions(), DiffHighlight.configure({ added: new Set(diff?.addedBlockIds ?? []), changed: new Set(diff?.changedBlockIds ?? []) })],
    editorProps: { attributes: { class: 'nw-prose', 'aria-label': t('历史版本正文') } },
  });
  return <EditorContent editor={editor} />;
}
