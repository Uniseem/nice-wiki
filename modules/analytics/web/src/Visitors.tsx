import { Button, Drawer, Table } from '@heroui/react';
import { api, EmptyBlock, ErrorBlock, fmtDateTime, fmtNumber, LoadingBlock, MiniStat, Panel, SelectInput, useLoad, t } from '@ui';
import { useMemo, useState } from 'react';

type Visitor = { visitorId: string; ip: string; userAgent: string; device: string; browser: string; os: string; country: string; firstSeen: string; lastSeen: string; pageViews: number; activities: number };
type Activity = { id: string; documentId: string; locale: string; eventType: string; ip: string; userAgent: string; occurredAt: string; referrerDomain: string };
type Result = { visitors: Visitor[]; activities: Activity[]; totalVisitors: number; pageViews: number; pageInfo: { hasMore: boolean; nextCursor?: string } };
const eventLabels: Record<string, string> = { 'page-view': t('浏览页面'), 'navigation-click': t('点击导航'), 'qa-ask': t('提问'), search: t('搜索') };
const deviceLabels: Record<string, string> = { desktop: t('电脑'), mobile: t('手机'), tablet: t('平板') };

export default function Visitors() {
  const [days, setDays] = useState('7');
  const [cursor, setCursor] = useState('');
  const [selected, setSelected] = useState<Visitor | null>(null);
  const interval = useMemo(() => { const to = new Date(); return new URLSearchParams({ from: new Date(to.getTime() - Number(days) * 86400_000).toISOString(), to: to.toISOString() }).toString(); }, [days]);
  const visitors = useLoad((signal) => api<Result>(`/v1/statistics/visitors?${interval}&limit=30${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ''}`, { signal }), [interval, cursor]);
  return <div className="flex flex-col gap-5">
    <div className="flex flex-wrap items-end justify-between gap-3"><SelectInput label={t('时间范围')} value={days} onChange={(value) => { setDays(value); setCursor(''); }} options={[{ value: '1', label: t('近 24 小时') }, { value: '7', label: t('近 7 天') }, { value: '30', label: t('近 30 天') }]} /><Button variant="secondary" onPress={visitors.reload}>{t('刷新')}</Button></div>
    <p className="text-sm text-muted">{t('按站内访客标识去重，同一访客跨页面和跨天只计一次。活动、IP 和设备信息保留 30 天。')}</p>
    {visitors.error ? <ErrorBlock error={visitors.error} onRetry={visitors.reload} /> : !visitors.data ? <LoadingBlock lines={5} /> : <>
      <div className="grid gap-4 sm:grid-cols-2"><MiniStat label={t('独立访客')} value={fmtNumber(visitors.data.totalVisitors)} /><MiniStat label={t('浏览量')} value={fmtNumber(visitors.data.pageViews)} /></div>
      {!visitors.data.visitors.length ? <EmptyBlock title={t('暂无访客记录')} /> : <Table><Table.ScrollContainer><Table.Content aria-label={t('访客')} className="min-w-[800px]">
        <Table.Header><Table.Column isRowHeader>{t('访客')}</Table.Column><Table.Column>IP</Table.Column><Table.Column>{t('设备')}</Table.Column><Table.Column>{t('浏览量')}</Table.Column><Table.Column>{t('最近活动')}</Table.Column><Table.Column>{t('操作')}</Table.Column></Table.Header>
        <Table.Body>{visitors.data.visitors.map((visitor) => <Table.Row id={visitor.visitorId} key={visitor.visitorId}>
          <Table.Cell><span className="font-mono text-xs" title={visitor.visitorId}>{visitor.visitorId.slice(0, 12)}</span></Table.Cell>
          <Table.Cell><span className="select-all font-mono text-xs">{visitor.ip}</span>{visitor.country && <span className="ms-2 text-xs text-muted">{visitor.country}</span>}</Table.Cell>
          <Table.Cell><span title={visitor.userAgent}>{deviceLabels[visitor.device] ?? visitor.device} · {visitor.os} · {visitor.browser}</span></Table.Cell>
          <Table.Cell>{fmtNumber(visitor.pageViews)}</Table.Cell><Table.Cell>{fmtDateTime(visitor.lastSeen)}</Table.Cell><Table.Cell><Button size="sm" variant="secondary" onPress={() => setSelected(visitor)}>{t('查看活动')}</Button></Table.Cell>
        </Table.Row>)}</Table.Body>
      </Table.Content></Table.ScrollContainer></Table>}
      <div className="flex justify-end gap-2">{cursor && <Button variant="tertiary" onPress={() => setCursor('')}>{t('第一页')}</Button>}{visitors.data.pageInfo.hasMore && <Button variant="secondary" onPress={() => setCursor(visitors.data!.pageInfo.nextCursor ?? '')}>{t('下一页')}</Button>}</div>
    </>}
    {selected && <VisitorDetails visitor={selected} interval={interval} onClose={() => setSelected(null)} />}
  </div>;
}

function VisitorDetails({ visitor, interval, onClose }: { visitor: Visitor; interval: string; onClose: () => void }) {
  const [cursor, setCursor] = useState('');
  const activities = useLoad((signal) => api<Result>(`/v1/statistics/visitors?${interval}&visitorId=${encodeURIComponent(visitor.visitorId)}&limit=50${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ''}`, { signal }), [visitor.visitorId, interval, cursor]);
  return <Drawer.Backdrop isOpen onOpenChange={(open) => { if (!open) onClose(); }}><Drawer.Content placement="right" className="w-full max-w-2xl"><Drawer.Dialog>
    <Drawer.CloseTrigger aria-label={t('关闭')} /><Drawer.Header><Drawer.Heading>{t('访客活动')}</Drawer.Heading></Drawer.Header><Drawer.Body className="flex flex-col gap-5">
      <Panel title={t('设备')}><dl className="space-y-2 text-sm"><div className="break-all font-mono">{visitor.visitorId}</div><div>IP: <span className="select-all">{visitor.ip}</span></div><div>{visitor.os} · {visitor.browser}</div><div className="break-all text-xs text-muted">{visitor.userAgent}</div><div>{t('首次活动')}: {fmtDateTime(visitor.firstSeen)}</div></dl></Panel>
      {activities.error ? <ErrorBlock error={activities.error} onRetry={activities.reload} /> : !activities.data ? <LoadingBlock lines={5} /> : <><ol className="flex flex-col gap-3">{activities.data.activities.map((activity) => <li key={activity.id} className="rounded-xl border border-border p-3 text-sm"><div className="flex justify-between gap-2"><strong>{eventLabels[activity.eventType] ?? activity.eventType}</strong><time className="text-xs text-muted">{fmtDateTime(activity.occurredAt)}</time></div><p className="mt-2 break-all font-mono text-xs">{activity.documentId === 'home' ? t('首页') : activity.documentId} · {activity.locale}</p><p className="mt-1 text-xs text-muted">{activity.ip}{activity.referrerDomain ? ` · ${activity.referrerDomain}` : ''}</p></li>)}</ol>{activities.data.pageInfo.hasMore && <Button variant="secondary" onPress={() => setCursor(activities.data!.pageInfo.nextCursor ?? '')}>{t('下一页')}</Button>}</>}
    </Drawer.Body></Drawer.Dialog></Drawer.Content></Drawer.Backdrop>;
}
