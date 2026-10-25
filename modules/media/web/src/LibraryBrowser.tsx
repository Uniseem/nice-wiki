import { File as FileIcon, Folder, ListUl, Picture, Plus, Square, TrashBin } from '@gravity-ui/icons';
import { Button, Modal, Table } from '@heroui/react';
import { api, askText, assetURL, confirmAction, EmptyBlock, ErrorBlock, FileButton, fmtBytes, fmtDateTime, LoadingBlock, newKey, notify, SelectInput, uploadAsset, uploadImage, useAction, useHasGrant, useLoad, type AssetRef, t } from '@ui';
import { useEffect, useState } from 'react';

type Asset = { ref: AssetRef; kind: string; originalName: string; fileName?: string; groupId?: string; createdAt: string; state: string };
type Group = { groupId: string; name: string; assetCount: number };
type Page = { assets: Asset[]; pageInfo: { hasMore: boolean; nextCursor?: string } };

// 资源分组、布局切换和图片选择交互参考Halo；使用本站组件和API独立实现。
// https://github.com/halo-dev/halo/tree/main/console/src/modules/contents/attachments
export function LibraryBrowser({ onSelect, imagesOnly = false }: { onSelect?: (ref: AssetRef) => void; imagesOnly?: boolean }) {
  const canManage = useHasGrant('media.manage');
  const canUpload = useHasGrant('media.upload');
  const [groupId, setGroupId] = useState('all');
  const [kind, setKind] = useState(imagesOnly ? 'image' : 'all');
  const [layout, setLayout] = useState<'grid' | 'list'>(() => { try { return localStorage.getItem('nw.media.layout') === 'list' ? 'list' : 'grid'; } catch { return 'grid'; } });
  const [cursor, setCursor] = useState('');
  const [preview, setPreview] = useState<Asset | null>(null);
  const { busy, run } = useAction();
  const groups = useLoad((signal) => api<{ groups: Group[] }>('/v1/asset-groups', { signal }).then((result) => result.groups), []);
  const query = new URLSearchParams({ limit: '40', ...(kind !== 'all' ? { kind } : {}), ...(groupId !== 'all' ? { groupId: groupId === 'ungrouped' ? '' : groupId } : {}), ...(cursor ? { cursor } : {}) });
  const assets = useLoad((signal) => api<Page>(`/v1/assets?${query}`, { signal }), [kind, groupId, cursor]);
  useEffect(() => { setCursor(''); }, [groupId, kind]);
  const refresh = () => { setCursor(''); assets.reload(); groups.reload(); };
  const chooseLayout = (value: 'grid' | 'list') => { setLayout(value); try { localStorage.setItem('nw.media.layout', value); } catch { /* 当前布局仍可使用 */ } };
  const move = (asset: Asset, target: string) => run(async () => {
    await api(`/v1/assets/${asset.ref.assetId}/group`, { method: 'PUT', body: { assetId: asset.ref.assetId, groupId: target } });
    setPreview(null); refresh();
  }, t('已移动'));
  const createGroup = async () => {
    const name = await askText({ title: t('新建分组'), label: t('名称'), confirmLabel: t('创建') });
    if (!name?.trim()) return;
    await run(async () => { const result = await api<{ group: Group }>('/v1/asset-groups', { method: 'POST', body: { name: name.trim() } }); groups.reload(); setGroupId(result.group.groupId); }, t('分组已创建'));
  };
  const removeGroup = async () => {
    if (!await confirmAction({ title: t('删除分组？'), body: t('分组内的文件会移到未分组，文件和已有链接保持有效。'), confirmLabel: t('删除'), tone: 'danger' })) return;
    await run(async () => { await api(`/v1/asset-groups/${groupId}`, { method: 'DELETE' }); setGroupId('all'); groups.reload(); }, t('分组已删除'));
  };
  const upload = (file: File) => run(async () => {
    const ref = file.type.startsWith('image/') ? await uploadAsset(file, 'image') : await uploadAsset(file, file.type.startsWith('video/') ? 'video' : 'attachment');
    if (canManage && groupId !== 'all' && groupId !== 'ungrouped') await api(`/v1/assets/${ref.assetId}/group`, { method: 'PUT', body: { assetId: ref.assetId, groupId } });
    refresh();
  }, t('上传完成'));
  const copy = (asset: Asset) => run(async () => { await navigator.clipboard.writeText(new URL(assetURL(asset.ref), location.origin).href); }, t('链接已复制'));
  const remove = async (asset: Asset) => {
    if (!await confirmAction({ title: t('删除“{0}”？', asset.originalName), body: t('资源会先检查引用；仍在使用的文件不能删除。删除在保护窗口后执行。'), confirmLabel: t('删除'), tone: 'danger' })) return;
    await run(async () => {
      const result = await api<{ plan: { planId: string; confirmationHash: string; revision: number; historyAffectedDocumentIds: string[] } }>('/v1/cleanup-plans', { method: 'POST', body: { assets: [asset.ref] } });
      const plan = result.plan;
      if (plan.historyAffectedDocumentIds.length) { notify.warning(t('资源仍被历史版本引用，请在清理计划中确认风险。')); return; }
      await api(`/v1/cleanup-plans/${plan.planId}:confirm`, { method: 'POST', body: { confirmationHash: plan.confirmationHash, acknowledgeHistoryLoss: false, expectedRevision: plan.revision } });
      setPreview(null); refresh();
    }, t('已加入删除计划'));
  };
  const pick = (asset: Asset) => { if (onSelect) onSelect(asset.ref); else setPreview(asset); };
  const groupOptions = [{ value: 'all', label: t('全部资源') }, { value: 'ungrouped', label: t('未分组') }, ...(groups.data ?? []).map((g) => ({ value: g.groupId, label: `${g.name} (${g.assetCount})` }))];
  return <div className="flex min-h-0 flex-col gap-4">
    <div className="flex flex-wrap items-end justify-between gap-3"><div className="flex flex-wrap items-end gap-2"><SelectInput label={t('分组')} options={groupOptions} value={groupId} onChange={setGroupId} className="w-48" />{!imagesOnly && <SelectInput label={t('类别')} value={kind} onChange={setKind} options={[{ value: 'all', label: t('全部类别') }, { value: 'image', label: t('图片') }, { value: 'video', label: t('视频') }, { value: 'attachment', label: t('附件') }]} className="w-36" />}{canManage && <><Button aria-label={t('新建分组')} variant="tertiary" onPress={() => void createGroup()}><Folder /><Plus /></Button>{!['all', 'ungrouped'].includes(groupId) && <Button aria-label={t('删除分组')} variant="tertiary" onPress={() => void removeGroup()}><TrashBin /></Button>}</>}</div>
      <div className="flex gap-2"><Button isIconOnly aria-label={t('网格')} aria-pressed={layout === 'grid'} variant={layout === 'grid' ? 'secondary' : 'tertiary'} onPress={() => chooseLayout('grid')}><Square /></Button><Button isIconOnly aria-label={t('列表')} aria-pressed={layout === 'list'} variant={layout === 'list' ? 'secondary' : 'tertiary'} onPress={() => chooseLayout('list')}><ListUl /></Button>{canUpload && <FileButton isPending={busy} accept={imagesOnly ? 'image/png,image/jpeg,image/webp,image/gif,image/avif' : undefined} onFile={(file) => void upload(file)}>{t('上传')}</FileButton>}<Button variant="secondary" onPress={refresh}>{t('刷新')}</Button></div>
    </div>
    {!!groups.error && <ErrorBlock error={groups.error} onRetry={groups.reload} />}
    {assets.error ? <ErrorBlock error={assets.error} onRetry={assets.reload} /> : !assets.data ? <LoadingBlock lines={5} /> : !assets.data.assets.length ? <EmptyBlock icon={<Picture />} title={t('这个分组还没有资源')} /> : layout === 'grid' ? <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">{assets.data.assets.map((asset) => <article key={asset.ref.assetId} className="group min-w-0 overflow-hidden rounded-xl border border-border bg-surface">
      <button type="button" className="flex aspect-[4/3] w-full items-center justify-center overflow-hidden bg-surface-secondary outline-none focus-visible:ring-2 focus-visible:ring-accent" aria-label={onSelect ? t('选择“{0}”', asset.originalName) : t('预览“{0}”', asset.originalName)} onClick={() => pick(asset)}>{asset.ref.mediaType.startsWith('image/') ? <img alt={asset.originalName} className="h-full w-full object-contain transition-transform group-hover:scale-105 motion-reduce:transition-none" loading="lazy" src={assetURL(asset.ref)} /> : <FileIcon className="size-10 text-muted" />}</button>
      <div className="p-3"><p className="truncate text-sm" title={asset.originalName}>{asset.originalName}</p><div className="mt-1 flex items-center justify-between text-xs text-muted"><span>{fmtBytes(asset.ref.byteSize)}</span><button type="button" className="hover:text-accent" onClick={() => setPreview(asset)}>{t('详情')}</button></div></div>
    </article>)}</div> : <Table><Table.ScrollContainer><Table.Content aria-label={t('资源列表')} className="min-w-[560px]"><Table.Header><Table.Column isRowHeader>{t('文件')}</Table.Column><Table.Column>{t('大小')}</Table.Column><Table.Column>{t('上传时间')}</Table.Column><Table.Column>{t('操作')}</Table.Column></Table.Header><Table.Body>{assets.data.assets.map((asset) => <Table.Row key={asset.ref.assetId} id={asset.ref.assetId}><Table.Cell><button type="button" className="flex items-center gap-3 text-start" onClick={() => pick(asset)}>{asset.ref.mediaType.startsWith('image/') ? <img className="size-10 rounded object-contain" src={assetURL(asset.ref)} alt="" loading="lazy" /> : <FileIcon className="size-7 text-muted" />}<span className="line-clamp-2">{asset.originalName}</span></button></Table.Cell><Table.Cell>{fmtBytes(asset.ref.byteSize)}</Table.Cell><Table.Cell>{fmtDateTime(asset.createdAt)}</Table.Cell><Table.Cell><Button size="sm" variant="tertiary" onPress={() => setPreview(asset)}>{t('详情')}</Button></Table.Cell></Table.Row>)}</Table.Body></Table.Content></Table.ScrollContainer></Table>}
    <div className="flex justify-end gap-2">{cursor && <Button variant="tertiary" onPress={() => setCursor('')}>{t('第一页')}</Button>}{assets.data?.pageInfo.hasMore && <Button variant="secondary" onPress={() => setCursor(assets.data!.pageInfo.nextCursor ?? '')}>{t('下一页')}</Button>}</div>
    {preview && <Modal.Backdrop isOpen onOpenChange={(open) => { if (!open) setPreview(null); }}><Modal.Container scroll="outside" size="lg"><Modal.Dialog><Modal.CloseTrigger aria-label={t('关闭')} /><Modal.Header><Modal.Heading className="break-all">{preview.originalName}</Modal.Heading></Modal.Header><Modal.Body className="flex flex-col gap-4">
      {preview.ref.mediaType.startsWith('image/') ? <img alt={preview.originalName} src={assetURL(preview.ref)} className="max-h-[55vh] w-full rounded-xl bg-surface-secondary object-contain" /> : preview.kind === 'video' ? <video controls preload="metadata" src={assetURL(preview.ref)} className="max-h-[55vh] w-full rounded-xl" /> : <div className="flex h-32 items-center justify-center rounded-xl bg-surface-secondary"><FileIcon className="size-12 text-muted" /></div>}
      <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2 text-sm"><dt className="text-muted">{t('存储名称')}</dt><dd className="select-all break-all font-mono text-xs">{preview.fileName ?? preview.ref.sha256}</dd><dt className="text-muted">{t('大小')}</dt><dd>{fmtBytes(preview.ref.byteSize)}</dd><dt className="text-muted">{t('上传时间')}</dt><dd>{fmtDateTime(preview.createdAt)}</dd></dl>
      {canManage && <SelectInput label={t('分组')} value={preview.groupId ?? ''} onChange={(target) => void move(preview, target)} options={[{ value: '', label: t('未分组') }, ...(groups.data ?? []).map((g) => ({ value: g.groupId, label: g.name }))]} />}
    </Modal.Body><Modal.Footer className="flex-wrap">{canManage && <Button variant="danger-soft" isDisabled={busy || preview.state !== 'ready'} onPress={() => void remove(preview)}>{t('删除')}</Button>}<Button variant="secondary" onPress={() => copy(preview)}>{t('复制链接')}</Button><a className="rounded-xl bg-default px-4 py-2 text-sm" download={preview.originalName} href={assetURL(preview.ref)}>{t('下载')}</a>{onSelect && <Button onPress={() => onSelect(preview.ref)}>{t('选择')}</Button>}</Modal.Footer></Modal.Dialog></Modal.Container></Modal.Backdrop>}
  </div>;
}

export function AssetPicker({ onSelect, onClose }: { onSelect: (ref: AssetRef) => void; onClose: () => void }) {
  return <Modal.Backdrop isOpen onOpenChange={(open) => { if (!open) onClose(); }}><Modal.Container scroll="outside" size="cover"><Modal.Dialog><Modal.CloseTrigger aria-label={t('关闭')} /><Modal.Header><Modal.Heading>{t('从图片库选择')}</Modal.Heading></Modal.Header><Modal.Body><LibraryBrowser imagesOnly onSelect={onSelect} /></Modal.Body><Modal.Footer><Button variant="secondary" onPress={onClose}>{t('取消')}</Button></Modal.Footer></Modal.Dialog></Modal.Container></Modal.Backdrop>;
}
