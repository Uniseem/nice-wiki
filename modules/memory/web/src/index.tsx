import { BookOpen, Pencil, Plus } from '@gravity-ui/icons';
import { Button, Card, Chip, Drawer, Form, Tabs } from '@heroui/react';
import { api, EmptyBlock, ErrorBlock, fmtRelative, FormGrid, LoadingBlock, newKey, NumberInput, PageHeader, TextAreaInput, TextInput, useAction, useLoad } from '@ui';
import { useState } from 'react';

type Entry = {
  entryId: string; version: number; title: string; body: string; bodyHash: string; language: string; tags: string[]; priority: number; state: string; updatedAt: string;
  description: string; maximumBytes: number; readonly: boolean; source: string; autoUpdateEnabled: boolean; fileRef: unknown;
};

const maxBytes = 4096;
const bytesOf = (text: string) => new TextEncoder().encode(text).length;

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
  }, entry.state === 'active' ? '已归档' : '已恢复');

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="长期记忆"
        description="写给 AI 的长期背景：术语、语气、禁忌、产品事实等。按优先级进入问答和翻译的上下文；修改不会调用模型。"
        actions={<Button onPress={() => setEditing('new')}><Plus />新建记忆</Button>}
      />
      <Tabs selectedKey={state} onSelectionChange={(key) => setState(key as 'active' | 'archived')}>
        <Tabs.ListContainer>
          <Tabs.List aria-label="记忆状态">
            <Tabs.Tab id="active">使用中<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="archived">已归档<Tabs.Indicator /></Tabs.Tab>
          </Tabs.List>
        </Tabs.ListContainer>
        <Tabs.Panel className="pt-4" id={state}>
          {entries.error ? <ErrorBlock error={entries.error} onRetry={entries.reload} /> : !entries.data ? <LoadingBlock lines={4} /> : !entries.data.length ? (
            <EmptyBlock
              icon={<BookOpen />}
              title={state === 'active' ? '还没有记忆' : '没有归档的记忆'}
              description={state === 'active' ? '比如“产品名统一写作 Nice Wiki”“回答保持简洁、先给结论”。' : undefined}
              action={state === 'active' ? <Button size="sm" onPress={() => setEditing('new')}><Plus />新建记忆</Button> : undefined}
            />
          ) : (
            <div className="grid gap-4 md:grid-cols-2">
              {[...entries.data].sort((a, b) => b.priority - a.priority).map((entry) => (
                <Card key={entry.entryId} className="gap-3 p-5">
                  <div className="flex items-start justify-between gap-3">
                    <Card.Header className="min-w-0 gap-1">
                      <Card.Title className="truncate text-base font-semibold">{entry.title}</Card.Title>
                      {entry.description && <Card.Description className="line-clamp-2">{entry.description}</Card.Description>}
                    </Card.Header>
                    <Chip color={entry.priority >= 80 ? 'accent' : 'default'} size="sm" variant="soft">优先级 {entry.priority}</Chip>
                  </div>
                  <p className="line-clamp-4 whitespace-pre-wrap rounded-xl bg-surface-secondary px-3 py-2 font-mono text-xs leading-5 text-muted">{entry.body}</p>
                  <div className="flex flex-wrap items-center gap-1.5">
                    <Chip size="sm" variant="soft">{entry.language}</Chip>
                    {entry.tags.map((tag) => <Chip key={tag} size="sm" variant="tertiary">#{tag}</Chip>)}
                    {entry.readonly && <Chip color="warning" size="sm" variant="soft">只读</Chip>}
                    <span className="ms-auto text-xs text-muted">更新于 {fmtRelative(entry.updatedAt)}</span>
                  </div>
                  <Card.Footer className="gap-2">
                    {!entry.readonly && entry.state === 'active' && <Button size="sm" variant="secondary" onPress={() => setEditing(entry)}><Pencil />编辑</Button>}
                    <Button isDisabled={busy} size="sm" variant="tertiary" onPress={() => toggleArchive(entry)}>{entry.state === 'active' ? '归档' : '恢复'}</Button>
                  </Card.Footer>
                </Card>
              ))}
            </div>
          )}
        </Tabs.Panel>
      </Tabs>
      {editing && <EntryDrawer entry={editing === 'new' ? null : editing} onClose={() => setEditing(null)} onSaved={() => { setEditing(null); entries.reload(); }} />}
    </div>
  );
}

function EntryDrawer({ entry, onClose, onSaved }: { entry: Entry | null; onClose: () => void; onSaved: () => void }) {
  const [form, setForm] = useState({
    title: entry?.title ?? '', description: entry?.description ?? '', body: entry?.body ?? '', language: entry?.language ?? 'zh-Hans', priority: entry?.priority ?? 50, tags: entry?.tags.join(', ') ?? '',
  });
  const { busy, run } = useAction();
  const size = bytesOf(form.body);

  const save = async () => {
    const done = await run(async () => {
      const tags = form.tags.split(/[,，]/).map((tag) => tag.trim()).filter(Boolean);
      const bodyHash = await sha256(form.body);
      const idempotencyKey = newKey();
      const now = new Date().toISOString().replace(/\.\d+Z$/, 'Z');
      if (entry) {
        await api(`/v1/entries/${entry.entryId}`, { method: 'PUT', idempotencyKey, body: { expectedVersion: entry.version, idempotencyKey, entry: { ...entry, title: form.title, description: form.description, body: form.body, bodyHash, language: form.language, priority: form.priority, tags, updatedAt: now } } });
      } else {
        await api('/v1/entries', {
          method: 'POST', idempotencyKey, body: {
            idempotencyKey, entry: {
              entryId: newKey(), version: 0, title: form.title, body: form.body, bodyHash, language: form.language, tags, priority: form.priority, state: 'active', updatedAt: now,
              description: form.description, maximumBytes: maxBytes, readonly: false, source: 'manual', autoUpdateEnabled: false,
              fileRef: { owner: 'memory', objectId: newKey(), version: 1, sha256: bodyHash, byteSize: bytesOf(form.body), mediaType: 'text/markdown' },
            },
          },
        });
      }
      return true;
    }, '已保存');
    if (done) onSaved();
  };

  return (
    <Drawer.Backdrop isOpen onOpenChange={(open) => { if (!open) onClose(); }}>
      <Drawer.Content placement="right">
        <Drawer.Dialog className="w-full max-w-xl">
          <Drawer.CloseTrigger aria-label="关闭" />
          <Drawer.Header><Drawer.Heading>{entry ? '编辑记忆' : '新建记忆'}</Drawer.Heading></Drawer.Header>
          <Form className="flex min-h-0 flex-1 flex-col" onSubmit={(event) => { event.preventDefault(); if (size <= maxBytes) void save(); }}>
            <Drawer.Body className="flex flex-col gap-4">
              <TextInput isRequired label="标题" maxLength={200} value={form.title} onChange={(title) => setForm({ ...form, title })} />
              <TextInput isRequired description="一句话说明它什么时候有用，例如“所有回答”“涉及价格时”。" label="说明" maxLength={300} value={form.description} onChange={(description) => setForm({ ...form, description })} />
              <TextAreaInput
                isRequired
                description={`${size} / ${maxBytes} 字节，支持 Markdown。`}
                error={size > maxBytes ? `超出 ${size - maxBytes} 字节，请精简` : null}
                label="正文"
                rows={10}
                value={form.body}
                onChange={(body) => setForm({ ...form, body })}
              />
              <FormGrid columns={3}>
                <TextInput label="语言" value={form.language} onChange={(language) => setForm({ ...form, language })} />
                <NumberInput description="越大越优先。" label="优先级" maxValue={100} minValue={1} value={form.priority} onChange={(priority) => setForm({ ...form, priority })} />
                <TextInput description="逗号分隔。" label="标签" value={form.tags} onChange={(tags) => setForm({ ...form, tags })} />
              </FormGrid>
            </Drawer.Body>
            <Drawer.Footer>
              <Button variant="tertiary" onPress={onClose}>取消</Button>
              <Button isDisabled={size > maxBytes} isPending={busy} type="submit">保存</Button>
            </Drawer.Footer>
          </Form>
        </Drawer.Dialog>
      </Drawer.Content>
    </Drawer.Backdrop>
  );
}
