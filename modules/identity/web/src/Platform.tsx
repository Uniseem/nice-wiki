import { Copy, Plus } from '@gravity-ui/icons';
import { Button, Chip, Form, Modal, Table, Tabs } from '@heroui/react';
import { api, askText, CheckGroupInput, confirmAction, EmptyBlock, ErrorBlock, fmtDateTime, fmtMoney, FormGrid, LoadingBlock, Mono, Notice, notify, NumberInput, PageHeader, Panel, SelectInput, SwitchInput, TextInput, useAction, useLoad, t } from '@ui';
import { downloadFile, localized, ThemeSwatch, type Manifest } from '@modules/site-config/web/src/themes';
import { useEffect, useState } from 'react';

type Provider = { provider: 'github' | 'google' | 'linuxdo' | 'gdrive' | 'onedrive' | 'dropbox'; enabled: boolean; clientId: string; clientSecretSet: boolean };
type Settings = { registrationOpen: boolean; emailRegistration: boolean; inviteRequired: boolean; defaultMaxSites: number; defaultStorageBytes: number; defaultVisitorQaDaily: number; platformHost: string; sitesDomain: string; providers: Provider[]; cloudflareTokenSet: boolean; mediaStorage: MediaStorage };
type MediaStorage = { enabled: boolean; endpoint: string; bucket: string; region: string; prefix: string; accessKeyId: string; secretSet: boolean };
type RegInvite = { inviteId: string; note: string; maxUses: number; used: number; expiresAt?: string; revoked: boolean; createdAt: string };
type AiInvite = { inviteId: string; note: string; capMicros: number; period: 'once' | 'monthly'; tasks: string[]; maxUses: number; used: number; maxSites?: number; storageBytes?: number; expiresAt?: string; revoked: boolean; createdAt: string };
type User = { userId: string; email: string; displayName: string; status: 'active' | 'disabled' | 'deleted'; siteCount: number; createdAt: string };
type AdminSite = { siteId: string; slug: string; name: string; ownerEmail: string; status: 'active' | 'suspended' | 'deleted'; createdAt: string };
type ThemeSubmission = { themeId: string; version: string; digest: string; manifest: Manifest; byteSize: number; publisher: string; status: 'pending' | 'approved' | 'rejected'; reviewNote: string; submittedAt: string };
type Report = { reportId: string; siteId: string; host: string; url: string; reason: string; detail: string; status: 'open' | 'dismissed' | 'takedown' | 'banned'; resolution: string; createdAt: string };

const providerNames = { github: 'GitHub', google: 'Google', linuxdo: 'Linux DO', gdrive: 'Google Drive', onedrive: 'OneDrive', dropbox: 'Dropbox' };
const driveProviders = ['gdrive', 'onedrive', 'dropbox'];
const GB = 1 << 30;
const taskOptions = [
  { value: 'qa', label: t('访客问答') }, { value: 'localization', label: t('翻译') }, { value: 'knowledge', label: t('知识库索引') },
  { value: 'memory', label: t('长期记忆助手') }, { value: 'content', label: t('文档标题与摘要') },
];
const reasonText: Record<string, string> = { spam: t('垃圾内容'), phishing: t('钓鱼或欺诈'), illegal: t('违法内容'), copyright: t('侵权'), other: t('其他') };

export default function PlatformPage() {
  const [tab, setTab] = useState('settings');
  return (
    <div className="flex flex-col gap-6">
      <PageHeader title={t('平台管理')} description={t('平台的域名、登录方式、邀请码、用户、站点和举报。只有站主能看到这里。')} />
      <Tabs selectedKey={tab} onSelectionChange={(key) => setTab(String(key))}>
        <Tabs.ListContainer className="mb-4 overflow-x-auto">
          <Tabs.List aria-label={t('平台管理')}>
            <Tabs.Tab id="settings">{t('设置')}<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="regs">{t('注册邀请码')}<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="ai">{t('AI 邀请码')}<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="users">{t('用户')}<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="sites">{t('站点')}<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="reports">{t('举报')}<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="themes">{t('外观审核')}<Tabs.Indicator /></Tabs.Tab>
          </Tabs.List>
        </Tabs.ListContainer>
        <Tabs.Panel id="settings"><SettingsTab /></Tabs.Panel>
        <Tabs.Panel id="regs"><RegInvitesTab /></Tabs.Panel>
        <Tabs.Panel id="ai"><AiInvitesTab /></Tabs.Panel>
        <Tabs.Panel id="users"><UsersTab /></Tabs.Panel>
        <Tabs.Panel id="sites"><SitesTab /></Tabs.Panel>
        <Tabs.Panel id="reports"><ReportsTab /></Tabs.Panel>
        <Tabs.Panel id="themes"><ThemeReviewsTab /></Tabs.Panel>
      </Tabs>
    </div>
  );
}

function SettingsTab() {
  const current = useLoad((signal) => api<Settings>('/v1/platform/settings', { signal }), []);
  const [form, setForm] = useState<Settings | null>(null);
  const [secrets, setSecrets] = useState<Record<string, string>>({});
  const [clearToken, setClearToken] = useState(false);
  const { busy, run } = useAction();
  useEffect(() => { if (current.data) setForm(current.data); }, [current.data]);
  if (current.error && !current.data) return <ErrorBlock error={current.error} onRetry={current.reload} />;
  if (!form) return <LoadingBlock lines={6} />;
  const set = <K extends keyof Settings>(key: K, value: Settings[K]) => setForm({ ...form, [key]: value });
  const setProvider = (provider: Provider['provider'], patch: Partial<Provider>) => set('providers', form.providers.map((item) => (item.provider === provider ? { ...item, ...patch } : item)));
  const save = async () => {
    const providers = form.providers.map((item) => ({ provider: item.provider, enabled: item.enabled, clientId: item.clientId, ...(secrets[item.provider] ? { clientSecret: secrets[item.provider] } : {}) }));
    const { cloudflareTokenSet: _, mediaStorage: { secretSet: __, ...media }, ...settings } = form;
    // 令牌框留空保持不变；点了“清除令牌”才清掉。
    const token = secrets.cloudflare ? { cloudflareToken: secrets.cloudflare } : clearToken ? { cloudflareToken: '' } : {};
    const saved = await run(() => api<Settings>('/v1/platform/settings', { method: 'PUT', body: { ...settings, providers, ...token, mediaStorage: { ...media, ...(secrets.media ? { secretAccessKey: secrets.media } : {}) } } }), t('平台设置已保存'));
    if (saved) { setSecrets({}); setClearToken(false); current.setData(saved); }
  };
  const providerCard = (item: Provider) => (
    <div key={item.provider} className="flex flex-col gap-3 rounded-xl border border-separator p-4">
      <SwitchInput isSelected={item.enabled} label={providerNames[item.provider]} onChange={(value) => setProvider(item.provider, { enabled: value })} />
      <FormGrid>
        <TextInput label="Client ID" value={item.clientId} onChange={(value) => setProvider(item.provider, { clientId: value.trim() })} />
        <TextInput autoComplete="off" label="Client secret" placeholder={item.clientSecretSet ? t('已设置，留空保持不变') : ''} type="password" value={secrets[item.provider] ?? ''} onChange={(value) => setSecrets({ ...secrets, [item.provider]: value })} />
      </FormGrid>
    </div>
  );
  return (
    <Form className="flex flex-col gap-6" onSubmit={(event) => { event.preventDefault(); void save(); }}>
      <Panel title={t('域名')} description={t('上线时在这里填写；改动后需要在入口（Caddy）侧解析对应的泛域名。')}>
        <FormGrid>
          <TextInput label={t('平台域名')} placeholder="app.example.com" value={form.platformHost} onChange={(value) => set('platformHost', value.trim())} description={t('用户登录、管理站点的地址。')} />
          <TextInput label={t('站点子域名后缀')} placeholder="example.site" value={form.sitesDomain} onChange={(value) => set('sitesDomain', value.trim())} description={t('用户站点是 <站点名>.<后缀>，需要泛解析到平台入口。')} />
          <TextInput autoComplete="off" label={t('Cloudflare API 令牌（泛域名证书）')} placeholder={form.cloudflareTokenSet ? t('已设置，留空保持不变') : ''} type="password" value={secrets.cloudflare ?? ''} onChange={(value) => setSecrets({ ...secrets, cloudflare: value })}
            description={form.cloudflareTokenSet ? clearToken ? t('保存后清除令牌，平台子域名回到逐个签证书。') : t('已设置：平台子域名用一张泛域名证书（DNS 验证）。') : t('子域名后缀的 DNS 托管在 Cloudflare 时填写（令牌只需该域名的 DNS 编辑权限），平台子域名就改用一张泛域名证书，不再逐个签。')} />
        </FormGrid>
        {form.cloudflareTokenSet && <div><Button isDisabled={clearToken} size="sm" variant="secondary" onPress={() => { setClearToken(true); setSecrets({ ...secrets, cloudflare: '' }); }}>{t('清除令牌')}</Button></div>}
      </Panel>
      <Panel title={t('媒体存储')} description={t('默认放在本机磁盘。开启 S3 兼容存储后，之后上传的图片、视频和附件会在后台搬到存储桶，之前的文件仍从本机读。')}>
        <SwitchInput isSelected={form.mediaStorage.enabled} label={t('使用 S3 兼容存储')} onChange={(value) => set('mediaStorage', { ...form.mediaStorage, enabled: value })} />
        {form.mediaStorage.enabled && (
          <FormGrid>
            <TextInput isRequired label={t('服务地址')} placeholder="https://s3.example.com" value={form.mediaStorage.endpoint} onChange={(value) => set('mediaStorage', { ...form.mediaStorage, endpoint: value.trim() })} />
            <TextInput isRequired label={t('存储桶')} value={form.mediaStorage.bucket} onChange={(value) => set('mediaStorage', { ...form.mediaStorage, bucket: value.trim() })} />
            <TextInput label={t('区域')} placeholder="us-east-1" value={form.mediaStorage.region} onChange={(value) => set('mediaStorage', { ...form.mediaStorage, region: value.trim() })} />
            <TextInput label={t('路径前缀')} placeholder="media" value={form.mediaStorage.prefix} onChange={(value) => set('mediaStorage', { ...form.mediaStorage, prefix: value.trim() })} />
            <TextInput isRequired label={t('访问密钥 ID')} value={form.mediaStorage.accessKeyId} onChange={(value) => set('mediaStorage', { ...form.mediaStorage, accessKeyId: value.trim() })} />
            <TextInput autoComplete="off" isRequired={!form.mediaStorage.secretSet} label={t('访问密钥')} placeholder={form.mediaStorage.secretSet ? t('已设置，留空保持不变') : ''} type="password" value={secrets.media ?? ''} onChange={(value) => setSecrets({ ...secrets, media: value })} />
          </FormGrid>
        )}
      </Panel>
      <Panel title={t('注册')}>
        <div className="flex flex-col gap-4">
          <SwitchInput isSelected={form.registrationOpen} label={t('开放注册')} description={t('关闭时，只有拿到注册邀请码的人能注册（第三方登录首次登录也受此限制）。')} onChange={(value) => set('registrationOpen', value)} />
          <SwitchInput isSelected={form.emailRegistration} label={t('允许邮箱注册')} onChange={(value) => set('emailRegistration', value)} />
          <SwitchInput isSelected={form.inviteRequired} label={t('邮箱注册需要邀请码')} description={t('邮箱注册不验证邮箱地址。')} onChange={(value) => set('inviteRequired', value)} />
        </div>
      </Panel>
      <Panel title={t('新用户的默认额度')}>
        <FormGrid columns={3}>
          <NumberInput label={t('站点数上限')} minValue={0} maxValue={1000} value={form.defaultMaxSites} onChange={(value) => set('defaultMaxSites', value)} />
          <NumberInput label={t('存储上限（GB）')} minValue={0} value={Math.round(form.defaultStorageBytes / GB)} onChange={(value) => set('defaultStorageBytes', value * GB)} />
          <NumberInput label={t('访客每日提问上限（每站）')} minValue={0} maxValue={1000000} value={form.defaultVisitorQaDaily} onChange={(value) => set('defaultVisitorQaDaily', value)} />
        </FormGrid>
      </Panel>
      <Panel title={t('第三方登录')} description={t('在对应平台创建 OAuth 应用，回调地址填 {0}。密钥保存后不再显示。', `${location.origin}/admin/oauth/<provider>`)}>
        <div className="flex flex-col gap-5">{form.providers.filter((item) => !driveProviders.includes(item.provider)).map(providerCard)}</div>
      </Panel>
      <Panel title={t('网盘备份')} description={t('站点所有者可以把备份存到自己的网盘。在对应平台创建 OAuth 应用并只申请应用文件夹权限（Google 用 drive.file，OneDrive 用 Files.ReadWrite.AppFolder，Dropbox 选 App folder），回调地址填 {0}。', `${location.origin}/admin/oauth/<gdrive|onedrive|dropbox>`)}>
        <div className="flex flex-col gap-5">{form.providers.filter((item) => driveProviders.includes(item.provider)).map(providerCard)}</div>
      </Panel>
      <div><Button isPending={busy} type="submit">{t('保存')}</Button></div>
    </Form>
  );
}

// 新生成的邀请码只显示这一次。
function CodeDialog({ code, onClose }: { code: string; onClose: () => void }) {
  return (
    <Modal.Backdrop isOpen onOpenChange={(open) => { if (!open) onClose(); }}>
      <Modal.Container size="md">
        <Modal.Dialog>
          <Modal.CloseTrigger aria-label={t('关闭')} />
          <Modal.Header><Modal.Heading>{t('邀请码已生成')}</Modal.Heading></Modal.Header>
          <Modal.Body className="flex flex-col gap-3">
            <Notice status="warning">{t('现在复制并发给对方。关闭后不能再查看。')}</Notice>
            <div className="flex items-center gap-2 rounded-xl bg-surface-secondary p-3"><Mono className="flex-1 break-all text-base">{code}</Mono>
              <Button isIconOnly aria-label={t('复制')} size="sm" variant="secondary" onPress={() => { void navigator.clipboard.writeText(code).then(() => notify.success(t('已复制'))); }}><Copy /></Button></div>
          </Modal.Body>
          <Modal.Footer><Button onPress={onClose}>{t('完成')}</Button></Modal.Footer>
        </Modal.Dialog>
      </Modal.Container>
    </Modal.Backdrop>
  );
}

function RegInvitesTab() {
  const invites = useLoad((signal) => api<{ invites: RegInvite[] }>('/v1/registration-invites', { signal }).then((result) => result.invites), []);
  const [creating, setCreating] = useState(false);
  const [code, setCode] = useState('');
  const [note, setNote] = useState('');
  const [uses, setUses] = useState(1);
  const { busy, run } = useAction();
  const create = async () => {
    const result = await run(() => api<{ code: string }>('/v1/registration-invites', { method: 'POST', body: { note, maxUses: uses } }));
    if (result) { setCreating(false); setNote(''); setUses(1); setCode(result.code); invites.reload(); }
  };
  const revoke = async (invite: RegInvite) => {
    if (!(await confirmAction({ title: t('作废这个邀请码？'), confirmLabel: t('作废'), tone: 'danger' }))) return;
    await run(() => api(`/v1/registration-invites/${invite.inviteId}`, { method: 'DELETE' }), t('已作废'));
    invites.reload();
  };
  return (
    <div className="flex flex-col gap-4">
      <div className="flex justify-end"><Button onPress={() => setCreating(true)}><Plus />{t('生成邀请码')}</Button></div>
      {invites.error ? <ErrorBlock error={invites.error} onRetry={invites.reload} /> : !invites.data ? <LoadingBlock lines={3} /> : (
        <SimpleTable label={t('注册邀请码')} empty={t('还没有邀请码')} columns={[t('备注'), t('已用 / 可用'), t('状态'), '']}
          rows={invites.data.map((invite) => [invite.note || '—', `${invite.used} / ${invite.maxUses}`, invite.revoked ? <Chip key="s" size="sm" variant="soft">{t('已作废')}</Chip> : <Chip key="s" color="success" size="sm" variant="soft">{t('可用')}</Chip>, invite.revoked ? null : <Button key="a" isDisabled={busy} size="sm" variant="ghost" onPress={() => void revoke(invite)}>{t('作废')}</Button>])} />
      )}
      {creating && (
        <Modal.Backdrop isOpen onOpenChange={(open) => { if (!open) setCreating(false); }}>
          <Modal.Container size="sm"><Modal.Dialog>
            <Modal.CloseTrigger aria-label={t('关闭')} />
            <Modal.Header><Modal.Heading>{t('生成注册邀请码')}</Modal.Heading></Modal.Header>
            <Form onSubmit={(event) => { event.preventDefault(); void create(); }}>
              <Modal.Body className="flex flex-col gap-4">
                <TextInput label={t('备注')} maxLength={200} value={note} onChange={setNote} />
                <NumberInput label={t('可用次数')} minValue={1} maxValue={10000} value={uses} onChange={setUses} />
              </Modal.Body>
              <Modal.Footer><Button slot="close" variant="secondary" onPress={() => setCreating(false)}>{t('取消')}</Button><Button isPending={busy} type="submit">{t('生成')}</Button></Modal.Footer>
            </Form>
          </Modal.Dialog></Modal.Container>
        </Modal.Backdrop>
      )}
      {code && <CodeDialog code={code} onClose={() => setCode('')} />}
    </div>
  );
}

function AiInvitesTab() {
  const invites = useLoad((signal) => api<{ invites: AiInvite[] }>('/v1/ai-invites', { signal }).then((result) => result.invites), []);
  const [creating, setCreating] = useState(false);
  const [code, setCode] = useState('');
  const [form, setForm] = useState({ note: '', dollars: 5, period: 'once' as 'once' | 'monthly', uses: 1, maxSites: 5, storageGB: 1, days: 0 });
  const [tasks, setTasks] = useState<string[]>([]);
  const { busy, run } = useAction();
  const create = async () => {
    const body = {
      note: form.note, capMicros: Math.round(form.dollars * 1_000_000), period: form.period, tasks, maxUses: form.uses, maxSites: form.maxSites, storageBytes: form.storageGB * GB,
      ...(form.days > 0 ? { expiresAt: new Date(Date.now() + form.days * 86400_000).toISOString() } : {}),
    };
    const result = await run(() => api<{ code: string }>('/v1/ai-invites', { method: 'POST', body }));
    if (result) { setCreating(false); setCode(result.code); invites.reload(); }
  };
  const revoke = async (invite: AiInvite) => {
    if (!(await confirmAction({ title: t('作废这个 AI 邀请码？'), body: t('已兑换的用户不受影响。'), confirmLabel: t('作废'), tone: 'danger' }))) return;
    await run(() => api(`/v1/ai-invites/${invite.inviteId}`, { method: 'DELETE' }), t('已作废'));
    invites.reload();
  };
  const set = <K extends keyof typeof form>(key: K, value: (typeof form)[K]) => setForm({ ...form, [key]: value });
  return (
    <div className="flex flex-col gap-4">
      <div className="flex justify-end"><Button onPress={() => setCreating(true)}><Plus />{t('生成 AI 邀请码')}</Button></div>
      {invites.error ? <ErrorBlock error={invites.error} onRetry={invites.reload} /> : !invites.data ? <LoadingBlock lines={3} /> : (
        <SimpleTable label={t('AI 邀请码')} empty={t('还没有 AI 邀请码')} columns={[t('备注'), t('花费上限'), t('周期'), t('已用 / 可用'), t('状态'), '']}
          rows={invites.data.map((invite) => [invite.note || '—', fmtMoney(invite.capMicros), invite.period === 'monthly' ? t('每月') : t('一次性'), `${invite.used} / ${invite.maxUses}`,
            invite.revoked ? <Chip key="s" size="sm" variant="soft">{t('已作废')}</Chip> : <Chip key="s" color="success" size="sm" variant="soft">{t('可用')}</Chip>,
            invite.revoked ? null : <Button key="a" isDisabled={busy} size="sm" variant="ghost" onPress={() => void revoke(invite)}>{t('作废')}</Button>])} />
      )}
      {creating && (
        <Modal.Backdrop isOpen onOpenChange={(open) => { if (!open) setCreating(false); }}>
          <Modal.Container size="md"><Modal.Dialog>
            <Modal.CloseTrigger aria-label={t('关闭')} />
            <Modal.Header><Modal.Heading>{t('生成 AI 邀请码')}</Modal.Heading></Modal.Header>
            <Form onSubmit={(event) => { event.preventDefault(); void create(); }}>
              <Modal.Body className="flex flex-col gap-4">
                <TextInput label={t('备注')} maxLength={200} value={form.note} onChange={(value) => set('note', value)} />
                <FormGrid>
                  <NumberInput label={t('花费上限（美元）')} minValue={0} step={1} value={form.dollars} onChange={(value) => set('dollars', value)} description={t('平台渠道的花费，超过后只能用自带 key。')} />
                  <SelectInput label={t('额度周期')} value={form.period} onChange={(value) => set('period', value)} options={[{ value: 'once', label: t('一次性') }, { value: 'monthly', label: t('每月重置') }]} />
                  <NumberInput label={t('可兑换次数')} minValue={1} maxValue={10000} value={form.uses} onChange={(value) => set('uses', value)} />
                  <NumberInput label={t('有效天数（0 为不过期）')} minValue={0} maxValue={3650} value={form.days} onChange={(value) => set('days', value)} />
                  <NumberInput label={t('站点数上限')} minValue={0} maxValue={1000} value={form.maxSites} onChange={(value) => set('maxSites', value)} />
                  <NumberInput label={t('存储上限（GB）')} minValue={0} value={form.storageGB} onChange={(value) => set('storageGB', value)} />
                </FormGrid>
                <CheckGroupInput label={t('平台渠道可用于')} description={t('都不勾选表示全部任务。')} value={tasks} onChange={setTasks} options={taskOptions} />
              </Modal.Body>
              <Modal.Footer><Button slot="close" variant="secondary" onPress={() => setCreating(false)}>{t('取消')}</Button><Button isPending={busy} type="submit">{t('生成')}</Button></Modal.Footer>
            </Form>
          </Modal.Dialog></Modal.Container>
        </Modal.Backdrop>
      )}
      {code && <CodeDialog code={code} onClose={() => setCode('')} />}
    </div>
  );
}

function UsersTab() {
  const users = useLoad((signal) => api<{ users: User[] }>('/v1/users?limit=100', { signal }).then((result) => result.users), []);
  const { busy, run } = useAction();
  const toggle = async (user: User) => {
    const status = user.status === 'active' ? 'disabled' : 'active';
    if (status === 'disabled' && !(await confirmAction({ title: t('停用用户“{0}”？', user.email || user.displayName), body: t('对方所有设备立即退出，无法再登录；名下站点一起下架，恢复用户时放回。'), confirmLabel: t('停用'), tone: 'warning' }))) return;
    await run(() => api(`/v1/users/${user.userId}/status`, { method: 'PUT', body: { status } }), status === 'active' ? t('已恢复') : t('已停用'));
    users.reload();
  };
  const resetPassword = async (user: User) => {
    const password = await askText({ title: t('重置“{0}”的口令', user.email || user.displayName), label: t('临时口令'), type: 'password', description: t('对方所有设备立即退出，用这个口令登录后必须先改口令。至少 8 位。') });
    if (!password) return;
    await run(() => api(`/v1/users/${user.userId}/password`, { method: 'PUT', body: { password } }), t('口令已重置'));
  };
  if (users.error) return <ErrorBlock error={users.error} onRetry={users.reload} />;
  if (!users.data) return <LoadingBlock lines={4} />;
  return (
    <SimpleTable label={t('用户')} empty={t('还没有用户')} columns={[t('用户'), t('站点数'), t('注册时间'), t('状态'), '']}
      rows={users.data.map((user) => [<span key="u"><span className="block font-medium text-foreground">{user.displayName}</span><span className="block text-xs text-muted">{user.email}</span></span>, String(user.siteCount), fmtDateTime(user.createdAt),
        <Chip key="s" color={user.status === 'active' ? 'success' : 'default'} size="sm" variant="soft">{user.status === 'active' ? t('正常') : t('已停用')}</Chip>,
        user.status === 'deleted' ? null : <span key="a" className="inline-flex gap-1">
          <Button isDisabled={busy} size="sm" variant="ghost" onPress={() => void resetPassword(user)}>{t('重置口令')}</Button>
          <Button isDisabled={busy} size="sm" variant="ghost" onPress={() => void toggle(user)}>{user.status === 'active' ? t('停用') : t('恢复')}</Button>
        </span>])} />
  );
}

function SitesTab() {
  const sites = useLoad((signal) => api<{ sites: AdminSite[] }>('/v1/admin/sites?limit=100', { signal }).then((result) => result.sites), []);
  const { busy, run } = useAction();
  const toggle = async (site: AdminSite) => {
    const status = site.status === 'active' ? 'suspended' : 'active';
    if (status === 'suspended' && !(await confirmAction({ title: t('下架站点“{0}”？', site.name), body: t('公开页立即返回“已下架”，所有者仍可登录后台。'), confirmLabel: t('下架'), tone: 'warning' }))) return;
    await run(() => api(`/v1/admin/sites/${site.siteId}/status`, { method: 'PUT', body: { status } }), status === 'active' ? t('已恢复') : t('已下架'));
    sites.reload();
  };
  if (sites.error) return <ErrorBlock error={sites.error} onRetry={sites.reload} />;
  if (!sites.data) return <LoadingBlock lines={4} />;
  return (
    <SimpleTable label={t('全部站点')} empty={t('还没有站点')} columns={[t('站点'), t('所有者'), t('创建时间'), t('状态'), '']}
      rows={sites.data.filter((site) => site.status !== 'deleted').map((site) => [<span key="n"><span className="block font-medium text-foreground">{site.name}</span><span className="block text-xs text-muted">{site.slug}</span></span>, site.ownerEmail || '—', fmtDateTime(site.createdAt),
        <Chip key="s" color={site.status === 'active' ? 'success' : 'warning'} size="sm" variant="soft">{site.status === 'active' ? t('正常') : t('已下架')}</Chip>,
        <Button key="a" isDisabled={busy} size="sm" variant="ghost" onPress={() => void toggle(site)}>{site.status === 'active' ? t('下架') : t('恢复')}</Button>])} />
  );
}

function ReportsTab() {
  const reports = useLoad((signal) => api<{ reports: Report[] }>('/v1/reports', { signal }).then((result) => result.reports), []);
  const { busy, run } = useAction();
  const resolve = async (report: Report, action: 'dismiss' | 'takedown' | 'ban_owner') => {
    if (action !== 'dismiss' && !(await confirmAction({ title: action === 'takedown' ? t('下架被举报的站点？') : t('下架并封禁所有者？'), body: action === 'takedown' ? t('站点立即下线，所有者仍可登录。') : t('站点立即下线，所有者账号被停用，名下站点全部下架。'), confirmLabel: t('确认'), tone: 'danger' }))) return;
    await run(() => api(`/v1/reports/${report.reportId}:resolve`, { method: 'POST', body: { action } }), t('已处理'));
    reports.reload();
  };
  if (reports.error) return <ErrorBlock error={reports.error} onRetry={reports.reload} />;
  if (!reports.data) return <LoadingBlock lines={4} />;
  const statusLabel = { open: t('待处理'), dismissed: t('已驳回'), takedown: t('已下架'), banned: t('已封禁') };
  return (
    <SimpleTable label={t('举报')} empty={t('没有举报')} columns={[t('被举报'), t('原因'), t('时间'), t('状态'), '']}
      rows={reports.data.map((report) => [<span key="h"><span className="block font-medium text-foreground">{report.host}</span><span className="block text-xs text-muted">{report.url || report.detail.slice(0, 60)}</span></span>, reasonText[report.reason] ?? report.reason, fmtDateTime(report.createdAt),
        <Chip key="s" color={report.status === 'open' ? 'warning' : 'default'} size="sm" variant="soft">{statusLabel[report.status]}</Chip>,
        report.status === 'open' ? (
          <span key="a" className="inline-flex gap-1">
            <Button isDisabled={busy} size="sm" variant="ghost" onPress={() => void resolve(report, 'dismiss')}>{t('驳回')}</Button>
            <Button isDisabled={busy} size="sm" variant="ghost" onPress={() => void resolve(report, 'takedown')}>{t('下架')}</Button>
            <Button isDisabled={busy} size="sm" variant="ghost" onPress={() => void resolve(report, 'ban_owner')}>{t('封禁')}</Button>
          </span>
        ) : null])} />
  );
}

// 外观市场的审核：下载外观包检查 theme.css 与资源（不能有外部地址、不能隐藏举报入口），通过即上架，驳回要写原因。
function ThemeReviewsTab() {
  const list = useLoad((signal) => api<{ submissions: ThemeSubmission[] }>('/v1/platform/theme-reviews', { signal }).then((result) => result.submissions), []);
  const { busy, run } = useAction();
  const path = (item: ThemeSubmission) => `/v1/platform/theme-reviews/${item.themeId}/${item.version}`;
  const approve = (item: ThemeSubmission) => run(() => api(path(item), { method: 'PUT', body: { decision: 'approve' } }), t('已上架')).then(list.reload);
  const reject = async (item: ThemeSubmission) => {
    const note = await askText({ title: t('驳回 {0} {1}', item.themeId, item.version), label: t('驳回原因'), description: t('提交者会在“我提交的外观”里看到。'), multiline: true, required: true, confirmLabel: t('驳回') });
    if (note) await run(() => api(path(item), { method: 'PUT', body: { decision: 'reject', note } }), t('已驳回'));
    list.reload();
  };
  const takedown = async (item: ThemeSubmission) => {
    if (!(await confirmAction({ title: t('下架 {0} {1}？', item.themeId, item.version), body: t('市场里不再显示这个版本，已经装了它的站点不受影响。'), confirmLabel: t('下架'), tone: 'danger' }))) return;
    await run(() => api(`/v1/theme-market/${item.themeId}/${item.version}`, { method: 'DELETE' }), t('已下架'));
    list.reload();
  };
  if (list.error) return <ErrorBlock error={list.error} onRetry={list.reload} />;
  if (!list.data) return <LoadingBlock lines={4} />;
  const statusLabel = { pending: t('待审核'), approved: t('已上架'), rejected: t('已驳回') };
  const statusColor = { pending: 'warning', approved: 'success', rejected: 'default' } as const;
  return (
    <SimpleTable label={t('外观审核')} empty={t('没有外观提交')} columns={[t('外观'), t('提交者'), t('时间'), t('状态'), '']}
      rows={list.data.map((item) => [
        <span key="n" className="flex items-center gap-3"><ThemeSwatch className="!h-12 w-20 shrink-0" manifest={item.manifest} /><span><span className="block font-medium text-foreground">{localized(item.manifest.name)}</span><span className="block text-xs text-muted">{item.themeId} {item.version}</span></span></span>,
        item.publisher, fmtDateTime(item.submittedAt),
        <Chip key="s" color={statusColor[item.status]} size="sm" variant="soft">{statusLabel[item.status]}</Chip>,
        <span key="a" className="inline-flex gap-1">
          <Button isDisabled={busy} size="sm" variant="ghost" onPress={() => void downloadFile(`${path(item)}/package`, `${item.themeId}-${item.version}.zip`)}>{t('下载')}</Button>
          {item.status !== 'approved' && <Button isDisabled={busy} size="sm" variant="ghost" onPress={() => void approve(item)}>{t('通过')}</Button>}
          {item.status === 'pending' && <Button isDisabled={busy} size="sm" variant="ghost" onPress={() => void reject(item)}>{t('驳回')}</Button>}
          {item.status === 'approved' && <Button isDisabled={busy} size="sm" variant="ghost" onPress={() => void takedown(item)}>{t('下架')}</Button>}
        </span>,
      ])} />
  );
}

function SimpleTable({ label, columns, rows, empty }: { label: string; columns: string[]; rows: (React.ReactNode)[][]; empty: string }) {
  return (
    <Table>
      <Table.ScrollContainer>
        <Table.Content aria-label={label} className="min-w-[640px]">
          <Table.Header>
            {columns.map((column, index) => <Table.Column key={index} isRowHeader={index === 0} className={column ? undefined : 'w-40 text-end'}>{column}</Table.Column>)}
          </Table.Header>
          <Table.Body renderEmptyState={() => <EmptyBlock className="m-4" title={empty} />}>
            {rows.map((row, rowIndex) => (
              <Table.Row key={rowIndex} id={String(rowIndex)}>
                {row.map((cell, index) => <Table.Cell key={index} className={index === row.length - 1 && !columns[index] ? 'text-end' : undefined}>{cell}</Table.Cell>)}
              </Table.Row>
            ))}
          </Table.Body>
        </Table.Content>
      </Table.ScrollContainer>
    </Table>
  );
}
