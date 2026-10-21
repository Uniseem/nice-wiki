import { ArrowDown, ArrowUp, Key as KeyIcon, Plus, Thunderbolt, Xmark } from '@gravity-ui/icons';
import { Button, Chip, Form, Modal, Table, Tabs } from '@heroui/react';
import {
  api, askText, CheckGroupInput, ErrorBlock, FormGrid, LoadingBlock, Mono, newKey, Notice, notify, NumberInput, PageHeader, SelectInput, TextInput, useAction, useLoad,
} from '@ui';
import { useState, type ReactNode } from 'react';
import { AddModelsDialog, ConnectProviderDialog, protocolLabels } from './connect';

type Money = { currency: string; micros: number };
type Provider = { providerId: string; displayName: string; adapter: string; baseUrl: string; credentialConfigured: boolean; credentialVersion: number };
type Model = { modelId: string; version: number; providerId: string; providerModelVersion: string; capabilities: string[]; dimension: number; preprocessorVersion: string; maxInputTokens: number; maxOutputTokens: number; catalogOrigin: string; manualOverride: boolean; enabled: boolean };
type Profile = { profileId: string; version: number; costClass: string; tier: string; orderedModelIds: string[]; maximumCost: Money; dailyLimitCost: Money; concurrency: number };
type Assignment = { taskId: string; tier: string; version: number; overrideModelId?: string };
type Catalog = { providers: Provider[]; models: Model[]; profiles: Profile[]; taskAssignments: Assignment[] };

const tiers: Record<string, string> = { fast: '快速', accurate: '准确', vision: '视觉', embedding: '向量', rerank: '重排', decision: '决策' };
// 计费暂停：费用类别在界面上只当“用途”展示，不再有价格和费用上限。
const costClasses: Record<string, string> = { qa: '访客问答', background: '后台任务', 'admin-probe': '连接测试' };
const capabilities = [
  { value: 'chat', label: '对话', description: '回答、翻译、分类等文本生成' },
  { value: 'vision', label: '视觉', description: '读图、描述画板和封面' },
  { value: 'embedding', label: '向量', description: '知识库检索用的嵌入' },
  { value: 'rerank', label: '重排', description: '检索结果重排序' },
  { value: 'decision', label: '决策', description: '只返回带概率的选择结果，不生成文字（如 OpenRouter 上的 typesafe/jev-1.14）' },
];
// 目录里可能出现、但添加时不需要手选的能力，只用于显示。
const extraCapabilityLabels: Record<string, string> = { structured: '结构化输出', reasoning: '推理', translation: '翻译', file: '文件' };
// 知识库按分词器估算分块的词元数，只认这两种。
const tokenizers = [
  { value: 'none;tiktoken-go@v0.7.0;cl100k_base', label: 'cl100k_base', description: '多数向量模型可用的近似计数' },
  { value: 'none;tiktoken-go@v0.7.0;o200k_base', label: 'o200k_base', description: 'OpenAI 新一代模型的分词' },
];
// 任务目录与 contracts/ai-tasks.json 一致（21 项）；新增任务时这里要同步。
const tasks: [string, string, string][] = [
  ['qa.preflight', 'fast', '前置分类、改写、完整性与风险'], ['qa.inspect', 'fast', '检索核查'], ['qa.answer', 'accurate', '最终回答'],
  ['qa.cache-match', 'fast', '缓存同义复用判断'], ['qa.cache-translate', 'fast', '缓存回答翻译'], ['qa.compact', 'fast', '对话增量压缩'],
  ['knowledge.context', 'fast', '检索单元背景上下文'], ['knowledge.describe', 'fast', '文字附件/视频公开元信息描述'],
  ['knowledge.describe-visual', 'vision', '图片、画板、视频封面视觉描述'], ['knowledge.extract-attachment', 'vision', '非文本附件内容提取'],
  ['knowledge.embed', 'embedding', '生成向量'], ['knowledge.rerank', 'rerank', '召回重排'],
  ['localization.document', 'accurate', '块级文档翻译'], ['localization.ui', 'accurate', '站点/界面文字翻译'], ['localization.terms', 'accurate', '首次文档术语抽取'],
  ['localization.review', 'accurate', '可选反思润色'], ['localization.suggest', 'decision', '语言切换建议'],
  ['memory.overview', 'fast', '项目概述维护'], ['memory.assistant', 'accurate', '记忆助手'],
  ['content.slug', 'fast', '首次发布路径名'], ['content.seo', 'fast', '首次发布 SEO 描述'],
];
const tierCapability: Record<string, string> = { fast: 'chat', accurate: 'chat', vision: 'vision', embedding: 'embedding', rerank: 'rerank', decision: 'decision' };
const idPattern = '[A-Za-z0-9._:-]+';

export default function ModelsPage() {
  const catalog = useLoad((signal) => api<Catalog>('/v1/admin/catalog', { signal }), []);
  const [tab, setTab] = useState('providers');
  const [dialog, setDialog] = useState<'provider' | 'models' | 'manual-model' | Profile | 'profile' | null>(null);
  const [modelsProvider, setModelsProvider] = useState<string | undefined>();
  const { busy, run } = useAction();

  const autoSetup = async () => {
    const message = await run(async () => {
      const c = catalog.data!;
      const usable = c.models.filter((model) => model.enabled);
      const byCap = (cap: string) => usable.filter((model) => model.capabilities.includes(cap)).map((model) => model.modelId);
      const tierModels: Record<string, string[]> = { fast: byCap('chat'), accurate: byCap('chat'), vision: byCap('vision'), embedding: byCap('embedding'), rerank: byCap('rerank'), decision: byCap('decision') };
      let profiles = 0, assignments = 0;
      for (const costClass of ['qa', 'background']) {
        for (const [tier, ids] of Object.entries(tierModels)) {
          const profileId = `${costClass}-${tier}`;
          if (!ids.length || c.profiles.some((profile) => profile.costClass === costClass && profile.tier === tier)) continue;
          const idempotencyKey = newKey();
          await api(`/v1/admin/profiles/${profileId}`, { method: 'PUT', idempotencyKey, body: { expectedVersion: 0, idempotencyKey, profile: { profileId, version: 1, costClass, tier, orderedModelIds: ids, maximumCost: { currency: 'USD', micros: 0 }, concurrency: 4, dailyLimitCost: { currency: 'USD', micros: 0 } } } });
          profiles++;
        }
      }
      for (const [taskId, tier] of tasks) {
        // 即使该级别暂时没有模型也要分配：检索等流程会解析每个任务的分配，缺了会整轮失败；没有档位时按“该能力不可用”降级。
        if (c.taskAssignments.some((assignment) => assignment.taskId === taskId)) continue;
        const idempotencyKey = newKey();
        await api(`/v1/task-assignments/${taskId}`, { method: 'PUT', idempotencyKey, body: { expectedVersion: 0, idempotencyKey, assignment: { taskId, tier, version: 1 } } });
        assignments++;
      }
      catalog.reload();
      return `已生成 ${profiles} 个档位、${assignments} 个任务分配`;
    });
    if (message) notify.success(message, '已有的档位和分配不会被覆盖。');
  };

  const saveSecret = async (provider: Provider) => {
    const secret = await askText({ title: provider.credentialConfigured ? `轮换“${provider.displayName}”的密钥` : `设置“${provider.displayName}”的密钥`, label: 'API 密钥', type: 'password', description: '只在提交时传输一次，保存后加密存放，界面不会再显示。', confirmLabel: '保存密钥' });
    if (!secret) return;
    await run(async () => {
      const idempotencyKey = newKey();
      await api(`/v1/admin/providers/${provider.providerId}/credentials${provider.credentialConfigured ? '/rotate' : ''}`, { method: 'POST', idempotencyKey, body: { providerId: provider.providerId, secret, expectedVersion: provider.credentialVersion, idempotencyKey } });
      catalog.reload();
    }, '密钥已加密保存');
  };

  const toggleModel = (model: Model) => run(async () => {
    const idempotencyKey = newKey();
    await api(`/v1/admin/models/${encodeURIComponent(model.modelId)}`, { method: 'PUT', idempotencyKey, body: { expectedVersion: model.version, idempotencyKey, model: { ...model, version: model.version + 1, enabled: !model.enabled } } });
    catalog.reload();
  }, model.enabled ? '模型已停用，档位里不会再选它' : '模型已启用');

  const assign = (task: [string, string, string], modelId: string) => run(async () => {
    const [taskId, tier] = task;
    const existing = catalog.data!.taskAssignments.find((item) => item.taskId === taskId);
    const idempotencyKey = newKey();
    await api(`/v1/task-assignments/${taskId}`, { method: 'PUT', idempotencyKey, body: { expectedVersion: existing?.version ?? 0, idempotencyKey, assignment: { taskId, tier, version: (existing?.version ?? 0) + 1, ...(modelId ? { overrideModelId: modelId } : {}) } } });
    catalog.reload();
  }, '任务分配已保存');

  if (catalog.error && !catalog.data) return <ErrorBlock error={catalog.error} onRetry={catalog.reload} />;
  const c = catalog.data;
  const modelName = (id: string) => c?.models.find((model) => model.modelId === id)?.providerModelVersion ?? id;
  const ready = !!c && c.providers.some((provider) => provider.credentialConfigured) && c.models.length > 0 && c.taskAssignments.length >= tasks.length;

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="模型接入"
        description="配置供应方、密钥、模型、档位和任务分配。这里只做配置，不调用模型。"
        actions={c && <Button isDisabled={!c.models.length} isPending={busy} onPress={() => void autoSetup()}><Thunderbolt />一键生成档位和任务分配</Button>}
      />
      {!c ? <LoadingBlock lines={6} /> : (
        <>
          <Steps items={[
            ['添加供应方并设置密钥', c.providers.some((provider) => provider.credentialConfigured)],
            ['添加模型', c.models.length > 0],
            ['生成档位和任务分配', c.profiles.length > 0 && c.taskAssignments.length >= tasks.length],
          ]} />
          {ready && <Notice status="success" title="模型已就绪">接下来可以去“知识索引”建立索引，再到“问答”开启公开问答。</Notice>}
          <Tabs selectedKey={tab} onSelectionChange={(key) => setTab(String(key))}>
            <Tabs.ListContainer className="mb-4 overflow-x-auto">
              <Tabs.List aria-label="模型配置">
                <Tabs.Tab id="providers">供应方 <Count n={c.providers.length} /><Tabs.Indicator /></Tabs.Tab>
                <Tabs.Tab id="models">模型 <Count n={c.models.length} /><Tabs.Indicator /></Tabs.Tab>
                <Tabs.Tab id="profiles">档位 <Count n={c.profiles.length} /><Tabs.Indicator /></Tabs.Tab>
                <Tabs.Tab id="tasks">任务分配 <Count n={c.taskAssignments.length} /><Tabs.Indicator /></Tabs.Tab>
              </Tabs.List>
            </Tabs.ListContainer>
            <Tabs.Panel id="providers">
              <Section action={<Button variant="secondary" onPress={() => setDialog('provider')}><Plus />接入供应方</Button>} hint="从 models.dev 目录里选供应方，只需填密钥；同一个供应方的对话和向量模型共用一个接入。">
                <DataTable label="供应方" columns={['名称', '协议', '地址', '密钥', '']} empty="还没有供应方。先添加一个，再设置密钥。" rows={c.providers.map((provider) => [
                  provider.providerId,
                  <span><span className="block font-medium">{provider.displayName}</span><Mono>{provider.providerId}</Mono></span>,
                  protocolLabels[provider.adapter] ?? provider.adapter,
                  <span className="break-all text-sm text-muted">{provider.baseUrl}</span>,
                  provider.credentialConfigured ? <Chip color="success" size="sm" variant="soft">已配置 · v{provider.credentialVersion}</Chip> : <Chip color="warning" size="sm" variant="soft">未配置</Chip>,
                  <Button isDisabled={busy} size="sm" variant="secondary" onPress={() => saveSecret(provider)}><KeyIcon />{provider.credentialConfigured ? '轮换' : '设置密钥'}</Button>,
                ])} />
              </Section>
            </Tabs.Panel>
            <Tabs.Panel id="models">
              <Section action={<Button isDisabled={!c.providers.length} variant="secondary" onPress={() => { setModelsProvider(undefined); setDialog('models'); }}><Plus />添加模型</Button>} hint="从供应方拉取账号可用的模型勾选添加；能力决定它能进入哪个级别的档位。停用的模型不会再被选进档位。">
                <DataTable label="模型" columns={['模型', '供应方', '能力', '上下文 / 输出', '状态', '']} empty="还没有模型。" rows={c.models.map((model) => [
                  model.modelId,
                  <span><span className="block font-medium">{model.providerModelVersion}</span><Mono>{model.modelId}</Mono></span>,
                  c.providers.find((provider) => provider.providerId === model.providerId)?.displayName ?? model.providerId,
                  <span className="flex flex-wrap gap-1">{model.capabilities.map((cap) => <Chip key={cap} size="sm" variant="soft">{capabilities.find((item) => item.value === cap)?.label ?? extraCapabilityLabels[cap] ?? cap}</Chip>)}</span>,
                  <span className="tabular-nums">{model.maxInputTokens.toLocaleString()} / {model.maxOutputTokens.toLocaleString()}</span>,
                  <Chip color={model.enabled ? 'success' : 'default'} size="sm" variant="soft">{model.enabled ? '启用' : '停用'}</Chip>,
                  <Button isDisabled={busy} size="sm" variant="tertiary" onPress={() => void toggleModel(model)}>{model.enabled ? '停用' : '启用'}</Button>,
                ])} />
              </Section>
            </Tabs.Panel>
            <Tabs.Panel id="profiles">
              <Section action={<Button isDisabled={!c.models.length} variant="secondary" onPress={() => setDialog('profile')}><Plus />新建档位</Button>} hint="档位把模型按“快速/准确/视觉/向量/重排”分级并区分用途（访客问答或后台任务）；同一档位里排第一的模型为主，其余是备用。">
                <DataTable label="档位" columns={['档位', '用途', '级别', '模型（按顺序）', '']} empty="还没有档位。可以点右上角一键生成。" rows={c.profiles.map((profile) => [
                  profile.profileId,
                  <Mono>{profile.profileId}</Mono>,
                  costClasses[profile.costClass] ?? profile.costClass,
                  <Chip size="sm" variant="soft">{tiers[profile.tier] ?? profile.tier}</Chip>,
                  <span className="text-sm">{profile.orderedModelIds.map(modelName).join(' → ')}</span>,
                  <Button size="sm" variant="tertiary" onPress={() => setDialog(profile)}>编辑</Button>,
                ])} />
              </Section>
            </Tabs.Panel>
            <Tabs.Panel id="tasks">
              <Section hint="每个 AI 任务默认走对应级别的档位；需要时可以为单个任务指定某个模型。">
                <DataTable label="任务分配" columns={['任务', '默认级别', '指定模型', '状态']} empty="" rows={tasks.map((task) => {
                  const current = c.taskAssignments.find((item) => item.taskId === task[0]);
                  return [
                    task[0],
                    <span><span className="block">{task[2]}</span><Mono>{task[0]}</Mono></span>,
                    <Chip size="sm" variant="soft">{tiers[task[1]]}</Chip>,
                    <SelectInput aria-label={`${task[2]}指定模型`} className="w-56" isDisabled={busy} options={[{ value: '__tier', label: '按档位' }, ...c.models.map((model) => ({ value: model.modelId, label: model.providerModelVersion }))]} value={current?.overrideModelId ?? '__tier'} onChange={(value) => void assign(task, value === '__tier' ? '' : value)} variant="primary" />,
                    current ? <Chip color="success" size="sm" variant="soft">已分配</Chip> : <Chip color="warning" size="sm" variant="soft">未分配</Chip>,
                  ];
                })} />
              </Section>
            </Tabs.Panel>
          </Tabs>
        </>
      )}
      {dialog === 'provider' && c && <ConnectProviderDialog existing={c.providers} onClose={() => setDialog(null)} onConnected={(providerId) => { catalog.reload(); setModelsProvider(providerId); setDialog('models'); }} />}
      {dialog === 'models' && c && <AddModelsDialog initialProviderId={modelsProvider} models={c.models} providers={c.providers} onChanged={catalog.reload} onClose={() => setDialog(null)} onManual={() => setDialog('manual-model')} onSaved={() => { setDialog(null); catalog.reload(); setTab('models'); }} />}
      {dialog === 'manual-model' && c && <ModelDialog providers={c.providers} onClose={() => setDialog(null)} onSaved={() => { setDialog(null); catalog.reload(); setTab('models'); }} />}
      {(dialog === 'profile' || (dialog && typeof dialog === 'object')) && c && (
        <ProfileDialog models={c.models} profile={typeof dialog === 'object' ? dialog : null} onClose={() => setDialog(null)} onSaved={() => { setDialog(null); catalog.reload(); }} />
      )}
    </div>
  );
}

function Count({ n }: { n: number }) {
  return <span className="ms-1.5 rounded-full bg-default px-1.5 text-xs tabular-nums text-muted">{n}</span>;
}

function Steps({ items }: { items: [string, boolean][] }) {
  return (
    <ol className="grid gap-3 sm:grid-cols-3">
      {items.map(([label, done], index) => (
        <li key={label} className={`flex items-center gap-3 rounded-2xl border px-4 py-3 ${done ? 'border-success/30 bg-success-soft' : 'border-border bg-surface'}`}>
          <span className={`flex size-7 shrink-0 items-center justify-center rounded-full text-xs font-semibold ${done ? 'bg-success text-success-foreground' : 'bg-default text-muted'}`}>{done ? '✓' : index + 1}</span>
          <span className="text-sm font-medium text-foreground">{label}</span>
        </li>
      ))}
    </ol>
  );
}

function Section({ hint, action, children }: { hint: string; action?: ReactNode; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="max-w-3xl text-sm text-muted">{hint}</p>
        {action}
      </div>
      {children}
    </div>
  );
}

function DataTable({ label, columns, rows, empty }: { label: string; columns: string[]; rows: (string | ReactNode)[][]; empty: string }) {
  return (
    <Table>
      <Table.ScrollContainer>
        <Table.Content aria-label={label} className="min-w-[640px]">
          <Table.Header>
            {columns.map((column, index) => <Table.Column key={index} isRowHeader={index === 0} className={index === columns.length - 1 && !column ? 'text-end' : undefined}>{column}</Table.Column>)}
          </Table.Header>
          <Table.Body renderEmptyState={() => <p className="py-8 text-center text-sm text-muted">{empty}</p>}>
            {rows.map(([id, ...cells]) => (
              <Table.Row key={String(id)} id={String(id)}>
                {cells.map((cell, index) => <Table.Cell key={index} className={index === cells.length - 1 && !columns[index] ? 'text-end' : undefined}>{cell}</Table.Cell>)}
              </Table.Row>
            ))}
          </Table.Body>
        </Table.Content>
      </Table.ScrollContainer>
    </Table>
  );
}

function Dialog({ title, onClose, onSubmit, busy, submitLabel, children }: { title: string; onClose: () => void; onSubmit: () => void; busy: boolean; submitLabel: string; children: ReactNode }) {
  return (
    <Modal.Backdrop isOpen onOpenChange={(open) => { if (!open) onClose(); }}>
      <Modal.Container scroll="outside" size="lg">
        <Modal.Dialog>
          <Modal.CloseTrigger aria-label="关闭" />
          <Modal.Header><Modal.Heading>{title}</Modal.Heading></Modal.Header>
          <Form className="flex flex-col" onSubmit={(event) => { event.preventDefault(); onSubmit(); }}>
            <Modal.Body className="flex flex-col gap-4">{children}</Modal.Body>
            <Modal.Footer>
              <Button variant="tertiary" onPress={onClose}>取消</Button>
              <Button isPending={busy} type="submit">{submitLabel}</Button>
            </Modal.Footer>
          </Form>
        </Modal.Dialog>
      </Modal.Container>
    </Modal.Backdrop>
  );
}

function ModelDialog({ providers, onClose, onSaved }: { providers: Provider[]; onClose: () => void; onSaved: () => void }) {
  const [form, setForm] = useState({ modelId: '', providerId: providers[0]?.providerId ?? '', providerModelVersion: '', capabilities: ['chat'] as string[], dimension: 0, tokenizer: tokenizers[0].value, maxInputTokens: 128000, maxOutputTokens: 4096 });
  const { busy, run } = useAction();
  const submit = async () => {
    const done = await run(async () => {
      const idempotencyKey = newKey();
      await api(`/v1/admin/models/${form.modelId}`, {
        method: 'PUT', idempotencyKey, body: {
          expectedVersion: 0, idempotencyKey,
          model: {
            modelId: form.modelId, version: 1, providerId: form.providerId, providerModelVersion: form.providerModelVersion, capabilities: form.capabilities, dimension: form.dimension,
            preprocessorVersion: form.capabilities.includes('embedding') ? form.tokenizer : 'none', maxInputTokens: form.maxInputTokens, maxOutputTokens: form.maxOutputTokens, catalogOrigin: 'manual', manualOverride: true, enabled: true,
          },
        },
      });
      return true;
    }, '模型已添加');
    if (done) onSaved();
  };
  return (
    <Dialog busy={busy} submitLabel="添加" title="手动添加模型" onClose={onClose} onSubmit={() => void submit()}>
      <FormGrid>
        <TextInput isRequired description="网关内部使用的 ID。" label="模型 ID" pattern={idPattern} placeholder="gpt-main" value={form.modelId} onChange={(modelId) => setForm({ ...form, modelId })} />
        <SelectInput isRequired label="供应方" options={providers.map((provider) => ({ value: provider.providerId, label: provider.displayName }))} value={form.providerId} onChange={(providerId) => setForm({ ...form, providerId })} />
      </FormGrid>
      <TextInput isRequired description="调用时传给供应方的模型名。" label="供应方模型名" value={form.providerModelVersion} onChange={(providerModelVersion) => setForm({ ...form, providerModelVersion })} />
      <CheckGroupInput label="能力" options={capabilities} value={form.capabilities} onChange={(value) => setForm({ ...form, capabilities: value })} />
      <FormGrid>
        <NumberInput label="上下文窗口（词元）" minValue={1} value={form.maxInputTokens} onChange={(maxInputTokens) => setForm({ ...form, maxInputTokens })} />
        <NumberInput label="最大输出（词元）" minValue={1} value={form.maxOutputTokens} onChange={(maxOutputTokens) => setForm({ ...form, maxOutputTokens })} />
      </FormGrid>
      {form.capabilities.includes('embedding') && (
        <FormGrid>
          <NumberInput description="向量模型的输出维度。" label="向量维度" minValue={0} value={form.dimension} onChange={(dimension) => setForm({ ...form, dimension })} />
          <SelectInput description="知识库用它估算分块的词元数。" label="分词器" options={tokenizers} value={form.tokenizer} onChange={(tokenizer) => setForm({ ...form, tokenizer })} />
        </FormGrid>
      )}
    </Dialog>
  );
}

function ProfileDialog({ models, profile, onClose, onSaved }: { models: Model[]; profile: Profile | null; onClose: () => void; onSaved: () => void }) {
  const [form, setForm] = useState({
    profileId: profile?.profileId ?? '', costClass: profile?.costClass ?? 'background', tier: profile?.tier ?? 'fast', modelIds: profile?.orderedModelIds ?? [] as string[],
    concurrency: profile?.concurrency ?? 4,
  });
  const { busy, run } = useAction();
  const move = (index: number, delta: number) => setForm((current) => {
    const ids = [...current.modelIds];
    const target = index + delta;
    if (target < 0 || target >= ids.length) return current;
    [ids[index], ids[target]] = [ids[target], ids[index]];
    return { ...current, modelIds: ids };
  });
  const submit = async () => {
    if (!form.modelIds.length) return;
    const done = await run(async () => {
      const idempotencyKey = newKey();
      await api(`/v1/admin/profiles/${form.profileId}`, {
        method: 'PUT', idempotencyKey, body: {
          expectedVersion: profile?.version ?? 0, idempotencyKey, profile: {
            profileId: form.profileId, version: (profile?.version ?? 0) + 1, costClass: form.costClass, tier: form.tier, orderedModelIds: form.modelIds,
            // 费用上限不再展示：沿用已有值（计费暂停期间不生效），新档位为不限。
            maximumCost: profile?.maximumCost ?? { currency: 'USD', micros: 0 }, concurrency: form.concurrency, dailyLimitCost: profile?.dailyLimitCost ?? { currency: 'USD', micros: 0 },
          },
        },
      });
      return true;
    }, '档位已保存');
    if (done) onSaved();
  };
  // 只列出具备该级别所需能力的模型（快速/准确要对话，视觉要视觉……）。
  const remaining = models.filter((model) => model.enabled && !form.modelIds.includes(model.modelId) && model.capabilities.includes(tierCapability[form.tier] ?? 'chat'));
  return (
    <Dialog busy={busy} submitLabel="保存" title={profile ? `编辑档位 ${profile.profileId}` : '新建档位'} onClose={onClose} onSubmit={() => void submit()}>
      <FormGrid columns={3}>
        <TextInput isReadOnly={!!profile} isRequired label="档位 ID" pattern={idPattern} placeholder="background-fast" value={form.profileId} onChange={(profileId) => setForm({ ...form, profileId })} />
        <SelectInput label="用途" options={Object.entries(costClasses).map(([value, label]) => ({ value, label }))} value={form.costClass} onChange={(costClass) => setForm({ ...form, costClass })} />
        <SelectInput label="级别" options={Object.entries(tiers).map(([value, label]) => ({ value, label }))} value={form.tier} onChange={(tier) => setForm({ ...form, tier })} />
      </FormGrid>
      <div className="flex flex-col gap-2">
        <span className="text-sm font-medium text-foreground">模型（排第一的为主，其余按顺序备用）</span>
        {!form.modelIds.length && <p className="text-sm text-muted">从下面选择至少一个模型。</p>}
        <ol className="flex flex-col gap-1.5">
          {form.modelIds.map((id, index) => (
            <li key={id} className="flex items-center gap-2 rounded-xl border border-border bg-surface-secondary px-3 py-2 text-sm">
              <span className="flex size-5 items-center justify-center rounded-full bg-default text-xs font-semibold">{index + 1}</span>
              <span className="flex-1 truncate">{models.find((model) => model.modelId === id)?.providerModelVersion ?? id}</span>
              {models.find((model) => model.modelId === id)?.enabled === false && <Chip color="warning" size="sm" variant="soft">已停用</Chip>}
              <Button isIconOnly aria-label="上移" isDisabled={index === 0} size="sm" variant="ghost" onPress={() => move(index, -1)}><ArrowUp /></Button>
              <Button isIconOnly aria-label="下移" isDisabled={index === form.modelIds.length - 1} size="sm" variant="ghost" onPress={() => move(index, 1)}><ArrowDown /></Button>
              <Button isIconOnly aria-label="移除" size="sm" variant="ghost" onPress={() => setForm({ ...form, modelIds: form.modelIds.filter((item) => item !== id) })}><Xmark /></Button>
            </li>
          ))}
        </ol>
        {remaining.length > 0 && (
          <div className="flex flex-wrap gap-1.5">
            {remaining.map((model) => <Button key={model.modelId} size="sm" variant="tertiary" onPress={() => setForm({ ...form, modelIds: [...form.modelIds, model.modelId] })}><Plus />{model.providerModelVersion}</Button>)}
          </div>
        )}
      </div>
      <NumberInput className="max-w-48" description="同时进行的调用数。" label="并发" minValue={0} value={form.concurrency} onChange={(concurrency) => setForm({ ...form, concurrency })} />
    </Dialog>
  );
}
