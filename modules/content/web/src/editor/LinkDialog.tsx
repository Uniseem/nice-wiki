import { ArrowUpRightFromSquare, FileText } from '@gravity-ui/icons';
import { Button, cn, Input, Label, Modal, TextField } from '@heroui/react';
import { t } from '@ui';
import { useMemo, useState } from 'react';
import { ancestors, flatten, type Meta } from '../types';

export type LinkChoice = { kind: 'external'; href: string } | { kind: 'internal'; documentId: string; title: string };

const external = /^(https?:|mailto:)/i;

// 插入/编辑链接：同一个输入框既能填外部地址，也能按标题搜索站内文档（站内链接只存文档 ID，移动、改名后仍有效）。
export function LinkDialog({ docs, currentId, initial, editing, onSubmit, onRemove, onClose }: {
  docs: Meta[];
  currentId: string | null;
  initial: { href: string; documentId: string | null };
  editing: boolean;
  onSubmit: (choice: LinkChoice) => void;
  onRemove: () => void;
  onClose: () => void;
}) {
  const [query, setQuery] = useState(initial.href);
  const [picked, setPicked] = useState<string | null>(initial.documentId);
  const [index, setIndex] = useState(0);
  const [stepped, setStepped] = useState(false);
  const isURL = external.test(query.trim());
  const matches = useMemo(() => {
    if (isURL) return [];
    const q = query.trim().toLowerCase();
    const found = flatten(docs).map((row) => row.doc).filter((doc) => doc.documentId !== currentId && (!q || doc.title.toLowerCase().includes(q)));
    // 编辑已有站内链接且还没输入时，原目标排第一个，直接确定不会改掉它。
    const first = !q && picked ? found.filter((doc) => doc.documentId === picked) : [];
    return [...first, ...found.filter((doc) => !first.includes(doc))].slice(0, 8);
  }, [docs, query, isURL, currentId, picked]);
  const target = docs.find((doc) => doc.documentId === picked);
  const invalid = query.trim() !== '' && !isURL && !matches.length && !target;

  // 什么都没输入时只能点列表里的文档或保留原目标，回车不会随手链到列表第一篇。
  const typed = query.trim() !== '' || stepped;
  const submit = (doc?: Meta) => {
    const chosen = doc ?? (isURL ? undefined : typed ? matches[index] : target);
    if (isURL) onSubmit({ kind: 'external', href: query.trim() });
    else if (chosen) onSubmit({ kind: 'internal', documentId: chosen.documentId, title: chosen.title });
  };

  return (
    <Modal.Backdrop isOpen onOpenChange={(open) => { if (!open) onClose(); }}>
      <Modal.Container size="md">
        <Modal.Dialog>
          <Modal.CloseTrigger aria-label={t('关闭')} />
          <Modal.Header><Modal.Heading>{editing ? t('编辑链接') : t('插入链接')}</Modal.Heading></Modal.Header>
          <Modal.Body className="flex flex-col gap-3">
            {target && !query && <p className="flex items-center gap-2 text-sm text-muted"><FileText className="size-4" />{t('当前链接到：{0}', target.title)}</p>}
            <TextField autoFocus aria-label={t('链接地址或文档标题')} className="w-full" value={query} onChange={(value) => { setQuery(value); setIndex(0); setPicked(null); }}>
              <Label>{t('链接地址或文档标题')}</Label>
              <Input
                placeholder={t('粘贴 https:// 地址，或输入标题搜索站内文档')}
                variant="secondary"
                onKeyDown={(event) => {
                  if (event.key === 'ArrowDown' && matches.length) { event.preventDefault(); setStepped(true); setIndex((value) => (value + 1) % matches.length); }
                  if (event.key === 'ArrowUp' && matches.length) { event.preventDefault(); setStepped(true); setIndex((value) => (value + matches.length - 1) % matches.length); }
                  if (event.key === 'Enter' && !event.nativeEvent.isComposing) { event.preventDefault(); submit(); }
                }}
              />
            </TextField>
            {isURL ? (
              <p className="flex items-center gap-2 text-sm text-muted"><ArrowUpRightFromSquare className="size-4" />{t('将链接到外部地址')}</p>
            ) : matches.length ? (
              <ul aria-label={t('站内文档')} className="flex max-h-72 flex-col overflow-y-auto rounded-xl border border-border bg-surface p-1" role="listbox">
                {matches.map((doc, position) => {
                  const trail = ancestors(docs, doc.documentId).map((item) => item.title).join(' / ');
                  return (
                    <li key={doc.documentId} aria-selected={position === index && (typed || !!target)} role="option">
                      <button
                        className={cn('flex w-full items-center gap-2.5 rounded-lg px-2.5 py-2 text-start transition-colors', position === index && (typed || target) ? 'bg-default' : 'hover:bg-default/60')}
                        type="button"
                        onClick={() => submit(doc)}
                        onMouseEnter={() => setIndex(position)}
                      >
                        <FileText className="size-4 shrink-0 text-muted" />
                        <span className="min-w-0 flex-1">
                          <span className="block truncate text-sm text-foreground">{doc.title || t('无标题')}</span>
                          {trail && <span className="block truncate text-xs text-muted">{trail}</span>}
                        </span>
                        {doc.state === 'draft' && <span className="shrink-0 rounded border border-border px-1 text-[10px] leading-4 text-muted">{t('草稿')}</span>}
                      </button>
                    </li>
                  );
                })}
              </ul>
            ) : invalid ? <p className="text-sm text-danger">{t('没有找到这个标题的文档；外部地址要以 http://、https:// 或 mailto: 开头。')}</p> : null}
            <p className="text-xs text-muted">{t('站内链接在公开页按当前语言和文档位置生成地址，文档移动或改名后仍然有效；链接到草稿时，发布前访客看到的是不可点击的文字。')}</p>
          </Modal.Body>
          <Modal.Footer>
            {editing && <Button className="me-auto" variant="danger-soft" onPress={onRemove}>{t('移除链接')}</Button>}
            <Button variant="tertiary" onPress={onClose}>{t('取消')}</Button>
            <Button isDisabled={!isURL && !(typed ? matches[index] : target)} onPress={() => submit()}>{t('确定')}</Button>
          </Modal.Footer>
        </Modal.Dialog>
      </Modal.Container>
    </Modal.Backdrop>
  );
}
