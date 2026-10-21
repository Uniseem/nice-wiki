import { ArrowRightFromSquare, ArrowUpRightFromSquare, Bars, Person } from '@gravity-ui/icons';
import { Button, buttonVariants, cn, Drawer, Dropdown, Label, ScrollShadow, Separator } from '@heroui/react';
import type { Key } from '@heroui/react';
import { CenteredSpinner, ErrorBlock, LanguageMenu, navigate, Notice, ThemeMenu, useGrants, usePath, t } from '@ui';
import { Component, createContext, Suspense, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import type { Account } from './App';
import { canOpen, legacyPath, pageFor, pages, sections, type Page } from './routes';
import { SiteMark, useSiteBrand } from './brand';

const AccountContext = createContext<Account | null>(null);
export const useAccount = () => useContext(AccountContext)!;

export function AdminLayout({ account, onSignOut }: { account: Account; onAccountChange: (account: Account) => void; onSignOut: () => void }) {
  const path = usePath();
  const grants = useGrants();
  const [navOpen, setNavOpen] = useState(false);
  const brand = useSiteBrand();

  // 必须先改口令的子账号只开放个人设置；其余按授权项显示（只是少展示，权限仍在服务端校验）。
  const visible = useMemo(() => account.mustChangePassword ? pages.filter((page) => page.key === 'me') : pages.filter((page) => canOpen(page, grants)), [account.mustChangePassword, grants]);
  const current = pageFor(path);
  const page = current && visible.includes(current) ? current : undefined;

  useEffect(() => {
    const moved = legacyPath(path);
    if (moved) { navigate(moved, true); return; }
    if (!page && visible.length) navigate(account.mustChangePassword ? '/admin/me' : visible[0].path, true);
  }, [path, page, visible, account.mustChangePassword]);

  const nav = <Navigation pages={visible} current={page} onNavigate={() => setNavOpen(false)} />;

  return (
    <AccountContext.Provider value={account}>
      <div className="flex min-h-dvh bg-background">
        <aside className="sticky top-0 hidden h-dvh w-64 shrink-0 flex-col border-e border-separator bg-surface lg:flex">
          <Brand brand={brand} />
          <ScrollShadow className="flex-1 px-3 pb-4" hideScrollBar>{nav}</ScrollShadow>
          <AccountBox account={account} onSignOut={onSignOut} />
        </aside>

        <Drawer.Backdrop isOpen={navOpen} onOpenChange={setNavOpen}>
          <Drawer.Content placement="left">
            <Drawer.Dialog aria-label={t('后台导航')} className="flex h-full w-72 max-w-[85vw] flex-col p-0">
              <Brand brand={brand} />
              <div className="flex-1 overflow-y-auto px-3 pb-4">{nav}</div>
              <AccountBox account={account} onSignOut={onSignOut} />
            </Drawer.Dialog>
          </Drawer.Content>
        </Drawer.Backdrop>

        <div className="flex min-w-0 flex-1 flex-col">
          <header className="sticky top-0 z-30 flex h-14 shrink-0 items-center gap-2 border-b border-separator bg-background/85 px-3 backdrop-blur-md sm:px-4 lg:px-8">
            <Button isIconOnly aria-label={t('打开导航')} className="lg:hidden" size="sm" variant="ghost" onPress={() => setNavOpen(true)}><Bars /></Button>
            <div className="flex min-w-0 items-center gap-2 text-sm">
              {page?.section && <span className="hidden text-muted sm:inline">{page.section}</span>}
              {page?.section && <span className="hidden text-muted sm:inline">/</span>}
              <span className="truncate font-medium text-foreground">{page?.label ?? ''}</span>
            </div>
            <div className="ms-auto flex items-center gap-1">
              <a className={buttonVariants({ size: 'sm', variant: 'ghost' })} href="/" rel="noopener" target="_blank">
                <ArrowUpRightFromSquare />
                <span className="hidden sm:inline">{t('查看站点')}</span>
              </a>
              <LanguageMenu />
              <ThemeMenu />
            </div>
          </header>
          <main className={page?.bleed ? 'flex min-h-0 flex-1 flex-col' : 'mx-auto w-full max-w-7xl flex-1 px-4 py-6 sm:px-6 lg:px-8 lg:py-8'}>
            {account.mustChangePassword && page?.key !== 'me' && <Notice status="warning">{t('请先在个人设置里修改初始口令。')}</Notice>}
            {page && (
              <PageBoundary key={page.key}>
                <Suspense fallback={<CenteredSpinner />}>
                  <page.component />
                </Suspense>
              </PageBoundary>
            )}
          </main>
        </div>
      </div>
    </AccountContext.Provider>
  );
}

function Brand({ brand }: { brand: ReturnType<typeof useSiteBrand> }) {
  return (
    <a className="flex h-14 shrink-0 items-center gap-2.5 px-5" href="/admin/dashboard" onClick={(event) => { event.preventDefault(); navigate('/admin/dashboard'); }}>
      <SiteMark brand={brand} className="size-8 shrink-0 rounded-lg text-sm" />
      <span className="min-w-0">
        <span className="block truncate text-sm font-semibold text-foreground">{brand.name || 'nice-wiki'}</span>
        <span className="block text-xs text-muted">{t('后台管理')}</span>
      </span>
    </a>
  );
}

function Navigation({ pages: items, current, onNavigate }: { pages: Page[]; current?: Page; onNavigate: () => void }) {
  const nav = useRef<HTMLElement>(null);
  // 侧栏较长时，让当前页面的条目保持在可见范围内。
  useEffect(() => { nav.current?.querySelector('[aria-current="page"]')?.scrollIntoView({ block: 'nearest' }); }, [current?.key]);
  return (
    <nav ref={nav} aria-label={t('后台页面')} className="flex flex-col gap-4 pt-2">
      {sections.map((section) => {
        const group = items.filter((page) => page.navigation && page.section === section);
        if (!group.length) return null;
        return (
          <div key={section || 'top'} className="flex flex-col gap-0.5">
            {section && <div className="px-3 pb-1 text-xs font-medium text-muted">{section}</div>}
            {group.map((page) => {
              const Icon = page.icon;
              const active = current?.key === page.key;
              return (
                <a
                  key={page.key}
                  aria-current={active ? 'page' : undefined}
                  className={cn('flex items-center gap-3 rounded-xl px-3 py-2 text-sm font-medium transition-colors', active ? 'bg-accent-soft text-accent-soft-foreground' : 'text-muted hover:bg-default hover:text-foreground')}
                  href={page.path}
                  onClick={(event) => {
                    if (event.metaKey || event.ctrlKey || event.shiftKey || event.button !== 0) return;
                    event.preventDefault();
                    navigate(page.path);
                    onNavigate();
                  }}
                >
                  <Icon className="size-4 shrink-0" />
                  <span className="truncate">{page.label}</span>
                </a>
              );
            })}
          </div>
        );
      })}
    </nav>
  );
}

function AccountBox({ account, onSignOut }: { account: Account; onSignOut: () => void }) {
  return (
    <div className="shrink-0 border-t border-separator p-3">
      <Dropdown>
        <Button className="h-auto w-full justify-start gap-3 px-2 py-2" variant="ghost">
          <span className="flex size-8 shrink-0 items-center justify-center rounded-full bg-default text-sm font-semibold text-foreground">{account.accountName.slice(0, 1).toUpperCase()}</span>
          <span className="min-w-0 text-start">
            <span className="block truncate text-sm font-medium text-foreground">{account.accountName}</span>
            <span className="block text-xs font-normal text-muted">{account.isSuperadmin ? t('超级管理员') : t('子账号')}</span>
          </span>
        </Button>
        <Dropdown.Popover className="min-w-52" placement="top start">
          <Dropdown.Menu onAction={(key: Key) => { if (key === 'me') navigate('/admin/me'); if (key === 'logout') onSignOut(); }}>
            <Dropdown.Item id="me" textValue={t('个人设置')}>
              <Person className="size-4 text-muted" />
              <Label>{t('个人设置')}</Label>
            </Dropdown.Item>
            <Dropdown.Item id="logout" textValue={t('退出登录')} variant="danger">
              <ArrowRightFromSquare className="size-4 rtl:-scale-x-100" />
              <Label>{t('退出登录')}</Label>
            </Dropdown.Item>
          </Dropdown.Menu>
        </Dropdown.Popover>
      </Dropdown>
    </div>
  );
}

// 单个页面出错不拖垮整个后台：显示错误和重试。
class PageBoundary extends Component<{ children: ReactNode }, { error: unknown }> {
  state = { error: null as unknown };
  static getDerivedStateFromError(error: unknown) { return { error }; }
  render() {
    if (this.state.error) {
      return (
        <div className="p-6">
          <ErrorBlock error={this.state.error} title={t('页面出错了')} onRetry={() => this.setState({ error: null })} />
          <Separator className="my-4" />
          <p className="text-sm text-muted">{t('如果反复出现，请刷新浏览器；仍无法使用时把错误信息发给管理员。')}</p>
        </div>
      );
    }
    return this.props.children;
  }
}
