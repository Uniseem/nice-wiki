import { Ellipsis, Persons, Plus } from '@gravity-ui/icons';
import { Button, Chip, Dropdown, Form, Label, Modal, Table } from '@heroui/react';
import type { Key } from '@heroui/react';
import { api, askText, CheckGroupInput, confirmAction, EmptyBlock, ErrorBlock, fmtDateTime, LoadingBlock, PageHeader, TextInput, useAction, useLoad, t } from '@ui';
import { useState } from 'react';

type Account = { accountId: string; accountName: string; isSuperadmin: boolean; permissions: string[]; status: string; revision: number; mustChangePassword: boolean; createdAt?: string };

// 功能权限只有这 10 项开关；技术授权由 identity 按它们派生，浏览器不能自选。
const groups = [
  { value: 'edit_documents', label: t('编辑文档'), description: t('新建、编辑文档和上传资源') },
  { value: 'publish_documents', label: t('发布与删除'), description: t('发布、撤回、回滚、移入回收站') },
  { value: 'manage_translations', label: t('管理翻译'), description: t('发起和取消文档翻译') },
  { value: 'manage_languages', label: t('管理语言'), description: t('添加、启用和停用语言') },
  { value: 'manage_resources', label: t('管理资源'), description: t('资源库与清理') },
  { value: 'manage_qa', label: t('问答管理'), description: t('问答策略、记录和精选缓存') },
  { value: 'manage_ai', label: t('AI 与防滥用'), description: t('知识库索引、调试台、防滥用策略') },
  { value: 'manage_memory', label: t('长期记忆'), description: t('维护给 AI 的长期背景') },
  { value: 'manage_site', label: t('站点设置'), description: t('站名、首页、横幅和页脚') },
  { value: 'view_stats', label: t('查看统计'), description: t('访问统计与管理审计') },
] as const;
const labelOf = (key: string) => groups.find((group) => group.value === key)?.label ?? key;
const statusText: Record<string, { label: string; tone: 'success' | 'default' | 'danger' }> = { active: { label: t('正常'), tone: 'success' }, disabled: { label: t('已停用'), tone: 'default' }, deleted: { label: t('已删除'), tone: 'danger' } };
const passwordRule = (value: string) => value.length >= 8 ? null : t('至少 8 位');

export default function AccountsPage() {
  const accounts = useLoad((signal) => api<{ accounts: Account[] }>('/v1/accounts', { signal }).then((result) => result.accounts), []);
  const [editing, setEditing] = useState<Account | 'new' | null>(null);
  const { busy, run } = useAction();

  const setStatus = async (account: Account, status: 'active' | 'disabled' | 'deleted') => {
    if (status !== 'active') {
      const ok = await confirmAction(status === 'deleted'
        ? { title: t('删除账号“{0}”？', account.accountName), body: t('删除后该账号无法登录，账号名也不能再用于新账号。'), confirmLabel: t('删除'), tone: 'danger' }
        : { title: t('停用账号“{0}”？', account.accountName), body: t('停用后该账号所有设备立即退出，之后可以恢复。'), confirmLabel: t('停用'), tone: 'warning' });
      if (!ok) return;
    }
    await run(() => api(`/v1/accounts/${account.accountId}/status`, { method: 'PUT', body: { status, expectedRevision: account.revision } }), status === 'active' ? t('已恢复') : status === 'disabled' ? t('已停用') : t('已删除'));
    accounts.reload();
  };

  const resetPassword = async (account: Account) => {
    const value = await askText({ title: t('重置“{0}”的口令', account.accountName), label: t('新的初始口令'), type: 'password', description: t('对方下次登录必须先修改口令。'), confirmLabel: t('重置'), validate: passwordRule });
    if (!value) return;
    await run(() => api(`/v1/accounts/${account.accountId}/password:reset`, { method: 'POST', body: { initialPassword: value, expectedRevision: account.revision } }), t('口令已重置，对方下次登录需要修改'));
    accounts.reload();
  };

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title={t('账号与权限')}
        description={t('超级管理员可以建子账号并勾选它能用的功能。子账号首次登录必须先改口令。')}
        actions={<Button onPress={() => setEditing('new')}><Plus />{t('新建子账号')}</Button>}
      />
      {accounts.error ? <ErrorBlock error={accounts.error} onRetry={accounts.reload} /> : !accounts.data ? <LoadingBlock lines={5} /> : (
        <Table>
          <Table.ScrollContainer>
            <Table.Content aria-label={t('账号列表')} className="min-w-[640px]">
              <Table.Header>
                <Table.Column isRowHeader>{t('账号')}</Table.Column>
                <Table.Column>{t('状态')}</Table.Column>
                <Table.Column>{t('功能权限')}</Table.Column>
                <Table.Column className="w-16 text-end">{t('操作')}</Table.Column>
              </Table.Header>
              <Table.Body renderEmptyState={() => <EmptyBlock className="m-4" icon={<Persons />} title={t('还没有子账号')} />}>
                {accounts.data.filter((account) => account.status !== 'deleted').map((account) => (
                  <Table.Row key={account.accountId} id={account.accountId}>
                    <Table.Cell>
                      <span className="flex items-center gap-3">
                        <span className="flex size-8 shrink-0 items-center justify-center rounded-full bg-default text-sm font-semibold">{account.accountName.slice(0, 1).toUpperCase()}</span>
                        <span>
                          <span className="block font-medium text-foreground">{account.accountName}</span>
                          <span className="block text-xs text-muted">{account.isSuperadmin ? t('超级管理员') : account.mustChangePassword ? t('待首次修改口令') : account.createdAt ? t('创建于 {0}', fmtDateTime(account.createdAt)) : t('子账号')}</span>
                        </span>
                      </span>
                    </Table.Cell>
                    <Table.Cell><Chip color={statusText[account.status]?.tone ?? 'default'} size="sm" variant="soft">{statusText[account.status]?.label ?? account.status}</Chip></Table.Cell>
                    <Table.Cell>
                      {account.isSuperadmin ? <span className="text-sm text-muted">{t('全部权限')}</span> : account.permissions.length ? (
                        <span className="flex flex-wrap gap-1">{account.permissions.map((key) => <Chip key={key} size="sm" variant="soft">{labelOf(key)}</Chip>)}</span>
                      ) : <span className="text-sm text-muted">{t('无')}</span>}
                    </Table.Cell>
                    <Table.Cell className="text-end">
                      {!account.isSuperadmin && (
                        <Dropdown>
                          <Button isIconOnly aria-label={t('账号操作')} isDisabled={busy} size="sm" variant="ghost"><Ellipsis /></Button>
                          <Dropdown.Popover placement="bottom end">
                            <Dropdown.Menu onAction={(key: Key) => {
                              if (key === 'permissions') setEditing(account);
                              if (key === 'reset') void resetPassword(account);
                              if (key === 'disable') void setStatus(account, 'disabled');
                              if (key === 'enable') void setStatus(account, 'active');
                              if (key === 'delete') void setStatus(account, 'deleted');
                            }}>
                              <Dropdown.Item id="permissions" textValue={t('编辑权限')}><Label>{t('编辑权限')}</Label></Dropdown.Item>
                              <Dropdown.Item id="reset" textValue={t('重置口令')}><Label>{t('重置口令')}</Label></Dropdown.Item>
                              {account.status === 'active'
                                ? <Dropdown.Item key="disable" id="disable" textValue={t('停用')}><Label>{t('停用')}</Label></Dropdown.Item>
                                : <Dropdown.Item key="enable" id="enable" textValue={t('恢复')}><Label>{t('恢复')}</Label></Dropdown.Item>}
                              <Dropdown.Item id="delete" textValue={t('删除')} variant="danger"><Label>{t('删除')}</Label></Dropdown.Item>
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
      {editing && <AccountDialog account={editing === 'new' ? null : editing} onClose={() => setEditing(null)} onSaved={() => { setEditing(null); accounts.reload(); }} />}
    </div>
  );
}

function AccountDialog({ account, onClose, onSaved }: { account: Account | null; onClose: () => void; onSaved: () => void }) {
  const [name, setName] = useState('');
  const [password, setPassword] = useState('');
  const [chosen, setChosen] = useState<string[]>(account ? account.permissions : groups.map((group) => group.value));
  const { busy, run } = useAction();
  const submit = async () => {
    const done = await run(async () => {
      if (account) await api(`/v1/accounts/${account.accountId}/permissions`, { method: 'PUT', body: { permissions: chosen, expectedRevision: account.revision } });
      else await api('/v1/accounts', { method: 'POST', body: { accountName: name, initialPassword: password, permissions: chosen } });
      return true;
    }, account ? t('权限已更新') : t('子账号已创建，对方首次登录需要修改口令'));
    if (done) onSaved();
  };
  return (
    <Modal.Backdrop isOpen onOpenChange={(open) => { if (!open) onClose(); }}>
      <Modal.Container size="md">
        <Modal.Dialog>
          <Modal.CloseTrigger aria-label={t('关闭')} />
          <Modal.Header><Modal.Heading>{account ? t('编辑“{0}”的权限', account.accountName) : t('新建子账号')}</Modal.Heading></Modal.Header>
          <Form className="flex flex-col" onSubmit={(event) => { event.preventDefault(); void submit(); }}>
            <Modal.Body className="flex flex-col gap-5">
              {!account && (
                <>
                  <TextInput isRequired autoComplete="off" description={t('创建后不可修改。')} label={t('账号名')} maxLength={64} value={name} onChange={setName} />
                  <TextInput isRequired autoComplete="new-password" description={t('至少 8 位，对方首次登录后必须修改。')} label={t('初始口令')} minLength={8} type="password" validate={passwordRule} value={password} onChange={setPassword} />
                </>
              )}
              <CheckGroupInput label={t('功能权限')} options={groups.map((group) => ({ value: group.value, label: group.label, description: group.description }))} value={chosen} onChange={setChosen} />
            </Modal.Body>
            <Modal.Footer>
              <Button variant="tertiary" onPress={onClose}>{t('取消')}</Button>
              <Button isPending={busy} type="submit">{account ? t('保存') : t('创建')}</Button>
            </Modal.Footer>
          </Form>
        </Modal.Dialog>
      </Modal.Container>
    </Modal.Backdrop>
  );
}
