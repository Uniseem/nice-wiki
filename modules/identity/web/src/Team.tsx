import { Ellipsis, Plus } from '@gravity-ui/icons';
import { Button, Chip, Dropdown, Form, Label, Modal, Table, Tabs } from '@heroui/react';
import type { Key } from '@heroui/react';
import { api, confirmAction, EmptyBlock, ErrorBlock, fmtDateTime, LoadingBlock, Mono, Notice, PageHeader, SelectInput, TextInput, useAction, useCurrentSite, useLoad, t } from '@ui';
import { useState } from 'react';

type Member = { userId: string; displayName: string; email: string; role: 'owner' | 'editor' | 'viewer'; createdAt: string };
type Domain = { host: string; kind: 'subdomain' | 'custom'; status: 'pending' | 'verified' | 'failed'; primary: boolean; verifyName: string; verifyToken: string; verifiedAt?: string; createdAt: string };

const roleText = { owner: t('所有者'), editor: t('编辑'), viewer: t('只读') };
const roleOptions = [{ value: 'editor' as const, label: t('编辑'), description: t('可以写文档、发布、翻译、管理资源') }, { value: 'viewer' as const, label: t('只读'), description: t('只能查看后台内容，不能修改') }];
const domainStatus = { pending: { label: t('待验证'), tone: 'warning' as const }, verified: { label: t('已验证'), tone: 'success' as const }, failed: { label: t('验证失败'), tone: 'danger' as const } };

export default function TeamPage() {
  const site = useCurrentSite();
  const [tab, setTab] = useState('members');
  if (!site) return <EmptyBlock title={t('先选一个站点')} description={t('在左上角切换到要管理的站点。')} />;
  return (
    <div className="flex flex-col gap-6">
      <PageHeader title={t('团队与域名')} description={t('邀请协作者、设置他们的角色；为站点绑定自己的域名。')} />
      <Tabs selectedKey={tab} onSelectionChange={(key) => setTab(String(key))}>
        <Tabs.ListContainer className="mb-4 overflow-x-auto">
          <Tabs.List aria-label={t('团队与域名')}>
            <Tabs.Tab id="members">{t('成员')}<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="domains">{t('域名')}<Tabs.Indicator /></Tabs.Tab>
          </Tabs.List>
        </Tabs.ListContainer>
        <Tabs.Panel id="members"><Members siteId={site} /></Tabs.Panel>
        <Tabs.Panel id="domains"><Domains siteId={site} /></Tabs.Panel>
      </Tabs>
    </div>
  );
}

function Members({ siteId }: { siteId: string }) {
  const members = useLoad((signal) => api<{ members: Member[] }>(`/v1/sites/${siteId}/members`, { signal }).then((result) => result.members), [siteId]);
  const [adding, setAdding] = useState(false);
  const { busy, run } = useAction();
  const change = async (member: Member, role: 'editor' | 'viewer') => {
    await run(() => api(`/v1/sites/${siteId}/members/${member.userId}`, { method: 'PUT', body: { role } }), t('角色已更新'));
    members.reload();
  };
  const remove = async (member: Member) => {
    const ok = await confirmAction({ title: t('移除成员“{0}”？', member.displayName || member.email), body: t('对方立即失去这个站点的后台权限。'), confirmLabel: t('移除'), tone: 'danger' });
    if (!ok) return;
    await run(() => api(`/v1/sites/${siteId}/members/${member.userId}`, { method: 'DELETE' }), t('已移除'));
    members.reload();
  };
  return (
    <div className="flex flex-col gap-4">
      <div className="flex justify-end"><Button onPress={() => setAdding(true)}><Plus />{t('添加成员')}</Button></div>
      {members.error ? <ErrorBlock error={members.error} onRetry={members.reload} /> : !members.data ? <LoadingBlock lines={3} /> : (
        <Table>
          <Table.ScrollContainer>
            <Table.Content aria-label={t('成员列表')} className="min-w-[560px]">
              <Table.Header>
                <Table.Column isRowHeader>{t('成员')}</Table.Column>
                <Table.Column>{t('角色')}</Table.Column>
                <Table.Column>{t('加入时间')}</Table.Column>
                <Table.Column className="w-16 text-end">{t('操作')}</Table.Column>
              </Table.Header>
              <Table.Body renderEmptyState={() => <EmptyBlock className="m-4" title={t('还没有成员')} />}>
                {members.data.map((member) => (
                  <Table.Row key={member.userId} id={member.userId}>
                    <Table.Cell><span className="block font-medium text-foreground">{member.displayName}</span><span className="block text-xs text-muted">{member.email}</span></Table.Cell>
                    <Table.Cell><Chip size="sm" variant="soft">{roleText[member.role]}</Chip></Table.Cell>
                    <Table.Cell>{fmtDateTime(member.createdAt)}</Table.Cell>
                    <Table.Cell className="text-end">
                      {member.role !== 'owner' && (
                        <Dropdown>
                          <Button isIconOnly aria-label={t('成员操作')} isDisabled={busy} size="sm" variant="ghost"><Ellipsis /></Button>
                          <Dropdown.Popover placement="bottom end">
                            <Dropdown.Menu onAction={(key: Key) => { if (key === 'editor' || key === 'viewer') void change(member, key); if (key === 'remove') void remove(member); }}>
                              <Dropdown.Item id="editor" textValue={t('设为编辑')}><Label>{t('设为编辑')}</Label></Dropdown.Item>
                              <Dropdown.Item id="viewer" textValue={t('设为只读')}><Label>{t('设为只读')}</Label></Dropdown.Item>
                              <Dropdown.Item id="remove" textValue={t('移除')} variant="danger"><Label>{t('移除')}</Label></Dropdown.Item>
                            </Dropdown.Menu>
                          </Dropdown.Popover>
                        </Dropdown>
                      )}
                    </Table.Cell>
                  </Table.Row>
                ))}
              </Table.Body>
            </Table.Content>
          </Table.ScrollContainer>
        </Table>
      )}
      {adding && <AddMemberDialog siteId={siteId} onClose={() => setAdding(false)} onSaved={() => { setAdding(false); members.reload(); }} />}
    </div>
  );
}

function AddMemberDialog({ siteId, onClose, onSaved }: { siteId: string; onClose: () => void; onSaved: () => void }) {
  const [email, setEmail] = useState('');
  const [role, setRole] = useState<'editor' | 'viewer'>('editor');
  const { busy, run } = useAction();
  const submit = async () => {
    const done = await run(() => api(`/v1/sites/${siteId}/members`, { method: 'POST', body: { email, role } }), t('已添加'));
    if (done !== undefined) onSaved();
  };
  return (
    <Modal.Backdrop isOpen onOpenChange={(open) => { if (!open) onClose(); }}>
      <Modal.Container size="md">
        <Modal.Dialog>
          <Modal.CloseTrigger aria-label={t('关闭')} />
          <Modal.Header><Modal.Heading>{t('添加成员')}</Modal.Heading></Modal.Header>
          <Form className="flex flex-col" onSubmit={(event) => { event.preventDefault(); void submit(); }}>
            <Modal.Body className="flex flex-col gap-4">
              <TextInput isRequired autoFocus label={t('对方的注册邮箱')} type="email" value={email} onChange={setEmail} description={t('对方需要先在平台注册账号。')} />
              <SelectInput label={t('角色')} value={role} onChange={setRole} options={roleOptions} />
            </Modal.Body>
            <Modal.Footer>
              <Button slot="close" variant="secondary" onPress={onClose}>{t('取消')}</Button>
              <Button isPending={busy} type="submit">{t('添加')}</Button>
            </Modal.Footer>
          </Form>
        </Modal.Dialog>
      </Modal.Container>
    </Modal.Backdrop>
  );
}

function Domains({ siteId }: { siteId: string }) {
  const domains = useLoad((signal) => api<{ domains: Domain[] }>(`/v1/sites/${siteId}/domains`, { signal }).then((result) => result.domains), [siteId]);
  const [host, setHost] = useState('');
  const { busy, run } = useAction();
  const action = async (domain: Domain, kind: 'verify' | 'primary' | 'remove') => {
    if (kind === 'remove') {
      const ok = await confirmAction({ title: t('移除域名“{0}”？', domain.host), body: t('移除后这个域名不再指向本站点。'), confirmLabel: t('移除'), tone: 'danger' });
      if (!ok) return;
      await run(() => api(`/v1/sites/${siteId}/domains/${encodeURIComponent(domain.host)}`, { method: 'DELETE' }), t('已移除'));
    } else {
      await run(() => api(`/v1/sites/${siteId}/domains/${encodeURIComponent(domain.host)}:${kind}`, { method: 'POST', body: {} }), kind === 'verify' ? t('验证完成') : t('已设为主域名'));
    }
    domains.reload();
  };
  const add = async () => {
    const done = await run(() => api(`/v1/sites/${siteId}/domains`, { method: 'POST', body: { host: host.trim().toLowerCase() } }), t('域名已添加，按下面的说明解析后点“验证”'));
    if (done !== undefined) { setHost(''); domains.reload(); }
  };
  const pending = domains.data?.filter((domain) => domain.kind === 'custom' && domain.status !== 'verified') ?? [];
  return (
    <div className="flex flex-col gap-4">
      <Form className="flex items-end gap-2" onSubmit={(event) => { event.preventDefault(); void add(); }}>
        <TextInput className="max-w-sm" label={t('绑定自己的域名')} placeholder="docs.example.com" value={host} onChange={setHost} />
        <Button isDisabled={!host.trim()} isPending={busy} type="submit"><Plus />{t('添加')}</Button>
      </Form>
      {pending.map((domain) => (
        <Notice key={domain.host} status="warning" title={t('验证 {0}', domain.host)}>
          <p className="text-sm leading-6">{t('在域名解析里添加一条 TXT 记录：')}</p>
          <p className="text-sm"><Mono>{domain.verifyName}</Mono> → <Mono>{domain.verifyToken}</Mono></p>
          <p className="text-sm leading-6">{t('并把域名 CNAME 指向平台地址。证书会在第一次访问时自动签发。')}</p>
        </Notice>
      ))}
      {domains.error ? <ErrorBlock error={domains.error} onRetry={domains.reload} /> : !domains.data ? <LoadingBlock lines={3} /> : (
        <Table>
          <Table.ScrollContainer>
            <Table.Content aria-label={t('域名列表')} className="min-w-[560px]">
              <Table.Header>
                <Table.Column isRowHeader>{t('域名')}</Table.Column>
                <Table.Column>{t('类型')}</Table.Column>
                <Table.Column>{t('状态')}</Table.Column>
                <Table.Column className="w-16 text-end">{t('操作')}</Table.Column>
              </Table.Header>
              <Table.Body renderEmptyState={() => <EmptyBlock className="m-4" title={t('还没有域名')} />}>
                {domains.data.map((domain) => (
                  <Table.Row key={domain.host} id={domain.host}>
                    <Table.Cell><span className="font-medium text-foreground">{domain.host}</span>{domain.primary && <Chip className="ms-2" color="accent" size="sm" variant="soft">{t('主域名')}</Chip>}</Table.Cell>
                    <Table.Cell>{domain.kind === 'subdomain' ? t('平台子域名') : t('自有域名')}</Table.Cell>
                    <Table.Cell><Chip color={domainStatus[domain.status].tone} size="sm" variant="soft">{domainStatus[domain.status].label}</Chip></Table.Cell>
                    <Table.Cell className="text-end">
                      <Dropdown>
                        <Button isIconOnly aria-label={t('域名操作')} isDisabled={busy} size="sm" variant="ghost"><Ellipsis /></Button>
                        <Dropdown.Popover placement="bottom end">
                          <Dropdown.Menu onAction={(key: Key) => void action(domain, key as 'verify' | 'primary' | 'remove')}>
                            {domain.status !== 'verified' ? <Dropdown.Item id="verify" textValue={t('验证')}><Label>{t('验证')}</Label></Dropdown.Item> : <Dropdown.Item id="primary" textValue={t('设为主域名')}><Label>{t('设为主域名')}</Label></Dropdown.Item>}
                            {domain.kind === 'custom' ? <Dropdown.Item id="remove" textValue={t('移除')} variant="danger"><Label>{t('移除')}</Label></Dropdown.Item> : null}
                          </Dropdown.Menu>
                        </Dropdown.Popover>
                      </Dropdown>
                    </Table.Cell>
                  </Table.Row>
                ))}
              </Table.Body>
            </Table.Content>
          </Table.ScrollContainer>
        </Table>
      )}
    </div>
  );
}
