import { LibraryBrowser } from './LibraryBrowser';
import { Brush, File as FileIcon, Link as LinkIcon, Picture, TrashBin, Video } from '@gravity-ui/icons';
import { Button, Checkbox, Chip, Drawer, Table, Tabs } from '@heroui/react';
import type { Selection } from '@heroui/react';
import { api, assetURL, confirmAction, EmptyBlock, ErrorBlock, fmtBytes, fmtDateTime, LoadingBlock, Mono, navigate, notify, Notice, PageHeader, Panel, SelectInput, useAction, t } from '@ui';
import { useCallback, useEffect, useState, type MouseEvent } from 'react';

type AssetRef = { assetId: string; version: number; sha256: string; mediaType: string; byteSize: number };
type Asset = { ref: AssetRef; kind: string; originalName: string; createdAt: string; state: string };
type Plan = { planId: string; confirmationHash: string; revision: number; assets: AssetRef[]; historyAffectedDocumentIds: string[]; state: string; notBefore: string };
// 引用来源：content 的 sourceObjectId 就是文档 ID；site-config 等其他来源是各自的对象编号。
type Reference = { sourceObjectId: string; sourceService: string; sourceRevisionId: string };
type Snapshot = { currentReferences: Reference[]; historyReferences: Reference[] };

const kinds: Record<string, string> = { image: t('图片'), attachment: t('附件'), video: t('视频'), drawing_scene: t('画板场景'), drawing_svg: t('画板图像'), platform_cover: t('平台封面') };
const sources: Record<string, string> = { content: t('文档'), 'site-config': t('站点设置') };
// 只给非“可用”状态打标；待删除的资源在保护窗口结束前仍在列表里。
const states: Record<string, { label: string; tone: 'default' | 'warning' | 'danger' }> = {
  uploading: { label: t('上传中'), tone: 'default' }, validating: { label: t('校验中'), tone: 'default' }, blocked: { label: t('已拦截'), tone: 'danger' },
  delete_pending: { label: t('待删除'), tone: 'warning' }, deleted: { label: t('已删除'), tone: 'danger' },
};
const planStates: Record<string, string> = { proposed: t('待确认'), confirmed: t('等待删除'), waiting: t('等待删除') };
const kindIcon = (kind: string) => kind === 'image' || kind === 'platform_cover' ? Picture : kind === 'video' ? Video : kind.startsWith('drawing') ? Brush : FileIcon;

export default function LibraryPage() {
 const [tab,setTab]=useState('library');
 return <div className="flex flex-col gap-6"><PageHeader title={t('资源库')} description={t('本地存储的图片、视频和附件，按分组管理、预览和分享。')} /><Tabs selectedKey={tab} onSelectionChange={(key)=>setTab(String(key))}><Tabs.ListContainer><Tabs.List aria-label={t('资源库')}><Tabs.Tab id="library">{t('全部资源')}<Tabs.Indicator /></Tabs.Tab><Tabs.Tab id="cleanup">{t('引用与清理')}<Tabs.Indicator /></Tabs.Tab></Tabs.List></Tabs.ListContainer><Tabs.Panel id="library"><LibraryBrowser /></Tabs.Panel><Tabs.Panel id="cleanup"><CleanupPage /></Tabs.Panel></Tabs></div>;
}

function CleanupPage() {
  const [tab, setTab] = useState<'all' | 'unused'>('all');
  const [kind, setKind] = useState('all');
  const [assets, setAssets] = useState<Asset[]>([]);
  const [cursor, setCursor] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<unknown>(null);
  const [selected, setSelected] = useState<Selection>(new Set());
  const [plans, setPlans] = useState<Plan[]>([]);
  const [refs, setRefs] = useState<{ asset: Asset; snapshot: Snapshot; titles: Map<string, string> } | null>(null);
  const { busy, run } = useAction();

  const load = useCallback(async (more: boolean, from: string | null) => {
    setLoading(true);
    setError(null);
    try {
      const query = new URLSearchParams({ limit: '40' });
      if (kind !== 'all') query.set('kind', kind);
      if (more && from) query.set('cursor', from);
      const result = await api<{ assets: Asset[]; pageInfo?: { nextCursor?: string | null } }>(`/v1/assets${tab === 'unused' ? '/unused' : ''}?${query}`);
      setAssets((current) => more ? [...current, ...result.assets] : result.assets);
      setCursor(result.pageInfo?.nextCursor ?? null);
    } catch (problem) { setError(problem); } finally { setLoading(false); }
  }, [tab, kind]);

  useEffect(() => { setSelected(new Set()); void load(false, null); }, [load]);

  // 还没结束的清理计划：确认后要等保护窗口结束才删，期间可以撤销。
  const loadPlans = useCallback(async () => {
    setPlans((await api<{ plans: Plan[] }>('/v1/cleanup-plans')).plans);
  }, []);
  useEffect(() => { void run(loadPlans); }, [run, loadPlans]);

  const chosen = selected === 'all' ? assets : assets.filter((asset) => (selected as Set<string>).has(asset.ref.assetId));

  // 引用列表里显示文档标题；没有文档读取权限时退回显示编号。
  const showReferences = (asset: Asset) => run(async () => {
    const [result, titles] = await Promise.all([
      api<{ snapshot: Snapshot }>(`/v1/assets/${asset.ref.assetId}/references`),
      api<{ documents: { documentId: string; title: string }[] }>('/v1/tree')
        .then((tree) => new Map(tree.documents.map((doc) => [doc.documentId, doc.title])))
        .catch(() => new Map<string, string>()),
    ]);
    setRefs({ asset, snapshot: result.snapshot, titles });
  });

  const propose = () => run(async () => {
    await api<{ plan: Plan }>('/v1/cleanup-plans', { method: 'POST', body: { assets: chosen.map((asset) => asset.ref) } });
    setSelected(new Set());
    await loadPlans();
  });

  const confirmPlan = async (plan: Plan) => {
    const risky = plan.historyAffectedDocumentIds.length > 0;
    const ok = await confirmAction({
      title: t('删除这 {0} 个资源？', plan.assets.length),
      body: risky ? t('其中有资源仍被 {0} 篇文档的历史版本引用，删除后回滚到那些版本会缺少文件。删除会在 24 小时的迟到引用保护窗口之后执行。', plan.historyAffectedDocumentIds.length) : t('删除会在 24 小时的迟到引用保护窗口之后执行，在此之前重新被引用的资源会被保留。'),
      confirmLabel: risky ? t('接受风险并删除') : t('确认删除'), tone: 'danger',
    });
    if (!ok) return;
    await run(async () => {
      await api(`/v1/cleanup-plans/${plan.planId}:confirm`, { method: 'POST', body: { confirmationHash: plan.confirmationHash, acknowledgeHistoryLoss: risky, expectedRevision: plan.revision } });
      await loadPlans();
      await load(false, null);
    }, t('已确认，保护窗口结束后删除'));
  };

  const cancelPlan = (plan: Plan) => run(async () => {
    await api(`/v1/cleanup-plans/${plan.planId}`, { method: 'DELETE', body: {} });
    await loadPlans();
    await load(false, null);
  }, plan.state === 'proposed' ? t('已放弃清理计划') : t('已撤销，这些资源不会被删除'));

  return (
    <div className="flex flex-col gap-6">
      <PageHeader title={t('资源库')} description={t('文档和站点设置里上传的图片、视频、附件与画板。删除需要先生成清理计划，并经过保护窗口。')} />
      <div className="flex flex-wrap items-end justify-between gap-3">
        <Tabs selectedKey={tab} onSelectionChange={(key) => setTab(key as 'all' | 'unused')}>
          <Tabs.ListContainer>
            <Tabs.List aria-label={t('资源范围')}>
              <Tabs.Tab id="all">{t('全部资源')}<Tabs.Indicator /></Tabs.Tab>
              <Tabs.Tab id="unused">{t('未被引用')}<Tabs.Indicator /></Tabs.Tab>
            </Tabs.List>
          </Tabs.ListContainer>
          <Tabs.Panel id="all"><span /></Tabs.Panel>
          <Tabs.Panel id="unused"><span /></Tabs.Panel>
        </Tabs>
        <SelectInput aria-label={t('类别')} className="w-44" options={[{ value: 'all', label: t('全部类别') }, ...Object.entries(kinds).map(([value, label]) => ({ value, label }))]} placeholder={t('全部类别')} value={kind} onChange={setKind} variant="primary" />
      </div>
      {tab === 'unused' && <Notice status="accent">{t('这里只列当前没有任何文档或站点设置引用的资源。勾选后生成清理计划；仍被历史版本引用的会单独提示风险。')}</Notice>}

      {plans.length > 0 && (
        <Panel title={t('清理计划')} description={t('确认后要等 24 小时保护窗口结束才真正删除，在此之前可以撤销。')}>
          <div className="flex flex-col divide-y divide-border">
            {plans.map((item) => (
              <div key={item.planId} className="flex flex-wrap items-center justify-between gap-3 py-3 first:pt-0 last:pb-0">
                <div className="flex min-w-0 flex-col gap-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <Chip color={item.state === 'proposed' ? 'default' : 'warning'} size="sm" variant="soft">{planStates[item.state] ?? item.state}</Chip>
                    <span className="text-sm">{t('共 {0} 个资源{1}。', item.assets.length, item.historyAffectedDocumentIds.length ? t('，历史版本涉及 {0} 篇文档', item.historyAffectedDocumentIds.length) : '')}</span>
                    <span className="text-sm text-muted">{fmtBytes(item.assets.reduce((sum, asset) => sum + asset.byteSize, 0))}</span>
                  </div>
                  <span className="text-xs text-muted">{item.state === 'proposed' ? t('还没确认，不会删除任何文件。') : t('将在 {0} 之后删除。', fmtDateTime(item.notBefore))}</span>
                </div>
                <div className="flex gap-2">
                  {item.state === 'proposed' ? (
                    <>
                      <Button isDisabled={busy} size="sm" variant="tertiary" onPress={() => void cancelPlan(item)}>{t('放弃计划')}</Button>
                      <Button isDisabled={busy} size="sm" variant="danger" onPress={() => void confirmPlan(item)}><TrashBin />{t('确认清理')}</Button>
                    </>
                  ) : (
                    <Button isDisabled={busy} size="sm" variant="secondary" onPress={() => void cancelPlan(item)}>{t('撤销删除')}</Button>
                  )}
                </div>
              </div>
            ))}
          </div>
        </Panel>
      )}

      {error ? <ErrorBlock error={error} onRetry={() => void load(false, null)} /> : loading && !assets.length ? <LoadingBlock lines={6} /> : !assets.length ? (
        <EmptyBlock icon={<Picture />} title={tab === 'unused' ? t('没有未被引用的资源') : t('还没有资源')} description={tab === 'unused' ? t('所有资源都在被使用。') : t('在文档编辑器里插入图片、视频或附件后，会出现在这里。')} />
      ) : (
        <>
          {tab === 'unused' && (
            <div className="flex items-center gap-3">
              <span className="text-sm text-muted">{t('已选 {0} 项', chosen.length)}</span>
              <Button isDisabled={busy || !chosen.length} size="sm" variant="secondary" onPress={propose}>{t('为选中的资源生成清理计划')}</Button>
            </div>
          )}
          <Table key={tab}>
            <Table.ScrollContainer>
              <Table.Content
                aria-label={t('资源列表')}
                className="min-w-[600px]"
                selectedKeys={tab === 'unused' ? selected : undefined}
                selectionMode={tab === 'unused' ? 'multiple' : 'none'}
                onSelectionChange={tab === 'unused' ? setSelected : undefined}
              >
                <Table.Header>
                  {tab === 'unused' ? (
                    <Table.Column className="w-10 pe-0">
                      <Checkbox aria-label={t('全选')} slot="selection"><Checkbox.Content><Checkbox.Control><Checkbox.Indicator /></Checkbox.Control></Checkbox.Content></Checkbox>
                    </Table.Column>
                  ) : null}
                  <Table.Column isRowHeader>{t('资源')}</Table.Column>
                  <Table.Column>{t('类别')}</Table.Column>
                  <Table.Column>{t('大小')}</Table.Column>
                  <Table.Column>{t('上传时间')}</Table.Column>
                  <Table.Column className="text-end">{t('引用')}</Table.Column>
                </Table.Header>
                <Table.Body>
                  {assets.map((asset) => {
                    const Icon = kindIcon(asset.kind);
                    return (
                      <Table.Row key={asset.ref.assetId} id={asset.ref.assetId}>
                        {tab === 'unused' ? (
                          <Table.Cell className="pe-0">
                            <Checkbox aria-label={t('选择 {0}', asset.originalName)} slot="selection" variant="secondary"><Checkbox.Content><Checkbox.Control><Checkbox.Indicator /></Checkbox.Control></Checkbox.Content></Checkbox>
                          </Table.Cell>
                        ) : null}
                        <Table.Cell>
                          <span className="flex min-w-0 items-center gap-3">
                            <span className="flex size-12 shrink-0 items-center justify-center overflow-hidden rounded-lg border border-border bg-surface-secondary">
                              {asset.ref.mediaType.startsWith('image/') ? <img alt="" className="size-full object-cover" loading="lazy" src={assetURL(asset.ref)} /> : <Icon className="size-5 text-muted" />}
                            </span>
                            <span className="min-w-0">
                              <span className="block truncate font-medium text-foreground">{asset.originalName}</span>
                              <span className="block truncate text-xs text-muted">{asset.ref.mediaType}</span>
                            </span>
                          </span>
                        </Table.Cell>
                        <Table.Cell>
                          <span className="flex flex-wrap gap-1">
                            <Chip size="sm" variant="soft">{kinds[asset.kind] ?? asset.kind}</Chip>
                            {states[asset.state] && <Chip color={states[asset.state].tone} size="sm" variant="soft">{states[asset.state].label}</Chip>}
                          </span>
                        </Table.Cell>
                        <Table.Cell className="tabular-nums">{fmtBytes(asset.ref.byteSize)}</Table.Cell>
                        <Table.Cell>{fmtDateTime(asset.createdAt)}</Table.Cell>
                        <Table.Cell className="text-end"><Button isDisabled={busy} size="sm" variant="tertiary" onPress={() => showReferences(asset)}><LinkIcon />{t('查看')}</Button></Table.Cell>
                      </Table.Row>
                    );
                  })}
                </Table.Body>
              </Table.Content>
            </Table.ScrollContainer>
          </Table>
          {cursor && <div className="flex justify-center"><Button isPending={loading} variant="secondary" onPress={() => void load(true, cursor)}>{t('加载更多')}</Button></div>}
        </>
      )}

      {refs && (
        <Drawer.Backdrop isOpen onOpenChange={(open) => { if (!open) setRefs(null); }}>
          <Drawer.Content placement="right">
            <Drawer.Dialog className="w-full max-w-md">
              <Drawer.CloseTrigger aria-label={t('关闭')} />
              <Drawer.Header><Drawer.Heading className="truncate">{t('“{0}”的引用', refs.asset.originalName)}</Drawer.Heading></Drawer.Header>
              <Drawer.Body className="flex flex-col gap-5">
                {[[t('当前引用'), refs.snapshot.currentReferences], [t('历史版本引用'), refs.snapshot.historyReferences]].map(([label, list]) => (
                  <div key={label as string} className="flex flex-col gap-2">
                    <span className="text-sm font-medium text-foreground">{label as string}</span>
                    {!(list as Reference[]).length ? <span className="text-sm text-muted">{t('无')}</span> : (
                      <ul className="flex flex-col gap-1.5">
                        {bySource(list as Reference[]).map(({ ref, count }) => (
                          <li key={ref.sourceService + ref.sourceObjectId} className="flex items-center justify-between gap-2 rounded-lg bg-surface-secondary px-3 py-2 text-sm">
                            <SourceLink reference={ref} titles={refs.titles} />
                            {count > 1 && <span className="shrink-0 text-xs text-muted">{t('{0} 处', count)}</span>}
                          </li>
                        ))}
                      </ul>
                    )}
                  </div>
                ))}
              </Drawer.Body>
            </Drawer.Dialog>
          </Drawer.Content>
        </Drawer.Backdrop>
      )}
    </div>
  );
}

// 同一来源（文档或站点设置对象）的多处引用合并成一行。
function bySource(list: Reference[]) {
  const groups = new Map<string, { ref: Reference; count: number }>();
  for (const ref of list) {
    const key = ref.sourceService + ref.sourceObjectId;
    const group = groups.get(key);
    if (group) group.count++;
    else groups.set(key, { ref, count: 1 });
  }
  return [...groups.values()];
}

function SourceLink({ reference, titles }: { reference: Reference; titles: Map<string, string> }) {
  const go = (path: string) => (event: MouseEvent) => { event.preventDefault(); navigate(path); };
  if (reference.sourceService === 'content') {
    const path = `/admin/content?doc=${reference.sourceObjectId}`;
    const title = titles.get(reference.sourceObjectId);
    return title
      ? <a className="truncate text-accent hover:underline" href={path} onClick={go(path)}>{title}</a>
      : <span className="truncate">{t('文档')} <Mono>{reference.sourceObjectId}</Mono></span>;
  }
  if (reference.sourceService === 'site-config') return <a className="truncate text-accent hover:underline" href="/admin/site" onClick={go('/admin/site')}>{t('站点设置（品牌图片、首页或页脚）')}</a>;
  return <span className="truncate">{sources[reference.sourceService] ?? reference.sourceService} <Mono>{reference.sourceObjectId}</Mono></span>;
}
