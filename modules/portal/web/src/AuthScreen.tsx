import { ArrowRight, Globe, Lock, Person } from '@gravity-ui/icons';
import { Button, Card, Form, Spinner } from '@heroui/react';
import { adminLanguage, api, ApiError, chosenAdminLanguage, errorText, findSiteLanguage, LanguageMenu, newKey, Notice, setAdminLanguage, siteLanguages, storeSession, TextInput, ThemeMenu, t } from '@ui';
import { solveChallenge } from '@ui/pow';
import { useEffect, useState, type ReactNode } from 'react';
import { ensureBaseLanguage, type Account } from './App';
import { SiteMark, useSiteBrand } from './brand';

// 进入后台的向导。首次进站：选语言 → 注册唯一的超级管理员 → 填站点名完成初始化；所选语言既是站点基准语言，也是后台界面语言。
// 已有账号 → 登录；登录后站点还没初始化 → 继续选语言和填站点名。
type Step = 'loading' | 'language' | 'register' | 'login' | 'signup' | 'site';
type Options = { registrationOpen: boolean; emailRegistration: boolean; inviteRequired: boolean; providers: { provider: string; label: string }[] };
type SessionReply = { session: { csrfToken: string; sessionId: string; auth?: { grants?: string[] } }; account?: Account };

// 登录设备列表里显示的说明，如“Chrome · macOS”；只取浏览器和系统名，不带版本号。
function deviceLabel(): string {
  const agent = navigator.userAgent;
  const browser = /Edg\//.test(agent) ? 'Edge' : /OPR\//.test(agent) ? 'Opera' : /Firefox\//.test(agent) ? 'Firefox' : /Chrome\//.test(agent) ? 'Chrome' : /Safari\//.test(agent) ? 'Safari' : t('浏览器');
  const system = /iPhone|iPad/.test(agent) ? 'iOS' : /Android/.test(agent) ? 'Android' : /Mac OS X/.test(agent) ? 'macOS' : /Windows/.test(agent) ? 'Windows' : /Linux/.test(agent) ? 'Linux' : '';
  return system ? `${browser} · ${system}` : browser;
}

export function AuthScreen({ onSignedIn, pending }: { onSignedIn: (account: Account) => Promise<void>; pending?: Account }) {
  const [step, setStep] = useState<Step>('loading');
  const [accountName, setAccountName] = useState('');
  const [password, setPassword] = useState('');
  const [confirmation, setConfirmation] = useState('');
  const [siteName, setSiteName] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [account, setAccount] = useState<Account | null>(null);
  const [options, setOptions] = useState<Options | null>(null);
  const [displayName, setDisplayName] = useState('');
  const [inviteCode, setInviteCode] = useState('');
  const [setupToken, setSetupToken] = useState('');
  const brand = useSiteBrand();

  // 站点还没初始化时，没选过语言就先选语言（选完整页刷新，界面换成该语言）。
  const setupStep = (next: Step): Step => (chosenAdminLanguage() ? next : 'language');

  useEffect(() => {
    if (pending) {
      setAccount(pending);
      setStep(setupStep('site'));
      return;
    }
    api<{ registrationOpen: boolean }>('/v1/registration')
      .then((state) => {
        // 只有创建第一个管理员时预填 admin；登录页留空，子账号不用先删掉它。
        if (state.registrationOpen) setAccountName('admin');
        setStep(state.registrationOpen ? setupStep('register') : 'login');
        if (!state.registrationOpen) void api<Options>('/v1/auth/options').then(setOptions).catch(() => {});
      })
      .catch((problem) => { setError(errorText(problem)); setStep('login'); });
  }, [pending]);

  async function run(action: () => Promise<void>) {
    setBusy(true);
    setError('');
    try { await action(); } catch (problem) { setError(errorText(problem)); } finally { setBusy(false); }
  }

  async function afterLogin(result: SessionReply) {
    storeSession(result.session);
    const signedIn = result.account ?? (await api<{ account: Account }>('/v1/self')).account;
    // 普通用户的家账户不属于任何站点，没有“站点初始化”这一步；站主账号才可能需要初始化平台自己的站点。
    if (signedIn.siteId?.startsWith('home:')) { await onSignedIn(signedIn); return; }
    const init = await api<{ initialized: boolean }>('/v1/initialization');
    if (init.initialized) await onSignedIn(signedIn);
    else { setAccount(signedIn); setStep(setupStep('site')); }
  }

  const submit = () => run(async () => {
    if (step === 'register') {
      if (password !== confirmation) throw new Error(t('两次输入的口令不一致'));
      await afterLogin(await api<SessionReply>('/v1/registration', { method: 'POST', body: { accountName, password, passwordConfirmation: confirmation, setupToken: setupToken.trim() } }));
    } else if (step === 'signup') {
      if (password !== confirmation) throw new Error(t('两次输入的口令不一致'));
      await afterLogin(await api<SessionReply>('/v1/users:register', { method: 'POST', body: { email: accountName.trim(), password, passwordConfirmation: confirmation, displayName: displayName.trim() || accountName.split('@')[0], ...(inviteCode.trim() ? { inviteCode: inviteCode.trim() } : {}) } }));
    } else if (step === 'login') {
      const body = { accountName: accountName.trim(), password, deviceLabel: deviceLabel() };
      try {
        await afterLogin(await api<SessionReply>('/v1/sessions', { method: 'POST', body }));
      } catch (problem) {
        // 账号近期多次登录失败：浏览器先解一道工作量证明，带上它重试这一次登录（每次尝试都要一份新的）。
        if (!(problem instanceof ApiError && problem.code === 'CHALLENGE_REQUIRED')) throw problem;
        const issued = await api<{ challengeId: string; nonce: string; difficulty: number }>('/v1/challenges', { method: 'POST', body: { requestId: newKey(), method: 'login-pow' } });
        const challengeProof = await solveChallenge(issued.nonce, issued.difficulty);
        await afterLogin(await api<SessionReply>('/v1/sessions', { method: 'POST', body: { ...body, challengeId: issued.challengeId, challengeProof } }));
      }
    } else if (step === 'site') {
      await api('/v1/initialization', { method: 'POST', body: { name: siteName, baseLanguage: adminLanguage() } });
      await ensureBaseLanguage(adminLanguage());
      await onSignedIn(account!);
    }
  });

  const heading: Record<Step, [string, string]> = {
    loading: ['', ''],
    language: ['', ''],
    register: [t('创建管理员账号'), t('这是站点的第一个账号，将成为唯一的超级管理员。')],
    login: [t('登录'), t('使用邮箱和口令登录，或用第三方账号登录。')],
    signup: [t('注册'), options?.inviteRequired ? t('需要邀请码。邮箱不会验证，请确认填写正确。') : t('邮箱不会验证，请确认填写正确。')],
    site: [t('初始化站点'), t('给站点起个名字。初始化不调用模型。')],
  };

  return (
    <div className="relative flex min-h-dvh flex-col items-center justify-center bg-background px-4 py-10">
      {/* 登录页可以切换界面语言；初始化流程里语言就是站点基准语言，在选语言页和站点页换。 */}
      <div className="absolute end-4 top-4 flex items-center gap-1">{(step === 'login' || step === 'signup') && <LanguageMenu />}<ThemeMenu /></div>
      <div className="mb-8 flex items-center gap-3">
        <SiteMark brand={brand} className="size-10 shrink-0 rounded-xl text-lg" />
        <div>
          <div className="text-lg font-semibold text-foreground">{brand.name || 'nice-wiki'}</div>
          <div className="text-xs text-muted">{t('文档站后台')}</div>
        </div>
      </div>
      <Card className={step === 'language' ? 'w-full max-w-lg gap-6 p-6 sm:p-8' : 'w-full max-w-sm gap-6 p-6 sm:p-8'}>
        {step === 'loading' ? (
          <div className="flex justify-center py-10"><Spinner /></div>
        ) : step === 'language' ? (
          <LanguagePicker />
        ) : (
          <>
            <Card.Header className="gap-1.5">
              <Card.Title className="text-xl font-semibold">{heading[step][0]}</Card.Title>
              <Card.Description className="leading-6">{heading[step][1]}</Card.Description>
            </Card.Header>
            <Form className="flex flex-col gap-4" onSubmit={(event) => { event.preventDefault(); void submit(); }}>
              {step === 'site' ? (
                <>
                  <TextInput autoFocus isRequired label={t('站点名称')} maxLength={120} placeholder={t('例如：产品文档')} value={siteName} onChange={setSiteName} />
                  <div className="flex items-center justify-between gap-3 rounded-xl bg-surface-secondary px-3 py-2.5">
                    <div className="min-w-0">
                      <div className="text-xs text-muted">{t('基准语言')}</div>
                      <div className="text-sm font-medium text-foreground">{findSiteLanguage(adminLanguage())?.native}</div>
                    </div>
                    <Button size="sm" variant="ghost" onPress={() => setAdminLanguage('')}>{t('更换')}</Button>
                  </div>
                  <p className="text-xs leading-5 text-muted">{t('文档按这个语言写作，其他语言通过翻译生成；初始化后不能更改。后台界面也用这个语言。')}</p>
                </>
              ) : (
                <>
                  {step === 'signup' && <TextInput label={t('昵称')} maxLength={100} value={displayName} onChange={setDisplayName} />}
                  <IconField icon={<Person />}>
                    <TextInput isRequired autoComplete="username" label={step === 'login' || step === 'signup' ? t('邮箱') : t('账号名')} value={accountName} onChange={setAccountName} />
                  </IconField>
                  <IconField icon={<Lock />}>
                    <TextInput isRequired autoComplete={step === 'register' || step === 'signup' ? 'new-password' : 'current-password'} label={t('口令')} minLength={step === 'register' || step === 'signup' ? 8 : undefined} type="password" value={password} onChange={setPassword} description={step === 'register' || step === 'signup' ? t('至少 8 位，建议使用密码管理器生成。') : undefined} />
                  </IconField>
                  {(step === 'register' || step === 'signup') && (
                    <TextInput isRequired autoComplete="new-password" label={t('确认口令')} type="password" value={confirmation} onChange={setConfirmation} error={confirmation && confirmation !== password ? t('两次输入的口令不一致') : null} />
                  )}
                  {step === 'register' && <TextInput isRequired autoComplete="off" label={t('安装令牌')} value={setupToken} onChange={setSetupToken} description={t('在部署机上查看：{0}', 'docker compose exec account cat /data/identity/control/setup-token')} />}
                  {step === 'signup' && options?.inviteRequired && <TextInput isRequired label={t('邀请码')} placeholder="nwr-…" value={inviteCode} onChange={setInviteCode} />}
                </>
              )}
              {error && <Notice status="danger">{error}</Notice>}
              <Button fullWidth isPending={busy} size="lg" type="submit">
                {busy ? <Spinner color="current" size="sm" /> : step === 'site' ? <Globe /> : null}
                {step === 'register' ? t('创建并继续') : step === 'login' ? t('登录') : step === 'signup' ? t('注册并登录') : t('完成初始化')}
                {!busy && step !== 'site' && <ArrowRight className="rtl:-scale-x-100" />}
              </Button>
            </Form>
          </>
        )}
      </Card>
      {(step === 'login' || step === 'signup') && (
        <div className="mt-5 flex w-full max-w-sm flex-col items-center gap-3">
          {options?.providers.length ? (
            <div className="flex w-full flex-col gap-2">
              {options.providers.map((item) => (
                <Button key={item.provider} fullWidth isDisabled={busy} variant="secondary" onPress={() => void run(async () => {
                  const started = await api<{ authorizeUrl: string }>(`/v1/oauth/${item.provider}:start`, { method: 'POST', body: { redirectUri: `${location.origin}/admin/oauth/${item.provider}` } });
                  location.assign(started.authorizeUrl);
                })}>{t('用 {0} 登录', item.label)}</Button>
              ))}
            </div>
          ) : null}
          {options && options.emailRegistration && (
            <Button size="sm" variant="ghost" onPress={() => { setError(''); setStep(step === 'login' ? 'signup' : 'login'); }}>{step === 'login' ? t('没有账号？注册') : t('已有账号？登录')}</Button>
          )}
          {step === 'login' && <p className="text-xs text-muted">{t('忘记口令请联系站主重置。')}</p>}
        </div>
      )}
    </div>
  );
}

// 首次进站的第一页：选站点语言。这时还不知道访客读哪种语言，标题用双语，选项用各语言自己的名字。
function LanguagePicker() {
  return (
    <>
      <Card.Header className="gap-1.5">
        {/* i18n-ignore：选语言之前的固定双语标题 */}
        <Card.Title className="text-xl font-semibold">选择语言 · Choose a language</Card.Title>
        {/* i18n-ignore */}
        <Card.Description className="leading-6">站点文档和后台都用这个语言，之后还能添加其他语言。<br />Your docs and this admin will use it. You can add more languages later.</Card.Description>
      </Card.Header>
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-3">
        {siteLanguages.map((language) => (
          <Button key={language.tag} fullWidth className="justify-start" variant="tertiary" onPress={() => setAdminLanguage(language.tag)}>
            <span dir={language.direction} lang={language.tag}>{language.native}</span>
          </Button>
        ))}
      </div>
    </>
  );
}

function IconField({ icon, children }: { icon: ReactNode; children: ReactNode }) {
  // 图标只做视觉提示，挂在标签旁边，不改变输入框结构。
  return <div className="relative [&_label]:ps-6"><span className="pointer-events-none absolute start-0 top-0.5 text-muted [&>svg]:size-4">{icon}</span>{children}</div>;
}
