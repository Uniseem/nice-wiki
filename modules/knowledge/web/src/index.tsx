import { Database } from '@gravity-ui/icons';
import { Button, Chip, Form, ProgressBar, Table } from '@heroui/react';
import { api, codeMeaning, confirmAction, EmptyBlock, ErrorBlock, fmtRelative, FormGrid, LoadingBlock, newKey, Notice, NumberInput, PageHeader, Panel, SelectInput, SwitchInput, useAction, useHasGrant, useLoad, t } from '@ui';
import { useEffect, useMemo, useState } from 'react';

type ProfileOption = { profileId: string; label: string; tier: string; usable: boolean; space?: unknown };
type DocOption = { documentId: string; title: string; reference: unknown };
type Options = { modelsState: string; modelErrorCode?: string; profiles: ProfileOption[]; documents: DocOption[]; baseLanguage: string; siteOverview?: string; siteOverviewUpdatedAt?: string };
type Config = { configId: string; version: number; enabled: boolean; embeddingProfileId: string; space: unknown; chunkTokens: number; overlapTokens: number; language: string };
type Build = { buildId: string; document: { documentId: string; publishedRevisionId?: string }; space?: unknown; state: string; failureCode?: string | null; completedChunks: number; totalChunks: number; createdAt?: string };

const terminal = new Set(['active', 'cancelled', 'failed', 'superseded']);
const stateText: Record<string, { label: string; tone: 'success' | 'accent' | 'danger' | 'default' | 'warning' }> = {
  active: { label: t('生效中'), tone: 'success' }, queued: { label: t('排队中'), tone: 'default' }, fetching: { label: t('读取正文'), tone: 'accent' },
  extracting: { label: t('提取内容'), tone: 'accent' }, chunking: { label: t('切块中'), tone: 'accent' }, embedding: { label: t('生成向量'), tone: 'accent' },
  validating: { label: t('校验中'), tone: 'accent' }, failed: { label: t('失败'), tone: 'danger' }, cancelled: { label: t('已取消'), tone: 'default' }, superseded: { label: t('已被替换'), tone: 'default' },
};

export default function KnowledgePage() {
  const options = useLoad((signal) => api<Options>('/v1/index-options', { signal }), []);
  const config = useLoad((signal) => api<{ config: Config }>('/v1/index-config', { signal }).then((result) => result.config).catch(() => null), []);
  const builds = useLoad((signal) => api<{ builds: Build[] }>('/v1/builds?limit=50', { signal }).then((result) => result.builds), []);
  const { busy, run } = useAction();
  const [form, setForm] = useState({ profileId: '', enabled: true, chunkTokens: 512, overlapTokens: 64 });

  useEffect(() => {
    const c = config.data;
    if (c) setForm({ profileId: c.embeddingProfileId, enabled: c.enabled, chunkTokens: c.chunkTokens, overlapTokens: c.overlapTokens });
  }, [config.data]);

  const running = (builds.data ?? []).some((build) => !terminal.has(build.state));
  useEffect(() => {
    if (!running) return;
    const timer = setInterval(builds.reload, 5000);
    return () => clearInterval(timer);
  }, [running, builds.reload]);

  // 每篇文档只看“当前发布版本 + 当前向量空间”的构建：旧版本或换模型前的索引不算数。
  // 同一版本有多条时，生效的优先，其次进行中的，最后才是失败/取消的。
  const latest = useMemo(() => {
    const current = new Map((options.data?.documents ?? []).map((doc) => [doc.documentId, (doc.reference as { publishedRevisionId?: string } | null)?.publishedRevisionId]));
    const space = JSON.stringify(config.data?.space ?? null);
    const rank = (state: string) => state === 'active' ? 3 : terminal.has(state) ? 1 : 2;
    const map = new Map<string, Build>();
    for (const build of builds.data ?? []) {
      if (build.document.publishedRevisionId !== current.get(build.document.documentId) || JSON.stringify(build.space ?? null) !== space) continue;
      const seen = map.get(build.document.documentId);
      if (!seen || rank(build.state) > rank(seen.state)) map.set(build.document.documentId, build);
    }
    return map;
  }, [builds.data, options.data, config.data]);

  const saveConfig = () => run(async () => {
    const profile = options.data?.profiles.find((item) => item.profileId === form.profileId);
    if (!profile?.space) throw new Error(t('请选择可用的嵌入档位'));
    const current = config.data;
    const idempotencyKey = newKey();
    await api('/v1/index-config', {
      method: 'PUT', idempotencyKey, body: {
        idempotencyKey, expectedVersion: current?.version ?? 0, config: {
          configId: current?.configId ?? newKey(), version: current?.version ?? 0, enabled: form.enabled, embeddingProfileId: profile.profileId, space: profile.space,
          chunkTokens: form.chunkTokens, overlapTokens: form.overlapTokens, language: options.data!.baseLanguage,
        },
      },
    });
    config.reload();
  }, t('索引配置已保存'));

  const build = async (docs: DocOption[]) => {
    const ok = await confirmAction({
      title: docs.length === 1 ? t('为“{0}”建立索引？', docs[0].title) : t('为全部 {0} 篇文档建立索引？', docs.length),
      body: t('会调用嵌入模型（向量）处理正文。已经是最新的文档会直接复用。'), confirmLabel: t('开始'), tone: 'warning',
    });
    if (!ok) return;
    await run(async () => {
      for (const doc of docs) {
        const idempotencyKey = newKey();
        await api('/v1/builds', { method: 'POST', idempotencyKey, body: { idempotencyKey, document: doc.reference, force: false } });
      }
      builds.reload();
    }, t('已提交索引任务'));
  };

  // 全站概括每天自动重算；这里可以立即重算（在后台进行，几分钟后刷新查看）。
  const refreshOverview = () => run(async () => {
    const idempotencyKey = newKey();
    await api('/v1/site-overview:refresh', { method: 'POST', idempotencyKey, body: { idempotencyKey } });
  }, t('已开始重新生成，几分钟后刷新查看'));

  const cancel = (item: Build) => run(async () => {
    const idempotencyKey = newKey();
    await api(`/v1/builds/${item.buildId}/cancel`, { method: 'POST', idempotencyKey, body: { buildId: item.buildId, reason: t('管理员取消'), idempotencyKey } });
    builds.reload();
  }, t('已取消'));

  if (options.error && !options.data) return <ErrorBlock error={options.error} onRetry={options.reload} />;
  const o = options.data;
  const embeddingProfiles = o?.profiles.filter((profile) => profile.tier === 'embedding') ?? [];
  // 任务里的文档可能已撤回或删除，不在已发布列表里。
  const titleOf = (id: string) => o?.documents.find((doc) => doc.documentId === id)?.title ?? t('已撤回或删除的文档');
  const enabled = !!config.data?.enabled;
  const operator = useHasGrant('model.manage');

  return (
    <div className="flex flex-col gap-6">
      <PageHeader title={t('知识索引')} description={t('把已发布的文档切块并生成向量，供 AI 问答检索。建立索引会调用嵌入模型。')} />
      {!o ? <LoadingBlock lines={5} /> : (
        <>
          {o.modelsState !== 'ready' && <Notice status="warning" title={t('模型目录暂不可用')}>{operator ? t('无法配置索引（{0}）。请先检查“模型接入”。', o.modelErrorCode ?? o.modelsState) : t('无法配置索引（{0}）。请稍后重试，或联系站主。', o.modelErrorCode ?? o.modelsState)}</Notice>}
          {o.modelsState === 'ready' && !embeddingProfiles.length && <Notice status="warning" title={t('还没有向量档位')}>{operator ? t('先在“模型接入”里添加带“向量”能力的模型，并生成档位。') : t('平台还没有配置向量档位，请联系站主。')}</Notice>}
          <div className="grid gap-6 xl:grid-cols-5">
            <Panel className="xl:col-span-2" title={t('入库配置')} description={t('切块越小检索越精细，但块数更多、调用次数更多。')}>
              <Form className="flex flex-col gap-4" onSubmit={(event) => { event.preventDefault(); void saveConfig(); }}>
                <SelectInput
                  isRequired
                  label={t('嵌入档位')}
                  options={embeddingProfiles.map((profile) => ({ value: profile.profileId, label: profile.usable ? profile.label : t('{0}（不可用）', profile.label), disabled: !profile.usable }))}
                  value={form.profileId}
                  onChange={(profileId) => setForm({ ...form, profileId })}
                />
                <FormGrid>
                  <NumberInput label={t('分块大小（词元）')} maxValue={1024} minValue={128} step={64} value={form.chunkTokens} onChange={(chunkTokens) => setForm({ ...form, chunkTokens })} />
                  <NumberInput label={t('块间重叠（词元）')} maxValue={128} minValue={0} step={16} value={form.overlapTokens} onChange={(overlapTokens) => setForm({ ...form, overlapTokens })} />
                </FormGrid>
                <SwitchInput description={t('关闭后不再为文档建立新索引。')} isSelected={form.enabled} label={t('启用入库')} onChange={(value) => setForm({ ...form, enabled: value })} />
                <div className="flex justify-end"><Button isPending={busy} type="submit">{t('保存配置')}</Button></div>
              </Form>
            </Panel>
            <Panel
              className="xl:col-span-3"
              title={t('已发布文档')}
              description={enabled ? t('为文档建立或刷新索引。') : t('保存并启用入库配置后才能建立索引。')}
              actions={o.documents.length > 0 ? <Button isDisabled={busy || !enabled} size="sm" variant="secondary" onPress={() => build(o.documents)}>{t('全部建立索引')}</Button> : undefined}
            >
              {!o.documents.length ? <EmptyBlock icon={<Database />} title={t('还没有已发布文档')} description={t('发布文档后再来建立索引。')} /> : (
                <Table variant="secondary">
                  <Table.ScrollContainer>
                    <Table.Content aria-label={t('已发布文档')} className="min-w-[480px]">
                      <Table.Header>
                        <Table.Column isRowHeader>{t('文档')}</Table.Column>
                        <Table.Column>{t('当前版本的索引')}</Table.Column>
                        <Table.Column className="text-end">{t('操作')}</Table.Column>
                      </Table.Header>
                      <Table.Body>
                        {o.documents.map((doc) => {
                          const last = latest.get(doc.documentId);
                          const state = last ? stateText[last.state] ?? { label: last.state, tone: 'default' as const } : null;
                          return (
                            <Table.Row key={doc.documentId} id={doc.documentId}>
                              <Table.Cell>{doc.title}</Table.Cell>
                              <Table.Cell>
                                {state ? <Chip color={state.tone} size="sm" variant="soft">{state.label}</Chip> : <span className="text-sm text-muted">{t('未建立')}</span>}
                                {last?.failureCode && <span className="mt-1 block text-xs text-danger" title={last.failureCode}>{codeMeaning(last.failureCode)}</span>}
                              </Table.Cell>
                              <Table.Cell className="text-end"><Button isDisabled={busy || !enabled} size="sm" variant="secondary" onPress={() => build([doc])}>{t('建立索引')}</Button></Table.Cell>
                            </Table.Row>
                          );
                        })}
                      </Table.Body>
                    </Table.Content>
                  </Table.ScrollContainer>
                </Table>
              )}
            </Panel>
          </div>
          <Panel
            title={t('全站概括')}
            description={o.siteOverviewUpdatedAt ? t('每天由各文档的层级摘要自下而上合成，AI 问答每次都会参考。生成于 {0}。', fmtRelative(o.siteOverviewUpdatedAt)) : t('每天由各文档的层级摘要自下而上合成，AI 问答每次都会参考。')}
            actions={<Button isDisabled={busy} size="sm" variant="tertiary" onPress={() => void refreshOverview()}>{t('立即重新生成')}</Button>}
          >
            {o.siteOverview ? <p className="max-h-96 overflow-auto whitespace-pre-wrap rounded-xl bg-surface-secondary px-4 py-3 text-sm leading-6 text-foreground/90">{o.siteOverview}</p> : <p className="py-4 text-center text-sm text-muted">{t('还没有生成。建好索引后会自动生成，也可以点“立即重新生成”。')}</p>}
          </Panel>
          <Panel title={t('索引任务')} description={running ? t('有任务在进行，每 5 秒自动刷新。') : t('最近 50 个任务。')} actions={<Button size="sm" variant="tertiary" onPress={builds.reload}>{t('刷新')}</Button>}>
            {builds.error ? <ErrorBlock error={builds.error} onRetry={builds.reload} /> : !builds.data ? <LoadingBlock lines={3} /> : !builds.data.length ? (
              <p className="py-4 text-center text-sm text-muted">{t('暂无任务。')}</p>
            ) : (
              <Table variant="secondary">
                <Table.ScrollContainer>
                  <Table.Content aria-label={t('索引任务')} className="min-w-[640px]">
                    <Table.Header>
                      <Table.Column isRowHeader>{t('文档')}</Table.Column>
                      <Table.Column>{t('状态')}</Table.Column>
                      <Table.Column>{t('进度')}</Table.Column>
                      <Table.Column className="text-end">{t('操作')}</Table.Column>
                    </Table.Header>
                    <Table.Body>
                      {builds.data.map((item) => {
                        const state = stateText[item.state] ?? { label: item.state, tone: 'default' as const };
                        const percent = item.totalChunks ? Math.round((item.completedChunks / item.totalChunks) * 100) : 0;
                        return (
                          <Table.Row key={item.buildId} id={item.buildId}>
                            <Table.Cell>
                              <span className="block">{titleOf(item.document.documentId)}</span>
                              {item.createdAt && <span className="text-xs text-muted">{fmtRelative(item.createdAt)}</span>}
                            </Table.Cell>
                            <Table.Cell>
                              <Chip color={state.tone} size="sm" variant="soft">{state.label}</Chip>
                              {item.failureCode && <span className="mt-1 block text-xs text-danger" title={item.failureCode}>{codeMeaning(item.failureCode)}</span>}
                            </Table.Cell>
                            <Table.Cell>
                              <div className="flex min-w-40 items-center gap-2">
                                <ProgressBar aria-label={t('索引进度')} className="flex-1" color={item.state === 'failed' ? 'danger' : 'accent'} size="sm" value={percent}>
                                  <ProgressBar.Track><ProgressBar.Fill /></ProgressBar.Track>
                                </ProgressBar>
                                <span className="text-xs tabular-nums text-muted">{item.completedChunks}/{item.totalChunks}</span>
                              </div>
                            </Table.Cell>
                            <Table.Cell className="text-end">{!terminal.has(item.state) && <Button isDisabled={busy} size="sm" variant="tertiary" onPress={() => cancel(item)}>{t('取消')}</Button>}</Table.Cell>
                          </Table.Row>
                        );
                      })}
                    </Table.Body>
                  </Table.Content>
                </Table.ScrollContainer>
              </Table>
            )}
          </Panel>
        </>
      )}
    </div>
  );
}
