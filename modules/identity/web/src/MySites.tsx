import { ArrowRight, Ellipsis, Globe, Plus } from '@gravity-ui/icons';
import { Button, Chip, Dropdown, Form, Label, Modal, Table } from '@heroui/react';
import type { Key } from '@heroui/react';
import { adminLanguage, api, confirmAction, currentSite, EmptyBlock, ErrorBlock, findSiteLanguage, fmtBytes, fmtDateTime, FormGrid, LoadingBlock, navigate, newKey, NumberInput, PageHeader, selectSite, SelectInput, siteLanguages, SwitchInput, TextInput, useAction, useLoad, t } from '@ui';
import { useState } from 'react';

export type Site = {
  siteId: string; slug: string; name: string; status: 'active' | 'suspended' | 'deleted'; role: 'owner' | 'editor' | 'viewer';
  visibility: 'public' | 'private'; primaryHost: string; createdAt: string; visitorQaDaily: number; qaPausedOn: string; grants: string[];
  purgeAt?: string; // 已删除站点的永久清除时间，在这之前所有者可以恢复
  failedDomains: number; // 复核时验证记录查不到、已失效的自定义域名个数
  renameAfter?: string; // 站点名改过后要到这个时间才能再改
};
type Limits = { maxSites: number; siteCount: number; storageBytes: number };
type Usage = { siteBytes: number; ownerBytes: number; ownerLimitBytes: number };

const roleText: Record<Site['role'], string> = { owner: t('所有者'), editor: t('编辑'), viewer: t('只读') };
const statusText: Record<string, { label: string; tone: 'success' | 'warning' | 'danger' }> = {
  active: { label: t('正常'), tone: 'success' }, suspended: { label: t('已下架'), tone: 'warning' }, deleted: { label: t('已删除'), tone: 'danger' },
};

// 选中站点后，后台其余页面都在这个站点里工作。
export function openSite(site: Site, path = '/admin/dashboard') {
  selectSite({ siteId: site.siteId, grants: site.grants });
  navigate(path);
}

// 站点配置里的公开地址（站内链接、站点地图、规范地址用）跟着主地址走：改站点名、换主域名后调用。
export async function syncPublicAddress(siteId: string) {
  const { site } = await api<{ site: Site }>(`/v1/sites/${siteId}`);
  const url = `${location.protocol}//${site.primaryHost}${location.port && !site.primaryHost.includes(':') && site.primaryHost.endsWith('.localhost') ? `:${location.port}` : ''}`;
  const { config } = await api<{ config: { publicBaseURL: string; titleTemplate: string; description: string; indexingEnabled: boolean; resourceLimits: unknown; securityPolicy: unknown; revision: number } }>('/v1/config', { site: siteId });
  if (!site.primaryHost || config.publicBaseURL === url) return;
  await api('/v1/config', { method: 'PUT', site: siteId, body: { publicBaseURL: url, titleTemplate: config.titleTemplate, description: config.description, indexingEnabled: config.indexingEnabled, resourceLimits: config.resourceLimits, securityPolicy: config.securityPolicy, expectedRevision: config.revision } });
}

export default function MySitesPage() {
  const sites = useLoad((signal) => api<{ sites: Site[]; limits: Limits }>('/v1/sites', { signal }), []);
  const [creating, setCreating] = useState(false);
  const [editing, setEditing] = useState<Site | null>(null);
  const { busy, run } = useAction();

  const remove = async (site: Site) => {
    const ok = await confirmAction({ title: t('删除站点“{0}”？', site.name), body: t('站点会立即下线，30 天内可以在这里恢复，之后数据会被永久清除，站点名会被放出。'), confirmLabel: t('删除'), tone: 'danger' });
    if (!ok) return;
    const done = await run(() => api(`/v1/sites/${site.siteId}`, { method: 'DELETE' }), t('站点已删除'));
    if (done !== undefined) {
      if (currentSite() === site.siteId) selectSite(null);
      sites.reload();
    }
  };

  const restore = async (site: Site) => {
    const done = await run(() => api(`/v1/me/deleted-sites/${site.siteId}/restore`, { method: 'POST' }), t('站点已恢复'));
    if (done !== undefined) sites.reload();
  };

  const limits = sites.data?.limits;
  // 存储额度是名下站点共用的：随便拿一个自己的正常站点向 media 读合计用量。
  const owned = sites.data?.sites.find((site) => site.role === 'owner' && site.status === 'active')?.siteId;
  const usage = useLoad((signal) => owned ? api<Usage>('/v1/storage-usage', { signal, site: owned }) : Promise.resolve(null), [owned]);
  const storage = usage.data ? ' ' + t('存储已用 {0} / {1}。', fmtBytes(usage.data.ownerBytes), fmtBytes(usage.data.ownerLimitBytes)) : '';
  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title={t('我的站点')}
        description={limits ? t('已建 {0} / {1} 个站点。每个站点有自己的子域名，也可以绑定自己的域名。', limits.siteCount, limits.maxSites) + storage : t('每个站点有自己的子域名，也可以绑定自己的域名。')}
        actions={<Button isDisabled={!!limits && limits.siteCount >= limits.maxSites} onPress={() => setCreating(true)}><Plus />{t('新建站点')}</Button>}
      />
      {sites.error ? <ErrorBlock error={sites.error} onRetry={sites.reload} /> : !sites.data ? <LoadingBlock lines={4} /> : (
        <Table>
          <Table.ScrollContainer>
            <Table.Content aria-label={t('站点列表')} className="min-w-[720px]">
              <Table.Header>
                <Table.Column isRowHeader>{t('站点')}</Table.Column>
                <Table.Column>{t('我的角色')}</Table.Column>
                <Table.Column>{t('状态')}</Table.Column>
                <Table.Column className="w-48 text-end">{t('操作')}</Table.Column>
              </Table.Header>
              <Table.Body renderEmptyState={() => <EmptyBlock className="m-4" icon={<Globe />} title={t('还没有站点')} description={t('新建一个站点，开始写文档。')} />}>
                {sites.data.sites.map((site) => (
                  <Table.Row key={site.siteId} id={site.siteId}>
                    <Table.Cell>
                      <span className="block font-medium text-foreground">{site.name}</span>
                      <span className="block text-xs text-muted">{site.primaryHost || site.slug}{site.visibility === 'private' ? ` · ${t('私有')}` : ''}</span>
                    </Table.Cell>
                    <Table.Cell><Chip size="sm" variant="soft">{roleText[site.role]}</Chip></Table.Cell>
                    <Table.Cell>
                      <Chip color={statusText[site.status]?.tone ?? 'default'} size="sm" variant="soft">{statusText[site.status]?.label ?? site.status}</Chip>
                      {site.purgeAt && <span className="ms-2 text-xs text-muted">{t('{0} 天后永久清除', Math.max(1, Math.ceil((Date.parse(site.purgeAt) - Date.now()) / 86400000)))}</span>}
                      {site.failedDomains > 0 && <Chip className="ms-1" color="danger" size="sm" variant="soft">{t('自定义域名已失效')}</Chip>}
                      {site.qaPausedOn === new Date().toISOString().slice(0, 10) && <Chip className="ms-1" color="warning" size="sm" variant="soft">{t('今日提问已达上限')}</Chip>}
                    </Table.Cell>
                    <Table.Cell className="text-end">
                      {site.status === 'deleted' ? (
                        <Button isDisabled={busy} size="sm" variant="secondary" onPress={() => void restore(site)}>{t('恢复')}</Button>
                      ) : (
                        <span className="inline-flex items-center gap-1">
                          <Button size="sm" variant="secondary" onPress={() => openSite(site)}>{t('进入')}<ArrowRight className="rtl:-scale-x-100" /></Button>
                          {site.role === 'owner' && (
                            <Dropdown>
                              <Button isIconOnly aria-label={t('站点操作')} isDisabled={busy} size="sm" variant="ghost"><Ellipsis /></Button>
                              <Dropdown.Popover placement="bottom end">
                                <Dropdown.Menu onAction={(key: Key) => { if (key === 'edit') setEditing(site); if (key === 'delete') void remove(site); }}>
                                  <Dropdown.Item id="edit" textValue={t('站点设置')}><Label>{t('站点设置')}</Label></Dropdown.Item>
                                  <Dropdown.Item id="delete" textValue={t('删除')} variant="danger"><Label>{t('删除')}</Label></Dropdown.Item>
                                </Dropdown.Menu>
                              </Dropdown.Popover>
                            </Dropdown>
                          )}
                        </span>
                      )}
                    </Table.Cell>
                  </Table.Row>
                ))}
              </Table.Body>
            </Table.Content>
          </Table.ScrollContainer>
        </Table>
      )}
      {creating && <CreateDialog onClose={() => setCreating(false)} />}
      {editing && <SettingsDialog site={editing} onClose={() => setEditing(null)} onSaved={() => { setEditing(null); sites.reload(); }} />}
    </div>
  );
}

function CreateDialog({ onClose }: { onClose: () => void }) {
  const [slug, setSlug] = useState('');
  const [name, setName] = useState('');
  const [language, setLanguage] = useState(adminLanguage());
  const { busy, run } = useAction();
  const submit = async () => {
    const created = await run(async () => {
      const { site } = await api<{ site: Site }>('/v1/sites', { method: 'POST', body: { slug, name, baseLanguage: language } });
      // 之后的请求都落在新站点里：初始化站点配置，再把基准语言登记成默认语言，公开页才能渲染。
      selectSite({ siteId: site.siteId, grants: site.grants });
      await api('/v1/initialization', { method: 'POST', body: { name, baseLanguage: language, publicBaseURL: `${location.protocol}//${site.primaryHost}${location.port && !site.primaryHost.includes(':') ? `:${location.port}` : ''}` } });
      const key = newKey();
      await api(`/v1/languages/${encodeURIComponent(language)}`, {
        method: 'PUT', idempotencyKey: key, body: {
          expectedVersion: 0, idempotencyKey: key,
          language: { tag: language, version: 1, enabled: true, isDefault: true, automaticTranslation: false, direction: findSiteLanguage(language)?.direction ?? 'ltr', reflectionEnabled: false, batchOutputTokens: 1000, translationMemoryMinimumCharacters: 20 },
        },
      });
      return site;
    }, t('站点已创建'));
    if (created) navigate('/admin/dashboard');
  };
  return (
    <Modal.Backdrop isOpen onOpenChange={(open) => { if (!open) onClose(); }}>
      <Modal.Container size="md">
        <Modal.Dialog>
          <Modal.CloseTrigger aria-label={t('关闭')} />
          <Modal.Header><Modal.Heading>{t('新建站点')}</Modal.Heading></Modal.Header>
          <Form className="flex flex-col" onSubmit={(event) => { event.preventDefault(); void submit(); }}>
            <Modal.Body className="flex flex-col gap-4">
              <TextInput isRequired autoFocus label={t('站点名称')} maxLength={100} placeholder={t('例如：产品文档')} value={name} onChange={setName} />
              <TextInput isRequired label={t('子域名')} maxLength={63} minLength={3} pattern="[a-z0-9]([a-z0-9-]*[a-z0-9])?" placeholder="my-docs" value={slug} onChange={(value) => setSlug(value.toLowerCase())} description={t('3–63 位小写字母、数字或连字符，创建后不能改。')} />
              <SelectInput label={t('基准语言')} value={language} onChange={setLanguage} options={siteLanguages.map((item) => ({ value: item.tag, label: item.native }))} description={t('文档按这个语言写作，其他语言通过翻译生成；创建后不能更改。')} />
            </Modal.Body>
            <Modal.Footer>
              <Button slot="close" variant="secondary" onPress={onClose}>{t('取消')}</Button>
              <Button isPending={busy} type="submit">{t('创建')}</Button>
            </Modal.Footer>
          </Form>
        </Modal.Dialog>
      </Modal.Container>
    </Modal.Backdrop>
  );
}

function SettingsDialog({ site, onClose, onSaved }: { site: Site; onClose: () => void; onSaved: () => void }) {
  const [name, setName] = useState(site.name);
  const [slug, setSlug] = useState(site.slug);
  const [isPrivate, setPrivate] = useState(site.visibility === 'private');
  const [daily, setDaily] = useState(site.visitorQaDaily);
  const { busy, run } = useAction();
  const submit = async () => {
    const renamed = slug.trim().toLowerCase() !== site.slug;
    const done = await run(async () => {
      await api(`/v1/sites/${site.siteId}`, { method: 'PATCH', body: { name, visibility: isPrivate ? 'private' : 'public', visitorQaDaily: daily, ...(renamed ? { slug: slug.trim().toLowerCase() } : {}) } });
      if (renamed) await syncPublicAddress(site.siteId);
    }, t('站点设置已保存'));
    if (done !== undefined) onSaved();
  };
  return (
    <Modal.Backdrop isOpen onOpenChange={(open) => { if (!open) onClose(); }}>
      <Modal.Container size="md">
        <Modal.Dialog>
          <Modal.CloseTrigger aria-label={t('关闭')} />
          <Modal.Header><Modal.Heading>{t('站点设置')}</Modal.Heading></Modal.Header>
          <Form className="flex flex-col" onSubmit={(event) => { event.preventDefault(); void submit(); }}>
            <Modal.Body className="flex flex-col gap-4">
              <TextInput isRequired label={t('站点名称')} maxLength={100} value={name} onChange={setName} />
              <TextInput isRequired isDisabled={!!site.renameAfter} label={t('站点地址名')} maxLength={63} value={slug} onChange={setSlug}
                description={site.renameAfter ? t('{0} 之后才能再改。', fmtDateTime(site.renameAfter)) : t('子域名的前缀。30 天内只能改一次；旧地址保留 30 天并跳转到新地址，之后放出给别人用。')} />
              <SwitchInput isSelected={isPrivate} label={t('私有站点')} description={t('只有站点成员、知道访问密码或拿到分享链接的人能看到公开页（密码和链接在“团队与域名”里设置）。')} onChange={setPrivate} />
              <FormGrid columns={1}>
                <NumberInput label={t('访客每日提问上限')} minValue={0} maxValue={100000} value={daily} onChange={setDaily} description={t('达到后当天暂停访客提问，0 表示不限。花的是你的 AI 额度或你自己的 key。')} />
              </FormGrid>
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
