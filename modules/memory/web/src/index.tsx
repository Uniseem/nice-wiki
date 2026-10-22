import { BookOpen, Pencil, Plus } from '@gravity-ui/icons';
import { Button, Card, Chip, Drawer, Form, Tabs } from '@heroui/react';
import { api, CheckGroupInput, EmptyBlock, ErrorBlock, errorText, fmtRelative, FormGrid, LoadingBlock, newKey, NumberInput, PageHeader, SelectInput, siteLanguages, TextAreaInput, TextInput, useAction, useLoad, t } from '@ui';
import { useState } from 'react';

type Entry = {
  entryId: string; version: number; title: string; body: string; bodyHash: string; language: string; tags: string[]; priority: number; state: string; updatedAt: string;
  description: string; maximumBytes: number; readonly: boolean; source: string; autoUpdateEnabled: boolean; fileRef: unknown;
};

type Mapping = { mappingId: string; taskId: string; entryId: string; language: string; order: number; enabled: boolean; revision: number };

// 记忆用在哪些 AI 任务：后台只给几个大类，保存时展开成按任务的映射。翻译目前不读记忆，所以不列出。
const scopes = [
  { id: 'qa', label: t('问答回答'), description: t('访客提问时放在回答模型系统提示的最前面。'), tasks: ['qa.answer'] },
  { id: 'knowledge', label: t('知识索引'), description: t('建索引时生成背景短句和图片描述用；改动后下次建索引会重新生成（会调用模型）。'), tasks: ['knowledge.context', 'knowledge.describe', 'knowledge.describe-visual'] },
];
const scopeTasks = scopes.flatMap((scope) => scope.tasks);

async function loadMappings(signal: AbortSignal): Promise<Map<string, Mapping[]>> {
  const lists = await Promise.all(scopeTasks.map((task) => api<{ mappings: Mapping[] }>(`/v1/mappings?taskId=${encodeURIComponent(task)}`, { signal }).then((result) => result.mappings)));
  const byEntry = new Map<string, Mapping[]>();
  for (const mapping of lists.flat()) byEntry.set(mapping.entryId, [...(byEntry.get(mapping.entryId) ?? []), mapping]);
  return byEntry;
}

const scopesOf = (mappings: Mapping[]) => scopes.filter((scope) => mappings.some((mapping) => mapping.enabled && scope.tasks.includes(mapping.taskId))).map((scope) => scope.id);

// 让条目的任务映射与勾选一致：补上缺的、删掉取消的、语言跟着条目走。
async function syncMappings(entryId: string, language: string, chosen: string[], existing: Mapping[]) {
  const wanted = new Set(scopes.filter((scope) => chosen.includes(scope.id)).flatMap((scope) => scope.tasks));
  for (const mapping of existing) {
    const idempotencyKey = newKey();
    if (!wanted.has(mapping.taskId)) {
      await api(`/v1/mappings/${mapping.mappingId}?expectedRevision=${mapping.revision}`, { method: 'DELETE', idempotencyKey });
    } else if (mapping.language !== language || !mapping.enabled) {
      await api(`/v1/mappings/${mapping.mappingId}`, { method: 'PUT', idempotencyKey, body: { expectedRevision: mapping.revision, idempotencyKey, mapping: { ...mapping, language, enabled: true } } });
    }
    wanted.delete(mapping.taskId);
  }
  for (const taskId of wanted) {
    const idempotencyKey = newKey();
    const mappingId = newKey();
    await api(`/v1/mappings/${mappingId}`, { method: 'PUT', idempotencyKey, body: { expectedRevision: 0, idempotencyKey, mapping: { mappingId, taskId, entryId, language, order: 0, enabled: true, revision: 0 } } });
  }
}

// 问答按访客语言、索引按基准语言挑记忆，只认相同语言或 und（所有语言）；记忆是给模型的指令，默认对所有语言生效。
const languageOptions = (current: string) => [
  { value: 'und', label: t('所有语言') },
  ...siteLanguages.map((language) => ({ value: language.tag, label: `${language.native} (${language.tag})` })),
  ...(current === 'und' || siteLanguages.some((language) => language.tag === current) ? [] : [{ value: current, label: current }]),
];

const maxBytes = 4096;
const bytesOf = (text: string) => new TextEncoder().encode(text).length;

async function sha256(text: string): Promise<string> {
  const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(text));
  return Array.from(new Uint8Array(digest), (b) => b.toString(16).padStart(2, '0')).join('');
}

export default function MemoryPage() {
  const [state, setState] = useState<'active' | 'archived'>('active');
  const entries = useLoad((signal) => api<{ entries: Entry[] }>(`/v1/entries?state=${state}&limit=100`, { signal }).then((result) => result.entries), [state]);
  const mappings = useLoad(loadMappings, []);
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
        description={t('写给 AI 的长期背景：术语、语气、禁忌、产品事实等。按优先级放进所选 AI 任务的系统提示最前面；修改不会调用模型。')}
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
              {[...entries.data].sort((a, b) => b.priority - a.priority).map((entry) => (
                <Card key={entry.entryId} className="gap-3 p-5">
                  <div className="flex items-start justify-between gap-3">
                    <Card.Header className="min-w-0 gap-1">
                      <Card.Title className="truncate text-base font-semibold">{entry.title}</Card.Title>
                      {entry.description && <Card.Description className="line-clamp-2">{entry.description}</Card.Description>}
                    </Card.Header>
                    <Chip color={entry.priority >= 80 ? 'accent' : 'default'} size="sm" variant="soft">{t('优先级 {0}', entry.priority)}</Chip>
                  </div>
                  <p className="line-clamp-4 whitespace-pre-wrap rounded-xl bg-surface-secondary px-3 py-2 font-mono text-xs leading-5 text-muted">{entry.body}</p>
                  <div className="flex flex-wrap items-center gap-1.5">
                    <Chip size="sm" variant="soft">{entry.language === 'und' ? t('所有语言') : entry.language}</Chip>
                    {entry.tags.map((tag) => <Chip key={tag} size="sm" variant="tertiary">#{tag}</Chip>)}
                    {entry.readonly && <Chip color="warning" size="sm" variant="soft">{t('只读')}</Chip>}
                    {mappings.data && (() => {
                      const used = scopesOf(mappings.data.get(entry.entryId) ?? []);
                      return used.length
                        ? scopes.filter((scope) => used.includes(scope.id)).map((scope) => <Chip key={scope.id} color="accent" size="sm" variant="soft">{scope.label}</Chip>)
                        : <Chip color="warning" size="sm" variant="soft">{t('未用于任何任务')}</Chip>;
                    })()}
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
          existing={editing === 'new' ? [] : mappings.data?.get(editing.entryId) ?? (mappings.data ? [] : undefined)}
          onClose={() => setEditing(null)}
          onSaved={() => { setEditing(null); entries.reload(); mappings.reload(); }}
        />
      )}
    </div>
  );
}

// existing 是该条目现有的任务映射；还没读到时为 undefined，先不让保存，免得重复添加。
function EntryDrawer({ entry, existing, onClose, onSaved }: { entry: Entry | null; existing: Mapping[] | undefined; onClose: () => void; onSaved: () => void }) {
  const [form, setForm] = useState({
    title: entry?.title ?? '', description: entry?.description ?? '', body: entry?.body ?? '', language: entry?.language ?? 'und', priority: entry?.priority ?? 50, tags: entry?.tags.join(', ') ?? '',
  });
  const [chosen, setChosen] = useState<string[] | null>(entry ? null : ['qa']);
  const used = chosen ?? (existing ? scopesOf(existing) : []);
  const { busy, run } = useAction();
  const size = bytesOf(form.body);

  const save = async () => {
    const entryId = entry?.entryId ?? newKey();
    const saved = await run(async () => {
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
              entryId, version: 0, title: form.title, body: form.body, bodyHash, language: form.language, tags, priority: form.priority, state: 'active', updatedAt: now,
              description: form.description, maximumBytes: maxBytes, readonly: false, source: 'manual', autoUpdateEnabled: false,
              fileRef: { owner: 'memory', objectId: newKey(), version: 1, sha256: bodyHash, byteSize: bytesOf(form.body), mediaType: 'text/markdown' },
            },
          },
        });
      }
      return true;
    });
    if (!saved) return;
    // 条目已经保存；映射失败也关掉抽屉重新读取，再编辑时按最新状态补齐。
    await run(async () => {
      try {
        await syncMappings(entryId, form.language, used, existing ?? []);
      } catch (problem) {
        throw new Error(t('记忆已保存，但“用于”没有更新：{0}', errorText(problem)));
      }
    }, t('已保存'));
    onSaved();
  };

  return (
    <Drawer.Backdrop isOpen onOpenChange={(open) => { if (!open) onClose(); }}>
      <Drawer.Content placement="right">
        <Drawer.Dialog className="w-full max-w-xl">
          <Drawer.CloseTrigger aria-label={t('关闭')} />
          <Drawer.Header><Drawer.Heading>{entry ? t('编辑记忆') : t('新建记忆')}</Drawer.Heading></Drawer.Header>
          <Form className="flex min-h-0 flex-1 flex-col" onSubmit={(event) => { event.preventDefault(); if (size <= maxBytes && existing) void save(); }}>
            <Drawer.Body className="flex flex-col gap-4">
              <TextInput isRequired label={t('标题')} maxLength={200} value={form.title} onChange={(title) => setForm({ ...form, title })} />
              <TextInput isRequired description={t('一句话说明它什么时候有用，例如“所有回答”“涉及价格时”。')} label={t('说明')} maxLength={300} value={form.description} onChange={(description) => setForm({ ...form, description })} />
              <TextAreaInput
                isRequired
                description={t('{0} / {1} 字节，支持 Markdown。', size, maxBytes)}
                error={size > maxBytes ? t('超出 {0} 字节，请精简', size - maxBytes) : null}
                label={t('正文')}
                rows={10}
                value={form.body}
                onChange={(body) => setForm({ ...form, body })}
              />
              <FormGrid columns={3}>
                <SelectInput description={t('选了某种语言时，只有这种语言的问答（以及基准语言是它时的索引）会用到；一般选“所有语言”。')} label={t('适用语言')} options={languageOptions(form.language)} value={form.language} onChange={(language) => setForm({ ...form, language })} />
                <NumberInput description={t('越大越优先。')} label={t('优先级')} maxValue={100} minValue={1} value={form.priority} onChange={(priority) => setForm({ ...form, priority })} />
                <TextInput description={t('逗号分隔。')} label={t('标签')} value={form.tags} onChange={(tags) => setForm({ ...form, tags })} />
              </FormGrid>
              <CheckGroupInput
                columns={1}
                description={used.length ? undefined : t('不勾选时这条记忆不会进入任何 AI 任务。')}
                isDisabled={!existing}
                label={t('用于')}
                options={scopes.map((scope) => ({ value: scope.id, label: scope.label, description: scope.description }))}
                value={used}
                onChange={setChosen}
              />
            </Drawer.Body>
            <Drawer.Footer>
              <Button variant="tertiary" onPress={onClose}>{t('取消')}</Button>
              <Button isDisabled={size > maxBytes || !existing} isPending={busy} type="submit">{t('保存')}</Button>
            </Drawer.Footer>
          </Form>
        </Drawer.Dialog>
      </Drawer.Content>
    </Drawer.Backdrop>
  );
}
