import { api, CenteredSpinner, ErrorBlock, errorText, t } from '@ui';
import { useEffect, useState } from 'react';

// 私有站点的入口：站点域名把访客带到平台域名确认身份，这里取一次性码，再带回站点域名换取该站点专用的短期访问。
export default function PrivateAccessPage() {
  const [error, setError] = useState<unknown>(null);
  useEffect(() => {
    const query = new URLSearchParams(location.search);
    const site = query.get('site') ?? '';
    const back = query.get('return') ?? '/';
    if (!site) { setError(new Error(t('缺少站点'))); return; }
    api<{ code: string; host: string }>(`/v1/sites/${encodeURIComponent(site)}/private-access`, { method: 'POST', body: {} })
      .then((result) => location.assign(`${location.protocol}//${result.host}/_pv/accept?code=${encodeURIComponent(result.code)}&return=${encodeURIComponent(back)}`))
      .catch(setError);
  }, []);
  if (error) return <ErrorBlock error={error} title={t('不能进入这个私有站点')} onRetry={() => location.reload()} />;
  return <CenteredSpinner label={t('正在进入私有站点…')} />;
}
