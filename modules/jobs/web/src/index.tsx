import { ArrowsRotateRight, ClockArrowRotateLeft, ListCheck } from '@gravity-ui/icons';
import { Button, Chip, Drawer, Form, Modal, ProgressBar, Table, Tabs } from '@heroui/react';
import { api, apiClient, confirmAction, Details, EmptyBlock, ErrorBlock, fmtDateTime, fmtRelative, LoadingBlock, Mono, newKey, notify, Notice, PageHeader, SelectInput, SwitchInput, TextInput, useAction, useLoad } from '@ui';
import { useEffect, useRef, useState, type ReactNode } from 'react';
import type { components } from './gen/api.gen';
import { ConfigureProgressAPI, ReconnectProgress, type ProgressEvent } from './progress';

type Job = components['schemas']['JobSummary'];
type JobDetail = components['schemas']['ReadJobResult'];
type Delivery = components['schemas']['DeliverySummary'];
type Schedule = components['schemas']['Schedule'];

ConfigureProgressAPI(apiClient);

const states: Record<string, { label: string; tone: 'default' | 'accent' | 'success' | 'warning' | 'danger' }> = {
  queued: { label: '排队中', tone: 'default' }, leased: { label: '已领取', tone: 'accent' }, running: { label: '运行中', tone: 'accent' }, 'retry-wait': { label: '等待重试', tone: 'warning' },
  'dead-letter': { label: '死信', tone: 'danger' }, 'cancel-requested': { label: '取消中', tone: 'warning' }, cancelled: { label: '已取消', tone: 'default' }, succeeded: { label: '成功', tone: 'success' }, orphan: { label: '无主', tone: 'warning' },
};
const terminal = new Set(['succeeded', 'cancelled', 'dead-letter']);
const stateChip = (state: string) => <Chip color={states[state]?.tone ?? 'default'} size="sm" variant="soft">{states[state]?.label ?? state}</Chip>;

export default function JobsPage() {
  const [tab, setTab] = useState('jobs');
  return (
    <div className="flex flex-col gap-6">
      <PageHeader title="任务队列" description="后台任务、失败后进入死信的事件投递，以及定时调度。取消只记录意图，由任务所属服务确认安全终态。" />
      <Tabs selectedKey={tab} onSelectionChange={(key) => setTab(String(key))}>
        <Tabs.ListContainer className="mb-4">
          <Tabs.List aria-label="任务">
            <Tabs.Tab id="jobs">任务<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="deliveries">死信投递<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="schedules">定时调度<Tabs.Indicator /></Tabs.Tab>
          </Tabs.List>
        </Tabs.ListContainer>
        <Tabs.Panel id="jobs"><JobsTab /></Tabs.Panel>
        <Tabs.Panel id="deliveries"><DeliveriesTab /></Tabs.Panel>
        <Tabs.Panel id="schedules"><SchedulesTab /></Tabs.Panel>
      </Tabs>
    </div>
  );
}

function JobsTab() {
  const [state, setState] = useState('active');
  const [owner, setOwner] = useState('');
  const [jobs, setJobs] = useState<Job[] | null>(null);
  const [cursor, setCursor] = useState('');
  const [error, setError] = useState<unknown>(null);
  const [loading, setLoading] = useState(false);
  const [selected, setSelected] = useState<Job | null>(null);
  const { busy, run } = useAction();

  const load = async (more: boolean) => {
    setLoading(true);
    setError(null);
    try {
      const query = new URLSearchParams({ state, limit: '50' });
      if (owner.trim()) query.set('owner', owner.trim());
      if (more && cursor) query.set('cursor', cursor);
      const page = await api<components['schemas']['ListJobsResult']>(`/v1/jobs?${query}`);
      setJobs((current) => more && current ? [...current, ...page.items] : page.items);
      setCursor(page.pageInfo.hasMore ? page.pageInfo.nextCursor ?? '' : '');
    } catch (problem) { setError(problem); } finally { setLoading(false); }
  };
  useEffect(() => { void load(false); }, [state]); // 状态筛选变化时重新读取

  const retry = (job: Job) => run(async () => {
    await api(`/v1/jobs/${encodeURIComponent(job.jobId)}:retry`, { method: 'POST', body: { jobId: job.jobId }, idempotencyKey: newKey() });
    await load(false);
  }, '任务已重新排队');

  return (
    <div className="flex flex-col gap-4">
      <Form className="flex flex-wrap items-end gap-3" onSubmit={(event) => { event.preventDefault(); void load(false); }}>
        <SelectInput className="w-40" label="状态" options={[{ value: 'active', label: '活动' }, { value: 'failed', label: '失败' }, { value: 'dead-letter', label: '死信' }, { value: 'all', label: '全部' }]} value={state} onChange={setState} variant="primary" />
        <TextInput className="w-48" label="所属服务" maxLength={64} placeholder="例如 knowledge" value={owner} onChange={setOwner} variant="primary" />
        <Button isPending={loading} type="submit" variant="secondary"><ArrowsRotateRight />刷新</Button>
      </Form>
      {error ? <ErrorBlock error={error} onRetry={() => void load(false)} /> : !jobs ? <LoadingBlock lines={5} /> : !jobs.length ? (
        <EmptyBlock icon={<ListCheck />} title="当前筛选下没有任务" />
      ) : (
        <Table>
          <Table.ScrollContainer>
            <Table.Content aria-label="任务列表" className="min-w-[680px]">
              <Table.Header>
                <Table.Column isRowHeader>任务</Table.Column>
                <Table.Column>状态</Table.Column>
                <Table.Column>进度</Table.Column>
                <Table.Column>尝试</Table.Column>
                <Table.Column className="text-end">操作</Table.Column>
              </Table.Header>
              <Table.Body>
                {jobs.map((job) => {
                  const progress = job.progress;
                  const percent = progress && progress.totalUnits > 0 ? Math.round((progress.completedUnits / progress.totalUnits) * 100) : null;
                  return (
                    <Table.Row key={job.jobId} id={job.jobId}>
                      <Table.Cell>
                        <span className="block font-medium">{job.owner} · {job.type}</span>
                        <Mono className="text-[11px]">{job.jobId}</Mono>
                      </Table.Cell>
                      <Table.Cell>{stateChip(job.state)}{job.lastErrorCode && <span className="ms-2 text-xs text-danger">{job.lastErrorCode}</span>}</Table.Cell>
                      <Table.Cell>
                        {progress ? (
                          <div className="flex min-w-36 flex-col gap-1">
                            <span className="text-xs text-muted">{progress.phase}</span>
                            {percent !== null && <ProgressBar aria-label="进度" size="sm" value={percent}><ProgressBar.Track><ProgressBar.Fill /></ProgressBar.Track></ProgressBar>}
                          </div>
                        ) : <span className="text-xs text-muted">—</span>}
                      </Table.Cell>
                      <Table.Cell className="tabular-nums">{job.attempt}</Table.Cell>
                      <Table.Cell className="text-end">
                        <span className="inline-flex gap-2">
                          {job.state === 'dead-letter' && <Button isDisabled={busy} size="sm" variant="secondary" onPress={() => retry(job)}>重试</Button>}
                          <Button size="sm" variant="tertiary" onPress={() => setSelected(job)}>详情</Button>
                        </span>
                      </Table.Cell>
                    </Table.Row>
                  );
                })}
              </Table.Body>
            </Table.Content>
          </Table.ScrollContainer>
        </Table>
      )}
      {cursor && <div className="flex justify-center"><Button isPending={loading} variant="secondary" onPress={() => void load(true)}>加载更多</Button></div>}
      {selected && <JobDrawer job={selected} onClose={() => setSelected(null)} onChanged={() => void load(false)} />}
    </div>
  );
}

// 任务详情：读取最新状态并连接进度流（断开按 1/2/4/8/16 秒退避重连，任务结束后停止）。
function JobDrawer({ job, onClose, onChanged }: { job: Job; onClose: () => void; onChanged: () => void }) {
  const detail = useLoad((signal) => api<JobDetail>(`/v1/jobs/${encodeURIComponent(job.jobId)}`, { signal }), [job.jobId]);
  const [events, setEvents] = useState<ProgressEvent[]>([]);
  const [stream, setStream] = useState('正在连接进度…');
  const { busy, run } = useAction();
  const list = useRef<HTMLOListElement>(null);

  useEffect(() => {
    const controller = new AbortController();
    void ReconnectProgress({ jobId: job.jobId, lastEventId: job.progress?.seq ?? 0 }, controller.signal, (event) => {
      setEvents((current) => [...current.slice(-199), event]);
      setStream('进度已连接');
    }).then(() => setStream('任务已结束')).catch((problem: unknown) => {
      if ((problem as { name?: string })?.name !== 'AbortError') setStream(`进度已停止：${(problem as { code?: string })?.code ?? '未知原因'}`);
    });
    return () => controller.abort();
  }, [job.jobId, job.progress?.seq]);

  useEffect(() => { list.current?.lastElementChild?.scrollIntoView({ block: 'nearest' }); }, [events]);

  const cancel = async () => {
    if (!(await confirmAction({ title: '请求取消这个任务？', body: '取消请求会交给任务所属服务，在安全的检查点停止；已经发生的费用无法撤销。', confirmLabel: '请求取消', tone: 'warning' }))) return;
    const result = await run(() => api<{ state: string }>(`/v1/jobs/${encodeURIComponent(job.jobId)}:cancel`, { method: 'POST', body: {}, idempotencyKey: newKey() }));
    if (result) { notify.success('已记录取消请求', `当前状态：${states[result.state]?.label ?? result.state}`); detail.reload(); onChanged(); }
  };

  const state = detail.data?.state ?? job.state;
  return (
    <Drawer.Backdrop isOpen onOpenChange={(open) => { if (!open) onClose(); }}>
      <Drawer.Content placement="right">
        <Drawer.Dialog className="w-full max-w-lg">
          <Drawer.CloseTrigger />
          <Drawer.Header><Drawer.Heading>{job.owner} · {job.type}</Drawer.Heading></Drawer.Header>
          <Drawer.Body className="flex flex-col gap-5">
            <Details items={[
              ['任务 ID', <Mono>{job.jobId}</Mono>],
              ['状态', stateChip(state)],
              ['修订', `#${detail.data?.revision ?? job.revision}`],
              ['尝试次数', String(job.attempt)],
              ...(job.lastErrorCode ? [['最近错误', <span className="text-danger">{job.lastErrorCode}</span>] as [string, ReactNode]] : []),
              ...(detail.data?.resultRefs.length ? [['结果引用', <span className="flex flex-col gap-1">{detail.data.resultRefs.map((ref) => <Mono key={ref}>{ref}</Mono>)}</span>] as [string, ReactNode]] : []),
            ]} />
            {!terminal.has(state) && <div><Button isPending={busy} variant="danger-soft" onPress={cancel}>请求取消</Button></div>}
            <div className="flex flex-col gap-2">
              <div className="flex items-center justify-between"><span className="text-sm font-medium">进度</span><span className="text-xs text-muted">{stream}</span></div>
              {!events.length ? <p className="text-sm text-muted">尚无保留的进度检查点。</p> : (
                <ol ref={list} className="flex max-h-80 flex-col divide-y divide-separator overflow-y-auto rounded-xl border border-border">
                  {events.map((event) => (
                    <li key={event.seq} className="flex items-center justify-between gap-3 px-3 py-2 text-sm">
                      <span className="truncate">{event.phase}</span>
                      <span className="tabular-nums text-muted">{event.totalUnits > 0 ? `${event.completedUnits}/${event.totalUnits}` : event.completedUnits}</span>
                    </li>
                  ))}
                </ol>
              )}
            </div>
          </Drawer.Body>
        </Drawer.Dialog>
      </Drawer.Content>
    </Drawer.Backdrop>
  );
}

function DeliveriesTab() {
  const deliveries = useLoad((signal) => api<components['schemas']['ListDeliveriesResult']>('/v1/deliveries?limit=50', { signal }).then((page) => page.items), []);
  const { busy, run } = useAction();
  const replay = (delivery: Delivery) => run(async () => {
    await api(`/v1/deliveries/${encodeURIComponent(delivery.deliveryId)}:replay`, { method: 'POST', body: {}, idempotencyKey: newKey() });
    deliveries.reload();
  }, '已重新排队');
  if (deliveries.error && !deliveries.data) return <ErrorBlock error={deliveries.error} onRetry={deliveries.reload} />;
  if (!deliveries.data) return <LoadingBlock lines={4} />;
  if (!deliveries.data.length) return <EmptyBlock icon={<ListCheck />} title="没有死信投递" description="事件多次投递失败后会停在这里，修好原因后可以重放。" />;
  return (
    <Table>
      <Table.ScrollContainer>
        <Table.Content aria-label="死信投递" className="min-w-[640px]">
          <Table.Header>
            <Table.Column isRowHeader>事件</Table.Column>
            <Table.Column>来源 → 接收方</Table.Column>
            <Table.Column>尝试</Table.Column>
            <Table.Column>最近错误</Table.Column>
            <Table.Column className="text-end">操作</Table.Column>
          </Table.Header>
          <Table.Body>
            {deliveries.data.map((delivery) => (
              <Table.Row key={delivery.deliveryId} id={delivery.deliveryId}>
                <Table.Cell><span className="font-medium">{delivery.eventType}</span><span className="text-xs text-muted"> v{delivery.eventVersion}</span></Table.Cell>
                <Table.Cell>{delivery.producer} → {delivery.consumer}</Table.Cell>
                <Table.Cell className="tabular-nums">{delivery.attempts}</Table.Cell>
                <Table.Cell><span className="text-xs text-danger">{delivery.lastErrorCode}</span></Table.Cell>
                <Table.Cell className="text-end"><Button isDisabled={busy} size="sm" variant="secondary" onPress={() => replay(delivery)}>重放</Button></Table.Cell>
              </Table.Row>
            ))}
          </Table.Body>
        </Table.Content>
      </Table.ScrollContainer>
    </Table>
  );
}

function SchedulesTab() {
  const schedules = useLoad((signal) => api<{ schedules: Schedule[] }>('/v1/schedules', { signal }).then((result) => result.schedules), []);
  const [editing, setEditing] = useState<Schedule | null>(null);
  if (schedules.error && !schedules.data) return <ErrorBlock error={schedules.error} onRetry={schedules.reload} />;
  if (!schedules.data) return <LoadingBlock lines={4} />;
  if (!schedules.data.length) return <EmptyBlock icon={<ClockArrowRotateLeft />} title="当前没有定时任务" />;
  return (
    <>
      <Table>
        <Table.ScrollContainer>
          <Table.Content aria-label="定时调度" className="min-w-[640px]">
            <Table.Header>
              <Table.Column isRowHeader>任务</Table.Column>
              <Table.Column>周期</Table.Column>
              <Table.Column>状态</Table.Column>
              <Table.Column>下次运行</Table.Column>
              <Table.Column className="text-end">操作</Table.Column>
            </Table.Header>
            <Table.Body>
              {schedules.data.map((schedule) => (
                <Table.Row key={schedule.scheduleId} id={schedule.scheduleId}>
                  <Table.Cell><span className="font-medium">{schedule.owner} · {schedule.type}</span></Table.Cell>
                  <Table.Cell><Mono>{schedule.cron}</Mono> <span className="text-xs text-muted">{schedule.timezone}</span></Table.Cell>
                  <Table.Cell><Chip color={schedule.enabled ? 'success' : 'default'} size="sm" variant="soft">{schedule.enabled ? '启用' : '停用'}</Chip></Table.Cell>
                  <Table.Cell><span title={fmtDateTime(schedule.nextAt)}>{fmtRelative(schedule.nextAt)}</span></Table.Cell>
                  <Table.Cell className="text-end"><Button size="sm" variant="tertiary" onPress={() => setEditing(schedule)}>编辑</Button></Table.Cell>
                </Table.Row>
              ))}
            </Table.Body>
          </Table.Content>
        </Table.ScrollContainer>
      </Table>
      {editing && <ScheduleDialog schedule={editing} onClose={() => setEditing(null)} onSaved={() => { setEditing(null); schedules.reload(); }} />}
    </>
  );
}

function ScheduleDialog({ schedule, onClose, onSaved }: { schedule: Schedule; onClose: () => void; onSaved: () => void }) {
  const [form, setForm] = useState({ cron: schedule.cron, timezone: schedule.timezone, enabled: schedule.enabled });
  const { busy, run } = useAction();
  const submit = async () => {
    const done = await run(async () => {
      await api(`/v1/schedules/${encodeURIComponent(schedule.scheduleId)}`, { method: 'PUT', body: { expectedRevision: schedule.revision, ...form }, idempotencyKey: newKey() });
      return true;
    }, '定时已保存');
    if (done) onSaved();
  };
  return (
    <Modal.Backdrop isOpen onOpenChange={(open) => { if (!open) onClose(); }}>
      <Modal.Container size="sm">
        <Modal.Dialog>
          <Modal.CloseTrigger />
          <Modal.Header><Modal.Heading>编辑定时：{schedule.owner} · {schedule.type}</Modal.Heading></Modal.Header>
          <Form className="flex flex-col" onSubmit={(event) => { event.preventDefault(); void submit(); }}>
            <Modal.Body className="flex flex-col gap-4">
              <TextInput isRequired description="5 字段 cron，例如 0 3 * * * 表示每天 03:00。" label="周期" maxLength={200} value={form.cron} onChange={(cron) => setForm({ ...form, cron })} />
              <TextInput isRequired description="IANA 时区，例如 Asia/Shanghai。" label="时区" maxLength={64} value={form.timezone} onChange={(timezone) => setForm({ ...form, timezone })} />
              <SwitchInput isSelected={form.enabled} label="启用" onChange={(enabled) => setForm({ ...form, enabled })} />
              <Notice status="default">错过的运行时刻会合并为最近一次。</Notice>
            </Modal.Body>
            <Modal.Footer>
              <Button variant="tertiary" onPress={onClose}>取消</Button>
              <Button isPending={busy} type="submit">保存</Button>
            </Modal.Footer>
          </Form>
        </Modal.Dialog>
      </Modal.Container>
    </Modal.Backdrop>
  );
}
