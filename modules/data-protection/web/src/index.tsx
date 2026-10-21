import { Archive, ArrowDownToLine, Lock, Plus } from '@gravity-ui/icons';
import { Button, Chip, Disclosure, Modal, Table } from '@heroui/react';
import {
  api, ApiError, Details, EmptyBlock, ErrorBlock, fmtBytes, fmtDateTime, fmtRelative, LoadingBlock, Mono, newKey, Notice, notify, NumberInput, PageHeader, Panel, SwitchInput, TextInput,
  useAction, useHasGrant, useLoad,
} from '@ui';
import { useState } from 'react';
import type { components } from './gen/api.gen';

type Backup = components['schemas']['ReadBackupResult'] & { backupId: string };
type Manifest = { createdAt?: string; byteSize?: number; includeSecrets?: boolean; sha256?: string; domains?: { slug: string; files: number; bytes: number }[] };
const stateText: Record<string, { label: string; tone: 'success' | 'accent' | 'danger' | 'default' }> = {
  ready: { label: '就绪', tone: 'success' }, requested: { label: '排队中', tone: 'default' }, running: { label: '进行中', tone: 'accent' }, failed: { label: '失败', tone: 'danger' },
};
const parse = (raw: string): Manifest => { try { return JSON.parse(raw) as Manifest; } catch { return {}; } };

export default function BackupPage() {
  const isSuperadmin = useHasGrant('superadmin');
  const [pages, setPages] = useState<string[]>(['']);
  const backups = useLoad(async (signal) => {
    const page = await api<components['schemas']['ListBackupsResult']>(`/v1/backups?limit=20${pages[pages.length - 1] ? `&cursor=${encodeURIComponent(pages[pages.length - 1])}` : ''}`, { signal });
    const rows = await Promise.all(page.backupIds.map(async (backupId) => ({ ...await api<components['schemas']['ReadBackupResult']>(`/v1/backups/${encodeURIComponent(backupId)}`, { signal }), backupId })));
    return { rows, next: page.nextCursor };
  }, [pages]);
  const [creating, setCreating] = useState(false);
  const [downloading, setDownloading] = useState<Backup | null>(null);

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="备份与恢复"
        description="完整备份包含全部服务的数据（SQLite 取一致快照）。备份期间站点照常运行。"
        actions={<Button onPress={() => setCreating(true)}><Plus />创建备份</Button>}
      />
      <div className="grid gap-6 xl:grid-cols-3">
        <Schedule />
        <Panel className="xl:col-span-2" title="如何恢复" description="恢复会替换全站数据，需要在服务器上停服务后进行，目前不能在后台直接恢复。">
          <ol className="flex list-decimal flex-col gap-2 ps-5 text-sm leading-6 text-foreground">
            <li>下载备份，放到服务器上。</li>
            <li>在部署目录运行 <Mono>deploy/restore-archive.sh &lt;备份文件的绝对路径&gt;</Mono>。</li>
            <li>加密备份（.enc）会先提示输入口令，也可以设环境变量 <Mono>NW_BACKUP_PASSPHRASE</Mono> 免交互。</li>
            <li>脚本会校验归档、停服务、逐个数据卷替换并重新启动；含密钥的备份把密钥解到 <Mono>deploy/secrets-restored</Mono>，确认后再覆盖 <Mono>deploy/secrets</Mono>。</li>
            <li>恢复完成后用备份里的账号重新登录。版本镜像与远程同步尚未提供。</li>
          </ol>
        </Panel>
      </div>
      <Panel title="备份列表" actions={<Button size="sm" variant="tertiary" onPress={backups.reload}>刷新</Button>}>
        {backups.error && !backups.data ? <ErrorBlock error={backups.error} onRetry={backups.reload} /> : !backups.data ? <LoadingBlock lines={4} /> : !backups.data.rows.length ? (
          <EmptyBlock icon={<Archive />} title="还没有备份" description="点右上角“创建备份”，或开启每日自动备份。" />
        ) : (
          <Table variant="secondary">
            <Table.ScrollContainer>
              <Table.Content aria-label="备份" className="min-w-[640px]">
                <Table.Header>
                  <Table.Column isRowHeader>备份</Table.Column>
                  <Table.Column>状态</Table.Column>
                  <Table.Column>大小</Table.Column>
                  <Table.Column>内容</Table.Column>
                  <Table.Column className="text-end">操作</Table.Column>
                </Table.Header>
                <Table.Body>
                  {backups.data.rows.map((backup) => {
                    const manifest = parse(backup.manifestJSON);
                    const state = stateText[backup.state] ?? { label: backup.state, tone: 'default' as const };
                    return (
                      <Table.Row key={backup.backupId} id={backup.backupId}>
                        <Table.Cell>
                          <span className="block font-medium">{manifest.createdAt ? fmtDateTime(manifest.createdAt) : '—'}</span>
                          <span className="text-xs text-muted">{manifest.createdAt ? fmtRelative(manifest.createdAt) : ''} · <Mono className="text-[11px]">{backup.backupId.slice(0, 12)}</Mono></span>
                        </Table.Cell>
                        <Table.Cell>
                          <Chip color={state.tone} size="sm" variant="soft">{state.label}</Chip>
                          {backup.issues.length > 0 && <span className="ms-2 text-xs text-danger">{backup.issues.map((issue) => `${issue.owner}：${issue.reason}`).join('；')}</span>}
                        </Table.Cell>
                        <Table.Cell className="tabular-nums">{fmtBytes(manifest.byteSize)}</Table.Cell>
                        <Table.Cell>
                          <span className="flex flex-wrap items-center gap-1">
                            {manifest.includeSecrets && <Chip color="warning" size="sm" variant="soft"><Lock />含密钥</Chip>}
                            <span className="text-xs text-muted">{manifest.domains?.length ?? 0} 个数据卷</span>
                          </span>
                        </Table.Cell>
                        <Table.Cell className="text-end">
                          {backup.state === 'ready' && isSuperadmin && <Button size="sm" variant="secondary" onPress={() => setDownloading(backup)}><ArrowDownToLine />下载</Button>}
                        </Table.Cell>
                      </Table.Row>
                    );
                  })}
                </Table.Body>
              </Table.Content>
            </Table.ScrollContainer>
          </Table>
        )}
        {backups.data && (pages.length > 1 || backups.data.next) && (
          <div className="flex justify-center gap-3">
            <Button isDisabled={pages.length <= 1} size="sm" variant="tertiary" onPress={() => setPages(pages.slice(0, -1))}>上一页</Button>
            <Button isDisabled={!backups.data.next} size="sm" variant="tertiary" onPress={() => setPages([...pages, backups.data!.next])}>下一页</Button>
          </div>
        )}
      </Panel>
      {creating && <CreateDialog isSuperadmin={isSuperadmin} onClose={() => setCreating(false)} onCreated={() => { setCreating(false); setPages(['']); backups.reload(); }} />}
      {downloading && <DownloadDialog backup={downloading} onClose={() => setDownloading(null)} />}
    </div>
  );
}

function Schedule() {
  const settings = useLoad((signal) => api<components['schemas']['ReadProtectionSettingsResult']>('/v1/settings', { signal }).catch(() => ({} as components['schemas']['ReadProtectionSettingsResult'])), []);
  const [keepCount, setKeepCount] = useState<number | null>(null);
  const { busy, run } = useAction();
  const schedule = settings.data?.schedule;
  const value = keepCount ?? schedule?.keepCount ?? 7;
  const save = () => run(async () => {
    const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone;
    await api('/v1/local-backup-schedule', { method: 'PUT', body: { keepCount: value, timezone }, idempotencyKey: newKey() });
    settings.reload();
  }, '已开启每日自动备份');
  return (
    <Panel title="每日自动备份" description="每天 03:00（按浏览器所在时区）自动创建一份，超出份数的旧备份会被清理。">
      {schedule ? <Chip color="success" variant="soft">已开启 · {schedule.timezone} · 保留 {schedule.keepCount} 份</Chip> : <Chip variant="soft">未开启</Chip>}
      <NumberInput label="保留份数" maxValue={60} minValue={1} value={value} onChange={setKeepCount} />
      <div><Button isPending={busy} variant="secondary" onPress={save}>{schedule ? '更新设置' : '开启自动备份'}</Button></div>
    </Panel>
  );
}

function CreateDialog({ isSuperadmin, onClose, onCreated }: { isSuperadmin: boolean; onClose: () => void; onCreated: () => void }) {
  const [includeSecrets, setIncludeSecrets] = useState(false);
  const { busy, run } = useAction();
  const [operationId] = useState(newKey);
  const create = async () => {
    const result = await run(() => api<components['schemas']['CreateBackupResult']>('/v1/backups', { method: 'POST', body: { includeSecrets: isSuperadmin && includeSecrets }, idempotencyKey: operationId }));
    if (result) { notify.success('备份已受理', '完整就绪后才能下载，稍后刷新列表查看。'); onCreated(); }
  };
  return (
    <Modal.Backdrop isOpen onOpenChange={(open) => { if (!open) onClose(); }}>
      <Modal.Container size="sm">
        <Modal.Dialog>
          <Modal.CloseTrigger aria-label="关闭" />
          <Modal.Header><Modal.Heading>创建完整备份</Modal.Heading></Modal.Header>
          <Modal.Body className="flex flex-col gap-4">
            <p className="text-sm leading-6 text-muted">把各服务的数据目录打成一个 tar.gz，SQLite 取一致快照。站点照常运行。</p>
            {isSuperadmin && <SwitchInput description="包含各服务的签名密钥和加密主密钥。含密钥的备份下载时必须设口令。" isSelected={includeSecrets} label="包含密钥和敏感配置" onChange={setIncludeSecrets} />}
          </Modal.Body>
          <Modal.Footer>
            <Button variant="tertiary" onPress={onClose}>取消</Button>
            <Button isPending={busy} onPress={create}>创建</Button>
          </Modal.Footer>
        </Modal.Dialog>
      </Modal.Container>
    </Modal.Backdrop>
  );
}

// 便携包可能很大：用浏览器的文件流写盘，不把整包读进内存。
function DownloadDialog({ backup, onClose }: { backup: Backup; onClose: () => void }) {
  const manifest = parse(backup.manifestJSON);
  const [passphrase, setPassphrase] = useState('');
  const [busy, setBusy] = useState(false);
  const required = manifest.includeSecrets === true;
  const tooShort = passphrase !== '' && passphrase.length < 12;
  const picker = (window as Window & { showSaveFilePicker?: (options: { suggestedName: string }) => Promise<{ createWritable(): Promise<WritableStream<Uint8Array>> }> }).showSaveFilePicker;

  const download = async () => {
    if (!picker) return;
    setBusy(true);
    let output: WritableStream<Uint8Array> | undefined;
    try {
      const handle = await picker.call(window, { suggestedName: `backup-${backup.backupId}.tar.gz${passphrase ? '.enc' : ''}` });
      const authorization = await api<components['schemas']['AuthorizeBackupDownloadResult']>(`/v1/backups/${encodeURIComponent(backup.backupId)}:download`, { method: 'POST', body: {}, idempotencyKey: newKey() });
      const response = await api<Response>(`/v1/backups/${encodeURIComponent(backup.backupId)}/download`, { responseType: 'stream', backupDownloadToken: authorization.downloadToken, backupPassphrase: passphrase || undefined });
      if (!response.ok || !response.body) throw new Error('备份数据流不可用');
      output = await handle.createWritable();
      await response.body.pipeTo(output);
      output = undefined;
      notify.success('备份已保存', passphrase ? '已用口令加密，请妥善保管口令。' : '请妥善保管。');
      onClose();
    } catch (problem) {
      if (output && !output.locked) await output.abort().catch(() => {});
      if ((problem as { name?: string })?.name !== 'AbortError') notify.error('下载失败', problem instanceof ApiError ? problem.message : (problem as Error).message);
    } finally { setBusy(false); }
  };

  return (
    <Modal.Backdrop isOpen onOpenChange={(open) => { if (!open) onClose(); }}>
      <Modal.Container size="md">
        <Modal.Dialog>
          <Modal.CloseTrigger aria-label="关闭" />
          <Modal.Header><Modal.Heading>下载备份</Modal.Heading></Modal.Header>
          <Modal.Body className="flex flex-col gap-4">
            <Details items={[['创建时间', fmtDateTime(manifest.createdAt)], ['大小', fmtBytes(manifest.byteSize)], ['含密钥', required ? '是' : '否']]} />
            {!picker && <Notice status="warning">当前浏览器不支持流式保存大文件，请使用 Chrome 或 Edge。</Notice>}
            <TextInput
              autoComplete="new-password"
              description={<>可选；{required ? '这份备份含密钥，必须设口令。' : ''}至少 12 个字符。解密：<Mono>openssl enc -d -aes-256-cbc -pbkdf2 -iter 100000</Mono></>}
              error={tooShort ? '至少 12 个字符' : null}
              isRequired={required}
              label="下载口令"
              type="password"
              value={passphrase}
              onChange={setPassphrase}
            />
            {manifest.domains && (
              <Disclosure>
                <Disclosure.Heading><Disclosure.Trigger className="text-sm text-muted">查看包含的数据卷<Disclosure.Indicator /></Disclosure.Trigger></Disclosure.Heading>
                <Disclosure.Content>
                  <ul className="mt-2 flex flex-col gap-1 text-sm">
                    {manifest.domains.map((domain) => <li key={domain.slug} className="flex justify-between gap-3"><span>{domain.slug}</span><span className="tabular-nums text-muted">{domain.files} 个文件 · {fmtBytes(domain.bytes)}</span></li>)}
                  </ul>
                </Disclosure.Content>
              </Disclosure>
            )}
          </Modal.Body>
          <Modal.Footer>
            <Button variant="tertiary" onPress={onClose}>取消</Button>
            <Button isDisabled={!picker || tooShort || (required && !passphrase)} isPending={busy} onPress={download}><ArrowDownToLine />选择位置并下载</Button>
          </Modal.Footer>
        </Modal.Dialog>
      </Modal.Container>
    </Modal.Backdrop>
  );
}
