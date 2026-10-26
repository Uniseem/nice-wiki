import { AssetPicker } from '@modules/media/web/src/LibraryBrowser';
import { Picture, Plus, TrashBin } from '@gravity-ui/icons';
import { Button, Card, Chip, Form, Tabs } from '@heroui/react';
import {
  api, assetURL, ColorInput, Details, ErrorBlock, FileButton, findSiteLanguage, FormGrid, LoadingBlock, NumberInput, PageHeader, Panel, RadioInput, SelectInput, SwitchInput,
  TextInput, uploadImage, useAction, useLoad, type AssetRef, t,
} from '@ui';
import { useEffect, useState, type ReactNode } from 'react';

type Target = { kind: 'external'; url: string } | { kind: 'document'; documentId: string; blockId: null } | { kind: 'qa' };
type Config = {
  name: string; logo: AssetRef | null; favicon: AssetRef | null; themeColor: string; defaultShareImage: AssetRef | null;
  publicBaseURL: string; baseLanguage: string; titleTemplate: string; description: string; indexingEnabled: boolean;
  home: { headerLinks?: { text: string; url: string }[]; mainTitle: string; subtitle: string; backgroundText: string; background: { kind: string; color?: string }; overlayPercent: number; textColor: string; buttons: { buttonId: string; text: string; style: 'primary' | 'secondary'; target: Target }[]; revision: number };
  footer: { copyright: string; links: { linkId: string; text: string; target: Target }[]; icpNumber: string | null; icpLink: string | null; sourceCodeURL: string };
  banner: { announcementId: string; revisionId: string; bodyHash: string; enabled: boolean } | null;
  resourceLimits: unknown; securityPolicy: unknown; revision: number;
};


const newId = () => crypto.randomUUID().replace(/-/g, '');

export default function SitePage() {
  const config = useLoad((signal) => api<{ config: Config }>('/v1/config', { signal }).then((result) => result.config), []);
  const [tab, setTab] = useState('basic');
  if (config.error && !config.data) return <ErrorBlock error={config.error} onRetry={config.reload} />;
  const c = config.data;
  return (
    <div className="flex flex-col gap-6">
      <PageHeader title={t('站点设置')} description={t('站名、品牌、首页、公告横幅和页脚。保存后公开站几秒内生效。')} />
      {!c ? <LoadingBlock lines={6} /> : (
        <Tabs selectedKey={tab} onSelectionChange={(key) => setTab(String(key))}>
          <Tabs.ListContainer className="mb-6 overflow-x-auto">
            <Tabs.List aria-label={t('设置分组')}>
              <Tabs.Tab id="basic">{t('基本信息')}<Tabs.Indicator /></Tabs.Tab>
              <Tabs.Tab id="brand">{t('品牌')}<Tabs.Indicator /></Tabs.Tab>
              <Tabs.Tab id="home">{t('首页')}<Tabs.Indicator /></Tabs.Tab>
              <Tabs.Tab id="banner">{t('公告横幅')}<Tabs.Indicator /></Tabs.Tab>
              <Tabs.Tab id="footer">{t('页脚')}<Tabs.Indicator /></Tabs.Tab>
            </Tabs.List>
          </Tabs.ListContainer>
          <Tabs.Panel id="basic"><BasicTab config={c} onSaved={config.reload} /></Tabs.Panel>
          <Tabs.Panel id="brand"><BrandTab config={c} onSaved={config.reload} /></Tabs.Panel>
          <Tabs.Panel id="home"><HomeTab config={c} onSaved={config.reload} /></Tabs.Panel>
          <Tabs.Panel id="banner"><BannerTab config={c} onSaved={config.reload} /></Tabs.Panel>
          <Tabs.Panel id="footer"><FooterTab config={c} onSaved={config.reload} /></Tabs.Panel>
        </Tabs>
      )}
    </div>
  );
}

type TabProps = { config: Config; onSaved: () => void };

function SaveBar({ busy, dirty = true }: { busy: boolean; dirty?: boolean }) {
  return <div className="flex justify-end"><Button isDisabled={!dirty} isPending={busy} type="submit">{t('保存')}</Button></div>;
}

function BasicTab({ config, onSaved }: TabProps) {
  const [form, setForm] = useState({ name: config.name, themeColor: config.themeColor, titleTemplate: config.titleTemplate, description: config.description, indexingEnabled: config.indexingEnabled });
  useEffect(() => setForm({ name: config.name, themeColor: config.themeColor, titleTemplate: config.titleTemplate, description: config.description, indexingEnabled: config.indexingEnabled }), [config]);
  const { busy, run } = useAction();
  const set = <K extends keyof typeof form>(key: K) => (value: (typeof form)[K]) => setForm((current) => ({ ...current, [key]: value }));
  const dirty = form.name !== config.name || form.themeColor !== config.themeColor || form.titleTemplate !== config.titleTemplate || form.description !== config.description || form.indexingEnabled !== config.indexingEnabled;
  const example = form.titleTemplate ? form.titleTemplate.replace('{title}', t('快速开始')).replace('{site}', form.name || t('站名')) : t('快速开始');

  const save = () => run(async () => {
    await api('/v1/branding', { method: 'PUT', body: { name: form.name, logo: config.logo, favicon: config.favicon, themeColor: form.themeColor, defaultShareImage: config.defaultShareImage, expectedRevision: config.revision } });
    const fresh = (await api<{ config: Config }>('/v1/config')).config;
    await api('/v1/config', { method: 'PUT', body: { publicBaseURL: fresh.publicBaseURL, titleTemplate: form.titleTemplate, description: form.description, indexingEnabled: form.indexingEnabled, resourceLimits: fresh.resourceLimits, securityPolicy: fresh.securityPolicy, expectedRevision: fresh.revision } });
    onSaved();
  }, t('基本信息已保存'));

  return (
    <div className="grid gap-6 xl:grid-cols-3">
      <Panel className="xl:col-span-2" title={t('基本信息')} description={t('显示在公开站页头、浏览器标签和搜索结果里。')}>
        <Form className="flex flex-col gap-5" onSubmit={(event) => { event.preventDefault(); void save(); }}>
          <FormGrid>
            <TextInput isRequired label={t('站点名称')} maxLength={120} value={form.name} onChange={set('name')} />
            <ColorInput description={t('公开站的按钮、链接等强调色。')} label={t('主题色')} value={form.themeColor} onChange={set('themeColor')} />
          </FormGrid>
          <TextInput description={t('示例：{0}', example)} label={t('标题模板')} placeholder="{title} | {site}" value={form.titleTemplate} onChange={set('titleTemplate')} />
          <TextInput description={t('没有单独摘要的页面用它做搜索摘要。')} label={t('站点描述')} maxLength={300} value={form.description} onChange={set('description')} />
          <SwitchInput description={t('关闭后 robots.txt、sitemap 与各页面都会声明不要索引。')} isSelected={form.indexingEnabled} label={t('允许搜索引擎收录')} onChange={set('indexingEnabled')} />
          <SaveBar busy={busy} dirty={dirty} />
        </Form>
      </Panel>
      <Panel title={t('站点信息')}>
        <Details items={[
          [t('公开地址'), <a className="break-all text-accent hover:underline" href={config.publicBaseURL} rel="noopener" target="_blank">{config.publicBaseURL}</a>],
          [t('基准语言'), <span><span lang={config.baseLanguage}>{findSiteLanguage(config.baseLanguage)?.native ?? config.baseLanguage}</span> <span className="text-muted">{config.baseLanguage}</span></span>],
          [t('配置版本'), `#${config.revision}`],
        ]} />
      </Panel>
    </div>
  );
}

function BrandTab({ config, onSaved }: TabProps) {
  const [picker,setPicker] = useState<'logo' | 'favicon' | 'defaultShareImage' | null>(null);
  const select = (ref: AssetRef) => run(async()=>{ if (!picker) return; await api('/v1/branding',{method:'PUT',body:{name:config.name,logo:config.logo,favicon:config.favicon,themeColor:config.themeColor,defaultShareImage:config.defaultShareImage,expectedRevision:config.revision,[picker]:ref}});setPicker(null);onSaved();},t('已更新'));
  const { busy, run } = useAction();
  const update = (field: 'logo' | 'favicon' | 'defaultShareImage', label: string, file: File | null) => run(async () => {
    const ref = file ? await uploadImage(file) : null;
    await api('/v1/branding', { method: 'PUT', body: { name: config.name, logo: config.logo, favicon: config.favicon, themeColor: config.themeColor, defaultShareImage: config.defaultShareImage, expectedRevision: config.revision, [field]: ref } });
    onSaved();
  }, file ? t('{0}已更新', label) : t('{0}已清除', label));
  const items: { field: 'logo' | 'favicon' | 'defaultShareImage'; label: string; hint: string; shape: string }[] = [
    { field: 'logo', label: t('站标'), hint: t('显示在公开站页头站名左侧。建议透明背景的方形或横向图片。'), shape: 'h-16 w-32' },
    { field: 'favicon', label: t('浏览器图标'), hint: t('浏览器标签页上的小图标（favicon）。建议 256×256 的方形图片。'), shape: 'size-16' },
    { field: 'defaultShareImage', label: t('默认分享图'), hint: t('分享到社交平台时的预览图（Open Graph）。建议 1200×630。'), shape: 'h-16 w-32' },
  ];
  return (
    <>
    {picker && <AssetPicker onClose={()=>setPicker(null)} onSelect={(ref)=>void select(ref)} />}
    <div className="grid gap-6 md:grid-cols-3">
      {items.map((item) => {
        const ref = config[item.field];
        return (
          <Card key={item.field} className="gap-4 p-5">
            <Card.Header className="gap-1">
              <Card.Title className="text-base font-semibold">{item.label}</Card.Title>
              <Card.Description className="leading-6">{item.hint}</Card.Description>
            </Card.Header>
            <div className={`flex items-center justify-center overflow-hidden rounded-xl border border-dashed border-border bg-surface-secondary ${item.shape}`}>
              {ref ? <img alt={item.label} className="max-h-full max-w-full object-contain" src={assetURL(ref)} /> : <Picture className="size-6 text-muted" />}
            </div>
            <Card.Footer className="flex flex-wrap gap-2">
              <Button isDisabled={busy} size="sm" variant="secondary" onPress={()=>setPicker(item.field)}>{t('从图片库选择')}</Button>
              <FileButton accept="image/png,image/jpeg,image/webp" isPending={busy} size="sm" onFile={(file) => void update(item.field, item.label, file)}>{ref ? t('更换') : t('上传')}</FileButton>
              {ref && <Button isDisabled={busy} size="sm" variant="tertiary" onPress={() => update(item.field, item.label, null)}>{t('清除')}</Button>}
            </Card.Footer>
          </Card>
        );
      })}
    </div>
    </>
  );
}

function HomeTab({ config, onSaved }: TabProps) {
  const fromConfig = () => ({
    mainTitle: config.home.mainTitle, subtitle: config.home.subtitle, backgroundText: config.home.backgroundText,
    mode: config.home.background?.kind === 'color' ? 'color' as const : 'theme' as const,
    color: config.home.background?.color ?? '#e6e6e6', overlayPercent: config.home.overlayPercent, textColor: config.home.textColor as 'light' | 'dark',
    headerLinks: config.home.headerLinks ?? [],
  });
  const [form, setForm] = useState(fromConfig);
  useEffect(() => setForm(fromConfig()), [config]); // eslint-disable-line
  const { busy, run } = useAction();
  const themed = form.mode === 'theme';
  const save = () => run(async () => {
    await api('/v1/home', { method: 'PUT', body: {
      home: { ...config.home, mainTitle: form.mainTitle, subtitle: form.subtitle, backgroundText: form.backgroundText,
        overlayPercent: form.overlayPercent, textColor: form.textColor,
        background: themed ? { kind: 'theme' } : { kind: 'color', color: form.color },
        buttons: [], headerLinks: form.headerLinks },
      expectedRevision: config.revision, expectedHomeRevision: config.home.revision,
    } });
    onSaved();
  }, t('首页已保存'));
  const setLink = (index: number, patch: Partial<{ text: string; url: string }>) => setForm((current) => ({ ...current, headerLinks: current.headerLinks.map((link, position) => position === index ? { ...link, ...patch } : link) }));

  return (
    <Form className="grid gap-6 xl:grid-cols-5" onSubmit={(event) => { event.preventDefault(); void save(); }}>
      <div className="flex flex-col gap-6 xl:col-span-3">
        <Panel title={t('首页主视觉')} description={t('首页固定一屏，问答在框内滚动，入口指向第一篇已发布文档。')}>
          <TextInput isRequired label={t('主标题')} maxLength={160} value={form.mainTitle} onChange={(mainTitle) => setForm({ ...form, mainTitle })} />
          <TextInput label={t('副标题')} maxLength={1000} value={form.subtitle} onChange={(subtitle) => setForm({ ...form, subtitle })} />
          <TextInput description={t('显示在主标题上方的小标签，可以留空。')} label={t('背景文字')} maxLength={200} value={form.backgroundText} onChange={(backgroundText) => setForm({ ...form, backgroundText })} />
          <RadioInput description={themed ? t('自绘 SVG 山雾背景，随浅色和深色模式切换。') : undefined} label={t('背景')} options={[{ value: 'theme', label: t('跟随主题') }, { value: 'color', label: t('纯色') }]} value={form.mode} onChange={(mode) => setForm({ ...form, mode, textColor: mode === 'color' && themed ? 'dark' : form.textColor })} />
          {!themed && <>
            <FormGrid>
              <ColorInput isRequired label={t('背景颜色')} value={form.color} onChange={(color) => setForm({ ...form, color })} />
              <NumberInput description={t('0 为不加遮罩，数值越大背景越暗。')} label={t('遮罩浓度（%）')} maxValue={100} minValue={0} value={form.overlayPercent} onChange={(overlayPercent) => setForm({ ...form, overlayPercent })} />
            </FormGrid>
            <RadioInput label={t('文字颜色')} options={[{ value: 'light', label: t('浅色（深色背景用）') }, { value: 'dark', label: t('深色（浅色背景用）') }]} value={form.textColor} onChange={(textColor) => setForm({ ...form, textColor })} />
          </>}
        </Panel>
        <Panel title={t('主页右上角链接')} description={t('按添加顺序显示，外链在新标签页打开，最多 6 个。')} actions={<Button isDisabled={form.headerLinks.length >= 6} size="sm" variant="secondary" onPress={() => setForm({ ...form, headerLinks: [...form.headerLinks, { text: '', url: '' }] })}><Plus />{t('添加链接')}</Button>}>
          {form.headerLinks.map((link, index) => <div key={index} className="flex items-end gap-3">
            <div className="min-w-0 flex-1"><TextInput isRequired label={t('文字')} maxLength={24} value={link.text} onChange={(text) => setLink(index, { text })} /></div>
            <div className="min-w-0 flex-[2]"><TextInput isRequired label={t('链接地址')} maxLength={2048} placeholder="https://example.com" type="url" value={link.url} onChange={(url) => setLink(index, { url })} /></div>
            <Button isIconOnly aria-label={t('移除')} size="sm" variant="ghost" onPress={() => setForm({ ...form, headerLinks: form.headerLinks.filter((_, position) => position !== index) })}><TrashBin /></Button>
          </div>)}
        </Panel>
        <SaveBar busy={busy} />
      </div>
      <div className="xl:col-span-2">
        <Panel title={t('预览')}>
          <div className="flex min-h-80 flex-col items-center justify-center rounded-2xl bg-background px-5 py-10 text-center">
            <div className="mb-8 flex w-full justify-end gap-4 text-xs text-muted">{form.headerLinks.map((link, index) => <span key={index}>{link.text}</span>)}</div>
            <p className="text-xs text-muted">{form.backgroundText}</p>
            <p className="mt-3 font-semibold text-foreground" style={{ fontSize: '1.75rem' }}>{form.mainTitle}</p>
            <p className="mt-2 text-sm text-muted">{form.subtitle}</p>
            <div className="my-6 w-full rounded-2xl border border-border bg-field px-4 py-6 text-start text-sm text-muted">{t('AI 问答')}</div>
            <span className="rounded-xl bg-accent px-5 py-2.5 text-sm text-accent-foreground">{t('进入 wiki')}</span>
          </div>
        </Panel>
      </div>
    </Form>
  );
}

function BannerTab({ config, onSaved }: TabProps) {
  const [text, setText] = useState('');
  const [url, setUrl] = useState('');
  const [enabled, setEnabled] = useState(config.banner?.enabled ?? true);
  const { busy, run } = useAction();
  useEffect(() => {
    setEnabled(config.banner?.enabled ?? true);
    const banner = config.banner;
    if (!banner) return;
    // 横幅正文另存为配置对象，这里取回来回填表单。
    api<{ text?: string; target?: { kind: string; url?: string } | null }>(`/v1/objects/${banner.revisionId}?version=1&expectedHash=${banner.bodyHash}`)
      .then((body) => { setText(body.text ?? ''); setUrl(body.target?.kind === 'external' ? body.target.url ?? '' : ''); })
      .catch(() => {});
  }, [config]);
  const save = () => run(async () => {
    await api('/v1/banner', {
      method: 'PUT', body: {
        announcementId: config.banner?.announcementId ?? null,
        body: { schemaVersion: 1, text, target: url ? { kind: 'external', url } : null },
        enabled, startsAt: null, endsAt: null, expectedRevision: config.revision,
      },
    });
    onSaved();
  }, t('公告横幅已保存'));
  return (
    <div className="grid gap-6 xl:grid-cols-3">
      <Panel className="xl:col-span-2" title={t('公告横幅')} description={t('显示在公开站每个页面顶部的一条通知，访客可以关闭。')}>
        <Form className="flex flex-col gap-5" onSubmit={(event) => { event.preventDefault(); void save(); }}>
          <TextInput isRequired label={t('横幅文字')} maxLength={2000} placeholder={t('例如：v2.0 已发布，查看更新说明')} value={text} onChange={setText} />
          <TextInput description={t('点击横幅时打开的地址，可以留空。')} label={t('链接')} placeholder="https://…" type="url" value={url} onChange={setUrl} />
          <SwitchInput description={t('关闭后保留内容，但不再显示。')} isSelected={enabled} label={t('显示横幅')} onChange={setEnabled} />
          <SaveBar busy={busy} />
        </Form>
      </Panel>
      <Panel title={t('当前状态')}>
        {config.banner ? <Chip color={config.banner.enabled ? 'success' : 'default'} variant="soft">{config.banner.enabled ? t('展示中') : t('已关闭')}</Chip> : <span className="text-sm text-muted">{t('还没有设置横幅。')}</span>}
        {text && <div className="rounded-xl bg-accent px-4 py-2 text-center text-sm text-accent-foreground">{text}</div>}
      </Panel>
    </div>
  );
}

function FooterTab({ config, onSaved }: TabProps) {
  const fromConfig = () => ({
    copyright: config.footer.copyright, icpNumber: config.footer.icpNumber ?? '', icpLink: config.footer.icpLink ?? '', sourceCodeURL: config.footer.sourceCodeURL,
    links: config.footer.links.filter((link) => link.target.kind === 'external').map((link) => ({ linkId: link.linkId, text: link.text, url: link.target.kind === 'external' ? link.target.url : '' })),
  });
  const [form, setForm] = useState(fromConfig);
  useEffect(() => setForm(fromConfig()), [config]); // eslint-disable-line
  const { busy, run } = useAction();
  const save = () => run(async () => {
    await api('/v1/footer', {
      method: 'PUT', body: {
        footer: {
          ...config.footer, copyright: form.copyright, icpNumber: form.icpNumber || null, icpLink: form.icpLink || null, sourceCodeURL: form.sourceCodeURL,
          // 这里只编辑外链；经接口设置的站内文档/问答链接原样保留。
          links: [...config.footer.links.filter((link) => link.target.kind !== 'external'), ...form.links.filter((link) => link.text && link.url).map((link) => ({ linkId: link.linkId, text: link.text, target: { kind: 'external' as const, url: link.url } }))],
        },
        expectedRevision: config.revision,
      },
    });
    onSaved();
  }, t('页脚已保存'));
  const kept = config.footer.links.filter((link) => link.target.kind !== 'external').length;
  const setLink = (index: number, patch: Partial<{ text: string; url: string }>) => setForm((current) => ({ ...current, links: current.links.map((link, i) => i === index ? { ...link, ...patch } : link) }));
  return (
    <Panel title={t('页脚')} description={t('显示在公开站每个页面底部。')}>
      <Form className="flex max-w-3xl flex-col gap-5" onSubmit={(event) => { event.preventDefault(); void save(); }}>
        <TextInput label={t('版权信息')} placeholder={t('© 2026 示例团队')} value={form.copyright} onChange={(copyright) => setForm({ ...form, copyright })} />
        <FormGrid>
          <TextInput label={t('ICP 备案号')} value={form.icpNumber} onChange={(icpNumber) => setForm({ ...form, icpNumber })} />
          <TextInput description={t('以 https:// 开头。')} label={t('ICP 备案链接')} type="url" value={form.icpLink} onChange={(icpLink) => setForm({ ...form, icpLink })} />
        </FormGrid>
        <TextInput isRequired description={t('页脚“源代码”链接指向的地址。')} label={t('源码链接')} type="url" value={form.sourceCodeURL} onChange={(sourceCodeURL) => setForm({ ...form, sourceCodeURL })} />
        <Group title={t('自定义链接')} action={form.links.length + kept < 10 ? <Button size="sm" variant="secondary" onPress={() => setForm({ ...form, links: [...form.links, { linkId: newId(), text: '', url: '' }] })}><Plus />{t('添加链接')}</Button> : null}>
          {!form.links.length && <p className="text-sm text-muted">{t('例如“联系我们”“隐私政策”，只支持 https 外链。')}</p>}
          {kept > 0 && <p className="text-sm text-muted">{t('另有 {0} 个站内链接不能在这里编辑，保存时原样保留。', kept)}</p>}
          {form.links.map((link, index) => (
            <div key={link.linkId} className="flex flex-wrap items-start gap-2">
              <TextInput aria-label={t('链接文字')} className="sm:w-48" placeholder={t('链接文字')} value={link.text} onChange={(text) => setLink(index, { text })} />
              <TextInput aria-label={t('链接地址')} className="min-w-56 flex-1" placeholder="https://…" type="url" value={link.url} onChange={(url) => setLink(index, { url })} />
              <Button isIconOnly aria-label={t('移除')} variant="tertiary" onPress={() => setForm({ ...form, links: form.links.filter((_, i) => i !== index) })}><TrashBin /></Button>
            </div>
          ))}
        </Group>
        <SaveBar busy={busy} />
      </Form>
    </Panel>
  );
}

function Group({ title, action, children }: { title: string; action?: ReactNode; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center justify-between"><span className="text-sm font-medium text-foreground">{title}</span>{action}</div>
      {children}
    </div>
  );
}
