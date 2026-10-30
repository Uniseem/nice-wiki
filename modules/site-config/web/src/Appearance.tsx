import { ArrowDownToLine, ArrowUpFromLine, Palette, ShoppingBag, TrashBin } from '@gravity-ui/icons';
import { Button, Card, Chip, Tabs } from '@heroui/react';
import {
  api, confirmAction, EmptyBlock, ErrorBlock, FileButton, fmtBytes, fmtDate, LoadingBlock, notify, PageHeader, Panel, SelectInput, t, TextInput, useAction, useLoad,
} from '@ui';
import { useState, type ReactNode } from 'react';
import { downloadFile, localized, ThemeSwatch, zipBody, type Manifest } from './themes';

// 站点外观（docs/theme-standard.md）：选用内置或本站装的外观、上传外观包、从外观市场安装，以及把自己的外观提交到市场。

type Scheme = 'system' | 'light' | 'dark';
type ThemeEntry = { digest: string; builtin: boolean; byteSize: number; installedAt?: string; manifest: Manifest };
type Appearance = { themeDigest: string; colorScheme: Scheme };
type Loaded = { appearance: Appearance; themes: ThemeEntry[] };
type MarketTheme = { themeId: string; version: string; digest: string; manifest: Manifest; byteSize: number; publisher: string; publishedAt: string };
type Submission = Omit<MarketTheme, 'publishedAt'> & { status: 'pending' | 'approved' | 'rejected'; reviewNote: string; submittedAt: string; reviewedAt?: string };

export default function AppearancePage() {
  const loaded = useLoad((signal) => api<Loaded>('/v1/appearance', { signal }), []);
  const [tab, setTab] = useState('installed');
  return (
    <div className="flex flex-col gap-6">
      <PageHeader title={t('外观')} description={t('给公开站换一套外观：配色、字体、栏宽和版式。可以用内置外观、上传外观包，或从外观市场安装。')} />
      <Tabs selectedKey={tab} onSelectionChange={(key) => setTab(String(key))}>
        <Tabs.ListContainer className="mb-6 overflow-x-auto">
          <Tabs.List aria-label={t('外观')}>
            <Tabs.Tab id="installed">{t('本站外观')}<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="market">{t('外观市场')}<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="mine">{t('我提交的外观')}<Tabs.Indicator /></Tabs.Tab>
          </Tabs.List>
        </Tabs.ListContainer>
        <Tabs.Panel id="installed">
          {loaded.error && !loaded.data ? <ErrorBlock error={loaded.error} onRetry={loaded.reload} /> : !loaded.data ? <LoadingBlock lines={6} /> : <InstalledTab data={loaded.data} reload={loaded.reload} />}
        </Tabs.Panel>
        <Tabs.Panel id="market">{loaded.data && <MarketTab data={loaded.data} reload={loaded.reload} />}</Tabs.Panel>
        <Tabs.Panel id="mine"><SubmissionsTab /></Tabs.Panel>
      </Tabs>
    </div>
  );
}

function useApply(data: Loaded, reload: () => void) {
  const { busy, run } = useAction();
  const apply = (digest: string, colorScheme = data.appearance.colorScheme) =>
    run(() => api('/v1/appearance', { method: 'PUT', body: { themeDigest: digest, colorScheme } }), t('外观已更新，公开站几秒内生效')).then(reload);
  return { busy, apply };
}

function InstalledTab({ data, reload }: { data: Loaded; reload: () => void }) {
  const { busy, apply } = useApply(data, reload);
  const upload = useAction();
  const current = data.themes.find((entry) => entry.digest === data.appearance.themeDigest);
  const single = current && current.manifest.colorSchemes.length === 1;
  const install = (file: File) => upload.run(() => api('/v1/appearance/themes', { method: 'POST', body: zipBody(file) }), t('外观包已装到本站')).then(reload);
  const remove = async (entry: ThemeEntry) => {
    if (!(await confirmAction({ title: t('删除外观“{0}”？', localized(entry.manifest.name)), body: t('只从本站删除这个外观包，以后可以重新上传或从市场安装。'), confirmLabel: t('删除'), tone: 'danger' }))) return;
    await upload.run(() => api(`/v1/appearance/themes/${entry.digest}`, { method: 'DELETE' }), t('已删除'));
    reload();
  };
  return (
    <div className="flex flex-col gap-6">
      <Panel
        title={t('访客默认明暗')}
        description={single ? t('当前外观只有一种配色，访客不能切换明暗。') : t('访客第一次打开时用的明暗；访客自己切换后按访客的选择。')}
        actions={<FileButton accept=".zip,application/zip" isPending={upload.busy} onFile={(file) => void install(file)} variant="secondary"><ArrowUpFromLine />{t('上传外观包')}</FileButton>}
      >
        <SelectInput<Scheme>
          aria-label={t('访客默认明暗')} className="max-w-xs" isDisabled={single || busy} value={data.appearance.colorScheme}
          options={[{ value: 'system', label: t('跟随系统') }, { value: 'light', label: t('浅色') }, { value: 'dark', label: t('深色') }]}
          onChange={(scheme) => void apply(data.appearance.themeDigest, scheme)}
        />
      </Panel>
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        {data.themes.map((entry) => {
          const active = entry.digest === data.appearance.themeDigest;
          return (
            <ThemeCard key={entry.digest} manifest={entry.manifest} chips={<>
              {active && <Chip color="success" size="sm" variant="soft">{t('使用中')}</Chip>}
              {entry.builtin && <Chip size="sm" variant="soft">{t('内置')}</Chip>}
            </>} footer={<>
              {!active && <Button isPending={busy} size="sm" onPress={() => void apply(entry.digest)}>{t('使用')}</Button>}
              <Button isIconOnly aria-label={t('导出')} size="sm" variant="ghost" onPress={() => void downloadFile(`/v1/appearance/packages/${entry.digest}`, `${entry.manifest.id}-${entry.manifest.version}.zip`)}><ArrowDownToLine /></Button>
              {!entry.builtin && !active && <Button isIconOnly aria-label={t('删除')} size="sm" variant="ghost" onPress={() => void remove(entry)}><TrashBin /></Button>}
            </>} />
          );
        })}
      </div>
      <p className="text-sm leading-6 text-muted">{t('外观包是一个 zip：theme.json 加可选的 theme.css、预览图和 assets/ 下的图片与字体，不能带脚本。可以先导出一个内置外观，改好 id 和配色再上传。')}</p>
    </div>
  );
}

function ThemeCard({ manifest, image, chips, footer, meta }: { manifest: Manifest; image?: string; chips?: ReactNode; footer?: ReactNode; meta?: ReactNode }) {
  return (
    <Card className="gap-3 p-4">
      {image ? <img alt="" className="h-28 w-full rounded-xl border border-border object-cover" loading="lazy" src={image} /> : <ThemeSwatch manifest={manifest} />}
      <div className="flex min-w-0 flex-wrap items-center gap-2">
        <span className="truncate font-medium text-foreground">{localized(manifest.name)}</span>
        <span className="text-xs text-muted">{manifest.version}</span>
        {chips}
      </div>
      {manifest.description && <p className="line-clamp-2 text-sm leading-6 text-muted">{localized(manifest.description)}</p>}
      <p className="text-xs text-muted">{meta ?? `${manifest.author} · ${manifest.license}`}</p>
      {footer && <div className="mt-auto flex flex-wrap items-center gap-1">{footer}</div>}
    </Card>
  );
}

function MarketTab({ data, reload }: { data: Loaded; reload: () => void }) {
  const market = useLoad((signal) => api<{ themes: MarketTheme[] }>('/v1/theme-market', { signal }), []);
  const { busy, apply } = useApply(data, reload);
  const [query, setQuery] = useState('');
  const [installing, setInstalling] = useState('');
  if (market.error && !market.data) return <ErrorBlock error={market.error} onRetry={market.reload} />;
  if (!market.data) return <LoadingBlock lines={6} />;
  const words = query.trim().toLowerCase();
  const shown = market.data.themes.filter((item) => !words || [item.themeId, localized(item.manifest.name), localized(item.manifest.description), item.publisher].join(' ').toLowerCase().includes(words));
  const install = async (item: MarketTheme) => {
    setInstalling(item.digest);
    try {
      const pkg = await api<Blob>(`/v1/theme-market/${item.themeId}/${item.version}/package`, { responseType: 'blob' });
      await api('/v1/appearance/themes', { method: 'POST', body: zipBody(pkg) });
      notify.success(t('外观已装到本站'));
      reload();
    } catch (error) {
      notify.error(t('安装失败'), error instanceof Error ? error.message : String(error));
    } finally { setInstalling(''); }
  };
  if (!market.data.themes.length) return <EmptyBlock icon={<ShoppingBag />} title={t('外观市场里还没有外观')} description={t('在“我提交的外观”里提交外观包，平台管理员审核通过后会出现在这里。')} />;
  return (
    <div className="flex flex-col gap-4">
      <TextInput aria-label={t('搜索外观')} className="max-w-sm" placeholder={t('搜索外观')} value={query} onChange={setQuery} />
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        {shown.map((item) => {
          const installed = data.themes.find((entry) => entry.digest === item.digest);
          const older = !installed && data.themes.some((entry) => !entry.builtin && entry.manifest.id === item.themeId);
          const active = installed?.digest === data.appearance.themeDigest;
          return (
            <ThemeCard
              key={item.digest} manifest={item.manifest}
              image={item.manifest.preview ? `/v1/theme-market/${item.themeId}/${item.version}/preview` : undefined}
              meta={`${item.publisher} · ${item.manifest.license} · ${fmtBytes(item.byteSize)}`}
              chips={active ? <Chip color="success" size="sm" variant="soft">{t('使用中')}</Chip> : installed && <Chip size="sm" variant="soft">{t('已安装')}</Chip>}
              footer={installed
                ? !active && <Button isPending={busy} size="sm" onPress={() => void apply(installed.digest)}>{t('使用')}</Button>
                : <Button isPending={installing === item.digest} size="sm" variant={older ? 'primary' : 'secondary'} onPress={() => void install(item)}>{older ? t('更新到 {0}', item.version) : t('安装')}</Button>}
            />
          );
        })}
      </div>
    </div>
  );
}

const statusLabel = (status: Submission['status']) => ({ pending: t('待审核'), approved: t('已上架'), rejected: t('已驳回') })[status];
const statusColor = { pending: 'warning', approved: 'success', rejected: 'danger' } as const;

function SubmissionsTab() {
  const list = useLoad((signal) => api<{ submissions: Submission[] }>('/v1/me/theme-submissions', { signal }), []);
  const { busy, run } = useAction();
  const submit = (file: File) => run(() => api('/v1/theme-market', { method: 'POST', body: zipBody(file) }), t('已提交，等平台管理员审核')).then(list.reload);
  const withdraw = async (item: Submission) => {
    if (!(await confirmAction({ title: t('撤回 {0} {1}？', localized(item.manifest.name), item.version), body: t('撤回后市场里不再显示这个版本，已经装了它的站点不受影响。'), confirmLabel: t('撤回'), tone: 'danger' }))) return;
    await run(() => api(`/v1/theme-market/${item.themeId}/${item.version}`, { method: 'DELETE' }), t('已撤回'));
    list.reload();
  };
  return (
    <div className="flex flex-col gap-6">
      <Panel
        title={t('提交到外观市场')}
        description={t('提交后由平台管理员审核，通过后所有站点都能在外观市场看到和安装。外观 id 第一次提交后归你所有；改了内容要在 theme.json 里升版本号再提交。')}
        actions={<FileButton accept=".zip,application/zip" isPending={busy} onFile={(file) => void submit(file)}><ArrowUpFromLine />{t('提交外观包')}</FileButton>}
      />
      {list.error && !list.data ? <ErrorBlock error={list.error} onRetry={list.reload} /> : !list.data ? <LoadingBlock lines={4} /> : !list.data.submissions.length
        ? <EmptyBlock icon={<Palette />} title={t('还没有提交过外观')} />
        : (
          <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
            {list.data.submissions.map((item) => (
              <ThemeCard
                key={`${item.themeId}@${item.version}`} manifest={item.manifest}
                meta={`${t('提交于 {0}', fmtDate(item.submittedAt))} · ${fmtBytes(item.byteSize)}`}
                chips={<Chip color={statusColor[item.status]} size="sm" variant="soft">{statusLabel(item.status)}</Chip>}
                footer={<>
                  {item.status === 'rejected' && item.reviewNote && <p className="w-full text-sm text-danger">{t('驳回原因：{0}', item.reviewNote)}</p>}
                  <Button size="sm" variant="ghost" onPress={() => void withdraw(item)}>{t('撤回')}</Button>
                </>}
              />
            ))}
          </div>
        )}
    </div>
  );
}
