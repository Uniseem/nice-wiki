import { Ban as BanIcon, Shield } from '@gravity-ui/icons';
import { Button, Chip, Form, Modal, Table, Tabs } from '@heroui/react';
import {
  api, ApiError, EmptyBlock, ErrorBlock, fmtDateTime, fmtRelative, FormGrid, LoadingBlock, Mono, NumberInput, PageHeader, Panel, SelectInput, shortId, SwitchInput, TextInput, useAction, useLoad, t,
} from '@ui';
import { useEffect, useState } from 'react';

type Policy = Record<string, number | boolean>;
type Ban = { ip: string; banId: string; subjectRef: string; reasonCode: string; state: string; expiresAt: string; revision: number };
type RiskEvent = { eventId: string; occurredAt: string; action: string; decision: string; networkKey: string; ip: string };

const labels: Record<string, [string, string?]> = {
  qaPerSubjectPerMinute: [t('每位访客每分钟提问上限')], qaBurst: [t('每位访客突发次数'), t('短时间内允许连续提问的次数')], qaPerSitePerMinute: [t('全站每分钟提问上限')],
  qaSubjectConcurrency: [t('每位访客并发')], qaSiteConcurrency: [t('全站并发')], qaPerNetworkPerMinute: [t('每个网络每分钟上限'), t('同一网络（如同一公司出口）合计')],
  challengeEnabled: [t('工作量证明挑战'), t('可疑请求先在浏览器里完成一次计算验证，挡住脚本刷量。')], externalChallengeEnabled: [t('外部挑战'), t('需要另行配置第三方验证的密钥。')],
};
// 防滥用默认策略；模型接入的一键配置在没保存过策略时也用它。
export const guardDefaults: Policy = { qaPerSubjectPerMinute: 30, qaBurst: 5, qaPerSitePerMinute: 600, qaSubjectConcurrency: 1, qaSiteConcurrency: 32, challengeEnabled: true, externalChallengeEnabled: false, qaPerNetworkPerMinute: 60 };
const banState: Record<string, { label: string; tone: 'danger' | 'default' }> = { active: { label: t('生效中'), tone: 'danger' }, expired: { label: t('已过期'), tone: 'default' }, revoked: { label: t('已撤销'), tone: 'default' } };
const durations = [{ value: '3600', label: t('1 小时') }, { value: '86400', label: t('1 天') }, { value: '604800', label: t('7 天') }, { value: '2592000', label: t('30 天') }];

export default function GuardPage() {
  const [tab, setTab] = useState('policy');
  const [banTarget, setBanTarget] = useState<string | null>(null);
  const bans = useLoad((signal) => api<{ items: Ban[] }>('/v1/bans?limit=50', { signal }).then((result) => result.items), []);
  return (
    <div className="flex flex-col gap-6">
      <PageHeader title={t('防滥用')} description={t('Agent 上报滥用信号，服务端控制频率、并发与人机验证。禁令按单个 IP 和时长执行，不影响阅读文档。')} actions={<Button variant="secondary" onPress={() => setBanTarget('')}><BanIcon />{t('新建禁令')}</Button>} />
      <Tabs selectedKey={tab} onSelectionChange={(key) => setTab(String(key))}>
        <Tabs.ListContainer className="mb-4">
          <Tabs.List aria-label={t('防滥用')}>
            <Tabs.Tab id="policy">{t('准入策略')}<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="bans">{t('禁令')}<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="events">{t('可疑事件')}<Tabs.Indicator /></Tabs.Tab>
          </Tabs.List>
        </Tabs.ListContainer>
        <Tabs.Panel id="policy"><PolicyTab /></Tabs.Panel>
        <Tabs.Panel id="bans"><BansTab bans={bans.data} error={bans.error} reload={bans.reload} /></Tabs.Panel>
        <Tabs.Panel id="events"><EventsTab onBan={setBanTarget} /></Tabs.Panel>
      </Tabs>
      {banTarget !== null && <BanDialog subjectRef={banTarget} onClose={() => setBanTarget(null)} onSaved={() => { setBanTarget(null); bans.reload(); setTab('bans'); }} />}
    </div>
  );
}

function PolicyTab() {
  const loaded = useLoad(async (signal) => {
    const [self, current] = await Promise.all([
      api<{ account: { siteId: string } }>('/v1/self', { signal }),
      // 还没保存过策略时给出默认值，保存后版本从 0 起。
      api<{ policyJSON: Policy; revision: number }>('/v1/policies', { signal }).catch((problem) => {
        if (problem instanceof ApiError && problem.status === 404) return { policyJSON: guardDefaults, revision: 0 };
        throw problem;
      }),
    ]);
    return { siteId: self.account.siteId, ...current };
  }, []);
  const [policy, setPolicy] = useState<Policy | null>(null);
  const { busy, run } = useAction();
  useEffect(() => { if (loaded.data) setPolicy(loaded.data.policyJSON); }, [loaded.data]);
  if (loaded.error && !loaded.data) return <ErrorBlock error={loaded.error} onRetry={loaded.reload} />;
  if (!policy || !loaded.data) return <LoadingBlock lines={5} />;
  const keys = Object.keys(policy);
  const save = () => run(async () => {
    await api('/v1/policies', { method: 'PUT', body: { siteId: loaded.data!.siteId, expectedRevision: loaded.data!.revision, policyJSON: policy } });
    loaded.reload();
  }, t('策略已保存'));
  return (
    <Form className="flex flex-col gap-6" onSubmit={(event) => { event.preventDefault(); void save(); }}>
      <Panel title={t('提问频率与并发')} description={loaded.data.revision ? t('策略版本 #{0}', loaded.data.revision) : t('还没有保存过，下面是默认值。')}>
        <FormGrid columns={3}>
          {keys.filter((key) => typeof policy[key] === 'number').map((key) => (
            <NumberInput key={key} description={labels[key]?.[1]} label={labels[key]?.[0] ?? key} minValue={1} value={policy[key] as number} onChange={(value) => setPolicy({ ...policy, [key]: value })} />
          ))}
        </FormGrid>
      </Panel>
      <Panel title={t('人机验证')}>
        <div className="grid gap-5 md:grid-cols-2">
          {keys.filter((key) => typeof policy[key] === 'boolean').map((key) => (
            <SwitchInput key={key} description={labels[key]?.[1]} isSelected={policy[key] as boolean} label={labels[key]?.[0] ?? key} onChange={(value) => setPolicy({ ...policy, [key]: value })} />
          ))}
        </div>
      </Panel>
      <div className="flex justify-end"><Button isPending={busy} type="submit">{t('保存策略')}</Button></div>
    </Form>
  );
}

function BansTab({ bans, error, reload }: { bans?: Ban[]; error: unknown; reload: () => void }) {
  const { busy, run } = useAction();
  const revoke = (ban: Ban) => run(async () => {
    await api(`/v1/bans/${ban.banId}`, { method: 'DELETE', body: { expectedRevision: ban.revision } });
    reload();
  }, t('禁令已撤销'));
  if (error && !bans) return <ErrorBlock error={error} onRetry={reload} />;
  if (!bans) return <LoadingBlock lines={4} />;
  if (!bans.length) return <EmptyBlock icon={<Shield />} title={t('没有禁令')} description={t('输入 IP 和时长即可封禁，也可以从可疑事件中选择。')} />;
  return (
    <Table>
      <Table.ScrollContainer>
        <Table.Content aria-label={t('禁令')} className="min-w-[640px]">
          <Table.Header>
            <Table.Column isRowHeader>IP</Table.Column>
            <Table.Column>{t('原因')}</Table.Column>
            <Table.Column>{t('状态')}</Table.Column>
            <Table.Column>{t('到期')}</Table.Column>
            <Table.Column className="text-end">{t('操作')}</Table.Column>
          </Table.Header>
          <Table.Body>
            {bans.map((ban) => (
              <Table.Row key={ban.banId} id={ban.banId}>
                <Table.Cell><Mono>{ban.ip || shortId(ban.subjectRef, 16)}</Mono></Table.Cell>
                <Table.Cell>{ban.reasonCode === 'abuse' ? t('滥用') : t('人工')}</Table.Cell>
                <Table.Cell><Chip color={banState[ban.state]?.tone ?? 'default'} size="sm" variant="soft">{banState[ban.state]?.label ?? ban.state}</Chip></Table.Cell>
                <Table.Cell>{fmtDateTime(ban.expiresAt)}</Table.Cell>
                <Table.Cell className="text-end">{ban.state === 'active' && <Button isDisabled={busy} size="sm" variant="tertiary" onPress={() => revoke(ban)}>{t('撤销')}</Button>}</Table.Cell>
              </Table.Row>
            ))}
          </Table.Body>
        </Table.Content>
      </Table.ScrollContainer>
    </Table>
  );
}

function EventsTab({ onBan }: { onBan: (subjectRef: string) => void }) {
  const events = useLoad((signal) => api<{ items: RiskEvent[] }>('/v1/risk-events?limit=50', { signal }).then((result) => result.items), []);
  if (events.error && !events.data) return <ErrorBlock error={events.error} onRetry={events.reload} />;
  if (!events.data) return <LoadingBlock lines={4} />;
  if (!events.data.length) return <EmptyBlock icon={<Shield />} title={t('暂无可疑事件')} description={t('被限流、要求验证或拒绝的请求会记在这里。')} />;
  return (
    <Table>
      <Table.ScrollContainer>
        <Table.Content aria-label={t('可疑事件')} className="min-w-[640px]">
          <Table.Header>
            <Table.Column isRowHeader>{t('时间')}</Table.Column>
            <Table.Column>{t('动作')}</Table.Column>
            <Table.Column>{t('处置')}</Table.Column>
            <Table.Column>{t('网络标识')}</Table.Column>
            <Table.Column className="text-end">{t('操作')}</Table.Column>
          </Table.Header>
          <Table.Body>
            {events.data.map((event) => (
              <Table.Row key={event.eventId} id={event.eventId}>
                <Table.Cell><span title={fmtDateTime(event.occurredAt)}>{fmtRelative(event.occurredAt)}</span></Table.Cell>
                <Table.Cell>{event.action}</Table.Cell>
                <Table.Cell><Chip color={event.decision === 'deny' ? 'danger' : 'warning'} size="sm" variant="soft">{event.decision}</Chip></Table.Cell>
                <Table.Cell><Mono>{event.ip || shortId(event.networkKey, 16)}</Mono></Table.Cell>
                <Table.Cell className="text-end">{/^[0-9a-f]{64}$/.test(event.networkKey) && <Button size="sm" variant="tertiary" onPress={() => onBan(event.ip)}><BanIcon />{t('封禁')}</Button>}</Table.Cell>
              </Table.Row>
            ))}
          </Table.Body>
        </Table.Content>
      </Table.ScrollContainer>
    </Table>
  );
}

function BanDialog({ subjectRef, onClose, onSaved }: { subjectRef: string; onClose: () => void; onSaved: () => void }) {
  const [ref, setRef] = useState(subjectRef);
  const [ttl, setTtl] = useState('86400');
  const [reason, setReason] = useState<'manual' | 'abuse'>('abuse');
  const { busy, run } = useAction();
  const valid = ref.trim().length > 1 && ref.trim().length <= 45;
  const submit = async () => {
    const done = await run(async () => {
      await api('/v1/bans', { method: 'POST', body: { ip: ref.trim(), ttlSeconds: Number(ttl), reasonCode: reason } });
      return true;
    }, t('禁令已建立'));
    if (done) onSaved();
  };
  return (
    <Modal.Backdrop isOpen onOpenChange={(open) => { if (!open) onClose(); }}>
      <Modal.Container size="md">
        <Modal.Dialog>
          <Modal.CloseTrigger aria-label={t('关闭')} />
          <Modal.Header><Modal.Heading>{t('新建禁令')}</Modal.Heading></Modal.Header>
          <Form className="flex flex-col" onSubmit={(event) => { event.preventDefault(); if (valid) void submit(); }}>
            <Modal.Body className="flex flex-col gap-4">
              <TextInput isRequired label="IP" value={ref} onChange={setRef} />
              <FormGrid>
                <NumberInput label={t('时长（分钟）')} minValue={1} maxValue={43200} value={Number(ttl) / 60} onChange={(value)=>setTtl(String(Math.round(value * 60)))} />
                <SelectInput label={t('原因')} options={[{ value: 'abuse', label: t('滥用') }, { value: 'manual', label: t('人工') }]} value={reason} onChange={setReason} />
              </FormGrid>
              <p className="text-sm text-muted">{t('禁令只限制 AI 问答，不影响阅读文档。')}</p>
            </Modal.Body>
            <Modal.Footer>
              <Button variant="tertiary" onPress={onClose}>{t('取消')}</Button>
              <Button isDisabled={!valid} isPending={busy} type="submit" variant="danger">{t('封禁')}</Button>
            </Modal.Footer>
          </Form>
        </Modal.Dialog>
      </Modal.Container>
    </Modal.Backdrop>
  );
}
