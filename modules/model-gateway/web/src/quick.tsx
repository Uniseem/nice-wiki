// OpenRouter 一键配置：只填一个密钥，供应方、模型、档位、任务分配，以及知识索引、防滥用和公开问答一次配好。
// 决策用 TypeSafe Jev，向量用 text-embedding-3-small，其余（快速、准确、视觉）都用 DeepSeek Flash。
import { Button, Form, Modal } from '@heroui/react';
import { api, ApiError, newKey, TextInput, useAction, t } from '@ui';
import { guardDefaults } from '@modules/traffic-guard/web/src';
import { useState } from 'react';

// 任务目录与 contracts/ai-tasks.json 一致（26 项）；新增任务时这里要同步。
export const tasks: [string, string, string][] = [
  ['qa.preflight', 'fast', t('前置分类、改写、完整性与风险')], ['qa.inspect', 'fast', t('检索核查')], ['qa.answer', 'accurate', t('最终回答')],
  ['qa.answer-fast', 'fast', t('简单问题的快速回答')], ['qa.decide', 'decision', t('每轮问答决策（意图、风险、难度）')],
  ['qa.english', 'fast', t('访客问题译成英文（供决策模型）')],
  ['qa.cache-match', 'fast', t('缓存同义复用判断')], ['qa.cache-translate', 'fast', t('缓存回答翻译')], ['qa.compact', 'fast', t('对话增量压缩')],
  ['knowledge.context', 'fast', t('检索单元背景上下文')], ['knowledge.summary', 'fast', t('文档卡片（摘要、主题、能回答的问题）')], ['knowledge.decide', 'decision', t('知识索引决策（旧卡片是否仍准确）')], ['knowledge.describe', 'fast', t('文字附件/视频公开元信息描述')],
  ['knowledge.describe-visual', 'vision', t('图片、画板、视频封面视觉描述')], ['knowledge.extract-attachment', 'vision', t('非文本附件内容提取')],
  ['knowledge.embed', 'embedding', t('生成向量')], ['knowledge.rerank', 'rerank', t('召回重排')],
  ['localization.document', 'accurate', t('块级文档翻译')], ['localization.ui', 'accurate', t('站点/界面文字翻译')], ['localization.terms', 'accurate', t('首次文档术语抽取')],
  ['localization.review', 'accurate', t('可选反思润色')], ['localization.suggest', 'decision', t('语言切换建议')],
  ['memory.overview', 'fast', t('项目概述维护')], ['memory.assistant', 'accurate', t('记忆助手')],
  ['content.slug', 'fast', t('首次发布路径名')], ['content.seo', 'fast', t('首次发布 SEO 描述')],
];

type Money = { currency: string; micros: number };
type Provider = { providerId: string; version: number; enabled: boolean; credentialConfigured: boolean; credentialVersion: number } & Record<string, unknown>;
type Model = { modelId: string; version: number; providerId: string; providerModelVersion: string; dimension: number; preprocessorVersion: string };
type Profile = { profileId: string; version: number; costClass: string; tier: string; maximumCost: Money; dailyLimitCost: Money; concurrency: number };
type Catalog = { providers: Provider[]; models: Model[]; profiles: Profile[]; taskAssignments: { taskId: string; version: number }[] };

const providerId = 'openrouter';
const baseUrl = 'https://openrouter.ai/api/v1';
const presetModels = [
  { modelId: 'deepseek-flash', providerModelVersion: '~deepseek/deepseek-flash-latest', capabilities: ['chat', 'vision'], dimension: 0, preprocessorVersion: 'none', maxInputTokens: 1048576, maxOutputTokens: 65536 },
  { modelId: 'jev', providerModelVersion: 'typesafe/jev-1.14', capabilities: ['decision'], dimension: 0, preprocessorVersion: 'none', maxInputTokens: 1000000, maxOutputTokens: 0 },
  { modelId: 'text-embedding-3-small', providerModelVersion: 'openai/text-embedding-3-small', capabilities: ['embedding'], dimension: 1536, preprocessorVersion: 'none;tiktoken-go@v0.7.0;cl100k_base', maxInputTokens: 8191, maxOutputTokens: 0 },
];
const tierModel: Record<string, string> = { fast: 'deepseek-flash', accurate: 'deepseek-flash', vision: 'deepseek-flash', embedding: 'text-embedding-3-small', decision: 'jev' };
const costClasses = ['qa', 'background', 'admin-probe'];

const put = (path: string, body: Record<string, unknown>, method: 'PUT' | 'POST' = 'PUT') => {
  const idempotencyKey = newKey();
  return api(path, { method, idempotencyKey, body: { ...body, idempotencyKey } });
};
const missing = (problem: unknown) => problem instanceof ApiError && problem.status === 404;

async function configure(secret: string, step: (label: string) => void) {
  const c = await api<Catalog>('/v1/admin/catalog');
  step(t('保存密钥'));
  const existing = c.providers.find((provider) => provider.providerId === providerId);
  if (!existing || !existing.enabled || existing.baseUrl !== baseUrl) {
    const { credentialConfigured: _configured, ...fields } = existing ?? {} as Provider;
    await put(`/v1/admin/providers/${providerId}`, {
      expectedVersion: existing?.version ?? 0, provider: {
        connectTimeoutMs: 5000, requestTimeoutMs: 60000, concurrencyLimit: 4, streamIdleTimeoutMs: 30000, extraHeaderNames: [], ...fields, credentialVersion: existing?.credentialVersion ?? 0,
        providerId, displayName: 'OpenRouter', adapter: 'openai_chat', baseUrl, enabled: true, lastProbe: undefined, extraHeaderSecretRefs: [], version: (existing?.version ?? 0) + 1,
      },
    });
  }
  if (existing?.credentialConfigured) await put(`/v1/admin/providers/${providerId}/credentials/rotate`, { providerId, newSecret: secret, expectedVersion: existing.credentialVersion }, 'POST');
  else await put(`/v1/admin/providers/${providerId}/credentials`, { providerId, secret, expectedVersion: 0 }, 'POST');

  step(t('添加模型'));
  // 之前接入过 OpenRouter 时：同一供应方、同一模型版本的直接复用（不管叫什么 ID）；
  // 预设 ID 已被别的模型占用（网关不允许改模型身份）时换一个新 ID。
  const ids: Record<string, string> = {};
  for (const preset of presetModels) {
    const current = c.models.find((model) => model.providerId === providerId && model.providerModelVersion === preset.providerModelVersion && model.dimension === preset.dimension && model.preprocessorVersion === preset.preprocessorVersion);
    let modelId = current?.modelId ?? preset.modelId;
    for (let n = 2; !current && c.models.some((model) => model.modelId === modelId); n++) modelId = `${providerId}.${preset.modelId}${n > 2 ? `-${n - 1}` : ''}`;
    ids[preset.modelId] = modelId;
    await put(`/v1/admin/models/${encodeURIComponent(modelId)}`, { expectedVersion: current?.version ?? 0, model: { ...preset, modelId, providerId, version: (current?.version ?? 0) + 1, catalogOrigin: 'manual', manualOverride: true, enabled: true } });
  }

  step(t('生成档位和任务分配'));
  for (const costClass of costClasses) {
    for (const [tier, modelId] of Object.entries(tierModel)) {
      const matches = c.profiles.filter((profile) => profile.costClass === costClass && profile.tier === tier);
      for (const profile of matches.length ? matches : [null]) {
        const profileId = profile?.profileId ?? `${costClass}-${tier}`;
        await put(`/v1/admin/profiles/${profileId}`, {
          expectedVersion: profile?.version ?? 0, profile: {
            profileId, version: (profile?.version ?? 0) + 1, costClass, tier, orderedModelIds: [ids[modelId]], concurrency: profile?.concurrency ?? 4,
            maximumCost: profile?.maximumCost ?? { currency: 'USD', micros: 0 }, dailyLimitCost: profile?.dailyLimitCost ?? { currency: 'USD', micros: 0 },
          },
        });
      }
    }
  }
  // 每个任务都按默认级别走档位，清掉之前为单个任务指定的模型；重排没有对应模型，按“能力不可用”降级。
  for (const [taskId, tier] of tasks) {
    const current = c.taskAssignments.find((assignment) => assignment.taskId === taskId);
    await put(`/v1/task-assignments/${taskId}`, { expectedVersion: current?.version ?? 0, assignment: { taskId, tier, version: (current?.version ?? 0) + 1 } });
  }

  step(t('开启知识索引'));
  type Options = { profiles: { profileId: string; space?: unknown }[]; documents: { reference: unknown }[]; baseLanguage: string };
  type IndexConfig = { configId: string; version: number; chunkTokens: number; overlapTokens: number };
  const options = await api<Options>('/v1/index-options');
  const embeddingProfile = c.profiles.find((profile) => profile.costClass === 'background' && profile.tier === 'embedding')?.profileId ?? 'background-embedding';
  const space = options.profiles.find((profile) => profile.profileId === embeddingProfile)?.space;
  if (!space) throw new Error(t('知识索引读不到刚配好的向量档位，请稍后重试。'));
  const index = await api<{ config: IndexConfig }>('/v1/index-config').then((result) => result.config).catch((problem) => { if (missing(problem)) return null; throw problem; });
  await put('/v1/index-config', {
    expectedVersion: index?.version ?? 0, config: {
      configId: index?.configId ?? newKey(), version: index?.version ?? 0, enabled: true, embeddingProfileId: embeddingProfile, space,
      chunkTokens: index?.chunkTokens ?? 512, overlapTokens: index?.overlapTokens ?? 64, language: options.baseLanguage,
    },
  });
  // 已发布文档补建一次索引（已是最新的直接复用）；之后每次发布自动建。
  for (const doc of options.documents) await put('/v1/builds', { document: doc.reference, force: false }, 'POST');

  step(t('打开公开问答'));
  const guard = await api('/v1/policies').then(() => true).catch((problem) => { if (missing(problem)) return false; throw problem; });
  if (!guard) {
    const self = await api<{ account: { siteId: string } }>('/v1/self');
    await api('/v1/policies', { method: 'PUT', body: { siteId: self.account.siteId, expectedRevision: 0, policyJSON: guardDefaults } });
  }
  const qa = await api<{ policy: Record<string, unknown>; version: number }>('/v1/admin/policy');
  await put('/v1/admin/policy', { expectedVersion: qa.version, policy: { ...qa.policy, enabled: true, retrievalSpace: space } });
}

export function OpenRouterSetupDialog({ onClose, onDone }: { onClose: () => void; onDone: () => void }) {
  const [secret, setSecret] = useState('');
  const [current, setCurrent] = useState('');
  const { busy, run } = useAction();
  const submit = async () => {
    const done = await run(async () => { await configure(secret.trim(), setCurrent); return true; }, t('AI 已全部配置好，翻译和索引会自动开始'));
    setCurrent('');
    if (done) onDone();
  };
  return (
    <Modal.Backdrop isOpen onOpenChange={(open) => { if (!open && !busy) onClose(); }}>
      <Modal.Container size="md">
        <Modal.Dialog>
          <Modal.CloseTrigger aria-label={t('关闭')} />
          <Modal.Header><Modal.Heading>{t('OpenRouter 一键配置')}</Modal.Heading></Modal.Header>
          <Form className="flex flex-col" onSubmit={(event) => { event.preventDefault(); if (secret.trim()) void submit(); }}>
            <Modal.Body className="flex flex-col gap-4">
              <p className="text-sm leading-6 text-muted">{t('填入 OpenRouter 的 API 密钥，一次配好全部 AI：决策用 Jev，向量用 text-embedding-3-small，其余都用 DeepSeek Flash；同时开启知识索引并为已发布文档建索引、保存防滥用默认值、打开公开问答。之后翻译和索引都会自动进行。')}</p>
              <TextInput isRequired autoComplete="off" description={t('只在提交时传输一次，保存后加密存放，界面不会再显示。')} label={t('OpenRouter API 密钥')} type="password" value={secret} onChange={setSecret} />
              {current && <p className="text-sm text-muted">{t('正在{0}…', current)}</p>}
            </Modal.Body>
            <Modal.Footer>
              <Button isDisabled={busy} variant="tertiary" onPress={onClose}>{t('取消')}</Button>
              <Button isDisabled={!secret.trim()} isPending={busy} type="submit">{t('开始配置')}</Button>
            </Modal.Footer>
          </Form>
        </Modal.Dialog>
      </Modal.Container>
    </Modal.Backdrop>
  );
}
