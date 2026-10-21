import { Brush, File as FileIcon, Link as LinkIcon, Picture, TrashBin, Video } from '@gravity-ui/icons';
import { Button, Checkbox, Chip, Drawer, Table, Tabs } from '@heroui/react';
import type { Selection } from '@heroui/react';
import { api, assetURL, confirmAction, Details, EmptyBlock, ErrorBlock, fmtBytes, fmtDateTime, LoadingBlock, Mono, navigate, notify, Notice, PageHeader, Panel, SelectInput, useAction } from '@ui';
import { useCallback, useEffect, useState, type MouseEvent } from 'react';

type AssetRef = { assetId: string; version: number; sha256: string; mediaType: string; byteSize: number };
type Asset = { ref: AssetRef; kind: string; originalName: string; createdAt: string; state: string };
type Plan = { planId: string; confirmationHash: string; revision: number; assets: AssetRef[]; historyAffectedDocumentIds: string[] };
type Reference = { documentId?: string; sourceObjectId?: string; sourceService: string; sourceRevisionId: string };
type Snapshot = { currentReferences: Reference[]; historyReferences: Reference[] };

const kinds: Record<string, string> = { image: '图片', attachment: '附件', video: '视频', drawing_scene: '画板场景', drawing_svg: '画板图像', platform_cover: '平台封面' };
const sources: Record<string, string> = { content: '文档', 'site-config': '站点设置' };
// 只给非“可用”状态打标；待删除的资源在保护窗口结束前仍在列表里。
const states: Record<string, { label: string; tone: 'default' | 'warning' | 'danger' }> = {
  uploading: { label: '上传中', tone: 'default' }, validating: { label: '校验中', tone: 'default' }, blocked: { label: '已拦截', tone: 'danger' },
  delete_pending: { label: '待删除', tone: 'warning' }, deleted: { label: '已删除', tone: 'danger' },
};
const kindIcon = (kind: string) => kind === 'image' || kind === 'platform_cover' ? Picture : kind === 'video' ? Video : kind.startsWith('drawing') ? Brush : FileIcon;

export default function LibraryPage() {
  const [tab, setTab] = useState<'all' | 'unused'>('all');
  const [kind, setKind] = useState('all');
  const [assets, setAssets] = useState<Asset[]>([]);
  const [cursor, setCursor] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<unknown>(null);
  const [selected, setSelected] = useState<Selection>(new Set());
  const [plan, setPlan] = useState<Plan | null>(null);
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

  useEffect(() => { setSelected(new Set()); setPlan(null); void load(false, null); }, [load]);

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
    const result = await api<{ plan: Plan }>('/v1/cleanup-plans', { method: 'POST', body: { assets: chosen.map((asset) => asset.ref) } });
    setPlan(result.plan);
  });

  const confirmPlan = async () => {
    if (!plan) return;
    const risky = plan.historyAffectedDocumentIds.length > 0;
    const ok = await confirmAction({
      title: `删除这 ${plan.assets.length} 个资源？`,
      body: risky ? `其中有资源仍被 ${plan.historyAffectedDocumentIds.length} 篇文档的历史版本引用，删除后回滚到那些版本会缺少文件。删除会在 24 小时的迟到引用保护窗口之后执行。` : '删除会在 24 小时的迟到引用保护窗口之后执行，在此之前重新被引用的资源会被保留。',
      confirmLabel: risky ? '接受风险并删除' : '确认删除', tone: 'danger',
    });
    if (!ok) return;
    await run(async () => {
      await api(`/v1/cleanup-plans/${plan.planId}:confirm`, { method: 'POST', body: { confirmationHash: plan.confirmationHash, acknowledgeHistoryLoss: risky, expectedRevision: plan.revision } });
      setPlan(null);
      setSelected(new Set());
      await load(false, null);
    }, '已确认，保护窗口结束后删除');
  };

  const cancelPlan = () => run(async () => {
    if (!plan) return;
    await api(`/v1/cleanup-plans/${plan.planId}`, { method: 'DELETE', body: {} });
    setPlan(null);
  }, '已放弃清理计划');

  return (
    <div className="flex flex-col gap-6">
      <PageHeader title="资源库" description="文档和站点设置里上传的图片、视频、附件与画板。删除需要先生成清理计划，并经过保护窗口。" />
      <div className="flex flex-wrap items-end justify-between gap-3">
        <Tabs selectedKey={tab} onSelectionChange={(key) => setTab(key as 'all' | 'unused')}>
          <Tabs.ListContainer>
            <Tabs.List aria-label="资源范围">
              <Tabs.Tab id="all">全部资源<Tabs.Indicator /></Tabs.Tab>
              <Tabs.Tab id="unused">未被引用<Tabs.Indicator /></Tabs.Tab>
            </Tabs.List>
          </Tabs.ListContainer>
          <Tabs.Panel id="all"><span /></Tabs.Panel>
          <Tabs.Panel id="unused"><span /></Tabs.Panel>
        </Tabs>
        <SelectInput aria-label="类别" className="w-44" options={[{ value: 'all', label: '全部类别' }, ...Object.entries(kinds).map(([value, label]) => ({ value, label }))]} placeholder="全部类别" value={kind} onChange={setKind} variant="primary" />
      </div>
      {tab === 'unused' && <Notice status="accent">这里只列当前没有任何文档或站点设置引用的资源。勾选后生成清理计划；仍被历史版本引用的会单独提示风险。</Notice>}

      {plan && (
        <Panel title="清理计划" description={`共 ${plan.assets.length} 个资源${plan.historyAffectedDocumentIds.length ? `，历史版本涉及 ${plan.historyAffectedDocumentIds.length} 篇文档` : ''}。`}
          footer={<><Button isDisabled={busy} variant="tertiary" onPress={cancelPlan}>放弃计划</Button><Button isDisabled={busy} variant="danger" onPress={confirmPlan}><TrashBin />确认清理</Button></>}>
          <Details items={[['计划编号', <Mono>{plan.planId}</Mono>], ['合计大小', fmtBytes(plan.assets.reduce((sum, asset) => sum + asset.byteSize, 0))]]} />
        </Panel>
      )}

      {error ? <ErrorBlock error={error} onRetry={() => void load(false, null)} /> : loading && !assets.length ? <LoadingBlock lines={6} /> : !assets.length ? (
        <EmptyBlock icon={<Picture />} title={tab === 'unused' ? '没有未被引用的资源' : '还没有资源'} description={tab === 'unused' ? '所有资源都在被使用。' : '在文档编辑器里插入图片、视频或附件后，会出现在这里。'} />
      ) : (
        <>
          {tab === 'unused' && (
            <div className="flex items-center gap-3">
              <span className="text-sm text-muted">已选 {chosen.length} 项</span>
              <Button isDisabled={busy || !chosen.length || !!plan} size="sm" variant="secondary" onPress={propose}>为选中的资源生成清理计划</Button>
            </div>
          )}
          <Table key={tab}>
            <Table.ScrollContainer>
              <Table.Content
                aria-label="资源列表"
                className="min-w-[600px]"
                selectedKeys={tab === 'unused' ? selected : undefined}
                selectionMode={tab === 'unused' ? 'multiple' : 'none'}
                onSelectionChange={tab === 'unused' ? setSelected : undefined}
              >
                <Table.Header>
                  {tab === 'unused' ? (
                    <Table.Column className="w-10 pe-0">
                      <Checkbox aria-label="全选" slot="selection"><Checkbox.Content><Checkbox.Control><Checkbox.Indicator /></Checkbox.Control></Checkbox.Content></Checkbox>
                    </Table.Column>
                  ) : null}
                  <Table.Column isRowHeader>资源</Table.Column>
                  <Table.Column>类别</Table.Column>
                  <Table.Column>大小</Table.Column>
                  <Table.Column>上传时间</Table.Column>
                  <Table.Column className="text-end">引用</Table.Column>
                </Table.Header>
                <Table.Body>
                  {assets.map((asset) => {
                    const Icon = kindIcon(asset.kind);
                    return (
                      <Table.Row key={asset.ref.assetId} id={asset.ref.assetId}>
                        {tab === 'unused' ? (
                          <Table.Cell className="pe-0">
                            <Checkbox aria-label={`选择 ${asset.originalName}`} slot="selection" variant="secondary"><Checkbox.Content><Checkbox.Control><Checkbox.Indicator /></Checkbox.Control></Checkbox.Content></Checkbox>
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
                        <Table.Cell className="text-end"><Button isDisabled={busy} size="sm" variant="tertiary" onPress={() => showReferences(asset)}><LinkIcon />查看</Button></Table.Cell>
                      </Table.Row>
                    );
                  })}
                </Table.Body>
              </Table.Content>
            </Table.ScrollContainer>
          </Table>
          {cursor && <div className="flex justify-center"><Button isPending={loading} variant="secondary" onPress={() => void load(true, cursor)}>加载更多</Button></div>}
        </>
      )}

      {refs && (
        <Drawer.Backdrop isOpen onOpenChange={(open) => { if (!open) setRefs(null); }}>
          <Drawer.Content placement="right">
            <Drawer.Dialog className="w-full max-w-md">
              <Drawer.CloseTrigger aria-label="关闭" />
              <Drawer.Header><Drawer.Heading className="truncate">“{refs.asset.originalName}”的引用</Drawer.Heading></Drawer.Header>
              <Drawer.Body className="flex flex-col gap-5">
                {[['当前引用', refs.snapshot.currentReferences], ['历史版本引用', refs.snapshot.historyReferences]].map(([label, list]) => (
                  <div key={label as string} className="flex flex-col gap-2">
                    <span className="text-sm font-medium text-foreground">{label as string}</span>
                    {!(list as Reference[]).length ? <span className="text-sm text-muted">无</span> : (
                      <ul className="flex flex-col gap-1.5">
                        {bySource(list as Reference[]).map(({ ref, count }) => (
                          <li key={ref.sourceService + (ref.documentId ?? ref.sourceObjectId)} className="flex items-center justify-between gap-2 rounded-lg bg-surface-secondary px-3 py-2 text-sm">
                            <SourceLink reference={ref} titles={refs.titles} />
                            {count > 1 && <span className="shrink-0 text-xs text-muted">{count} 处</span>}
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
    const key = ref.sourceService + (ref.documentId ?? ref.sourceObjectId);
    const group = groups.get(key);
    if (group) group.count++;
    else groups.set(key, { ref, count: 1 });
  }
  return [...groups.values()];
}

function SourceLink({ reference, titles }: { reference: Reference; titles: Map<string, string> }) {
  const go = (path: string) => (event: MouseEvent) => { event.preventDefault(); navigate(path); };
  if (reference.documentId) {
    const path = `/admin/content?doc=${reference.documentId}`;
    const title = titles.get(reference.documentId);
    return title
      ? <a className="truncate text-accent hover:underline" href={path} onClick={go(path)}>{title}</a>
      : <span className="truncate">文档 <Mono>{reference.documentId}</Mono></span>;
  }
  if (reference.sourceService === 'site-config') return <a className="truncate text-accent hover:underline" href="/admin/site" onClick={go('/admin/site')}>站点设置（品牌图片、首页或页脚）</a>;
  return <span className="truncate">{sources[reference.sourceService] ?? reference.sourceService} <Mono>{reference.sourceObjectId}</Mono></span>;
}
