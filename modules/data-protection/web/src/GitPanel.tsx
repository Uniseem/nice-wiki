import { ArrowDownToLine, ArrowUpFromLine } from '@gravity-ui/icons';
import { Button, Form } from '@heroui/react';
import { api, confirmAction, ErrorBlock, fmtRelative, LoadingBlock, navigate, Notice, notify, Panel, SwitchInput, TextInput, useAction, useCurrentSite, useLoad, t } from '@ui';
import { useEffect, useState } from 'react';

type Remote = { configured: boolean; url: string; branch: string; username: string; hasToken: boolean; pushOnPublish: boolean; lastPushAt: string; lastPullAt: string; lastStatus: string; lastMessage: string; lastCommit: string };
type Sync = { commit: string; changed: boolean; files: number; message: string; import?: { documents: number; failures: string[]; warnings: string[] } };

export function GitPanel() {
  const site = useCurrentSite();
  const remote = useLoad((signal) => api<{ remote: Remote }>('/v1/site-git', { signal }).then((result) => result.remote), [site]);
  const [url, setUrl] = useState('');
  const [branch, setBranch] = useState('main');
  const [username, setUsername] = useState('');
  const [token, setToken] = useState('');
  const [push, setPush] = useState(false);
  const [outcome, setOutcome] = useState<Sync | null>(null);
  const { busy, run } = useAction();
  const [working, setWorking] = useState<'push' | 'pull' | ''>('');
  useEffect(() => {
    if (!remote.data) return;
    setUrl(remote.data.url);
    setBranch(remote.data.branch || 'main');
    setUsername(remote.data.username);
    setPush(remote.data.pushOnPublish);
    setToken('');
  }, [remote.data]);
  const save = async () => {
    const done = await run(() => api('/v1/site-git', { method: 'PUT', body: { url, branch, username, token: token || undefined, pushOnPublish: push } }), t('已保存'));
    if (done !== undefined) remote.reload();
  };
  const sync = async (direction: 'push' | 'pull') => {
    if (direction === 'pull') {
      const ok = await confirmAction({ title: t('从仓库拉取？'), body: t('会把仓库里的文档导入成新的草稿，不会改动现有文档。重复拉取会产生重复文档。'), confirmLabel: t('拉取') });
      if (!ok) return;
    }
    setWorking(direction);
    setOutcome(null);
    try {
      const done = await run(() => api<Sync>(`/v1/site-git:${direction}`, { method: 'POST', body: {} }));
      if (done) { setOutcome(done); notify.success(done.message); }
    } finally { setWorking(''); remote.reload(); }
  };
  const remove = async () => {
    const ok = await confirmAction({ title: t('取消 Git 同步？'), body: t('只删除这里保存的配置和令牌，仓库里的内容不受影响。'), confirmLabel: t('取消同步'), tone: 'danger' });
    if (!ok) return;
    await run(() => api('/v1/site-git', { method: 'DELETE' }), t('已取消同步'));
    remote.reload();
  };
  if (remote.error) return <ErrorBlock error={remote.error} onRetry={remote.reload} />;
  if (!remote.data) return <LoadingBlock lines={4} />;
  const configured = remote.data.configured;
  return (
    <div className="flex flex-col gap-6">
      <Panel title={t('Git 仓库')} description={t('已发布的文档会以 Markdown 的形式推送到仓库（docs 目录），图片放在 media 目录；草稿不会推送。')}>
        <Form className="flex flex-col gap-4" onSubmit={(event) => { event.preventDefault(); void save(); }}>
          <TextInput isRequired label={t('仓库地址')} type="url" value={url} onChange={setUrl} placeholder="https://github.com/me/docs.git" description={t('必须是公网 https 地址，不要把账号口令写在地址里。')} />
          <div className="grid gap-4 sm:grid-cols-2">
            <TextInput label={t('分支')} value={branch} onChange={setBranch} placeholder="main" />
            <TextInput label={t('用户名')} value={username} onChange={setUsername} autoComplete="off" description={t('GitHub 等用令牌时可留空。')} />
          </div>
          <TextInput isRequired={!configured} label={t('访问令牌')} type="password" value={token} onChange={setToken} autoComplete="new-password" placeholder={remote.data.hasToken ? t('已保存，留空不修改') : ''} description={t('需要有推送权限（GitHub 的 Fine-grained token：Contents 读写）。加密保存，页面不再显示。')} />
          <SwitchInput label={t('发布后自动推送')} description={t('文档发布或调整后，约一分钟内自动推送到仓库。')} isSelected={push} onChange={setPush} />
          <div className="flex flex-wrap gap-2">
            <Button isPending={busy && !working} type="submit">{t('保存')}</Button>
            {configured && <Button isPending={working === 'push'} isDisabled={!!working} variant="secondary" onPress={() => void sync('push')}><ArrowUpFromLine />{t('立即推送')}</Button>}
            {configured && <Button isPending={working === 'pull'} isDisabled={!!working} variant="secondary" onPress={() => void sync('pull')}><ArrowDownToLine />{t('从仓库拉取')}</Button>}
            {configured && <Button isDisabled={!!working} variant="ghost" onPress={() => void remove()}>{t('取消同步')}</Button>}
          </div>
        </Form>
      </Panel>
      {configured && (remote.data.lastPushAt || remote.data.lastPullAt) && (
        <Panel title={t('同步状态')}>
          <div className="flex flex-col gap-1 text-sm">
            {remote.data.lastPushAt && <span>{t('最近推送：{0}', fmtRelative(remote.data.lastPushAt))}{remote.data.lastCommit && <span className="ms-2 font-mono text-xs text-muted">{remote.data.lastCommit.slice(0, 8)}</span>}</span>}
            {remote.data.lastPullAt && <span>{t('最近拉取：{0}', fmtRelative(remote.data.lastPullAt))}</span>}
            {remote.data.lastMessage && <span className={remote.data.lastStatus === 'failed' ? 'text-danger' : 'text-muted'}>{remote.data.lastMessage}</span>}
          </div>
        </Panel>
      )}
      {outcome?.import && (
        <Notice status={outcome.import.failures.length ? 'warning' : 'success'} title={t('已拉取并导入 {0} 篇草稿', outcome.import.documents)}>
          <button className="text-accent underline" type="button" onClick={() => navigate('/admin/content')}>{t('去文档管理查看')}</button>
          {outcome.import.failures.length > 0 && <ul className="mt-2 list-disc ps-5">{outcome.import.failures.map((line) => <li key={line}>{line}</li>)}</ul>}
        </Notice>
      )}
    </div>
  );
}
