import { ArrowRight, Globe, Lock, Person } from '@gravity-ui/icons';
import { Button, Card, Form, Spinner } from '@heroui/react';
import { api, errorText, Notice, SelectInput, storeSession, TextInput, ThemeMenu } from '@ui';
import { useEffect, useState, type ReactNode } from 'react';
import { ensureBaseLanguage, type Account } from './App';
import { SiteMark, useSiteBrand } from './brand';

// 进入后台的向导：还没有账号 → 注册唯一的超级管理员；已有账号 → 登录；登录后站点未初始化 → 填站点名与基准语言。
type Step = 'loading' | 'register' | 'login' | 'site';
type SessionReply = { session: { csrfToken: string; sessionId: string; auth?: { grants?: string[] } }; account?: Account };

// 登录设备列表里显示的说明，如“Chrome · macOS”；只取浏览器和系统名，不带版本号。
function deviceLabel(): string {
  const agent = navigator.userAgent;
  const browser = /Edg\//.test(agent) ? 'Edge' : /OPR\//.test(agent) ? 'Opera' : /Firefox\//.test(agent) ? 'Firefox' : /Chrome\//.test(agent) ? 'Chrome' : /Safari\//.test(agent) ? 'Safari' : '浏览器';
  const system = /iPhone|iPad/.test(agent) ? 'iOS' : /Android/.test(agent) ? 'Android' : /Mac OS X/.test(agent) ? 'macOS' : /Windows/.test(agent) ? 'Windows' : /Linux/.test(agent) ? 'Linux' : '';
  return system ? `${browser} · ${system}` : browser;
}

export function AuthScreen({ onSignedIn }: { onSignedIn: (account: Account) => Promise<void> }) {
  const [step, setStep] = useState<Step>('loading');
  const [accountName, setAccountName] = useState('');
  const [password, setPassword] = useState('');
  const [confirmation, setConfirmation] = useState('');
  const [siteName, setSiteName] = useState('');
  const [baseLanguage, setBaseLanguage] = useState<'zh-Hans' | 'en'>('zh-Hans');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [account, setAccount] = useState<Account | null>(null);
  const brand = useSiteBrand();

  useEffect(() => {
    api<{ registrationOpen: boolean }>('/v1/registration')
      .then((state) => {
        // 只有创建第一个管理员时预填 admin；登录页留空，子账号不用先删掉它。
        if (state.registrationOpen) setAccountName('admin');
        setStep(state.registrationOpen ? 'register' : 'login');
      })
      .catch((problem) => { setError(errorText(problem)); setStep('login'); });
  }, []);

  async function run(action: () => Promise<void>) {
    setBusy(true);
    setError('');
    try { await action(); } catch (problem) { setError(errorText(problem)); } finally { setBusy(false); }
  }

  async function afterLogin(result: SessionReply) {
    storeSession(result.session);
    const signedIn = result.account ?? (await api<{ account: Account }>('/v1/self')).account;
    const init = await api<{ initialized: boolean }>('/v1/initialization');
    if (init.initialized) await onSignedIn(signedIn);
    else { setAccount(signedIn); setStep('site'); }
  }

  const submit = () => run(async () => {
    if (step === 'register') {
      if (password !== confirmation) throw new Error('两次输入的口令不一致');
      await afterLogin(await api<SessionReply>('/v1/registration', { method: 'POST', body: { accountName, password, passwordConfirmation: confirmation } }));
    } else if (step === 'login') {
      await afterLogin(await api<SessionReply>('/v1/sessions', { method: 'POST', body: { accountName, password, deviceLabel: deviceLabel() } }));
    } else if (step === 'site') {
      await api('/v1/initialization', { method: 'POST', body: { name: siteName, baseLanguage } });
      await ensureBaseLanguage(baseLanguage);
      await onSignedIn(account!);
    }
  });

  const heading: Record<Step, [string, string]> = {
    loading: ['', ''],
    register: ['创建管理员账号', '这是站点的第一个账号，将成为唯一的超级管理员。'],
    login: ['登录后台', '使用管理员分配的账号和口令登录。'],
    site: ['初始化站点', '给站点起个名字，并选择文档的基准语言。初始化不调用模型，不产生费用。'],
  };

  return (
    <div className="relative flex min-h-dvh flex-col items-center justify-center bg-background px-4 py-10">
      <div className="absolute end-4 top-4"><ThemeMenu /></div>
      <div className="mb-8 flex items-center gap-3">
        <SiteMark brand={brand} className="size-10 shrink-0 rounded-xl text-lg" />
        <div>
          <div className="text-lg font-semibold text-foreground">{brand.name || 'nice-wiki'}</div>
          <div className="text-xs text-muted">文档站后台</div>
        </div>
      </div>
      <Card className="w-full max-w-sm gap-6 p-6 sm:p-8">
        {step === 'loading' ? (
          <div className="flex justify-center py-10"><Spinner /></div>
        ) : (
          <>
            <Card.Header className="gap-1.5">
              <Card.Title className="text-xl font-semibold">{heading[step][0]}</Card.Title>
              <Card.Description className="leading-6">{heading[step][1]}</Card.Description>
            </Card.Header>
            <Form className="flex flex-col gap-4" onSubmit={(event) => { event.preventDefault(); void submit(); }}>
              {step === 'site' ? (
                <>
                  <TextInput autoFocus isRequired label="站点名称" maxLength={120} placeholder="例如：产品文档" value={siteName} onChange={setSiteName} />
                  <SelectInput
                    description="文档按这个语言写作，其他语言通过翻译生成。"
                    label="基准语言"
                    options={[{ value: 'zh-Hans', label: '简体中文（zh-Hans）' }, { value: 'en', label: 'English（en）' }]}
                    value={baseLanguage}
                    onChange={setBaseLanguage}
                  />
                </>
              ) : (
                <>
                  <IconField icon={<Person />}>
                    <TextInput isRequired autoComplete="username" label="账号名" value={accountName} onChange={setAccountName} />
                  </IconField>
                  <IconField icon={<Lock />}>
                    <TextInput isRequired autoComplete={step === 'register' ? 'new-password' : 'current-password'} label="口令" minLength={step === 'register' ? 8 : undefined} type="password" value={password} onChange={setPassword} description={step === 'register' ? '至少 8 位，建议使用密码管理器生成。' : undefined} />
                  </IconField>
                  {step === 'register' && (
                    <TextInput isRequired autoComplete="new-password" label="确认口令" type="password" value={confirmation} onChange={setConfirmation} error={confirmation && confirmation !== password ? '两次输入的口令不一致' : null} />
                  )}
                </>
              )}
              {error && <Notice status="danger">{error}</Notice>}
              <Button fullWidth isPending={busy} size="lg" type="submit">
                {busy ? <Spinner color="current" size="sm" /> : step === 'site' ? <Globe /> : null}
                {step === 'register' ? '创建并继续' : step === 'login' ? '登录' : '完成初始化'}
                {!busy && step !== 'site' && <ArrowRight />}
              </Button>
            </Form>
          </>
        )}
      </Card>
      <p className="mt-6 text-xs text-muted">忘记口令请联系超级管理员重置。</p>
    </div>
  );
}

function IconField({ icon, children }: { icon: ReactNode; children: ReactNode }) {
  // 图标只做视觉提示，挂在标签旁边，不改变输入框结构。
  return <div className="relative [&_label]:ps-6"><span className="pointer-events-none absolute start-0 top-0.5 text-muted [&>svg]:size-4">{icon}</span>{children}</div>;
}
