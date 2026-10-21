import { Comments, PaperPlane, Star } from '@gravity-ui/icons';
import { Button, Chip, Form, Modal, Spinner, Table, Tabs } from '@heroui/react';
import {
  api, confirmAction, EmptyBlock, ErrorBlock, errorText, fmtDateTime, fmtRelative, FormGrid, LoadingBlock, newKey, Notice, NumberInput, PageHeader, Panel, SelectInput, SwitchInput, TextAreaInput, TextInput,
  useAction, useHasGrant, useLoad,
} from '@ui';
import { useEffect, useState } from 'react';
import { Markdown } from './island/markdown';

type Policy = Record<string, unknown>;
type Turn = { turnId: string; createdAt: string; questionPreview: string; answerKind?: string; feedback?: string };
type CacheEntry = { cacheId: string; version: number; question: string; answer?: string; language: string; curated?: boolean; expiresAt?: string | null; state: string };
type Citation = { citationId?: string; title?: string; href?: string };

const switches: Record<string, [string, string]> = {
  enabled: ['公开问答', '打开后访客可以在文档页右下角和独立问答页提问。'],
  cacheEnabled: ['答案缓存', '完整、可共享的问题会先查缓存，命中就不再调用回答模型。'],
  compactionEnabled: ['长对话压缩', '对话快占满回答模型的上下文窗口时，把早期内容压缩成摘要；关闭则只带得下的最近消息。'],
};
const numbers: Record<string, string> = {
  maximumOutputTokens: '回答最大输出词元', maximumHistoryTokens: '历史最大词元', maximumContextTokens: '上下文最大词元',
  maximumQuestionCharacters: '问题最大字符数', preflightOutputTokens: '预检输出词元', inspectionOutputTokens: '核查输出词元',
  maximumSubqueries: '最多子问题数', maximumRetrievalRounds: '最多检索轮数', maximumToolCalls: '最多工具调用', cacheTtlDays: '缓存保留天数',
  recordRetentionDays: '问答记录保留天数', retainedRecentTokens: '压缩时保留的近期词元', contextReservePercent: '上下文预留比例（%）',
  semanticThreshold: '语义相似阈值（0–1）',
};
const texts: Record<string, string> = { profileId: '答案模型档位', semanticMode: '语义判定方式' };
const semanticModes = [{ value: 'ai_equivalence', label: '由 AI 判断是否同一问题' }, { value: 'threshold', label: '按语义相似阈值' }];
type ProfileOption = { profileId: string; costClass: string; tier: string };
const chatTiers: Record<string, string> = { fast: '快速', accurate: '准确' };
// 模型目录需要“模型接入”权限；读不到时档位退回手填。
const useProfiles = () => useLoad((signal) => api<{ profiles: ProfileOption[] }>('/v1/admin/catalog', { signal }).then((result) => result.profiles).catch(() => null), []);

function ProfileField({ label, description, costClass, profiles, value, onChange }: {
  label: string; description?: string; costClass: string; profiles: ProfileOption[] | null | undefined; value: string; onChange: (value: string) => void;
}) {
  const options = (profiles ?? []).filter((profile) => profile.costClass === costClass && chatTiers[profile.tier]).map((profile) => ({ value: profile.profileId, label: `${profile.profileId}（${chatTiers[profile.tier]}）` }));
  if (!options.length) return <TextInput isRequired description={description} label={label} value={value} onChange={onChange} />;
  if (value && !options.some((option) => option.value === value)) options.unshift({ value, label: `${value}（模型接入里没有这个档位）` });
  return <SelectInput description={description} label={label} options={options} value={value} onChange={onChange} />;
}
// 历史预算跟随回答模型的上下文窗口，不再单独设上限。
const hidden = new Set(['version', 'retrievalSpace', 'maximumHistoryTokens']);
const answerKinds: Record<string, string> = { answered: '已回答', refused: '拒答', cached: '缓存命中', no_evidence: '无依据', out_of_scope: '离题/闲聊', clarification: '需澄清', failed: '失败', cancelled: '已取消' };

export default function QaPage() {
  const canManage = useHasGrant('qa.manage');
  const canDebug = useHasGrant('qa.debug');
  const [tab, setTab] = useState(canManage ? 'policy' : 'debug');
  return (
    <div className="flex flex-col gap-6">
      <PageHeader title="问答" description="访客问答的开关、额度与缓存。每次回答都会调用模型；开启前请先配好“模型接入”“知识索引”和“防滥用”。" />
      <Tabs selectedKey={tab} onSelectionChange={(key) => setTab(String(key))}>
        <Tabs.ListContainer className="mb-4 overflow-x-auto">
          <Tabs.List aria-label="问答管理">
            {canManage ? <Tabs.Tab id="policy">策略<Tabs.Indicator /></Tabs.Tab> : null}
            {canManage ? <Tabs.Tab id="turns">问答记录<Tabs.Indicator /></Tabs.Tab> : null}
            {canManage ? <Tabs.Tab id="cache">答案缓存<Tabs.Indicator /></Tabs.Tab> : null}
            {canDebug ? <Tabs.Tab id="debug">调试台<Tabs.Indicator /></Tabs.Tab> : null}
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
  const profiles = useProfiles();
  const [policy, setPolicy] = useState<Policy | null>(null);
  const { busy, run } = useAction();
  useEffect(() => { if (current.data) setPolicy(current.data.policy); }, [current.data]);
  if (current.error && !current.data) return <ErrorBlock error={current.error} onRetry={current.reload} />;
  if (!policy || !current.data) return <LoadingBlock lines={6} />;
  const set = (key: string, value: unknown) => setPolicy({ ...policy, [key]: value });
  const keys = Object.keys(policy).filter((key) => !hidden.has(key));
  const staleSpace = indexSpace.data != null && JSON.stringify(indexSpace.data) !== JSON.stringify(policy.retrievalSpace);
  const save = () => run(async () => {
    const idempotencyKey = newKey();
    const next = staleSpace ? { ...policy, retrievalSpace: indexSpace.data } : policy;
    await api('/v1/admin/policy', { method: 'PUT', idempotencyKey, body: { policy: next, expectedVersion: current.data!.version, idempotencyKey } });
    current.reload();
  }, '策略已保存');
  return (
    <Form className="flex flex-col gap-6" onSubmit={(event) => { event.preventDefault(); void save(); }}>
      {!policy.enabled && <Notice status="default" title="公开问答目前关闭">访客看不到问答入口。确认模型、索引和防滥用都已就绪后再打开。</Notice>}
      {staleSpace && <Notice status="warning" title="知识索引换了向量模型">问答还在用旧的向量空间检索，访客提问会失败。保存一次策略即可切换到新的空间。</Notice>}
      <Panel title="开关" description={`策略版本 #${current.data.version}`}>
        <div className="grid gap-5 md:grid-cols-3">
          {keys.filter((key) => typeof policy[key] === 'boolean').map((key) => (
            <SwitchInput key={key} description={switches[key]?.[1]} isSelected={policy[key] as boolean} label={switches[key]?.[0] ?? key} onChange={(value) => set(key, value)} />
          ))}
        </div>
      </Panel>
      <Panel title="模型与额度" description="数值越大回答越充分，但每次调用的词元也越多。">
        <FormGrid columns={3}>
          {keys.filter((key) => typeof policy[key] === 'string').map((key) => key === 'profileId'
            ? <ProfileField key={key} costClass="qa" description="只列出“访客问答”用途的对话档位。" label={texts[key]} profiles={profiles.data} value={String(policy[key] ?? '')} onChange={(value) => set(key, value)} />
            : key === 'semanticMode'
              ? <SelectInput key={key} label={texts[key]} options={semanticModes} value={String(policy[key] ?? '')} onChange={(value) => set(key, value)} />
              : <TextInput key={key} label={texts[key] ?? key} value={String(policy[key] ?? '')} onChange={(value) => set(key, value)} />)}
          {keys.filter((key) => typeof policy[key] === 'number').map((key) => (
            <NumberInput key={key} formatOptions={key === 'semanticThreshold' ? { maximumFractionDigits: 3 } : undefined} label={numbers[key] ?? key} minValue={0} step={key === 'semanticThreshold' ? 0.01 : 1} value={policy[key] as number} onChange={(value) => set(key, value)} />
          ))}
        </FormGrid>
      </Panel>
      <div className="flex justify-end"><Button isPending={busy} type="submit">保存策略</Button></div>
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
    }, '已提交为待批准的精选，请到“答案缓存”里批准');
    if (done) setCurate(null);
  };
  if (turns.error && !turns.data) return <ErrorBlock error={turns.error} onRetry={turns.reload} />;
  if (!turns.data) return <LoadingBlock lines={5} />;
  return (
    <>
      {!turns.data.length ? <EmptyBlock icon={<Comments />} title="还没有问答记录" description="开启公开问答后，访客的提问会出现在这里。" /> : (
        <Table>
          <Table.ScrollContainer>
            <Table.Content aria-label="问答记录" className="min-w-[640px]">
              <Table.Header>
                <Table.Column isRowHeader>问题</Table.Column>
                <Table.Column>结果</Table.Column>
                <Table.Column>时间</Table.Column>
                <Table.Column className="text-end">操作</Table.Column>
              </Table.Header>
              <Table.Body>
                {turns.data.map((turn) => (
                  <Table.Row key={turn.turnId} id={turn.turnId}>
                    <Table.Cell><span className="line-clamp-2">{turn.questionPreview}</span></Table.Cell>
                    <Table.Cell>
                      <span className="flex flex-wrap gap-1">
                        <Chip color={turn.answerKind === 'answered' ? 'success' : turn.answerKind === 'failed' ? 'danger' : 'default'} size="sm" variant="soft">{(turn.answerKind && answerKinds[turn.answerKind]) || turn.answerKind || '未完成'}</Chip>
                        {turn.feedback && <Chip color={turn.feedback === 'helpful' ? 'success' : 'warning'} size="sm" variant="soft">{turn.feedback === 'helpful' ? '有帮助' : '没帮助'}</Chip>}
                      </span>
                    </Table.Cell>
                    <Table.Cell><span title={fmtDateTime(turn.createdAt)}>{fmtRelative(turn.createdAt)}</span></Table.Cell>
                    <Table.Cell className="text-end">{turn.answerKind === 'answered' && <Button isDisabled={busy} size="sm" variant="secondary" onPress={() => open(turn)}><Star />设为精选</Button>}</Table.Cell>
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
              <Modal.CloseTrigger aria-label="关闭" />
              <Modal.Header><Modal.Heading>设为精选答案</Modal.Heading></Modal.Header>
              <Form className="flex flex-col" onSubmit={(event) => { event.preventDefault(); void submit(); }}>
                <Modal.Body className="flex flex-col gap-4">
                  <Notice status="accent">精选会公开给所有访客。去掉个人信息，把问题改成通用问法。批准后长期有效、优先命中，来源文档撤回或改动时自动失效。</Notice>
                  <TextInput isRequired label="公开问题" value={curate.question} onChange={(question) => setCurate({ ...curate, question })} />
                  <TextAreaInput isRequired label="公开答案" rows={8} value={curate.answer} onChange={(answer) => setCurate({ ...curate, answer })} />
                </Modal.Body>
                <Modal.Footer>
                  <Button variant="tertiary" onPress={() => setCurate(null)}>取消</Button>
                  <Button isPending={busy} type="submit">提交精选</Button>
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
  }, '精选已批准');
  const disable = async (entry: CacheEntry) => {
    if (!(await confirmAction({ title: '禁用这条缓存？', body: `禁用后不会再复用这个答案：“${entry.question}”`, confirmLabel: '禁用', tone: 'warning' }))) return;
    await run(async () => {
      const idempotencyKey = newKey();
      await api(`/v1/admin/cache/${entry.cacheId}/disable`, { method: 'POST', idempotencyKey, body: { cacheId: entry.cacheId, expectedVersion: entry.version, idempotencyKey } });
      entries.reload();
    }, '已禁用');
  };
  if (entries.error && !entries.data) return <ErrorBlock error={entries.error} onRetry={entries.reload} />;
  if (!entries.data) return <LoadingBlock lines={5} />;
  if (!entries.data.length) return <EmptyBlock icon={<Comments />} title="还没有缓存条目" description="打开“答案缓存”后，完整且可共享的问题回答完会自动写入；撤回或改动来源文档后，相关缓存不再命中。" />;
  return (
    <Table>
      <Table.ScrollContainer>
        <Table.Content aria-label="答案缓存" className="min-w-[680px]">
          <Table.Header>
            <Table.Column isRowHeader>问题</Table.Column>
            <Table.Column>答案</Table.Column>
            <Table.Column>状态</Table.Column>
            <Table.Column className="text-end">操作</Table.Column>
          </Table.Header>
          <Table.Body>
            {entries.data.map((entry) => (
              <Table.Row key={entry.cacheId} id={entry.cacheId}>
                <Table.Cell><span className="line-clamp-2 font-medium">{entry.question}</span><span className="text-xs text-muted">{entry.language}</span></Table.Cell>
                <Table.Cell><span className="line-clamp-2 text-sm text-muted">{entry.answer ?? ''}</span></Table.Cell>
                <Table.Cell>
                  <span className="flex flex-wrap gap-1">
                    {entry.curated ? <Chip color="accent" size="sm" variant="soft">精选</Chip> : <Chip size="sm" variant="soft">自动</Chip>}
                    {entry.state === 'pending' && <Chip color="warning" size="sm" variant="soft">待批准</Chip>}
                    {entry.state === 'disabled' && <Chip color="danger" size="sm" variant="soft">已禁用</Chip>}
                    {!entry.curated && entry.expiresAt && <span className="text-xs text-muted">{fmtDateTime(entry.expiresAt)} 过期</span>}
                  </span>
                </Table.Cell>
                <Table.Cell className="text-end">
                  <span className="inline-flex gap-2">
                    {entry.state === 'pending' && <Button isDisabled={busy} size="sm" onPress={() => approve(entry)}>批准</Button>}
                    {entry.state !== 'disabled' && <Button isDisabled={busy} size="sm" variant="tertiary" onPress={() => disable(entry)}>禁用</Button>}
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
  const [profileId, setProfileId] = useState('background-accurate');
  const profiles = useProfiles();
  const [state, setState] = useState<{ running: boolean; answer: string; citations: Citation[]; status: '' | 'running' | 'done' | 'failed'; error: string }>({ running: false, answer: '', citations: [], status: '', error: '' });

  async function ask() {
    if (!question.trim() || state.running) return;
    setState({ running: true, answer: '', citations: [], status: 'running', error: '' });
    try {
      const idempotencyKey = newKey();
      const created = await api<{ debug: { debugId: string } }>('/v1/admin/debug-turns', { method: 'POST', idempotencyKey, body: { question, profileId, language: 'zh-Hans', idempotencyKey } });
      for (let i = 0; i < 90; i++) {
        await new Promise((resolve) => setTimeout(resolve, 2000));
        const read = await api<{ events: { event: string; delta?: string; citation?: Citation; code?: string }[] }>(`/v1/admin/debug-turns/${created.debug.debugId}?afterSequence=0`);
        const answer = read.events.filter((e) => e.event === 'delta').map((e) => e.delta ?? '').join('');
        const citations = read.events.filter((e) => e.event === 'citation' && e.citation).map((e) => e.citation!);
        const last = read.events[read.events.length - 1];
        if (last?.event === 'completed') { setState({ running: false, answer, citations, status: 'done', error: '' }); return; }
        if (last?.event === 'failed') {
          setState({ running: false, answer, citations, status: 'failed', error: `失败原因：${last.code ?? '未知'}。常见原因：诊断档位不存在或不属于“后台任务”用途、知识库没有索引、模型没有配置。` });
          return;
        }
        setState((current) => ({ ...current, answer, citations }));
      }
      setState((current) => ({ ...current, running: false, status: 'failed', error: '等待超时（3 分钟），请稍后在问答记录里查看。' }));
    } catch (problem) {
      setState((current) => ({ ...current, running: false, status: 'failed', error: errorText(problem) }));
    }
  }

  return (
    <div className="grid gap-6 xl:grid-cols-5">
      <Panel className="xl:col-span-2" title="试问" description="检索知识库并调用模型回答，结果不影响访客对话和缓存。">
        <Form className="flex flex-col gap-4" onSubmit={(event) => { event.preventDefault(); void ask(); }}>
          <TextAreaInput isRequired label="问题" maxLength={500} placeholder="例如：如何安装？" rows={4} value={question} onChange={setQuestion} />
          <ProfileField costClass="background" description="只列出“后台任务”用途的对话档位。" label="诊断档位" profiles={profiles.data} value={profileId} onChange={setProfileId} />
          <div className="flex justify-end"><Button isPending={state.running} type="submit"><PaperPlane />{state.running ? '诊断中…' : '试问'}</Button></div>
        </Form>
      </Panel>
      <Panel className="xl:col-span-3" title="结果">
        {!state.status && <p className="text-sm text-muted">回答、引用和失败原因会显示在这里。</p>}
        {state.status === 'running' && !state.answer && <div className="flex items-center gap-2 text-sm text-muted"><Spinner size="sm" />正在检索并生成回答…</div>}
        {/* 与访客看到的一样渲染：Markdown 与上标引用编号。 */}
        {state.answer && <div className="rounded-xl bg-surface-secondary p-4 text-foreground"><Markdown citations={state.citations} text={state.answer} /></div>}
        {state.citations.length > 0 && (
          <div className="flex flex-col gap-1.5">
            <span className="text-xs font-medium text-muted">引用来源</span>
            {state.citations.map((citation, index) => (
              <a key={index} className="truncate text-sm text-accent hover:underline" href={citation.href} rel="noopener" target="_blank">{citation.title || citation.href}</a>
            ))}
          </div>
        )}
        {state.status === 'done' && <Chip color="success" size="sm" variant="soft">完成</Chip>}
        {state.error && <Notice status="danger">{state.error}</Notice>}
      </Panel>
    </div>
  );
}
