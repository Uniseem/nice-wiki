import { BookOpen, Pencil, Plus } from '@gravity-ui/icons';
import { Button, Card, Chip, Drawer, Form, Tabs } from '@heroui/react';
import { api, EmptyBlock, ErrorBlock, fmtRelative, LoadingBlock, newKey, PageHeader, TextAreaInput, TextInput, useAction, useLoad, t } from '@ui';
import { useState } from 'react';

type Entry = {
  entryId: string; version: number; title: string; body: string; bodyHash: string; language: string; tags: string[]; priority: number; state: string; updatedAt: string;
  description: string; maximumBytes: number; readonly: boolean; source: string; autoUpdateEnabled: boolean; fileRef: unknown;
};

// 记忆就是用户写的纯文本：所有启用的条目都会原样注入每次 AI 调用（问答、翻译、索引、发布时的路径和摘要），不分任务、不分语言。
const maxBytes = 65536;
const bytesOf = (text: string) => new TextEncoder().encode(text).length;
// 没填标题时取正文第一行。
const titleOf = (title: string, body: string) => title.trim() || body.trim().split('\n')[0].slice(0, 60) || t('记忆');

async function sha256(text: string): Promise<string> {
  const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(text));
  return Array.from(new Uint8Array(digest), (b) => b.toString(16).padStart(2, '0')).join('');
}

export default function MemoryPage() {
  const [state, setState] = useState<'active' | 'archived'>('active');
  const entries = useLoad((signal) => api<{ entries: Entry[] }>(`/v1/entries?state=${state}&limit=100`, { signal }).then((result) => result.entries), [state]);
  const [editing, setEditing] = useState<Entry | 'new' | null>(null);
  const { busy, run } = useAction();

  const toggleArchive = (entry: Entry) => run(async () => {
    const idempotencyKey = newKey();
    await api(`/v1/entries/${entry.entryId}/${entry.state === 'active' ? 'archive' : 'restore'}`, { method: 'POST', idempotencyKey, body: { entryId: entry.entryId, expectedVersion: entry.version, idempotencyKey } });
    entries.reload();
  }, entry.state === 'active' ? t('已归档') : t('已恢复'));

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title={t('长期记忆')}
        description={t('写给 AI 的背景知识和要求，纯文本即可。所有使用中的记忆都会原样放进 AI 调用（问答、知识索引等）的最前面；修改不会调用模型。')}
        actions={<Button onPress={() => setEditing('new')}><Plus />{t('新建记忆')}</Button>}
      />
      <Tabs selectedKey={state} onSelectionChange={(key) => setState(key as 'active' | 'archived')}>
        <Tabs.ListContainer>
          <Tabs.List aria-label={t('记忆状态')}>
            <Tabs.Tab id="active">{t('使用中')}<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="archived">{t('已归档')}<Tabs.Indicator /></Tabs.Tab>
          </Tabs.List>
        </Tabs.ListContainer>
        <Tabs.Panel className="pt-4" id={state}>
          {entries.error ? <ErrorBlock error={entries.error} onRetry={entries.reload} /> : !entries.data ? <LoadingBlock lines={4} /> : !entries.data.length ? (
            <EmptyBlock
              icon={<BookOpen />}
              title={state === 'active' ? t('还没有记忆') : t('没有归档的记忆')}
              description={state === 'active' ? t('比如“产品名统一写作 Nice Wiki”“回答保持简洁、先给结论”。') : undefined}
              action={state === 'active' ? <Button size="sm" onPress={() => setEditing('new')}><Plus />{t('新建记忆')}</Button> : undefined}
            />
          ) : (
            <div className="grid gap-4 md:grid-cols-2">
              {[...entries.data].sort((a, b) => b.updatedAt.localeCompare(a.updatedAt)).map((entry) => (
                <Card key={entry.entryId} className="gap-3 p-5">
                  <Card.Header className="min-w-0 gap-1">
                    <Card.Title className="truncate text-base font-semibold">{entry.title}</Card.Title>
                  </Card.Header>
                  <p className="line-clamp-6 whitespace-pre-wrap rounded-xl bg-surface-secondary px-3 py-2 text-sm leading-6 text-foreground/80">{entry.body}</p>
                  <div className="flex flex-wrap items-center gap-1.5">
                    {entry.readonly && <Chip color="warning" size="sm" variant="soft">{t('只读')}</Chip>}
                    <span className="ms-auto text-xs text-muted">{t('更新于 {0}', fmtRelative(entry.updatedAt))}</span>
                  </div>
                  <Card.Footer className="gap-2">
                    {!entry.readonly && entry.state === 'active' && <Button size="sm" variant="secondary" onPress={() => setEditing(entry)}><Pencil />{t('编辑')}</Button>}
                    <Button isDisabled={busy} size="sm" variant="tertiary" onPress={() => toggleArchive(entry)}>{entry.state === 'active' ? t('归档') : t('恢复')}</Button>
                  </Card.Footer>
                </Card>
              ))}
            </div>
          )}
        </Tabs.Panel>
      </Tabs>
      {editing && (
        <EntryDrawer
          entry={editing === 'new' ? null : editing}
          onClose={() => setEditing(null)}
          onSaved={() => { setEditing(null); entries.reload(); }}
        />
      )}
    </div>
  );
}

function EntryDrawer({ entry, onClose, onSaved }: { entry: Entry | null; onClose: () => void; onSaved: () => void }) {
  const [form, setForm] = useState({ title: entry?.title ?? '', body: entry?.body ?? '' });
  const { busy, run } = useAction();
  const size = bytesOf(form.body);

  const save = async () => {
    const saved = await run(async () => {
      const title = titleOf(form.title, form.body);
      const bodyHash = await sha256(form.body);
      const idempotencyKey = newKey();
      const now = new Date().toISOString().replace(/\.\d+Z$/, 'Z');
      if (entry) {
        await api(`/v1/entries/${entry.entryId}`, { method: 'PUT', idempotencyKey, body: { expectedVersion: entry.version, idempotencyKey, entry: { ...entry, title, description: title, body: form.body, bodyHash, maximumBytes: Math.max(entry.maximumBytes, maxBytes), updatedAt: now } } });
      } else {
        await api('/v1/entries', {
          method: 'POST', idempotencyKey, body: {
            idempotencyKey, entry: {
              entryId: newKey(), version: 0, title, body: form.body, bodyHash, language: 'und', tags: [], priority: 50, state: 'active', updatedAt: now,
              description: title, maximumBytes: maxBytes, readonly: false, source: 'manual', autoUpdateEnabled: false,
              fileRef: { owner: 'memory', objectId: newKey(), version: 1, sha256: bodyHash, byteSize: bytesOf(form.body), mediaType: 'text/markdown' },
            },
          },
        });
      }
      return true;
    }, t('已保存'));
    if (saved) onSaved();
  };

  return (
    <Drawer.Backdrop isOpen onOpenChange={(open) => { if (!open) onClose(); }}>
      <Drawer.Content placement="right">
        <Drawer.Dialog className="w-full max-w-xl">
          <Drawer.CloseTrigger aria-label={t('关闭')} />
          <Drawer.Header><Drawer.Heading>{entry ? t('编辑记忆') : t('新建记忆')}</Drawer.Heading></Drawer.Header>
          <Form className="flex min-h-0 flex-1 flex-col" onSubmit={(event) => { event.preventDefault(); if (size <= maxBytes && form.body.trim()) void save(); }}>
            <Drawer.Body className="flex flex-col gap-4">
              <TextInput description={t('可以不填，默认取正文第一行。')} label={t('标题')} maxLength={200} value={form.title} onChange={(title) => setForm({ ...form, title })} />
              <TextAreaInput
                isRequired
                description={t('写给 AI 的内容，纯文本即可，例如产品事实、术语、回答语气和禁忌。')}
                error={size > maxBytes ? t('超出 {0} 字节，请精简', size - maxBytes) : null}
                label={t('正文')}
                rows={16}
                value={form.body}
                onChange={(body) => setForm({ ...form, body })}
              />
            </Drawer.Body>
            <Drawer.Footer>
              <Button variant="tertiary" onPress={onClose}>{t('取消')}</Button>
              <Button isDisabled={size > maxBytes || !form.body.trim()} isPending={busy} type="submit">{t('保存')}</Button>
            </Drawer.Footer>
          </Form>
        </Drawer.Dialog>
      </Drawer.Content>
    </Drawer.Backdrop>
  );
}
