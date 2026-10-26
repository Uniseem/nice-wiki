import { I18nProvider, RouterProvider, Toast } from '@heroui/react';
import { adminLanguage, api, ApiError, CenteredSpinner, currentGrants, currentSessionId, currentSite, DialogHost, errorText, findSiteLanguage, navigate, newKey, notify, onUnauthorized, refresh, selectSite, storeSession, t } from '@ui';
import { useCallback, useEffect, useState } from 'react';
import { AdminLayout } from './AdminLayout';
import { AuthScreen } from './AuthScreen';

export type Account = {
  accountId: string;
  accountName: string;
  isSuperadmin: boolean;
  permissions: string[];
  mustChangePassword: boolean;
  status: string;
  siteId?: string;
};

// auth 阶段带 pending：已登录但站点还没初始化，直接进初始化向导。
type Phase = { kind: 'loading' } | { kind: 'auth'; pending?: Account } | { kind: 'app'; account: Account };

// React Aria 自带文字（日期选择等）按地区写法认语言。
const ariaLocale: Record<string, string> = { 'zh-Hans': 'zh-CN', 'zh-Hant': 'zh-TW', pt: 'pt-BR' };

// 站点初始化只写了站点配置；基准语言要在本地化域登记成默认语言，公开页才能渲染。已登录时补一次，幂等。
export async function ensureBaseLanguage(baseLanguage?: string): Promise<void> {
  try {
    const languages = await api<{ languages: { tag: string }[] }>('/v1/languages?includeDisabled=true');
    const tag = baseLanguage ?? (await api<{ config: { baseLanguage: string } }>('/v1/config')).config.baseLanguage;
    if (languages.languages.some((language) => language.tag === tag) || (!baseLanguage && languages.languages.length)) return;
    const idempotencyKey = newKey();
    await api(`/v1/languages/${encodeURIComponent(tag)}`, {
      method: 'PUT', idempotencyKey, body: {
        expectedVersion: 0, idempotencyKey,
        language: { tag, version: 1, enabled: true, isDefault: true, automaticTranslation: false, direction: findSiteLanguage(tag)?.direction ?? 'ltr', reflectionEnabled: false, batchOutputTokens: 1000, translationMemoryMinimumCharacters: 20 },
      },
    });
  } catch (error) { console.warn('基准语言登记失败', error); }
}

export function App() {
  const [phase, setPhase] = useState<Phase>({ kind: 'loading' });

  const enter = useCallback(async (account: Account) => {
    const home = !!account.siteId?.startsWith('home:');
    if (!home) {
      // 站主账号默认进入平台自己的站点。
      if (!currentSite() && account.siteId) {
        try {
          const mine = await api<{ sites: { siteId: string; grants: string[] }[] }>('/v1/sites');
          const own = mine.sites.find((site) => site.siteId === account.siteId);
          if (own) selectSite({ siteId: own.siteId, grants: own.grants });
        } catch (problem) { console.warn('读取站点失败', problem); }
      }
      await ensureBaseLanguage();
    }
    setPhase({ kind: 'app', account });
    if (['/admin', '/admin/', '/admin/login', '/admin/setup'].includes(location.pathname)) navigate(currentSite() ? '/admin/dashboard' : '/admin/sites', true);
  }, []);

  useEffect(() => {
    void (async () => {
      try {
        // 第三方登录回来：地址是 /admin/oauth/<提供方>?code=…&state=…，换成会话再进后台。
        const callback = location.pathname.match(/^\/admin\/oauth\/([a-z]+)$/);
        if (callback) {
          const query = new URLSearchParams(location.search);
          const result = await api<{ session: Parameters<typeof storeSession>[0]; account?: Account }>(`/v1/oauth/${callback[1]}:login`, { method: 'POST', body: { code: query.get('code') ?? '', state: query.get('state') ?? '', redirectUri: `${location.origin}/admin/oauth/${callback[1]}` } });
          storeSession(result.session);
          history.replaceState(null, '', '/admin');
        }
        const self = await api<{ account: Account }>('/v1/self');
        // 授权项只在登录/刷新响应里；本地还没有（比如从旧版本升级上来）时静默刷新一次补上。
        if (!currentGrants().length) await refresh();
        // 普通用户的家账户没有站点初始化这一步；站主账号才要看平台自己的站点是否初始化过。
        if (!self.account.siteId?.startsWith('home:')) {
          const init = await api<{ initialized: boolean }>('/v1/initialization');
          if (!init.initialized) { setPhase({ kind: 'auth', pending: self.account }); return; }
        }
        await enter(self.account);
      } catch (error) {
        if (!(error instanceof ApiError) || error.status !== 401) console.error(error);
        setPhase({ kind: 'auth' });
      }
    })();
  }, [enter]);

  useEffect(() => onUnauthorized(() => setPhase({ kind: 'auth' })), []);

  const signOut = useCallback(async () => {
    const id = currentSessionId();
    if (id) {
      try { await api(`/v1/sessions/${id}`, { method: 'DELETE', body: {} }); } catch (problem) {
        // 会话本来就失效了才算已退出；断网、校验失败时服务端会话还有效，不能显示成已退出。
        if (!(problem instanceof ApiError && problem.status === 401)) {
          notify.error(t('退出失败'), t('{0}。登录仍然有效，请稍后再试。', errorText(problem)));
          return;
        }
      }
    }
    storeSession(null);
    setPhase({ kind: 'auth' });
    navigate('/admin/login', true);
  }, []);

  return (
    <RouterProvider navigate={(path) => navigate(path)}>
      <I18nProvider locale={ariaLocale[adminLanguage()] ?? adminLanguage()}>
        <Toast.Provider placement="top" />
        <DialogHost />
        {phase.kind === 'loading' && <div className="flex min-h-dvh items-center justify-center"><CenteredSpinner /></div>}
        {phase.kind === 'auth' && <AuthScreen pending={phase.pending} onSignedIn={enter} />}
        {phase.kind === 'app' && <AdminLayout account={phase.account} onAccountChange={(account) => setPhase({ kind: 'app', account })} onSignOut={signOut} />}
      </I18nProvider>
    </RouterProvider>
  );
}
