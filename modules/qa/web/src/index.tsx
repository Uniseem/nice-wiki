import { Comments, PaperPlane, Star } from '@gravity-ui/icons';
import { Button, Chip, Form, Modal, Spinner, Table, Tabs } from '@heroui/react';
import {
  adminLanguage, api, ApiError, confirmAction, EmptyBlock, ErrorBlock, errorText, fmtDateTime, fmtRelative, FormGrid, LoadingBlock, newKey, Notice, NumberInput, PageHeader, Panel, SelectInput, SwitchInput, TextAreaInput, TextInput,
  navigate, useAction, useHasGrant, useLoad, t,
} from '@ui';
import { useEffect, useState } from 'react';
import { Markdown } from './island/markdown';

type Policy = Record<string, unknown>;
type Turn = { turnId: string; createdAt: string; questionPreview: string; answerKind?: string; feedback?: string };
type CacheEntry = { cacheId: string; version: number; question: string; answer?: string; language: string; curated?: boolean; expiresAt?: string | null; state: string };
type Citation = { citationId?: string; title?: string; href?: string };

const switches: Record<string, [string, string]> = {
  enabled: [t('公开问答'), t('打开后访客可以在文档页右下角和独立问答页提问。')],
  cacheEnabled: [t('答案缓存'), t('完整、可共享的问题会先查缓存，命中就不再调用回答模型。')],
  compactionEnabled: [t('长对话压缩'), t('对话快占满回答模型的上下文窗口时，把早期内容压缩成摘要；关闭则只带得下的最近消息。')],
};
// 表单按这里的先后排列：回答额度、对话压缩、缓存与保留期。
const numbers: Record<string, string> = {
  maximumContextTokens: t('上下文最大词元'), maximumOutputTokens: t('回答最大输出词元'), maximumQuestionCharacters: t('问题最大字符数'),
  contextReservePercent: t('上下文预留比例（%）'), retainedRecentTokens: t('压缩时保留的近期词元'), maximumHistoryTokens: t('历史最大词元'),
  semanticThreshold: t('语义相似阈值（0–1）'), cacheTtlDays: t('缓存保留天数'), recordRetentionDays: t('问答记录保留天数'),
};
const numberHints: Record<string, string> = {
  maximumContextTokens: t('交给回答模型的文章原文上限；实际还不超过回答模型窗口的三分之一。'),
};
const texts: Record<string, string> = { semanticMode: t('语义判定方式') };
const semanticModes = [{ value: 'ai_equivalence', label: t('由 AI 判断是否同一问题') }, { value: 'threshold', label: t('按语义相似阈值') }];
// 历史预算跟随回答模型的上下文窗口，不再单独设上限；预检、核查、子问题、多轮检索和工具调用这几项当前回答流程不用；
// 回答档位以“模型接入”里的任务分配为准（服务端按任务路线解析），不在这里选。
const hidden = new Set([
  'version', 'retrievalSpace', 'maximumHistoryTokens', 'profileId',
  'preflightOutputTokens', 'inspectionOutputTokens', 'maximumSubqueries', 'maximumRetrievalRounds', 'maximumToolCalls',
]);
const answerKinds: Record<string, string> = { answered: t('已回答'), refused: t('拒答'), cached: t('缓存命中'), no_evidence: t('无依据'), out_of_scope: t('离题/闲聊'), clarification: t('需澄清'), failed: t('失败'), cancelled: t('已取消') };

export default function QaPage() {
  const canManage = useHasGrant('qa.manage');
  const canDebug = useHasGrant('qa.debug');
  const [tab, setTab] = useState(canManage ? 'policy' : 'debug');
  return (
    <div className="flex flex-col gap-6">
      <PageHeader title={t('问答')} description={t('访客问答的开关、额度与缓存。每次回答都会调用模型；开启前请先配好“模型接入”“知识索引”和“防滥用”。')} />
      <Tabs selectedKey={tab} onSelectionChange={(key) => setTab(String(key))}>
        <Tabs.ListContainer className="mb-4 overflow-x-auto">
          <Tabs.List aria-label={t('问答管理')}>
            {canManage ? <Tabs.Tab id="policy">{t('策略')}<Tabs.Indicator /></Tabs.Tab> : null}
            {canManage ? <Tabs.Tab id="turns">{t('问答记录')}<Tabs.Indicator /></Tabs.Tab> : null}
            {canManage ? <Tabs.Tab id="cache">{t('答案缓存')}<Tabs.Indicator /></Tabs.Tab> : null}
            {canDebug ? <Tabs.Tab id="debug">{t('调试台')}<Tabs.Indicator /></Tabs.Tab> : null}
          </Tabs.List>
        </Tabs.ListContainer>
        {canManage ? <Tabs.Panel id="policy"><PolicyTab /></Tabs.Panel> : null}
        {canManage ? <Tabs.Panel id="turns"><TurnsTab /></Tabs.Panel> : null}
        {canManage ? <Tabs.Panel id="cache"><CacheTab /></Tabs.Panel> : null}
        {canDebug ? <Tabs.Panel id="debug"><DebugTab /></Tabs.Panel> : null}
      </Tabs>
    </div>
  );
}

function PolicyTab() {
  const current = useLoad((signal) => api<{ policy: Policy; version: number }>('/v1/admin/policy', { signal }), []);
  // 知识索引当前的向量空间：换了向量模型后，问答要跟着用新的空间检索（没有知识库权限时读不到，就不提示）。
  const indexSpace = useLoad((signal) => api<{ config: { space: unknown } }>('/v1/index-config', { signal }).then((result) => result.config.space).catch(() => null), []);
  // 防滥用策略从没保存过时，访客提问会被拒绝（没有防滥用权限时读不到，就不提示）。
  const guardMissing = useLoad((signal) => api('/v1/policies', { signal }).then(() => false).catch((problem) => problem instanceof ApiError && problem.code === 'NOT_FOUND'), []);
  const [policy, setPolicy] = useState<Policy | null>(null);
  const { busy, run } = useAction();
  useEffect(() => { if (current.data) setPolicy(current.data.policy); }, [current.data]);
  if (current.error && !current.data) return <ErrorBlock error={current.error} onRetry={current.reload} />;
  if (!policy || !current.data) return <LoadingBlock lines={6} />;
  const set = (key: string, value: unknown) => setPolicy({ ...policy, [key]: value });
  const order = [...Object.keys(switches), ...Object.keys(texts), ...Object.keys(numbers)];
  const rank = (key: string) => (order.includes(key) ? order.indexOf(key) : order.length);
  const keys = Object.keys(policy).filter((key) => !hidden.has(key)).sort((a, b) => rank(a) - rank(b));
  const staleSpace = indexSpace.data != null && JSON.stringify(indexSpace.data) !== JSON.stringify(policy.retrievalSpace);
  const save = () => run(async () => {
    const idempotencyKey = newKey();
    const next = staleSpace ? { ...policy, retrievalSpace: indexSpace.data } : policy;
    await api('/v1/admin/policy', { method: 'PUT', idempotencyKey, body: { policy: next, expectedVersion: current.data!.version, idempotencyKey } });
    current.reload();
  }, t('策略已保存'));
  return (
    <Form className="flex flex-col gap-6" onSubmit={(event) => { event.preventDefault(); void save(); }}>
      {!policy.enabled && <Notice status="default" title={t('公开问答目前关闭')}>{t('访客看不到问答入口。确认模型、索引和防滥用都已就绪后再打开。')}</Notice>}
      {guardMissing.data && (
        <Notice status="warning" title={t('防滥用策略还没有保存')}>
          <span className="flex flex-wrap items-center gap-2">{t('没保存过防滥用策略时，访客提问会被拒绝。到“防滥用”页面确认并保存一次即可（可以直接用默认值）。')}
            <Button size="sm" variant="secondary" onPress={() => navigate('/admin/guard')}>{t('去设置')}</Button>
          </span>
        </Notice>
      )}
      {staleSpace && <Notice status="warning" title={t('知识索引换了向量模型')}>{t('问答还在用旧的向量空间检索，访客提问会失败。保存一次策略即可切换到新的空间。')}</Notice>}
      <Panel title={t('开关')} description={t('策略版本 #{0}', current.data.version)}>
        <div className="grid gap-5 md:grid-cols-3">
          {keys.filter((key) => typeof policy[key] === 'boolean').map((key) => (
            <SwitchInput key={key} description={switches[key]?.[1]} isSelected={policy[key] as boolean} label={switches[key]?.[0] ?? key} onChange={(value) => set(key, value)} />
          ))}
        </div>
      </Panel>
      <Panel title={t('模型与额度')} description={t('回答用“模型接入”里为访客问答分配的档位。数值越大回答越充分，但每次调用的词元也越多。')}>
        <FormGrid columns={3}>
          {keys.filter((key) => typeof policy[key] === 'string').map((key) => key === 'semanticMode'
            ? <SelectInput key={key} label={texts[key]} options={semanticModes} value={String(policy[key] ?? '')} onChange={(value) => set(key, value)} />
            : <TextInput key={key} label={texts[key] ?? key} value={String(policy[key] ?? '')} onChange={(value) => set(key, value)} />)}
          {keys.filter((key) => typeof policy[key] === 'number').map((key) => (
            <NumberInput key={key} description={numberHints[key]} formatOptions={key === 'semanticThreshold' ? { maximumFractionDigits: 3 } : undefined} label={numbers[key] ?? key} minValue={0} step={key === 'semanticThreshold' ? 0.01 : 1} value={policy[key] as number} onChange={(value) => set(key, value)} />
          ))}
        </FormGrid>
      </Panel>
      <div className="flex justify-end"><Button isPending={busy} type="submit">{t('保存策略')}</Button></div>
    </Form>
  );
}

function TurnsTab() {
  const turns = useLoad((signal) => api<{ items?: Turn[] }>('/v1/admin/turns?limit=30', { signal }).then((result) => result.items ?? []), []);
  const [curate, setCurate] = useState<{ turnId: string; question: string; answer: string } | null>(null);
  const { busy, run } = useAction();
  const open = (turn: Turn) => run(async () => {
    const record = await api<{ question: string; answer: string }>(`/v1/admin/turns/${turn.turnId}`);
    setCurate({ turnId: turn.turnId, question: record.question, answer: record.answer });
  });
  const submit = async () => {
    if (!curate) return;
    const done = await run(async () => {
      const idempotencyKey = newKey();
      await api('/v1/admin/cache', { method: 'POST', idempotencyKey, body: { turnId: curate.turnId, publicQuestion: curate.question, publicAnswer: curate.answer, idempotencyKey } });
      return true;
    }, t('已提交为待批准的精选，请到“答案缓存”里批准'));
    if (done) setCurate(null);
  };
  if (turns.error && !turns.data) return <ErrorBlock error={turns.error} onRetry={turns.reload} />;
  if (!turns.data) return <LoadingBlock lines={5} />;
  return (
    <>
      {!turns.data.length ? <EmptyBlock icon={<Comments />} title={t('还没有问答记录')} description={t('开启公开问答后，访客的提问会出现在这里。')} /> : (
        <Table>
          <Table.ScrollContainer>
            <Table.Content aria-label={t('问答记录')} className="min-w-[640px]">
              <Table.Header>
                <Table.Column isRowHeader>{t('问题')}</Table.Column>
                <Table.Column>{t('结果')}</Table.Column>
                <Table.Column>{t('时间')}</Table.Column>
                <Table.Column className="text-end">{t('操作')}</Table.Column>
              </Table.Header>
              <Table.Body>
                {turns.data.map((turn) => (
                  <Table.Row key={turn.turnId} id={turn.turnId}>
                    <Table.Cell><span className="line-clamp-2">{turn.questionPreview}</span></Table.Cell>
                    <Table.Cell>
                      <span className="flex flex-wrap gap-1">
                        <Chip color={turn.answerKind === 'answered' ? 'success' : turn.answerKind === 'failed' ? 'danger' : 'default'} size="sm" variant="soft">{(turn.answerKind && answerKinds[turn.answerKind]) || turn.answerKind || t('未完成')}</Chip>
                        {turn.feedback && <Chip color={turn.feedback === 'helpful' ? 'success' : 'warning'} size="sm" variant="soft">{turn.feedback === 'helpful' ? t('有帮助') : t('没帮助')}</Chip>}
                      </span>
                    </Table.Cell>
                    <Table.Cell><span title={fmtDateTime(turn.createdAt)}>{fmtRelative(turn.createdAt)}</span></Table.Cell>
                    <Table.Cell className="text-end">{turn.answerKind === 'answered' && <Button isDisabled={busy} size="sm" variant="secondary" onPress={() => open(turn)}><Star />{t('设为精选')}</Button>}</Table.Cell>
                  </Table.Row>
                ))}
              </Table.Body>
            </Table.Content>
          </Table.ScrollContainer>
        </Table>
      )}
      {curate && (
        <Modal.Backdrop isOpen onOpenChange={(value) => { if (!value) setCurate(null); }}>
          <Modal.Container scroll="outside" size="lg">
            <Modal.Dialog>
              <Modal.CloseTrigger aria-label={t('关闭')} />
              <Modal.Header><Modal.Heading>{t('设为精选答案')}</Modal.Heading></Modal.Header>
              <Form className="flex flex-col" onSubmit={(event) => { event.preventDefault(); void submit(); }}>
                <Modal.Body className="flex flex-col gap-4">
                  <Notice status="accent">{t('精选会公开给所有访客。去掉个人信息，把问题改成通用问法。批准后长期有效、优先命中，来源文档撤回或改动时自动失效。')}</Notice>
                  <TextInput isRequired label={t('公开问题')} value={curate.question} onChange={(question) => setCurate({ ...curate, question })} />
                  <TextAreaInput isRequired label={t('公开答案')} rows={8} value={curate.answer} onChange={(answer) => setCurate({ ...curate, answer })} />
                </Modal.Body>
                <Modal.Footer>
                  <Button variant="tertiary" onPress={() => setCurate(null)}>{t('取消')}</Button>
                  <Button isPending={busy} type="submit">{t('提交精选')}</Button>
                </Modal.Footer>
              </Form>
            </Modal.Dialog>
          </Modal.Container>
        </Modal.Backdrop>
      )}
    </>
  );
}

function CacheTab() {
  const entries = useLoad((signal) => api<{ entries?: CacheEntry[] }>('/v1/admin/cache?limit=50', { signal }).then((result) => result.entries ?? []), []);
  const { busy, run } = useAction();
  const approve = (entry: CacheEntry) => run(async () => {
    const idempotencyKey = newKey();
    await api(`/v1/admin/cache/${entry.cacheId}/approve`, { method: 'POST', idempotencyKey, body: { cacheId: entry.cacheId, expectedVersion: entry.version, idempotencyKey } });
    entries.reload();
  }, t('精选已批准'));
  const disable = async (entry: CacheEntry) => {
    if (!(await confirmAction({ title: t('禁用这条缓存？'), body: t('禁用后不会再复用这个答案：“{0}”', entry.question), confirmLabel: t('禁用'), tone: 'warning' }))) return;
    await run(async () => {
      const idempotencyKey = newKey();
      await api(`/v1/admin/cache/${entry.cacheId}/disable`, { method: 'POST', idempotencyKey, body: { cacheId: entry.cacheId, expectedVersion: entry.version, idempotencyKey } });
      entries.reload();
    }, t('已禁用'));
  };
  if (entries.error && !entries.data) return <ErrorBlock error={entries.error} onRetry={entries.reload} />;
  if (!entries.data) return <LoadingBlock lines={5} />;
  if (!entries.data.length) return <EmptyBlock icon={<Comments />} title={t('还没有缓存条目')} description={t('打开“答案缓存”后，完整且可共享的问题回答完会自动写入；撤回或改动来源文档后，相关缓存不再命中。')} />;
  return (
    <Table>
      <Table.ScrollContainer>
        <Table.Content aria-label={t('答案缓存')} className="min-w-[680px]">
          <Table.Header>
            <Table.Column isRowHeader>{t('问题')}</Table.Column>
            <Table.Column>{t('答案')}</Table.Column>
            <Table.Column>{t('状态')}</Table.Column>
            <Table.Column className="text-end">{t('操作')}</Table.Column>
          </Table.Header>
          <Table.Body>
            {entries.data.map((entry) => (
              <Table.Row key={entry.cacheId} id={entry.cacheId}>
                <Table.Cell><span className="line-clamp-2 font-medium">{entry.question}</span><span className="text-xs text-muted">{entry.language}</span></Table.Cell>
                <Table.Cell><span className="line-clamp-2 text-sm text-muted">{(entry.answer ?? '').replace(/\s*\[\[citation\.[0-9a-f]+\]\]/g, '')}</span></Table.Cell>
                <Table.Cell>
                  <span className="flex flex-wrap gap-1">
                    {entry.curated ? <Chip color="accent" size="sm" variant="soft">{t('精选')}</Chip> : <Chip size="sm" variant="soft">{t('自动')}</Chip>}
                    {entry.state === 'pending' && <Chip color="warning" size="sm" variant="soft">{t('待批准')}</Chip>}
                    {entry.state === 'disabled' && <Chip color="danger" size="sm" variant="soft">{t('已禁用')}</Chip>}
                    {!entry.curated && entry.expiresAt && <span className="text-xs text-muted">{t('{0} 过期', fmtDateTime(entry.expiresAt))}</span>}
                  </span>
                </Table.Cell>
                <Table.Cell className="text-end">
                  <span className="inline-flex gap-2">
                    {entry.state === 'pending' && <Button isDisabled={busy} size="sm" onPress={() => approve(entry)}>{t('批准')}</Button>}
                    {entry.state !== 'disabled' && <Button isDisabled={busy} size="sm" variant="tertiary" onPress={() => disable(entry)}>{t('禁用')}</Button>}
                  </span>
                </Table.Cell>
              </Table.Row>
            ))}
          </Table.Body>
        </Table.Content>
      </Table.ScrollContainer>
    </Table>
  );
}

// 调试台：管理员直接试问，结果只存在隔离的诊断记录里（7 天），不影响访客对话和缓存；使用“后台任务”用途的档位。
function DebugTab() {
  const [question, setQuestion] = useState('');
  // 接口要求带档位，服务端以任务路线为准，这里只是占位。
  const profileId = 'background-accurate';
  const [state, setState] = useState<{ running: boolean; answer: string; citations: Citation[]; status: '' | 'running' | 'done' | 'failed'; error: string }>({ running: false, answer: '', citations: [], status: '', error: '' });

  async function ask() {
    if (!question.trim() || state.running) return;
    setState({ running: true, answer: '', citations: [], status: 'running', error: '' });
    try {
      const idempotencyKey = newKey();
      const created = await api<{ debug: { debugId: string } }>('/v1/admin/debug-turns', { method: 'POST', idempotencyKey, body: { question, profileId, language: adminLanguage(), idempotencyKey } });
      for (let i = 0; i < 90; i++) {
        await new Promise((resolve) => setTimeout(resolve, 2000));
        const read = await api<{ events: { event: string; delta?: string; citation?: Citation; code?: string }[] }>(`/v1/admin/debug-turns/${created.debug.debugId}?afterSequence=0`);
        const answer = read.events.filter((e) => e.event === 'delta').map((e) => e.delta ?? '').join('');
        const citations = read.events.filter((e) => e.event === 'citation' && e.citation).map((e) => e.citation!);
        const last = read.events[read.events.length - 1];
        if (last?.event === 'completed') { setState({ running: false, answer, citations, status: 'done', error: '' }); return; }
        if (last?.event === 'failed') {
          setState({ running: false, answer, citations, status: 'failed', error: t('失败原因：{0}。常见原因：诊断档位不存在或不属于“后台任务”用途、知识库没有索引、模型没有配置。', last.code ?? t('未知')) });
          return;
        }
        setState((current) => ({ ...current, answer, citations }));
      }
      setState((current) => ({ ...current, running: false, status: 'failed', error: t('等待超时（3 分钟），请稍后在问答记录里查看。') }));
    } catch (problem) {
      setState((current) => ({ ...current, running: false, status: 'failed', error: errorText(problem) }));
    }
  }

  return (
    <div className="grid gap-6 xl:grid-cols-5">
      <Panel className="xl:col-span-2" title={t('试问')} description={t('检索知识库并调用模型回答，结果不影响访客对话和缓存。')}>
        <Form className="flex flex-col gap-4" onSubmit={(event) => { event.preventDefault(); void ask(); }}>
          <TextAreaInput isRequired label={t('问题')} maxLength={500} placeholder={t('例如：如何安装？')} rows={4} value={question} onChange={setQuestion} />
          <p className="text-xs text-muted">{t('使用“模型接入”里为问答回答分配的后台档位。')}</p>
          <div className="flex justify-end"><Button isPending={state.running} type="submit"><PaperPlane />{state.running ? t('诊断中…') : t('试问')}</Button></div>
        </Form>
      </Panel>
      <Panel className="xl:col-span-3" title={t('结果')}>
        {!state.status && <p className="text-sm text-muted">{t('回答、引用和失败原因会显示在这里。')}</p>}
        {state.status === 'running' && !state.answer && <div className="flex items-center gap-2 text-sm text-muted"><Spinner size="sm" />{t('正在检索并生成回答…')}</div>}
        {/* 与访客看到的一样渲染：Markdown 与上标引用编号。 */}
        {state.answer && <div className="rounded-xl bg-surface-secondary p-4 text-foreground"><Markdown citations={state.citations} text={state.answer} /></div>}
        {state.citations.length > 0 && (
          <div className="flex flex-col gap-1.5">
            <span className="text-xs font-medium text-muted">{t('引用来源')}</span>
            {/* 同一篇文档的多段引用只列一次 */}
            {state.citations.filter((citation, index, all) => all.findIndex((other) => (other.title || other.href) === (citation.title || citation.href)) === index).map((citation, index) => (
              <a key={index} className="truncate text-sm text-accent hover:underline" href={citation.href} rel="noopener" target="_blank">{citation.title || citation.href}</a>
            ))}
          </div>
        )}
        {state.status === 'done' && <Chip color="success" size="sm" variant="soft">{t('完成')}</Chip>}
        {state.error && <Notice status="danger">{state.error}</Notice>}
      </Panel>
    </div>
  );
}
