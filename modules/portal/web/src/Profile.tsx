import { Display, Key as KeyIcon } from '@gravity-ui/icons';
import { Button, Chip, Form, Table } from '@heroui/react';
import { api, ApiError, confirmAction, currentSessionId, Details, ErrorBlock, fmtDateTime, fmtRelative, LoadingBlock, Notice, PageHeader, Panel, storeSession, TextInput, useAction, useGrants, useLoad } from '@ui';
import { useState } from 'react';
import { useAccount } from './AdminLayout';

type Session = { sessionId: string; issuedAt: string; absoluteExpiresAt: string; deviceLabel: string; restrictedToPasswordChange: boolean; revokedAt: string | null };

const groupLabels: Record<string, string> = {
  edit_documents: '编辑文档', publish_documents: '发布与删除', manage_translations: '管理翻译', manage_languages: '管理语言',
  manage_resources: '管理资源', manage_qa: '问答管理', manage_ai: 'AI 与防滥用', manage_memory: '长期记忆', manage_site: '站点设置', view_stats: '查看统计',
};

export default function Profile() {
  const account = useAccount();
  const grants = useGrants();
  const [oldPassword, setOldPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmation, setConfirmation] = useState('');
  const [error, setError] = useState('');
  const [done, setDone] = useState(false);
  const { busy, run } = useAction();
  const sessions = useLoad((signal) => api<{ sessions: Session[] }>('/v1/sessions?limit=100', { signal }).then((result) => result.sessions), []);

  async function changePassword() {
    setError('');
    if (newPassword !== confirmation) { setError('两次输入的新口令不一致。'); return; }
    if (newPassword === oldPassword) { setError('新口令不能与旧口令相同。'); return; }
    try {
      const self = await api<{ credentialRevision: number }>('/v1/self');
      const changed = await api<{ session?: { csrfToken: string; sessionId: string; auth?: { grants?: string[] } } }>('/v1/self/password:change', {
        method: 'POST', body: { oldPassword, newPassword, confirmation, expectedCredentialRevision: self.credentialRevision },
      });
      // 改口令换了新会话（带完整授权），先存下来，否则重载后还是改口令前的受限授权。
      if (changed.session) storeSession(changed.session);
      setDone(true);
      // 其他设备的会话随之失效；整页重载以取回新的授权和导航。
      setTimeout(() => location.assign('/admin/dashboard'), 1200);
    } catch (problem) {
      setError(problem instanceof ApiError && problem.status === 400 ? '口令不符合要求：旧口令不对，或新口令太简单、太短。' : (problem as Error).message);
    }
  }

  const revoke = async (session: Session) => {
    const ok = await confirmAction({ title: '让这个设备退出登录？', body: `${session.deviceLabel || '未知设备'}（登录于 ${fmtDateTime(session.issuedAt)}）将需要重新登录。`, confirmLabel: '退出该设备', tone: 'warning' });
    if (!ok) return;
    await run(() => api(`/v1/sessions/${session.sessionId}`, { method: 'DELETE', body: {} }), '已让该设备退出');
    sessions.reload();
  };

  const self = currentSessionId();
  // 访问期限过了仍可续期，只有撤销或超过绝对期限才算失效；当前设备排最前，其余按登录时间倒序。
  const now = new Date().toISOString();
  const active = (sessions.data ?? [])
    .filter((session) => !session.revokedAt && session.absoluteExpiresAt > now)
    .sort((a, b) => Number(b.sessionId === self) - Number(a.sessionId === self) || b.issuedAt.localeCompare(a.issuedAt));

  return (
    <div className="flex flex-col gap-6">
      <PageHeader title="个人设置" description="管理自己的口令和登录设备。" />
      {account.mustChangePassword && (
        <Notice status="warning" title="请先修改初始口令">这是超级管理员为你设置的初始口令，修改之后才能使用后台的其他功能。</Notice>
      )}
      <div className="grid gap-6 xl:grid-cols-3">
        <Panel className="xl:col-span-1" title="账号">
          <Details items={[
            ['账号名', account.accountName],
            ['身份', account.isSuperadmin ? '超级管理员' : '子账号'],
            ['功能权限', account.isSuperadmin ? '全部' : (
              <span className="flex flex-wrap gap-1">
                {account.permissions.length ? account.permissions.map((key) => <Chip key={key} size="sm" variant="soft">{groupLabels[key] ?? key}</Chip>) : '无'}
              </span>
            )],
            ['技术授权', <span className="text-xs text-muted">{grants.length} 项</span>],
          ]} />
        </Panel>

        <Panel className="xl:col-span-2" title="修改口令" description="至少 8 位。修改后其他设备上的登录会失效。">
          {done ? (
            <Notice status="success" title="口令已修改">正在重新进入后台…</Notice>
          ) : (
            <Form className="flex max-w-md flex-col gap-4" onSubmit={(event) => { event.preventDefault(); void run(changePassword); }}>
              <TextInput isRequired autoComplete="current-password" label="旧口令" type="password" value={oldPassword} onChange={setOldPassword} />
              <TextInput isRequired autoComplete="new-password" label="新口令" minLength={8} type="password" value={newPassword} onChange={setNewPassword} />
              <TextInput isRequired autoComplete="new-password" error={confirmation && confirmation !== newPassword ? '两次输入的新口令不一致' : null} label="确认新口令" minLength={8} type="password" value={confirmation} onChange={setConfirmation} />
              {error && <Notice status="danger">{error}</Notice>}
              <div><Button isPending={busy} type="submit"><KeyIcon />修改口令</Button></div>
            </Form>
          )}
        </Panel>
      </div>

      <Panel title="登录设备" description="当前仍有效的登录。看到不认识的设备时，立即让它退出并修改口令。">
        {sessions.error ? <ErrorBlock error={sessions.error} onRetry={sessions.reload} /> : sessions.loading && !sessions.data ? <LoadingBlock lines={3} /> : (
          <Table variant="secondary">
            <Table.ScrollContainer>
              <Table.Content aria-label="登录设备" className="min-w-[560px]">
                <Table.Header>
                  <Table.Column isRowHeader>设备</Table.Column>
                  <Table.Column>登录时间</Table.Column>
                  <Table.Column>有效期至</Table.Column>
                  <Table.Column className="text-end">操作</Table.Column>
                </Table.Header>
                <Table.Body renderEmptyState={() => <div className="py-6 text-center text-sm text-muted">没有有效的登录。</div>}>
                  {active.map((session) => (
                    <Table.Row key={session.sessionId} id={session.sessionId}>
                      <Table.Cell>
                        <span className="flex items-center gap-2">
                          <Display className="size-4 text-muted" />
                          <span className="truncate">{session.deviceLabel || '未知设备'}</span>
                          {session.sessionId === self && <Chip color="accent" size="sm" variant="soft">当前</Chip>}
                        </span>
                      </Table.Cell>
                      <Table.Cell>{fmtRelative(session.issuedAt)}</Table.Cell>
                      <Table.Cell>{fmtDateTime(session.absoluteExpiresAt)}</Table.Cell>
                      <Table.Cell className="text-end">
                        {session.sessionId !== self && <Button isDisabled={busy} size="sm" variant="tertiary" onPress={() => revoke(session)}>退出</Button>}
                      </Table.Cell>
                    </Table.Row>
                  ))}
                </Table.Body>
              </Table.Content>
            </Table.ScrollContainer>
          </Table>
        )}
      </Panel>
    </div>
  );
}
