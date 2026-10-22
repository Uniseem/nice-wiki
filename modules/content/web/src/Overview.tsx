import { CircleCheck, FileText, Pencil, Plus } from '@gravity-ui/icons';
import { Button, Spinner } from '@heroui/react';
import { EmptyBlock, fmtDateTime, fmtRelative, InHeader, useHasGrant, t } from '@ui';
import type { ReactNode } from 'react';
import { ancestors, flatten, liveDocs, type Meta } from './types';

// 没选文档时的概览：篇数统计、最近发布和尚未发布的草稿，点一下就打开。
export function Overview({ docs, leading, onCreate, onSelect }: { docs: Meta[] | null; leading: ReactNode; onCreate: () => void; onSelect: (id: string) => void }) {
  const canEdit = useHasGrant('content.edit');
  const live = docs ? liveDocs(docs) : [];
  const published = live.filter((doc) => doc.state === 'published');
  const recent = [...published].sort((a, b) => (b.lastPublishedAt ?? '').localeCompare(a.lastPublishedAt ?? '')).slice(0, 8);
  // 草稿按文档树顺序列出，和左侧看到的一致。
  const drafts = docs ? flatten(docs).map((row) => row.doc).filter((doc) => doc.state === 'draft').slice(0, 8) : [];

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <InHeader className="flex h-12 shrink-0 items-center gap-2 border-b border-separator px-3">
        {leading}
        <span className="text-sm font-medium text-foreground">{t('概览')}</span>
      </InHeader>
      <div className="min-h-0 flex-1 overflow-y-auto">
        {!docs ? <div className="flex justify-center py-24"><Spinner /></div> : !live.length ? (
          <div className="flex justify-center px-6 py-24">
            <EmptyBlock
              className="w-full max-w-md"
              icon={<FileText />}
              title={t('还没有文档')}
              description={t('新建第一篇文档，写完点“发布”，访客就能看到。编辑会自动保存为草稿。')}
              action={canEdit ? <Button onPress={onCreate}><Plus />{t('新建文档')}</Button> : undefined}
            />
          </div>
        ) : (
          <div className="mx-auto flex w-full max-w-4xl flex-col gap-8 px-6 py-10">
            <div className="flex flex-wrap items-end justify-between gap-4">
              <div>
                <h1 className="text-2xl font-semibold tracking-tight text-foreground">{t('文档')}</h1>
                <p className="mt-1 text-sm text-muted">{t('共 {0} 篇，已发布 {1} 篇，草稿 {2} 篇。从文档树选择一篇开始编辑。', live.length, published.length, live.length - published.length)}</p>
              </div>
              {canEdit && <Button onPress={onCreate}><Plus />{t('新建文档')}</Button>}
            </div>
            <div className="grid gap-6 lg:grid-cols-2">
              <DocList docs={docs} empty={t('还没有发布过文档。')} icon={<CircleCheck className="size-4 text-success" />} items={recent} title={t('最近发布')} onSelect={onSelect}
                meta={(doc) => doc.lastPublishedAt ? <span title={fmtDateTime(doc.lastPublishedAt)}>{fmtRelative(doc.lastPublishedAt)}</span> : null} />
              <DocList docs={docs} empty={t('没有未发布的草稿。')} icon={<Pencil className="size-4 text-muted" />} items={drafts} title={t('尚未发布的草稿')} onSelect={onSelect} />
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function DocList({ title, icon, items, docs, empty, meta, onSelect }: {
  title: string; icon: ReactNode; items: Meta[]; docs: Meta[]; empty: string; meta?: (doc: Meta) => ReactNode; onSelect: (id: string) => void;
}) {
  return (
    <section className="flex flex-col gap-2">
      <h2 className="flex items-center gap-2 px-1 text-sm font-medium text-foreground">{icon}{title}</h2>
      {!items.length ? <p className="rounded-xl border border-dashed border-border px-4 py-6 text-center text-sm text-muted">{empty}</p> : (
        <ul className="flex flex-col overflow-hidden rounded-xl border border-border bg-surface">
          {items.map((doc) => {
            const trail = ancestors(docs, doc.documentId).map((item) => item.title).join(' / ');
            return (
              <li key={doc.documentId} className="border-b border-separator last:border-b-0">
                <button className="flex w-full items-center gap-3 px-4 py-2.5 text-start transition-colors hover:bg-default/60" type="button" onClick={() => onSelect(doc.documentId)}>
                  <FileText className="size-4 shrink-0 text-muted" />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-sm font-medium text-foreground">{doc.title || t('无标题')}</span>
                    {trail && <span className="block truncate text-xs text-muted">{trail}</span>}
                  </span>
                  {meta && <span className="shrink-0 text-xs text-muted">{meta(doc)}</span>}
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}
