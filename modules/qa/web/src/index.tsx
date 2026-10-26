import { Comments, PaperPlane } from '@gravity-ui/icons';
import { Button, Chip, Form, Modal, Spinner, Table, Tabs } from '@heroui/react';
import {
  adminLanguage, api, ApiError, EmptyBlock, ErrorBlock, errorText, fmtDateTime, fmtRelative, FormGrid, LoadingBlock, newKey, Notice, NumberInput, PageHeader, Panel, SelectInput, SwitchInput, TextAreaInput, TextInput,
  navigate, useAction, useHasGrant, useLoad, t,
} from '@ui';
import { useEffect, useState } from 'react';
import { Markdown } from './island/markdown';

type Policy = Record<string, unknown>;
type Turn = { turnId: string; createdAt: string; questionPreview: string; answerKind?: string; feedback?: string };
type Citation = { citationId?: string; title?: string; href?: string };

const switches: Record<string, [string, string]> = {
  enabled: [t('公开问答'), t('打开后访客可以在文档页右下角和独立问答页提问。')],
  compactionEnabled: [t('长对话压缩'), t('对话快占满回答模型的上下文窗口时，把早期内容压缩成摘要；关闭则只带得下的最近消息。')],
};
// 表单按这里的先后排列：回答额度、对话压缩与保留期。
const numbers: Record<string, string> = {
  maximumContextTokens: t('上下文最大词元'), maximumOutputTokens: t('回答最大输出词元'), maximumQuestionCharacters: t('问题最大字符数'),
  contextReservePercent: t('上下文预留比例（%）'), retainedRecentTokens: t('压缩时保留的近期词元'), maximumHistoryTokens: t('历史最大词元'),
  maximumRetrievalRounds: t('最大模型轮数'), maximumToolCalls: t('最大工具调用数'), recordRetentionDays: t('问答记录保留天数'),
};
const numberHints: Record<string, string> = {
  maximumContextTokens: t('交给回答模型的文章原文上限；实际还不超过回答模型窗口的三分之一。'),
};
const hidden = new Set(['version', 'retrievalSpace', 'maximumHistoryTokens', 'profileId', 'maximumContextTokens', 'contextReservePercent', 'retainedRecentTokens', 'compactionEnabled', 'maximumRetrievalRounds', 'maximumToolCalls']);
const answerKinds: Record<string, string> = { answered: t('已回答'), refused: t('拒答'), no_evidence: t('无依据'), out_of_scope: t('离题/闲聊'), clarification: t('需澄清'), failed: t('失败'), cancelled: t('已取消') };

export default function QaPage() {
  const canManage = useHasGrant('qa.manage');
  const canDebug = useHasGrant('qa.debug');
  const [tab, setTab] = useState(canManage ? 'turns' : 'debug');
  return (
    <div className="flex flex-col gap-6">
      <PageHeader title={t('问答记录')} description={t('查看访客的问题和 AI 回答。记录默认保留 15 天，到期自动清除。')} />
      <Tabs selectedKey={tab} onSelectionChange={(key) => setTab(String(key))}>
        <Tabs.ListContainer className="mb-4 overflow-x-auto">
          <Tabs.List aria-label={t('问答管理')}>
            {canManage ? <Tabs.Tab id="policy">{t('策略')}<Tabs.Indicator /></Tabs.Tab> : null}
            {canManage ? <Tabs.Tab id="turns">{t('问答记录')}<Tabs.Indicator /></Tabs.Tab> : null}
            {canDebug ? <Tabs.Tab id="debug">{t('调试台')}<Tabs.Indicator /></Tabs.Tab> : null}
          </Tabs.List>
        </Tabs.ListContainer>
        {canManage ? <Tabs.Panel id="policy"><PolicyTab /></Tabs.Panel> : null}
        {canManage ? <Tabs.Panel id="turns"><TurnsTab /></Tabs.Panel> : null}
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
  const order = [...Object.keys(switches), ...Object.keys(numbers)];
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
          {keys.filter((key) => typeof policy[key] === 'number').map((key) => (
            <NumberInput key={key} description={numberHints[key]} label={numbers[key] ?? key} minValue={0} step={1} value={policy[key] as number} onChange={(value) => set(key, value)} />
          ))}
        </FormGrid>
      </Panel>
      <div className="flex justify-end"><Button isPending={busy} type="submit">{t('保存策略')}</Button></div>
    </Form>
  );
}

function TurnsTab() {
  const [cursor, setCursor] = useState('');
  const [filter, setFilter] = useState('all');
  const turns = useLoad((signal) => api<{ items: Turn[]; pageInfo: { hasMore: boolean; nextCursor?: string } }>(`/v1/admin/turns?limit=30&filter=${filter}${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ''}`, { signal }), [cursor, filter]);
  const [selected, setSelected] = useState<{ question: string; answer: string; sources?: Citation[] } | null>(null);
  const { busy, run } = useAction();
  const open = (turn: Turn) => run(async () => setSelected(await api(`/v1/admin/turns/${turn.turnId}`)));
  if (turns.error && !turns.data) return <ErrorBlock error={turns.error} onRetry={turns.reload} />;
  if (!turns.data) return <LoadingBlock lines={5} />;
  return <div className="flex flex-col gap-4">
    <div className="flex flex-wrap items-end justify-between gap-3">
      <SelectInput label={t('结果')} value={filter} onChange={(value) => { setFilter(value); setCursor(''); }} options={[
        { value: 'all', label: t('全部') }, { value: 'no_evidence', label: t('无依据') }, { value: 'failed', label: t('失败') }, { value: 'unhelpful', label: t('没帮助') },
      ]} />
      <Button variant="secondary" onPress={turns.reload}>{t('刷新')}</Button>
    </div>
    {!turns.data.items.length ? <EmptyBlock icon={<Comments />} title={t('还没有问答记录')} description={t('开启公开问答后，访客的提问会出现在这里。')} /> : <Table>
      <Table.ScrollContainer><Table.Content aria-label={t('问答记录')} className="min-w-[640px]">
        <Table.Header><Table.Column isRowHeader>{t('问题')}</Table.Column><Table.Column>{t('结果')}</Table.Column><Table.Column>{t('时间')}</Table.Column><Table.Column className="text-end">{t('操作')}</Table.Column></Table.Header>
        <Table.Body>{turns.data.items.map((turn) => <Table.Row key={turn.turnId} id={turn.turnId}>
          <Table.Cell><button className="line-clamp-2 text-start hover:text-accent" onClick={() => void open(turn)}>{turn.questionPreview}</button></Table.Cell>
          <Table.Cell><span className="flex flex-wrap gap-1"><Chip color={turn.answerKind === 'answered' ? 'success' : turn.answerKind === 'failed' ? 'danger' : 'default'} size="sm" variant="soft">{(turn.answerKind && answerKinds[turn.answerKind]) || turn.answerKind || t('未完成')}</Chip>{turn.feedback && <Chip size="sm" variant="soft">{turn.feedback === 'helpful' ? t('有帮助') : t('没帮助')}</Chip>}</span></Table.Cell>
          <Table.Cell><span title={fmtDateTime(turn.createdAt)}>{fmtRelative(turn.createdAt)}</span></Table.Cell>
          <Table.Cell className="text-end"><Button isDisabled={busy} size="sm" variant="secondary" onPress={() => open(turn)}>{t('查看')}</Button></Table.Cell>
        </Table.Row>)}</Table.Body>
      </Table.Content></Table.ScrollContainer>
    </Table>}
    <div className="flex justify-end gap-2">{cursor && <Button variant="tertiary" onPress={() => setCursor('')}>{t('第一页')}</Button>}{turns.data.pageInfo.hasMore && <Button variant="secondary" onPress={() => setCursor(turns.data!.pageInfo.nextCursor ?? '')}>{t('下一页')}</Button>}</div>
    {selected && <Modal.Backdrop isOpen onOpenChange={(value) => { if (!value) setSelected(null); }}><Modal.Container scroll="outside" size="lg"><Modal.Dialog>
      <Modal.CloseTrigger aria-label={t('关闭')} /><Modal.Header><Modal.Heading>{t('问答记录')}</Modal.Heading></Modal.Header>
      <Modal.Body className="flex flex-col gap-5"><section><h3 className="mb-2 text-sm font-medium text-muted">{t('问题')}</h3><p className="whitespace-pre-wrap break-words">{selected.question}</p></section><section><h3 className="mb-2 text-sm font-medium text-muted">{t('回答')}</h3>{selected.answer ? <Markdown text={selected.answer} citations={selected.sources ?? []} referenceLabel={t('参考文献')} /> : <p className="text-muted">{t('未完成')}</p>}</section></Modal.Body>
      <Modal.Footer><Button variant="secondary" onPress={() => setSelected(null)}>{t('关闭')}</Button></Modal.Footer>
    </Modal.Dialog></Modal.Container></Modal.Backdrop>}
  </div>;
}

// 调试台：管理员直接试问，结果只存在隔离的诊断记录里（7 天），不影响访客对话；使用“后台任务”用途的档位。
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
      <Panel className="xl:col-span-2" title={t('试问')} description={t('检索知识库并调用模型回答，结果不影响访客对话。')}>
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
        {state.answer && <div className="rounded-xl bg-surface-secondary p-4 text-foreground"><Markdown citations={state.citations} text={state.answer} referenceLabel={t('参考文献')} /></div>}
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
