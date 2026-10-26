import { ArrowRightFromSquare, ArrowUpRightFromSquare, Bars, ChevronsExpandVertical, LayoutSideContentLeft, Person } from '@gravity-ui/icons';
import { Button, buttonVariants, cn, Drawer, Dropdown, Label, ScrollShadow, Separator } from '@heroui/react';
import type { Key } from '@heroui/react';
import { api, CenteredSpinner, currentSite, ErrorBlock, HeaderSlotContext, LanguageMenu, navigate, Notice, ResizeHandle, selectSite, ThemeMenu, useCurrentSite, useGrants, useLoad, usePath, useResizablePane, useStoredState, t } from '@ui';
import { openSite, type Site } from '@modules/identity/web/src/MySites';
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
  const site = useCurrentSite();
  const sites = useLoad((signal) => api<{ sites: Site[] }>('/v1/sites', { signal }).then((result) => result.sites.filter((item) => item.status !== 'deleted')), []);
  const currentSiteInfo = sites.data?.find((item) => item.siteId === site);
  // 个人级页面（站点列表、AI 额度、平台管理）随时可用；其余页面在选中的站点里工作，先选站点才显示。
  const visible = useMemo(() => account.mustChangePassword ? pages.filter((page) => page.key === 'me') : pages.filter((page) => canOpen(page, grants) && (page.personal || !!site)), [account.mustChangePassword, grants, site]);
  const current = pageFor(path);
  const page = current && visible.includes(current) ? current : undefined;

  // 宽屏侧栏可收成只有图标的窄栏，只记在本浏览器。文档页自带文档树作为主侧栏（常见 wiki 的布局），
  // 默认收起全局导航把空间让给文档树和正文；文档页与其他页面各记各的选择。
  const documents = page?.key === 'content';
  const [railOther, setRailOther] = useStoredState('nw-admin-rail', false);
  const [railDocuments, setRailDocuments] = useStoredState('nw-admin-rail-documents', true);
  const rail = documents ? railDocuments : railOther;
  const setRail = documents ? setRailDocuments : setRailOther;
  // 侧栏边缘可以拖动改宽度，往里拖过阈值就收成图标窄栏，从窄栏往外拖又展开。
  const sidebar = useResizablePane({ widthKey: 'nw-admin-sidebar-width', initial: 256, min: 200, max: 360, collapsedWidth: 64, collapsed: rail, onCollapsedChange: setRail });

  useEffect(() => {
    const moved = legacyPath(path);
    if (moved) { navigate(moved, true); return; }
    if (!page && visible.length) navigate(account.mustChangePassword ? '/admin/me' : visible[0].path, true);
  }, [path, page, visible, account.mustChangePassword]);

  const nav = <Navigation pages={visible} current={page} onNavigate={() => setNavOpen(false)} />;
  // 整屏页面可以把自己的顶栏内容放进这里（见 InHeader），只留一层顶栏。
  const [slotElement, setSlotElement] = useState<HTMLDivElement | null>(null);
  const [claims, setClaims] = useState(0);
  const slot = useMemo(() => ({ element: slotElement, claim: (on: boolean) => setClaims((count) => count + (on ? 1 : -1)) }), [slotElement]);

  return (
    <AccountContext.Provider value={account}>
      <div className="flex min-h-dvh bg-background">
        <aside className={cn('sticky top-0 hidden h-dvh shrink-0 flex-col border-e border-separator bg-surface lg:flex', !sidebar.dragging && 'transition-[width] duration-200')} style={{ width: sidebar.width }}>
          <Brand brand={brand} compact={rail} />
          {!account.mustChangePassword && <SiteSwitcher compact={rail} sites={sites.data} />}
          <ScrollShadow className={cn('flex-1 pb-4', rail ? 'px-2' : 'px-3')} hideScrollBar>
            <Navigation compact={rail} pages={visible} current={page} onNavigate={() => setNavOpen(false)} />
          </ScrollShadow>
          <AccountBox account={account} compact={rail} onSignOut={onSignOut} />
          <ResizeHandle label={t('调整侧栏宽度')} pane={sidebar} />
        </aside>

        <Drawer.Backdrop isOpen={navOpen} onOpenChange={setNavOpen}>
          <Drawer.Content placement="left">
            <Drawer.Dialog aria-label={t('后台导航')} className="flex h-full w-72 max-w-[85vw] flex-col p-0">
              <Brand brand={brand} />
              {!account.mustChangePassword && <SiteSwitcher sites={sites.data} onPicked={() => setNavOpen(false)} />}
              <div className="flex-1 overflow-y-auto px-3 pb-4">{nav}</div>
              <AccountBox account={account} onSignOut={onSignOut} />
            </Drawer.Dialog>
          </Drawer.Content>
        </Drawer.Backdrop>

        <div className="flex min-w-0 flex-1 flex-col">
          <header className="sticky top-0 z-30 flex h-14 shrink-0 items-center gap-2 border-b border-separator bg-background/85 px-3 backdrop-blur-md sm:px-4 lg:px-8">
            <Button isIconOnly aria-label={t('打开导航')} className="lg:hidden" size="sm" variant="ghost" onPress={() => setNavOpen(true)}><Bars /></Button>
            <Button isIconOnly aria-label={rail ? t('展开侧栏') : t('收起侧栏')} className="hidden lg:inline-flex" size="sm" variant="ghost" onPress={() => setRail((value) => !value)}><LayoutSideContentLeft className="rtl:-scale-x-100" /></Button>
            {!claims && (
              <div className="flex min-w-0 items-center gap-2 text-sm">
                {page?.section && <span className="hidden text-muted sm:inline">{page.section}</span>}
                {page?.section && <span className="hidden text-muted sm:inline">/</span>}
                <span className="truncate font-medium text-foreground">{page?.label ?? ''}</span>
              </div>
            )}
            <div ref={setSlotElement} className={cn('@container min-w-0 items-center gap-2', claims ? 'flex flex-1' : 'hidden')} />
            {claims > 0 && <span className="mx-1 h-5 w-px shrink-0 bg-separator" />}
            <div className={cn('flex items-center gap-1', !claims && 'ms-auto')}>
              <a className={buttonVariants({ size: 'sm', variant: 'ghost' })} href={currentSiteInfo?.primaryHost ? `${location.protocol}//${currentSiteInfo.primaryHost}${location.port && !currentSiteInfo.primaryHost.includes(':') ? `:${location.port}` : ''}/` : '/'} rel="noopener" target="_blank">
                <ArrowUpRightFromSquare />
                <span className="hidden sm:inline">{t('查看站点')}</span>
              </a>
              <LanguageMenu />
              <ThemeMenu />
            </div>
          </header>
          <HeaderSlotContext.Provider value={slot}>
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
          </HeaderSlotContext.Provider>
        </div>
      </div>
    </AccountContext.Provider>
  );
}

// 站点切换：后台其余页面都在选中的站点里工作。列表里的站点也用来刷新当前站点的授权项（角色可能被改过）。
function SiteSwitcher({ compact = false, sites, onPicked }: { compact?: boolean; sites: Site[] | undefined; onPicked?: () => void }) {
  const site = useCurrentSite();
  const current = sites?.find((item) => item.siteId === site);
  useEffect(() => {
    if (!sites || !currentSite()) return;
    if (current) selectSite({ siteId: current.siteId, grants: current.grants });
    else selectSite(null);
  }, [sites, current]);
  return (
    <div className={cn('shrink-0 pb-2', compact ? 'px-2' : 'px-3')}>
      <Dropdown>
        <Button aria-label={t('切换站点')} className={cn('h-auto w-full gap-2 py-2', compact ? 'justify-center px-0' : 'justify-start px-3')} variant="secondary">
          <ChevronsExpandVertical className="size-4 shrink-0 text-muted" />
          <span className={cn('min-w-0 flex-1 truncate text-start text-sm', compact && 'hidden')}>{current?.name ?? t('选择站点')}</span>
        </Button>
        <Dropdown.Popover className="min-w-56" placement="bottom start">
          <Dropdown.Menu onAction={(key: Key) => {
            onPicked?.();
            if (key === 'all') { navigate('/admin/sites'); return; }
            const picked = sites?.find((item) => item.siteId === key);
            if (picked) openSite(picked);
          }}>
            {(sites ?? []).map((item) => <Dropdown.Item key={item.siteId} id={item.siteId} textValue={item.name}><Label>{item.name}</Label></Dropdown.Item>)}
            <Dropdown.Item id="all" textValue={t('管理我的站点')}><Label>{t('管理我的站点…')}</Label></Dropdown.Item>
          </Dropdown.Menu>
        </Dropdown.Popover>
      </Dropdown>
    </div>
  );
}

function Brand({ brand, compact = false }: { brand: ReturnType<typeof useSiteBrand>; compact?: boolean }) {
  return (
    <a className={cn('flex h-14 shrink-0 items-center gap-2.5', compact ? 'justify-center px-0' : 'px-5')} href="/admin/sites" title={compact ? brand.name || 'nice-wiki' : undefined} onClick={(event) => { event.preventDefault(); navigate(currentSite() ? '/admin/dashboard' : '/admin/sites'); }}>
      <SiteMark brand={brand} className="size-8 shrink-0 rounded-lg text-sm" />
      <span className={cn('min-w-0', compact && 'hidden')}>
        <span className="block truncate text-sm font-semibold text-foreground">{brand.name || 'nice-wiki'}</span>
        <span className="block text-xs text-muted">{t('后台管理')}</span>
      </span>
    </a>
  );
}

function Navigation({ pages: items, current, compact = false, onNavigate }: { pages: Page[]; current?: Page; compact?: boolean; onNavigate: () => void }) {
  const nav = useRef<HTMLElement>(null);
  // 侧栏较长时，让当前页面的条目保持在可见范围内。
  useEffect(() => { nav.current?.querySelector('[aria-current="page"]')?.scrollIntoView({ block: 'nearest' }); }, [current?.key]);
  return (
    <nav ref={nav} aria-label={t('后台页面')} className={cn('flex flex-col pt-2', compact ? 'gap-2' : 'gap-4')}>
      {sections.map((section) => {
        // 必须先改口令时只剩“个人设置”一页（它平时不在导航里），也显示出来，免得侧栏空着。
        const group = items.filter((page) => (page.navigation || items.length === 1) && page.section === section);
        if (!group.length) return null;
        return (
          <div key={section || 'top'} className="flex flex-col gap-0.5">
            {section && (compact ? <Separator className="mx-auto mb-1 w-6" /> : <div className="px-3 pb-1 text-xs font-medium text-muted">{section}</div>)}
            {group.map((page) => {
              const Icon = page.icon;
              const active = current?.key === page.key;
              return (
                <a
                  key={page.key}
                  aria-current={active ? 'page' : undefined}
                  aria-label={compact ? page.label : undefined}
                  className={cn('flex items-center gap-3 rounded-xl py-2 text-sm font-medium transition-colors', compact ? 'justify-center px-0' : 'px-3', active ? 'bg-default text-foreground' : 'text-muted hover:bg-default/70 hover:text-foreground')}
                  href={page.path}
                  title={compact ? page.label : undefined}
                  onClick={(event) => {
                    if (event.metaKey || event.ctrlKey || event.shiftKey || event.button !== 0) return;
                    event.preventDefault();
                    navigate(page.path);
                    onNavigate();
                  }}
                >
                  <Icon className="size-4 shrink-0" />
                  {!compact && <span className="truncate">{page.label}</span>}
                </a>
              );
            })}
          </div>
        );
      })}
    </nav>
  );
}

function AccountBox({ account, compact = false, onSignOut }: { account: Account; compact?: boolean; onSignOut: () => void }) {
  return (
    <div className={cn('shrink-0 border-t border-separator', compact ? 'p-2' : 'p-3')}>
      <Dropdown>
        <Button aria-label={account.accountName} className={cn('h-auto w-full gap-3 py-2', compact ? 'justify-center px-0' : 'justify-start px-2')} variant="ghost">
          <span className="flex size-8 shrink-0 items-center justify-center rounded-full bg-default text-sm font-semibold text-foreground">{account.accountName.slice(0, 1).toUpperCase()}</span>
          <span className={cn('min-w-0 text-start', compact && 'hidden')}>
            <span className="block truncate text-sm font-medium text-foreground">{account.accountName}</span>
            <span className="block text-xs font-normal text-muted">{account.isSuperadmin ? t('站主') : account.siteId?.startsWith('home:') ? t('用户') : t('子账号')}</span>
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
