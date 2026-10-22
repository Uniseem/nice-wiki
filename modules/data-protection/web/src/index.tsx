import { Archive, ArrowDownToLine, Lock, Plus } from '@gravity-ui/icons';
import { Button, Chip, Disclosure, Modal, Table } from '@heroui/react';
import {
  api, ApiError, Details, EmptyBlock, ErrorBlock, fmtBytes, fmtDateTime, fmtRelative, LoadingBlock, Mono, newKey, Notice, notify, NumberInput, PageHeader, Panel, SwitchInput, TextInput,
  useAction, useHasGrant, useLoad, t,
} from '@ui';
import { useState } from 'react';
import type { components } from './gen/api.gen';

type Backup = components['schemas']['ReadBackupResult'] & { backupId: string };
type Manifest = { createdAt?: string; byteSize?: number; includeSecrets?: boolean; sha256?: string; domains?: { slug: string; files: number; bytes: number }[] };
const stateText: Record<string, { label: string; tone: 'success' | 'accent' | 'danger' | 'default' }> = {
  ready: { label: t('就绪'), tone: 'success' }, requested: { label: t('排队中'), tone: 'default' }, running: { label: t('进行中'), tone: 'accent' }, failed: { label: t('失败'), tone: 'danger' },
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
        title={t('备份与恢复')}
        description={t('完整备份包含全部服务的数据，备份期间站点照常运行。各服务的数据库依次快照，相隔很短但不是同一时点，备份进行中的保存或发布可能只有一部分进备份；迁移服务器等需要严格一致时，在部署目录运行 deploy/backup.sh 停机备份。')}
        actions={<Button onPress={() => setCreating(true)}><Plus />{t('创建备份')}</Button>}
      />
      <div className="grid gap-6 xl:grid-cols-3">
        <Schedule />
        <Panel className="xl:col-span-2" title={t('如何恢复')} description={t('恢复会替换全站数据，需要在服务器上停服务后进行，目前不能在后台直接恢复。')}>
          <ol className="flex list-decimal flex-col gap-2 ps-5 text-sm leading-6 text-foreground">
            <li>{t('下载备份，放到服务器上。')}</li>
            <li>{t('在部署目录运行')} <Mono>{`deploy/restore-archive.sh <${t('备份文件的绝对路径')}>`}</Mono>{t('。')}</li>
            <li>{t('加密备份（.enc）会先提示输入口令，也可以设环境变量')} <Mono>NW_BACKUP_PASSPHRASE</Mono> {t('免交互。')}</li>
            <li>{t('脚本会校验归档、停服务、逐个数据卷替换并重新启动；含密钥的备份把密钥解到')} <Mono>deploy/secrets-restored</Mono>{t('，确认后再覆盖')} <Mono>deploy/secrets</Mono>{t('。')}</li>
            <li>{t('恢复完成后用备份里的账号重新登录。版本镜像与远程同步尚未提供。')}</li>
          </ol>
        </Panel>
      </div>
      <Panel title={t('备份列表')} actions={<Button size="sm" variant="tertiary" onPress={backups.reload}>{t('刷新')}</Button>}>
        {backups.error && !backups.data ? <ErrorBlock error={backups.error} onRetry={backups.reload} /> : !backups.data ? <LoadingBlock lines={4} /> : !backups.data.rows.length ? (
          <EmptyBlock icon={<Archive />} title={t('还没有备份')} description={t('点右上角“创建备份”，或开启每日自动备份。')} />
        ) : (
          <Table variant="secondary">
            <Table.ScrollContainer>
              <Table.Content aria-label={t('备份')} className="min-w-[640px]">
                <Table.Header>
                  <Table.Column isRowHeader>{t('备份')}</Table.Column>
                  <Table.Column>{t('状态')}</Table.Column>
                  <Table.Column>{t('大小')}</Table.Column>
                  <Table.Column>{t('内容')}</Table.Column>
                  <Table.Column className="text-end">{t('操作')}</Table.Column>
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
                          {backup.issues.length > 0 && <span className="ms-2 text-xs text-danger">{backup.issues.map((issue) => t('{0}：{1}', issue.owner, issue.reason)).join(t('；'))}</span>}
                        </Table.Cell>
                        <Table.Cell className="tabular-nums">{fmtBytes(manifest.byteSize)}</Table.Cell>
                        <Table.Cell>
                          <span className="flex flex-wrap items-center gap-1">
                            {manifest.includeSecrets && <Chip color="warning" size="sm" variant="soft"><Lock />{t('含密钥')}</Chip>}
                            <span className="text-xs text-muted">{t('{0} 个数据卷', manifest.domains?.length ?? 0)}</span>
                          </span>
                        </Table.Cell>
                        <Table.Cell className="text-end">
                          {backup.state === 'ready' && isSuperadmin && <Button size="sm" variant="secondary" onPress={() => setDownloading(backup)}><ArrowDownToLine />{t('下载')}</Button>}
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
            <Button isDisabled={pages.length <= 1} size="sm" variant="tertiary" onPress={() => setPages(pages.slice(0, -1))}>{t('上一页')}</Button>
            <Button isDisabled={!backups.data.next} size="sm" variant="tertiary" onPress={() => setPages([...pages, backups.data!.next])}>{t('下一页')}</Button>
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
  // 保留份数为 0 表示关闭；已有备份保留。
  const save = (keep: number, message: string) => run(async () => {
    const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone;
    await api('/v1/local-backup-schedule', { method: 'PUT', body: { keepCount: keep, timezone }, idempotencyKey: newKey() });
    setKeepCount(null);
    settings.reload();
  }, message);
  return (
    <Panel title={t('每日自动备份')} description={t('每天 03:00（按浏览器所在时区）自动创建一份，超出份数的旧备份会被清理。')}>
      {schedule ? <Chip color="success" variant="soft">{t('已开启 · {0} · 保留 {1} 份', schedule.timezone, schedule.keepCount)}</Chip> : <Chip variant="soft">{t('未开启')}</Chip>}
      <NumberInput label={t('保留份数')} maxValue={60} minValue={1} value={value} onChange={setKeepCount} />
      <div className="flex flex-wrap gap-2">
        <Button isPending={busy} variant="secondary" onPress={() => save(value, schedule ? t('自动备份设置已更新') : t('已开启每日自动备份'))}>{schedule ? t('更新设置') : t('开启自动备份')}</Button>
        {schedule && <Button isDisabled={busy} variant="tertiary" onPress={() => save(0, t('已关闭每日自动备份，已有备份保留'))}>{t('关闭自动备份')}</Button>}
      </div>
    </Panel>
  );
}

function CreateDialog({ isSuperadmin, onClose, onCreated }: { isSuperadmin: boolean; onClose: () => void; onCreated: () => void }) {
  const [includeSecrets, setIncludeSecrets] = useState(false);
  const { busy, run } = useAction();
  const [operationId] = useState(newKey);
  const create = async () => {
    const result = await run(() => api<components['schemas']['CreateBackupResult']>('/v1/backups', { method: 'POST', body: { includeSecrets: isSuperadmin && includeSecrets }, idempotencyKey: operationId }));
    if (result) { notify.success(t('备份已受理'), t('完整就绪后才能下载，稍后刷新列表查看。')); onCreated(); }
  };
  return (
    <Modal.Backdrop isOpen onOpenChange={(open) => { if (!open) onClose(); }}>
      <Modal.Container size="sm">
        <Modal.Dialog>
          <Modal.CloseTrigger aria-label={t('关闭')} />
          <Modal.Header><Modal.Heading>{t('创建完整备份')}</Modal.Heading></Modal.Header>
          <Modal.Body className="flex flex-col gap-4">
            <p className="text-sm leading-6 text-muted">{t('把各服务的数据目录打成一个 tar.gz，SQLite 取一致快照。站点照常运行。')}</p>
            {isSuperadmin && <SwitchInput description={t('包含各服务的签名密钥和加密主密钥。含密钥的备份下载时必须设口令。')} isSelected={includeSecrets} label={t('包含密钥和敏感配置')} onChange={setIncludeSecrets} />}
          </Modal.Body>
          <Modal.Footer>
            <Button variant="tertiary" onPress={onClose}>{t('取消')}</Button>
            <Button isPending={busy} onPress={create}>{t('创建')}</Button>
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

  // 支持文件流保存的浏览器（Chrome、Edge）边下边写盘；其他浏览器（Safari、Firefox）先整包读进内存再按普通下载保存。
  const download = async () => {
    setBusy(true);
    let output: WritableStream<Uint8Array> | undefined;
    try {
      const name = `backup-${backup.backupId}.tar.gz${passphrase ? '.enc' : ''}`;
      const handle = picker ? await picker.call(window, { suggestedName: name }) : null;
      const authorization = await api<components['schemas']['AuthorizeBackupDownloadResult']>(`/v1/backups/${encodeURIComponent(backup.backupId)}:download`, { method: 'POST', body: {}, idempotencyKey: newKey() });
      const response = await api<Response>(`/v1/backups/${encodeURIComponent(backup.backupId)}/download`, { responseType: 'stream', backupDownloadToken: authorization.downloadToken, backupPassphrase: passphrase || undefined });
      if (!response.ok || !response.body) throw new Error(t('备份数据流不可用'));
      if (handle) {
        output = await handle.createWritable();
        await response.body.pipeTo(output);
        output = undefined;
      } else {
        const url = URL.createObjectURL(await response.blob());
        const link = Object.assign(document.createElement('a'), { href: url, download: name });
        link.click();
        window.setTimeout(() => URL.revokeObjectURL(url), 60_000);
      }
      notify.success(t('备份已保存'), passphrase ? t('已用口令加密，请妥善保管口令。') : t('请妥善保管。'));
      onClose();
    } catch (problem) {
      if (output && !output.locked) await output.abort().catch(() => {});
      if ((problem as { name?: string })?.name !== 'AbortError') notify.error(t('下载失败'), problem instanceof ApiError ? problem.message : (problem as Error).message);
    } finally { setBusy(false); }
  };

  return (
    <Modal.Backdrop isOpen onOpenChange={(open) => { if (!open) onClose(); }}>
      <Modal.Container size="md">
        <Modal.Dialog>
          <Modal.CloseTrigger aria-label={t('关闭')} />
          <Modal.Header><Modal.Heading>{t('下载备份')}</Modal.Heading></Modal.Header>
          <Modal.Body className="flex flex-col gap-4">
            <Details items={[[t('创建时间'), fmtDateTime(manifest.createdAt)], [t('大小'), fmtBytes(manifest.byteSize)], [t('含密钥'), required ? t('是') : t('否')]]} />
            {!picker && <Notice status="default">{t('当前浏览器不支持边下边存，备份会先完整读入内存再保存；很大的备份建议用 Chrome 或 Edge 下载。')}</Notice>}
            <TextInput
              autoComplete="new-password"
              description={<>{t('可选；{0}至少 12 个字符。解密：', required ? t('这份备份含密钥，必须设口令。') : '')}<Mono>openssl enc -d -aes-256-cbc -pbkdf2 -iter 100000</Mono></>}
              error={tooShort ? t('至少 12 个字符') : null}
              isRequired={required}
              label={t('下载口令')}
              type="password"
              value={passphrase}
              onChange={setPassphrase}
            />
            {manifest.domains && (
              <Disclosure>
                <Disclosure.Heading><Disclosure.Trigger className="text-sm text-muted">{t('查看包含的数据卷')}<Disclosure.Indicator /></Disclosure.Trigger></Disclosure.Heading>
                <Disclosure.Content>
                  <ul className="mt-2 flex flex-col gap-1 text-sm">
                    {manifest.domains.map((domain) => <li key={domain.slug} className="flex justify-between gap-3"><span>{domain.slug}</span><span className="tabular-nums text-muted">{t('{0} 个文件 · {1}', domain.files, fmtBytes(domain.bytes))}</span></li>)}
                  </ul>
                </Disclosure.Content>
              </Disclosure>
            )}
          </Modal.Body>
          <Modal.Footer>
            <Button variant="tertiary" onPress={onClose}>{t('取消')}</Button>
            <Button isDisabled={tooShort || (required && !passphrase)} isPending={busy} onPress={download}><ArrowDownToLine />{picker ? t('选择位置并下载') : t('下载')}</Button>
          </Modal.Footer>
        </Modal.Dialog>
      </Modal.Container>
    </Modal.Backdrop>
  );
}
