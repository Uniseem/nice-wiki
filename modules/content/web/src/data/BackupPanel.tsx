import { ArrowDownToLine, Ellipsis, Plus } from '@gravity-ui/icons';
import { Button, Chip, Dropdown, Form, Label, Modal, Table } from '@heroui/react';
import type { Key } from '@heroui/react';
import { api, confirmAction, EmptyBlock, ErrorBlock, fmtBytes, fmtDateTime, fmtRelative, LoadingBlock, newKey, Notice, notify, NumberInput, Panel, SelectInput, TextInput, useAction, useLoad, t } from '@ui';
import { useEffect, useState } from 'react';

type Target = { targetId: string; kind: Kind; name: string; endpoint: string; bucket: string; region: string; pathPrefix: string; username: string; hasSecret: boolean; schedule: 'off' | 'daily' | 'weekly'; keep: number; lastRunAt: string; lastStatus: string; lastMessage: string };
type Kind = 's3' | 'webdav' | 'gdrive' | 'onedrive' | 'dropbox';
type Run = { runId: string; targetId: string; startedAt: string; finishedAt: string; status: string; objectName: string; bytes: number; message: string };

const scheduleText = { off: t('不自动备份'), daily: t('每天一次'), weekly: t('每周一次') };
const kindText = { s3: t('S3 兼容存储'), webdav: 'WebDAV', gdrive: 'Google Drive', onedrive: 'OneDrive', dropbox: 'Dropbox' };
const isDrive = (kind: Kind) => kind === 'gdrive' || kind === 'onedrive' || kind === 'dropbox';
const statusTone: Record<string, 'success' | 'danger' | 'default'> = { ok: 'success', failed: 'danger' };
const statusText: Record<string, string> = { ok: t('成功'), failed: t('失败'), running: t('进行中') };

export function BackupPanel({ site }: { site: string }) {
  const targets = useLoad((signal) => api<{ targets: Target[] }>('/v1/backup-targets', { signal }).then((result) => result.targets), [site]);
  const runs = useLoad((signal) => api<{ runs: Run[] }>('/v1/backup-runs', { signal }).then((result) => result.runs), [site]);
  const [editing, setEditing] = useState<Target | 'new' | null>(null);
  const { busy, run } = useAction();
  const [running, setRunning] = useState('');

  // 网盘授权跳回来后，后台入口把授权码放在 sessionStorage 里，这里接力完成。
  useEffect(() => {
    let saved: string | null = null;
    try { saved = sessionStorage.getItem('nw_drive_return'); sessionStorage.removeItem('nw_drive_return'); } catch { /* 浏览器禁用了存储就没法接力 */ }
    if (!saved) return;
    const back = JSON.parse(saved) as { provider: string; code: string; state: string; error: string };
    if (!back.code) { notify.error(t('网盘授权没有完成'), back.error || t('授权被取消了。')); return; }
    void run(() => api(`/v1/sites/${site}/drive-auth/complete`, { method: 'POST', body: { code: back.code, state: back.state, redirectUri: `${location.origin}/admin/oauth/${back.provider}` } }), t('网盘已授权')).then(() => targets.reload());
  }, []);
  const authorize = async (target: Target) => {
    const result = await run(() => api<{ authorizeUrl: string }>(`/v1/sites/${site}/drive-auth/start`, { method: 'POST', body: { provider: target.kind, targetId: target.targetId, redirectUri: `${location.origin}/admin/oauth/${target.kind}` } }));
    if (result) location.href = result.authorizeUrl;
  };

  const download = async () => {
    const blob = await run(() => api<Blob>('/v1/site-export', { responseType: 'blob' }));
    if (!blob) return;
    const link = document.createElement('a');
    link.href = URL.createObjectURL(blob);
    link.download = `nicewiki-${site.slice(0, 8)}-${new Date().toISOString().slice(0, 10)}.zip`;
    link.click();
    setTimeout(() => URL.revokeObjectURL(link.href), 10_000);
  };
  const backupNow = async (target: Target) => {
    setRunning(target.targetId);
    try {
      const done = await run(() => api<{ run: Run }>(`/v1/backup-targets/${target.targetId}:run`, { method: 'POST', body: {} }));
      if (done) notify.success(t('已备份到“{0}”', target.name));
    } finally { setRunning(''); targets.reload(); runs.reload(); }
  };
  const remove = async (target: Target) => {
    const ok = await confirmAction({ title: t('删除备份目标“{0}”？', target.name), body: t('只删除这里的配置，远端已有的备份文件不会被删除。'), confirmLabel: t('删除'), tone: 'danger' });
    if (!ok) return;
    await run(() => api(`/v1/backup-targets/${target.targetId}`, { method: 'DELETE' }), t('已删除'));
    targets.reload();
  };

  return (
    <div className="flex flex-col gap-6">
      <Panel title={t('下载整站导出')} description={t('把全部文档（含草稿）、图片和视频打成一个压缩包。里面有 Markdown 方便阅读，也有完整数据，随时可以在“导入”里还原。')}
        actions={<Button isPending={busy && !running} onPress={() => void download()}><ArrowDownToLine />{t('下载导出包')}</Button>} />
      <Panel title={t('备份目标')} description={t('把整站导出自动或手动备份到你自己的存储。密钥加密后保存，页面不再显示。')}
        actions={<Button onPress={() => setEditing('new')}><Plus />{t('添加备份目标')}</Button>}>
        {targets.error ? <ErrorBlock error={targets.error} onRetry={targets.reload} /> : !targets.data ? <LoadingBlock lines={3} /> : (
          <Table variant="secondary">
            <Table.ScrollContainer>
              <Table.Content aria-label={t('备份目标')} className="min-w-[680px]">
                <Table.Header>
                  <Table.Column isRowHeader>{t('名称')}</Table.Column>
                  <Table.Column>{t('类型')}</Table.Column>
                  <Table.Column>{t('计划')}</Table.Column>
                  <Table.Column>{t('最近一次')}</Table.Column>
                  <Table.Column className="w-24 text-end">{t('操作')}</Table.Column>
                </Table.Header>
                <Table.Body renderEmptyState={() => <EmptyBlock className="m-4" title={t('还没有备份目标')} description={t('添加一个 S3 兼容存储、WebDAV 或自己的网盘，就可以一键备份。')} />}>
                  {targets.data.map((target) => (
                    <Table.Row key={target.targetId} id={target.targetId}>
                      <Table.Cell><span className="block font-medium text-foreground">{target.name}</span><span className="block text-xs text-muted">{isDrive(target.kind) ? (target.hasSecret ? t('已授权') : t('未授权')) : target.endpoint}</span></Table.Cell>
                      <Table.Cell>{kindText[target.kind]}</Table.Cell>
                      <Table.Cell>{scheduleText[target.schedule]}<span className="block text-xs text-muted">{t('保留最近 {0} 份', target.keep)}</span></Table.Cell>
                      <Table.Cell>
                        {target.lastRunAt ? (<><Chip color={statusTone[target.lastStatus] ?? 'default'} size="sm" variant="soft">{statusText[target.lastStatus] ?? target.lastStatus}</Chip><span className="block text-xs text-muted">{fmtRelative(target.lastRunAt)}</span>{target.lastStatus === 'failed' && <span className="block max-w-xs text-xs text-danger">{target.lastMessage}</span>}</>) : <span className="text-muted">{t('还没备份过')}</span>}
                      </Table.Cell>
                      <Table.Cell className="text-end">
                        <div className="flex items-center justify-end gap-1">
                          {isDrive(target.kind) && !target.hasSecret
                            ? <Button isPending={busy} size="sm" onPress={() => void authorize(target)}>{t('授权')}</Button>
                            : <Button isPending={running === target.targetId} size="sm" variant="secondary" onPress={() => void backupNow(target)}>{t('立即备份')}</Button>}
                          <Dropdown>
                            <Button isIconOnly aria-label={t('更多操作')} size="sm" variant="ghost"><Ellipsis /></Button>
                            <Dropdown.Popover placement="bottom end">
                              <Dropdown.Menu onAction={(key: Key) => { if (key === 'edit') setEditing(target); if (key === 'reauthorize') void authorize(target); if (key === 'delete') void remove(target); }}>
                                <Dropdown.Item id="edit" textValue={t('编辑')}><Label>{t('编辑')}</Label></Dropdown.Item>
                                {isDrive(target.kind) && target.hasSecret && <Dropdown.Item id="reauthorize" textValue={t('重新授权')}><Label>{t('重新授权')}</Label></Dropdown.Item>}
                                <Dropdown.Item id="delete" textValue={t('删除')} variant="danger"><Label>{t('删除')}</Label></Dropdown.Item>
                              </Dropdown.Menu>
                            </Dropdown.Popover>
                          </Dropdown>
                        </div>
                      </Table.Cell>
                    </Table.Row>
                  ))}
                </Table.Body>
              </Table.Content>
            </Table.ScrollContainer>
          </Table>
        )}
      </Panel>
      <Panel title={t('备份记录')} actions={<Button size="sm" variant="tertiary" onPress={() => { runs.reload(); targets.reload(); }}>{t('刷新')}</Button>}>
        {runs.error ? <ErrorBlock error={runs.error} onRetry={runs.reload} /> : !runs.data ? <LoadingBlock lines={3} /> : !runs.data.length ? <EmptyBlock title={t('还没有备份记录')} /> : (
          <Table variant="secondary">
            <Table.ScrollContainer>
              <Table.Content aria-label={t('备份记录')} className="min-w-[560px]">
                <Table.Header>
                  <Table.Column isRowHeader>{t('时间')}</Table.Column>
                  <Table.Column>{t('结果')}</Table.Column>
                  <Table.Column>{t('文件')}</Table.Column>
                  <Table.Column className="text-end">{t('大小')}</Table.Column>
                </Table.Header>
                <Table.Body>
                  {runs.data.slice(0, 20).map((entry) => (
                    <Table.Row key={entry.runId} id={entry.runId}>
                      <Table.Cell>{fmtDateTime(entry.startedAt)}</Table.Cell>
                      <Table.Cell><Chip color={statusTone[entry.status] ?? 'default'} size="sm" variant="soft">{statusText[entry.status] ?? entry.status}</Chip>{entry.status === 'failed' && <span className="block max-w-xs text-xs text-danger">{entry.message}</span>}</Table.Cell>
                      <Table.Cell><span className="font-mono text-xs">{entry.objectName || '—'}</span></Table.Cell>
                      <Table.Cell className="text-end">{entry.bytes ? fmtBytes(entry.bytes) : '—'}</Table.Cell>
                    </Table.Row>
                  ))}
                </Table.Body>
              </Table.Content>
            </Table.ScrollContainer>
          </Table>
        )}
      </Panel>
      <Notice status="default" title={t('如何恢复')}>{t('在“导入”里选“本站导出包”，上传从备份目标下载的 .zip，会把文档和图片还原成草稿。')}</Notice>
      {editing && <TargetDialog target={editing === 'new' ? null : editing} onClose={() => setEditing(null)} onSaved={() => { setEditing(null); targets.reload(); }} />}
    </div>
  );
}

function TargetDialog({ target, onClose, onSaved }: { target: Target | null; onClose: () => void; onSaved: () => void }) {
  const [kind, setKind] = useState<Kind>(target?.kind ?? 's3');
  const [name, setName] = useState(target?.name ?? '');
  const [endpoint, setEndpoint] = useState(target?.endpoint ?? '');
  const [bucket, setBucket] = useState(target?.bucket ?? '');
  const [region, setRegion] = useState(target?.region ?? '');
  const [prefix, setPrefix] = useState(target?.pathPrefix ?? '');
  const [username, setUsername] = useState(target?.username ?? '');
  const [secret, setSecret] = useState('');
  const [schedule, setSchedule] = useState<'off' | 'daily' | 'weekly'>(target?.schedule ?? 'off');
  const [keep, setKeep] = useState(target?.keep ?? 7);
  const { busy, run } = useAction();
  const submit = async () => {
    const body = isDrive(kind)
      ? { kind, name, endpoint: '', pathPrefix: prefix, schedule, keep }
      : { kind, name, endpoint, bucket: kind === 's3' ? bucket : undefined, region: kind === 's3' ? region : undefined, pathPrefix: prefix, username, secret: secret || undefined, schedule, keep };
    const done = await run(() => api(`/v1/backup-targets/${target?.targetId ?? newKey()}`, { method: 'PUT', body }), t('已保存'));
    if (done !== undefined) onSaved();
  };
  return (
    <Modal.Backdrop isOpen onOpenChange={(open) => { if (!open) onClose(); }}>
      <Modal.Container size="lg" scroll="inside">
        <Modal.Dialog>
          <Modal.CloseTrigger aria-label={t('关闭')} />
          <Modal.Header><Modal.Heading>{target ? t('编辑备份目标') : t('添加备份目标')}</Modal.Heading></Modal.Header>
          <Form className="flex flex-col" onSubmit={(event) => { event.preventDefault(); void submit(); }}>
            <Modal.Body className="flex flex-col gap-4">
              <SelectInput label={t('类型')} value={kind} onChange={setKind} isDisabled={!!target && isDrive(target.kind)} options={[{ value: 's3', label: t('S3 兼容存储'), description: t('AWS S3、Cloudflare R2、Backblaze B2、MinIO 等') }, { value: 'webdav', label: 'WebDAV', description: t('坚果云、Nextcloud、群晖等') }, { value: 'gdrive', label: 'Google Drive', description: t('只访问本应用创建的文件') }, { value: 'onedrive', label: 'OneDrive', description: t('只访问应用专属文件夹') }, { value: 'dropbox', label: 'Dropbox', description: t('只访问应用文件夹') }]} />
              <TextInput isRequired label={t('名称')} value={name} onChange={setName} maxLength={80} />
              {isDrive(kind) && <Notice status="default" title={t('保存后点“授权”')}>{t('保存这个目标后，在列表里点“授权”，跳到网盘确认。平台只拿到应用文件夹的权限，授权可以随时在网盘里收回。')}</Notice>}
              {!isDrive(kind) && <TextInput isRequired label={t('地址')} type="url" value={endpoint} onChange={setEndpoint} placeholder={kind === 's3' ? 'https://s3.us-east-1.amazonaws.com' : 'https://dav.example.com/remote.php/dav/files/me'} description={t('必须是公网 https 地址。')} />}
              {kind === 's3' && <div className="grid gap-4 sm:grid-cols-2"><TextInput isRequired label={t('桶名')} value={bucket} onChange={setBucket} /><TextInput label={t('区域')} value={region} onChange={setRegion} placeholder="us-east-1" /></div>}
              <TextInput label={isDrive(kind) ? t('文件夹') : t('目录前缀')} value={prefix} onChange={setPrefix} placeholder="nicewiki" description={t('备份文件放在这个目录下；留空放在根目录。')} />
              {!isDrive(kind) && <div className="grid gap-4 sm:grid-cols-2">
                <TextInput isRequired label={kind === 's3' ? t('访问密钥 ID') : t('用户名')} value={username} onChange={setUsername} autoComplete="off" />
                <TextInput isRequired={!target} label={kind === 's3' ? t('访问密钥') : t('密码')} type="password" value={secret} onChange={setSecret} autoComplete="new-password" placeholder={target?.hasSecret ? t('已保存，留空不修改') : ''} />
              </div>}
              <div className="grid gap-4 sm:grid-cols-2">
                <SelectInput label={t('自动备份')} value={schedule} onChange={setSchedule} options={[{ value: 'off', label: scheduleText.off }, { value: 'daily', label: scheduleText.daily }, { value: 'weekly', label: scheduleText.weekly }]} />
                <NumberInput label={t('保留份数')} value={keep} onChange={setKeep} minValue={1} maxValue={100} description={t('超出的旧备份会被删除。')} />
              </div>
            </Modal.Body>
            <Modal.Footer>
              <Button slot="close" variant="secondary" onPress={onClose}>{t('取消')}</Button>
              <Button isPending={busy} type="submit">{t('保存')}</Button>
            </Modal.Footer>
          </Form>
        </Modal.Dialog>
      </Modal.Container>
    </Modal.Backdrop>
  );
}
