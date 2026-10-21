// 供应方接入与模型获取，参照 opencode（MIT，https://github.com/anomalyco/opencode）的做法：
// 先从 models.dev 目录里选供应方、只填密钥；再从供应方拉取账号可用的模型勾选添加。未复制其源码。
import { ArrowLeft, Magnifier } from '@gravity-ui/icons';
import { Button, Checkbox, Chip, Form, Input, Modal, ScrollShadow, Spinner } from '@heroui/react';
import { api, ErrorBlock, FormGrid, newKey, Notice, NumberInput, SelectInput, TextInput, useAction, useLoad } from '@ui';
import { useMemo, useState, type ReactNode } from 'react';

export type CatalogProvider = { catalogId: string; name: string; baseUrl: string; adapter: string; keyNames: string[]; modelCount: number; popular: number };
type AvailableModel = { providerModelId: string; name: string; capabilities: string[]; contextWindow?: number; maxOutputTokens?: number; inCatalog: boolean; listedByProvider: boolean };
type Discovery = { listed: boolean; listProblem?: string; catalogId?: string; models: AvailableModel[] };
type ExistingProvider = { providerId: string; displayName: string; credentialConfigured: boolean };
type ExistingModel = { modelId: string; providerId: string; providerModelVersion: string };

export const protocolLabels: Record<string, string> = {
  openai_chat: 'OpenAI 兼容（对话与向量）', anthropic_messages: 'Anthropic', gemini: 'Gemini',
  openai_embeddings: 'OpenAI 兼容（仅向量，旧）', gemini_embeddings: 'Gemini 向量（旧）', rerank: '重排序接口',
};
const capabilityLabels: Record<string, string> = { chat: '对话', vision: '视觉', embedding: '向量', rerank: '重排', reasoning: '推理' };
const tokenizers = [
  { value: 'none;tiktoken-go@v0.7.0;cl100k_base', label: 'cl100k_base' },
  { value: 'none;tiktoken-go@v0.7.0;o200k_base', label: 'o200k_base' },
];
const idChars = /[^A-Za-z0-9._:-]+/g;
const fmtTokens = (value?: number) => !value ? '' : value >= 1_000_000 ? `${+(value / 1_000_000).toFixed(1)}M` : value >= 1000 ? `${Math.round(value / 1000)}K` : String(value);
const hostOf = (url: string) => { try { return new URL(url).host; } catch { return url; } };

function Shell({ title, onClose, children, footer, onSubmit }: { title: string; onClose: () => void; children: ReactNode; footer: ReactNode; onSubmit?: () => void }) {
  return (
    <Modal.Backdrop isOpen onOpenChange={(open) => { if (!open) onClose(); }}>
      <Modal.Container scroll="inside" size="lg">
        <Modal.Dialog className="max-h-[85dvh]">
          <Modal.CloseTrigger aria-label="关闭" />
          <Modal.Header><Modal.Heading>{title}</Modal.Heading></Modal.Header>
          <Form className="flex min-h-0 flex-1 flex-col" onSubmit={(event) => { event.preventDefault(); onSubmit?.(); }}>
            <Modal.Body className="flex min-h-0 flex-col gap-4">{children}</Modal.Body>
            <Modal.Footer>{footer}</Modal.Footer>
          </Form>
        </Modal.Dialog>
      </Modal.Container>
    </Modal.Backdrop>
  );
}

function SearchField({ value, onChange, placeholder }: { value: string; onChange: (value: string) => void; placeholder: string }) {
  return (
    <label className="relative block">
      <Magnifier className="pointer-events-none absolute start-3 top-1/2 size-4 -translate-y-1/2 text-muted" />
      <Input aria-label={placeholder} className="w-full ps-9" placeholder={placeholder} value={value} variant="secondary" onChange={(event) => onChange(event.target.value)} />
    </label>
  );
}

// 第一步选目录里的供应方（或自定义），第二步确认地址并填密钥；保存后直接进入“添加模型”。
export function ConnectProviderDialog({ existing, onClose, onConnected }: { existing: ExistingProvider[]; onClose: () => void; onConnected: (providerId: string) => void }) {
  const directory = useLoad((signal) => api<{ providers: CatalogProvider[] }>('/v1/admin/provider-catalog', { signal }).then((result) => result.providers), []);
  const [query, setQuery] = useState('');
  const [picked, setPicked] = useState<CatalogProvider | 'custom' | null>(null);
  const [form, setForm] = useState({ providerId: '', displayName: '', adapter: 'openai_chat', baseUrl: 'https://', secret: '' });
  const { busy, run } = useAction();

  const choose = (item: CatalogProvider | 'custom') => {
    setPicked(item);
    if (item === 'custom') { setForm({ providerId: '', displayName: '', adapter: 'openai_chat', baseUrl: 'https://', secret: '' }); return; }
    // 同一目录供应方接第二个账号时自动加序号，避免 ID 冲突。
    const base = item.catalogId.replace(idChars, '-');
    let providerId = base;
    for (let n = 2; existing.some((provider) => provider.providerId === providerId); n++) providerId = `${base}-${n}`;
    setForm({ providerId, displayName: item.name, adapter: item.adapter, baseUrl: item.baseUrl, secret: '' });
  };

  const list = useMemo(() => {
    const keyword = query.trim().toLowerCase();
    const all = directory.data ?? [];
    const matched = keyword ? all.filter((item) => `${item.name} ${item.catalogId} ${item.baseUrl}`.toLowerCase().includes(keyword)) : all;
    return { popular: keyword ? [] : matched.filter((item) => item.popular > 0), rest: keyword ? matched : matched.filter((item) => item.popular === 0) };
  }, [directory.data, query]);

  const submit = async () => {
    const done = await run(async () => {
      let key = newKey();
      await api(`/v1/admin/providers/${form.providerId}`, {
        method: 'PUT', idempotencyKey: key, body: {
          expectedVersion: 0, idempotencyKey: key, provider: {
            providerId: form.providerId, version: 1, adapter: form.adapter, baseUrl: form.baseUrl.replace(/\/+$/, ''), enabled: true, credentialVersion: 0, connectTimeoutMs: 5000, requestTimeoutMs: 60000,
            displayName: form.displayName || form.providerId, extraHeaderNames: [], extraHeaderSecretRefs: [], concurrencyLimit: 4, streamIdleTimeoutMs: 30000,
          },
        },
      });
      key = newKey();
      await api(`/v1/admin/providers/${form.providerId}/credentials`, { method: 'POST', idempotencyKey: key, body: { providerId: form.providerId, secret: form.secret, expectedVersion: 0, idempotencyKey: key } });
      return true;
    }, '供应方已接入，接着选要用的模型');
    if (done) onConnected(form.providerId);
  };

  if (!picked) {
    return (
      <Shell footer={<Button variant="tertiary" onPress={onClose}>取消</Button>} title="接入供应方" onClose={onClose}>
        <SearchField placeholder="搜索供应方，如 百炼、DeepSeek、OpenRouter" value={query} onChange={setQuery} />
        {directory.error ? <ErrorBlock error={directory.error} onRetry={directory.reload} /> : !directory.data ? <div className="flex justify-center py-10"><Spinner /></div> : (
          <ScrollShadow className="-mx-1 max-h-[55dvh] px-1">
            {list.popular.length > 0 && <Group items={list.popular} label="常用" onPick={choose} />}
            <Group items={list.rest} label={query ? `找到 ${list.rest.length} 个` : '全部'} onPick={choose} />
            <button className="mt-2 w-full rounded-xl border border-dashed border-border px-4 py-3 text-start text-sm hover:bg-default" type="button" onClick={() => choose('custom')}>
              <span className="block font-medium text-foreground">自定义供应方</span>
              <span className="block text-xs text-muted">目录里没有的 OpenAI 兼容接口、自建网关或私有部署</span>
            </button>
          </ScrollShadow>
        )}
      </Shell>
    );
  }

  const keyHint = picked !== 'custom' && picked.keyNames.length ? `通常叫 ${picked.keyNames.join(' / ')}。` : '';
  return (
    <Shell
      footer={<><Button variant="tertiary" onPress={() => setPicked(null)}><ArrowLeft />重新选择</Button><Button isPending={busy} type="submit">接入</Button></>}
      title={picked === 'custom' ? '自定义供应方' : `接入 ${picked.name}`}
      onClose={onClose}
      onSubmit={() => void submit()}
    >
      <FormGrid>
        <TextInput isRequired description="字母、数字和 . _ : -，之后不能改。" label="供应方 ID" pattern="[A-Za-z0-9._:-]+" value={form.providerId} onChange={(providerId) => setForm({ ...form, providerId })} />
        <TextInput label="显示名" value={form.displayName} onChange={(displayName) => setForm({ ...form, displayName })} />
      </FormGrid>
      <SelectInput label="协议" options={['openai_chat', 'anthropic_messages', 'gemini'].map((value) => ({ value, label: protocolLabels[value] }))} value={form.adapter} onChange={(adapter) => setForm({ ...form, adapter })} />
      <TextInput isRequired description="接口根地址；从目录选的一般不用改。" label="地址" pattern="https://.+" type="url" value={form.baseUrl} onChange={(baseUrl) => setForm({ ...form, baseUrl })} />
      <TextInput isRequired autoComplete="off" description={`${keyHint}只在提交时传输一次，保存后加密存放，界面不会再显示。`} label="API 密钥" type="password" value={form.secret} onChange={(secret) => setForm({ ...form, secret })} />
    </Shell>
  );
}

function Group({ label, items, onPick }: { label: string; items: CatalogProvider[]; onPick: (item: CatalogProvider) => void }) {
  if (!items.length) return null;
  return (
    <div className="mb-3">
      <p className="mb-1.5 px-1 text-xs font-medium text-muted">{label}</p>
      <ul className="flex flex-col gap-1">
        {items.map((item) => (
          <li key={item.catalogId}>
            <button className="flex w-full items-center justify-between gap-3 rounded-xl px-3 py-2.5 text-start hover:bg-default" type="button" onClick={() => onPick(item)}>
              <span className="min-w-0">
                <span className="block truncate text-sm font-medium text-foreground">{item.name}</span>
                <span className="block truncate text-xs text-muted">{hostOf(item.baseUrl)}</span>
              </span>
              <span className="shrink-0 text-xs text-muted">{item.modelCount} 个模型</span>
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}

type Choice = { capabilities: string[]; dimension: number; tokenizer: string };

// 拉取供应方账号里可用的模型，勾选后批量添加；向量模型需要填维度。
export function AddModelsDialog({ providers, models, initialProviderId, onClose, onSaved, onManual }: {
  providers: ExistingProvider[]; models: ExistingModel[]; initialProviderId?: string; onClose: () => void; onSaved: () => void; onManual: () => void;
}) {
  const usable = providers.filter((provider) => provider.credentialConfigured);
  const [providerId, setProviderId] = useState(initialProviderId ?? usable[0]?.providerId ?? '');
  const discovery = useLoad((signal) => providerId ? api<Discovery>(`/v1/admin/providers/${encodeURIComponent(providerId)}/available-models`, { signal }) : Promise.resolve(null), [providerId]);
  const [query, setQuery] = useState('');
  const [showUnlisted, setShowUnlisted] = useState(false);
  const [chosen, setChosen] = useState<Record<string, Choice>>({});
  const { busy, run } = useAction();

  const added = new Set(models.filter((model) => model.providerId === providerId).map((model) => model.providerModelVersion));
  const all = discovery.data?.models ?? [];
  const unlistedCount = discovery.data?.listed ? all.filter((model) => !model.listedByProvider).length : 0;
  const keyword = query.trim().toLowerCase();
  const visible = all.filter((model) => (showUnlisted || !discovery.data?.listed || model.listedByProvider) && (!keyword || `${model.name} ${model.providerModelId}`.toLowerCase().includes(keyword)));
  const picked = Object.keys(chosen);

  const toggle = (model: AvailableModel, on: boolean) => setChosen((current) => {
    const next = { ...current };
    if (on) next[model.providerModelId] = { capabilities: model.capabilities.filter((cap) => cap !== 'reasoning'), dimension: 1024, tokenizer: tokenizers[0].value };
    else delete next[model.providerModelId];
    return next;
  });

  const submit = async () => {
    if (!picked.length) return;
    const done = await run(async () => {
      const taken = new Set(models.map((model) => model.modelId));
      for (const id of picked) {
        const model = all.find((item) => item.providerModelId === id)!;
        const choice = chosen[id];
        const embedding = choice.capabilities.includes('embedding');
        // 模型 ID 默认就是模型名；和别的供应方重名时加上供应方前缀。
        let modelId = id.replace(idChars, '-').replace(/^-+|-+$/g, '');
        if (taken.has(modelId)) modelId = `${providerId}.${modelId}`;
        taken.add(modelId);
        const key = newKey();
        await api(`/v1/admin/models/${encodeURIComponent(modelId)}`, {
          method: 'PUT', idempotencyKey: key, body: {
            expectedVersion: 0, idempotencyKey: key, model: {
              modelId, version: 1, providerId, providerModelVersion: id, capabilities: choice.capabilities, dimension: embedding ? choice.dimension : 0,
              preprocessorVersion: embedding ? choice.tokenizer : 'none', maxInputTokens: model.contextWindow ?? (embedding ? 8192 : 32768), maxOutputTokens: embedding ? 0 : model.maxOutputTokens ?? 8192,
              catalogOrigin: 'manual', manualOverride: true, enabled: true,
            },
          },
        });
      }
      return true;
    }, `已添加 ${picked.length} 个模型`);
    if (done) onSaved();
  };

  return (
    <Shell
      footer={<><Button variant="tertiary" onPress={onManual}>手动添加</Button><Button isDisabled={!picked.length} isPending={busy} type="submit">{picked.length ? `添加 ${picked.length} 个模型` : '添加'}</Button></>}
      title="添加模型"
      onClose={onClose}
      onSubmit={() => void submit()}
    >
      {!usable.length ? <Notice status="warning">先接入一个供应方并设置密钥。</Notice> : (
        <>
          <FormGrid>
            <SelectInput label="供应方" options={usable.map((provider) => ({ value: provider.providerId, label: provider.displayName }))} value={providerId} onChange={(value) => { setProviderId(value); setChosen({}); }} />
            <SearchField placeholder="搜索模型" value={query} onChange={setQuery} />
          </FormGrid>
          {discovery.error ? <ErrorBlock error={discovery.error} onRetry={discovery.reload} /> : !discovery.data ? <div className="flex items-center justify-center gap-2 py-10 text-sm text-muted"><Spinner size="sm" />正在向供应方查询可用模型…</div> : (
            <>
              {discovery.data.listed
                ? <p className="text-xs text-muted">以下是这个账号当前能用的模型{discovery.data.catalogId ? '，名称、上下文和能力来自 models.dev 目录' : ''}。{unlistedCount > 0 && <button className="ms-1 text-accent underline-offset-2 hover:underline" type="button" onClick={() => setShowUnlisted(!showUnlisted)}>{showUnlisted ? '只看账号可用的' : `还有 ${unlistedCount} 个目录里有、账号列表里没有的`}</button>}</p>
                : <Notice status="warning" title="没拿到供应方的模型列表">{discovery.data.listProblem ?? '供应方没有返回列表'}。下面是目录里这家供应方的模型，账号不一定都能用；也可以手动添加。</Notice>}
              <ScrollShadow className="-mx-1 max-h-[45dvh] px-1">
                {!visible.length ? <p className="py-8 text-center text-sm text-muted">没有匹配的模型。</p> : (
                  <ul className="flex flex-col gap-1">
                    {visible.map((model) => {
                      const exists = added.has(model.providerModelId);
                      const choice = chosen[model.providerModelId];
                      return (
                        <li key={model.providerModelId} className={`rounded-xl px-3 py-2 ${choice ? 'bg-accent-soft' : 'hover:bg-default'}`}>
                          <Checkbox isDisabled={exists} isSelected={exists || !!choice} onChange={(on) => toggle(model, on)}>
                            <Checkbox.Content className="flex min-w-0 flex-1 items-start gap-3">
                              <Checkbox.Control className="mt-0.5"><Checkbox.Indicator /></Checkbox.Control>
                              <span className="min-w-0 flex-1">
                                <span className="flex flex-wrap items-center gap-x-2 gap-y-1">
                                  <span className="text-sm font-medium text-foreground">{model.name}</span>
                                  {model.name !== model.providerModelId && <span className="font-mono text-xs text-muted">{model.providerModelId}</span>}
                                </span>
                                <span className="mt-1 flex flex-wrap items-center gap-1">
                                  {model.capabilities.map((cap) => <Chip key={cap} size="sm" variant="soft">{capabilityLabels[cap] ?? cap}</Chip>)}
                                  {model.contextWindow ? <span className="text-xs text-muted">上下文 {fmtTokens(model.contextWindow)}</span> : null}
                                  {exists && <Chip color="success" size="sm" variant="soft">已添加</Chip>}
                                  {discovery.data!.listed && !model.listedByProvider && <Chip color="warning" size="sm" variant="soft">账号列表里没有</Chip>}
                                </span>
                              </span>
                            </Checkbox.Content>
                          </Checkbox>
                          {choice?.capabilities.includes('embedding') && (
                            <FormGrid className="mt-2 ps-8">
                              <NumberInput description="向量维度，见供应方文档。" label="维度" minValue={1} value={choice.dimension} onChange={(dimension) => setChosen({ ...chosen, [model.providerModelId]: { ...choice, dimension } })} />
                              <SelectInput description="知识库用它估算分块词元数。" label="分词器" options={tokenizers} value={choice.tokenizer} onChange={(tokenizer) => setChosen({ ...chosen, [model.providerModelId]: { ...choice, tokenizer } })} />
                            </FormGrid>
                          )}
                        </li>
                      );
                    })}
                  </ul>
                )}
              </ScrollShadow>
            </>
          )}
        </>
      )}
    </Shell>
  );
}
