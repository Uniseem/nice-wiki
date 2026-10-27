import { ArrowDownToLine, ChartColumn, CircleCheck, CircleExclamation, ShieldCheck } from '@gravity-ui/icons';
import { Button, Chip, Drawer, Form, Table, Tabs, ToggleButton, ToggleButtonGroup } from '@heroui/react';
import type { Key } from '@heroui/react';
import { api, apiClient, Details, EmptyBlock, ErrorBlock, fmtDateTime, fmtNumber, LoadingBlock, MiniStat, Mono, notify, PageHeader, Panel, SelectInput, TextInput, useHasGrant, t } from '@ui';
import { useEffect, useMemo, useState } from 'react';
import Visitors from './Visitors';
import { createAnalyticsClient, problemText, type AuditResult, type ExportKind, type LoadStatisticsResult } from './client';

const client = createAnalyticsClient(apiClient);

// 区间按 UTC 整点；“近 N 天”从当前整点往前推。
function range(hours: number): { from: string; to: string } {
  const end = new Date();
  end.setUTCMinutes(0, 0, 0);
  end.setUTCHours(end.getUTCHours() + 1);
  return { from: new Date(end.getTime() - hours * 3600_000).toISOString(), to: end.toISOString() };
}
// 审计里常见动作的可读名称；没列出的显示原始动作键。
const actionLabels: Record<string, string> = {
  'content.createDocument': t('新建文档'),
  'content.updateDocumentMetadata': t('修改文档信息'),
  'content.saveDraft': t('保存草稿'),
  'content.publishDocument': t('发布文档'),
  'content.withdrawDocument': t('撤回发布'),
  'content.trashDocument': t('移入回收站'),
  'content.restoreDocument': t('从回收站恢复'),
  'content.purgeDocuments': t('彻底删除文档'),
  'content.moveDocument': t('移动文档'),
  'content.reorderChildren': t('调整文档顺序'),
  'content.rollbackPublishedRevision': t('回滚到历史版本'),
  'content.acquireEditLock': t('打开文档编辑'),
  'content.releaseEditLock': t('结束文档编辑'),
  'content.forceReleaseEditLock': t('强制解除编辑锁'),
  'content.createPreviewGrant': t('预览草稿'),
  'media.beginUpload': t('开始上传文件'),
  'media.completeUpload': t('完成上传'),
  'media.proposeManualCleanup': t('生成资源清理计划'),
  'media.confirmManualCleanup': t('确认资源清理'),
  'media.cancelManualCleanup': t('撤销资源清理'),
  'identity.createSession': t('登录'),
  'identity.revokeSession': t('撤销登录会话'),
  'identity.changePassword': t('修改口令'),
  'identity.createAccount': t('新建子账号'),
  'identity.setAccountPermissions': t('修改账号权限'),
  'identity.setAccountStatus': t('修改账号状态'),
  'identity.resetAccountPassword': t('重置账号口令'),
  'site-config.updateSiteConfig': t('修改站点基本信息'),
  'site-config.updateBranding': t('修改品牌'),
  'site-config.updateHome': t('修改首页'),
  'site-config.updateBanner': t('修改公告横幅'),
  'site-config.updateFooter': t('修改页脚'),
  'localization.putLanguage': t('修改语言设置'),
  'memory.createEntry': t('新建长期记忆'),
  'memory.updateEntry': t('修改长期记忆'),
  'memory.archiveEntry': t('归档长期记忆'),
  'qa.putQaPolicy': t('修改问答策略'),
  'traffic-guard.updatePolicy': t('修改防滥用策略'),
  'traffic-guard.createBan': t('新建禁令'),
  'model-gateway.putProvider': t('修改模型供应方'),
  'model-gateway.putModel': t('修改模型'),
};
function ActionName({ action }: { action: string }) {
  const label = actionLabels[action];
  return label ? <span className="flex flex-col"><span>{label}</span><span className="font-mono text-xs text-muted">{action}</span></span> : <Mono>{action}</Mono>;
}

const presets: Record<string, [string, number]> = { day: [t('近 24 小时'), 24], week: [t('近 7 天'), 24 * 7], month: [t('近 30 天'), 24 * 30] };

export default function AnalyticsPage() {
  const canStats = useHasGrant('stats.read');
  const canAudit = useHasGrant('audit.read');
  const [tab, setTab] = useState(canStats ? 'visitors' : 'audit');
  return (
    <div className="flex flex-col gap-6">
      <PageHeader title={t('访问统计')} description={t('公开页面的访问统计和后台的管理审计。')} />
      {!canStats && !canAudit ? <EmptyBlock icon={<ChartColumn />} title={t('当前账号没有统计或审计的读取权限')} /> : (
        <Tabs selectedKey={tab} onSelectionChange={(key) => setTab(String(key))}>
          <Tabs.ListContainer className="mb-4">
            <Tabs.List aria-label={t('统计')}>
              {canStats ? <Tabs.Tab id="visitors">{t('访客')}<Tabs.Indicator /></Tabs.Tab> : null}
              {canStats ? <Tabs.Tab id="stats">{t('访问')}<Tabs.Indicator /></Tabs.Tab> : null}
              {canAudit ? <Tabs.Tab id="audit">{t('管理审计')}<Tabs.Indicator /></Tabs.Tab> : null}
            </Tabs.List>
          </Tabs.ListContainer>
          {canStats ? <Tabs.Panel id="visitors"><Visitors /></Tabs.Panel> : null}
          {canStats ? <Tabs.Panel id="stats"><StatsTab /></Tabs.Panel> : null}
          {canAudit ? <Tabs.Panel id="audit"><AuditTab /></Tabs.Panel> : null}
        </Tabs>
      )}
    </div>
  );
}

async function download(kind: ExportKind, from: string, to: string) {
  try {
    const exported = await client.RequestExport({ kind, from, to, operationId: crypto.randomUUID() }, new AbortController().signal);
    await client.DownloadAuthorizedExport({ exportId: exported.exportId, kind: exported.kind }, new AbortController().signal);
    notify.success(t('已导出'), t('链接到期：{0}', fmtDateTime(exported.expiresAt)));
  } catch (problem) {
    notify.error(t('导出失败'), problemText(problem));
  }
}

function StatsTab() {
  const [preset, setPreset] = useState('week');
  const [result, setResult] = useState<LoadStatisticsResult | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [loading, setLoading] = useState(false);
  const [view, setView] = useState<'docs' | 'hours'>('docs');
  const [titles, setTitles] = useState<Map<string, string>>(new Map());
  const [page, setPage] = useState(0);
  const window_ = useMemo(() => range(presets[preset][1]), [preset]);

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError(null);
    client.LoadStatistics(window_, controller.signal).then((data) => { setResult(data); setPage(0); }).catch((problem) => { if (!controller.signal.aborted) setError(problem); }).finally(() => setLoading(false));
    return () => controller.abort();
  }, [window_]);

  useEffect(() => {
    api<{ documents: { documentId: string; title: string }[] }>('/v1/tree').then((tree) => setTitles(new Map(tree.documents.map((doc) => [doc.documentId, doc.title])))).catch(() => {});
  }, []);

  const byDoc = useMemo(() => {
    const map = new Map<string, { documentId: string; views: number; visits: number; locales: Set<string> }>();
    for (const row of result?.rows ?? []) {
      const entry = map.get(row.documentId) ?? { documentId: row.documentId, views: 0, visits: 0, locales: new Set<string>() };
      entry.views += row.views;
      entry.visits += row.visits;
      entry.locales.add(row.locale);
      map.set(row.documentId, entry);
    }
    return [...map.values()].sort((a, b) => b.views - a.views);
  }, [result]);
  const totals = byDoc.reduce((sum, row) => ({ views: sum.views + row.views, visits: sum.visits + row.visits }), { views: 0, visits: 0 });
  const hours = result?.rows.slice(page * 100, (page + 1) * 100) ?? [];

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <ToggleButtonGroup disallowEmptySelection selectedKeys={[preset]} selectionMode="single" size="sm" onSelectionChange={(keys: Set<Key>) => { const next = [...keys][0]; if (next) setPreset(String(next)); }}>
          {Object.entries(presets).map(([key, [label]], index) => (
            <ToggleButton key={key} id={key}>{index > 0 && <ToggleButtonGroup.Separator />}{label}</ToggleButton>
          ))}
        </ToggleButtonGroup>
        <Button isDisabled={!result} size="sm" variant="secondary" onPress={() => void download('pages', result!.fromUTC, result!.toUTC)}><ArrowDownToLine />{t('访问 CSV')}</Button>
      </div>
      {error ? <ErrorBlock error={error} /> : !result || loading ? <LoadingBlock lines={5} /> : (
        <>
          <div className="grid gap-3 sm:grid-cols-3">
            <MiniStat label={t('浏览量（PV）')} value={fmtNumber(totals.views)} />
            <MiniStat label={t('访次')} value={fmtNumber(totals.visits)} />
            <MiniStat label={t('有访问的文档')} value={fmtNumber(byDoc.length)} />
          </div>
          <p className="text-xs text-muted">
            {t('访次在每个小时内去重，跨小时相加不代表独立访客。访问水位 {0} · 漏数 {1}', result.watermark, result.dropCount)}
          </p>
          <Panel
            title={view === 'docs' ? t('热门文档') : t('按小时明细')}
            actions={<SelectInput aria-label={t('视图')} className="w-36" options={[{ value: 'docs', label: t('按文档') }, { value: 'hours', label: t('按小时') }]} value={view} onChange={(value) => setView(value as 'docs' | 'hours')} variant="primary" />}
          >
            {!result.rows.length ? <EmptyBlock icon={<ChartColumn />} title={t('这个区间还没有访问数据')} description={t('统计按小时聚合，新访问稍后才会出现。')} /> : view === 'docs' ? (
              <Table variant="secondary">
                <Table.ScrollContainer>
                  <Table.Content aria-label={t('热门文档')} className="min-w-[560px]">
                    <Table.Header>
                      <Table.Column isRowHeader>{t('文档')}</Table.Column>
                      <Table.Column>{t('语言')}</Table.Column>
                      <Table.Column className="text-end">{t('浏览量')}</Table.Column>
                      <Table.Column className="text-end">{t('访次')}</Table.Column>
                    </Table.Header>
                    <Table.Body>
                      {byDoc.slice(0, 50).map((row) => (
                        <Table.Row key={row.documentId} id={row.documentId}>
                          <Table.Cell>{titles.get(row.documentId) ?? (titles.size ? t('已撤回或删除的文档') : <Mono>{row.documentId}</Mono>)}</Table.Cell>
                          <Table.Cell><span className="flex flex-wrap gap-1">{[...row.locales].map((locale) => <Chip key={locale} size="sm" variant="soft">{locale}</Chip>)}</span></Table.Cell>
                          <Table.Cell className="text-end tabular-nums">{fmtNumber(row.views)}</Table.Cell>
                          <Table.Cell className="text-end tabular-nums">{fmtNumber(row.visits)}</Table.Cell>
                        </Table.Row>
                      ))}
                    </Table.Body>
                  </Table.Content>
                </Table.ScrollContainer>
              </Table>
            ) : (
              <>
                <Table variant="secondary">
                  <Table.ScrollContainer>
                    <Table.Content aria-label={t('按小时明细')} className="min-w-[640px]">
                      <Table.Header>
                        <Table.Column isRowHeader>{t('小时')}</Table.Column>
                        <Table.Column>{t('文档')}</Table.Column>
                        <Table.Column>{t('语言')}</Table.Column>
                        <Table.Column className="text-end">PV</Table.Column>
                        <Table.Column className="text-end">{t('访次')}</Table.Column>
                      </Table.Header>
                      <Table.Body>
                        {hours.map((row) => (
                          <Table.Row key={row.periodStart + row.documentId + row.locale} id={row.periodStart + row.documentId + row.locale}>
                            <Table.Cell>{fmtDateTime(row.periodStart)}</Table.Cell>
                            <Table.Cell>{titles.get(row.documentId) ?? (titles.size ? t('已撤回或删除的文档') : <Mono>{row.documentId}</Mono>)}</Table.Cell>
                            <Table.Cell>{row.locale}</Table.Cell>
                            <Table.Cell className="text-end tabular-nums">{row.views}</Table.Cell>
                            <Table.Cell className="text-end tabular-nums">{row.visits}</Table.Cell>
                          </Table.Row>
                        ))}
                      </Table.Body>
                    </Table.Content>
                  </Table.ScrollContainer>
                </Table>
                {result.rows.length > 100 && (
                  <div className="flex items-center justify-center gap-3 text-sm text-muted">
                    <Button isDisabled={page === 0} size="sm" variant="tertiary" onPress={() => setPage(page - 1)}>{t('上一页')}</Button>
                    {t('第 {0} / {1} 页', page + 1, Math.ceil(result.rows.length / 100))}
                    <Button isDisabled={(page + 1) * 100 >= result.rows.length} size="sm" variant="tertiary" onPress={() => setPage(page + 1)}>{t('下一页')}</Button>
                  </div>
                )}
              </>
            )}
          </Panel>
        </>
      )}
    </div>
  );
}

function AuditTab() {
  const [preset, setPreset] = useState('week');
  const [action, setAction] = useState('');
  const [cursors, setCursors] = useState<string[]>(['']);
  const [items, setItems] = useState<AuditResult[] | null>(null);
  const [next, setNext] = useState('');
  const [error, setError] = useState<unknown>(null);
  const [loading, setLoading] = useState(false);
  const [detail, setDetail] = useState<AuditResult | null>(null);
  const window_ = useMemo(() => range(presets[preset][1]), [preset]);
  // 主体显示账号名：只有超管能读账号列表，读不到就显示编号。
  const [names, setNames] = useState<Map<string, string>>(new Map());
  useEffect(() => {
    type Named = { accountId: string; accountName: string };
    void Promise.all([
      api<{ account: Named }>('/v1/self').then((result) => [result.account]).catch(() => [] as Named[]),
      api<{ accounts: Named[] }>('/v1/accounts?limit=100').then((result) => result.accounts).catch(() => [] as Named[]),
    ]).then(([self, others]) => setNames(new Map([...self, ...others].map((account) => [account.accountId, account.accountName]))));
  }, []);
  const actorOf = (id: string) => names.get(id) ?? <Mono>{id}</Mono>;

  const load = async (cursor: string) => {
    setLoading(true);
    setError(null);
    try {
      const signal = new AbortController().signal;
      const page = await client.LoadAuditPage({ cursor, limit: 20, action: action.trim() || undefined, from: window_.from, to: window_.to }, signal);
      // 列表只给编号；逐条读取详情，链校验结果随详情返回。
      setItems(await Promise.all(page.auditIds.map((id) => client.ReadAudit(id, signal))));
      setNext(page.nextCursor);
    } catch (problem) { setError(problem); } finally { setLoading(false); }
  };
  useEffect(() => { setCursors(['']); void load(''); }, [window_]); // 区间变化时从第一页重新读取

  return (
    <div className="flex flex-col gap-5">
      <Form className="flex flex-wrap items-end gap-3" onSubmit={(event) => { event.preventDefault(); setCursors(['']); void load(''); }}>
        <SelectInput className="w-36" label={t('区间')} options={Object.entries(presets).map(([value, [label]]) => ({ value, label }))} value={preset} onChange={setPreset} variant="primary" />
        <TextInput className="w-56" label={t('动作')} placeholder={t('留空查看全部')} value={action} onChange={setAction} variant="primary" />
        <Button isPending={loading} type="submit" variant="secondary">{t('查询')}</Button>
        <span className="flex-1" />
        <Button variant="secondary" onPress={() => void download('audits', window_.from, window_.to)}><ArrowDownToLine />{t('导出审计 CSV')}</Button>
      </Form>
      {error ? <ErrorBlock error={error} /> : !items ? <LoadingBlock lines={5} /> : !items.length ? (
        <EmptyBlock icon={<ShieldCheck />} title={t('当前条件没有审计记录')} description={t('默认保留 180 天。')} />
      ) : (
        <Table>
          <Table.ScrollContainer>
            <Table.Content aria-label={t('管理审计')} className="min-w-[680px]">
              <Table.Header>
                <Table.Column isRowHeader>{t('时间')}</Table.Column>
                <Table.Column>{t('动作')}</Table.Column>
                <Table.Column>{t('主体')}</Table.Column>
                <Table.Column>{t('结果')}</Table.Column>
                <Table.Column>{t('链校验')}</Table.Column>
                <Table.Column className="text-end">{t('操作')}</Table.Column>
              </Table.Header>
              <Table.Body>
                {items.map((item) => (
                  <Table.Row key={item.auditJSON.auditId} id={item.auditJSON.auditId}>
                    <Table.Cell>{fmtDateTime(item.auditJSON.occurredAt)}</Table.Cell>
                    <Table.Cell><ActionName action={item.auditJSON.action} /></Table.Cell>
                    <Table.Cell>{actorOf(item.auditJSON.actorId)}</Table.Cell>
                    <Table.Cell>{item.auditJSON.resultCode}</Table.Cell>
                    <Table.Cell>{item.chainValid ? <Chip color="success" size="sm" variant="soft"><CircleCheck />{t('通过')}</Chip> : <Chip color="danger" size="sm" variant="soft"><CircleExclamation />{t('未通过')}</Chip>}</Table.Cell>
                    <Table.Cell className="text-end"><Button size="sm" variant="tertiary" onPress={() => setDetail(item)}>{t('详情')}</Button></Table.Cell>
                  </Table.Row>
                ))}
              </Table.Body>
            </Table.Content>
          </Table.ScrollContainer>
        </Table>
      )}
      {(cursors.length > 1 || next) && (
        <div className="flex justify-center gap-3">
          <Button isDisabled={cursors.length <= 1 || loading} size="sm" variant="tertiary" onPress={() => { const previous = cursors.slice(0, -1); setCursors(previous); void load(previous[previous.length - 1]); }}>{t('上一页')}</Button>
          <Button isDisabled={!next || loading} size="sm" variant="tertiary" onPress={() => { setCursors([...cursors, next]); void load(next); }}>{t('下一页')}</Button>
        </div>
      )}
      {detail && (
        <Drawer.Backdrop isOpen onOpenChange={(open) => { if (!open) setDetail(null); }}>
          <Drawer.Content placement="right">
            <Drawer.Dialog className="w-full max-w-lg">
              <Drawer.CloseTrigger aria-label={t('关闭')} />
              <Drawer.Header><Drawer.Heading>{t('审计详情')}</Drawer.Heading></Drawer.Header>
              <Drawer.Body className="flex flex-col gap-4">
                {detail.chainValid ? <Chip color="success" variant="soft"><CircleCheck />{t('本条内容与前驱链校验通过')}</Chip> : <Chip color="danger" variant="soft"><CircleExclamation />{t('链完整性校验未通过，请核查')}</Chip>}
                <Details items={[
                  [t('动作'), <ActionName action={detail.auditJSON.action} />], [t('主体'), actorOf(detail.auditJSON.actorId)], [t('资源'), <Mono>{detail.auditJSON.resourceId}</Mono>],
                  [t('结果'), detail.auditJSON.resultCode], [t('发生时间'), fmtDateTime(detail.auditJSON.occurredAt)],
                ]} />
                <pre className="max-h-96 overflow-auto rounded-xl bg-surface-secondary p-3 font-mono text-xs leading-5">{JSON.stringify(detail.auditJSON, null, 2)}</pre>
              </Drawer.Body>
            </Drawer.Dialog>
          </Drawer.Content>
        </Drawer.Backdrop>
      )}
    </div>
  );
}
