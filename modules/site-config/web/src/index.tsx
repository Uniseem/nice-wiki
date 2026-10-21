import { Picture, Plus, TrashBin } from '@gravity-ui/icons';
import { Button, Card, Chip, Form, Tabs } from '@heroui/react';
import {
  api, assetURL, ColorInput, Details, ErrorBlock, FileButton, FormGrid, LoadingBlock, NumberInput, PageHeader, Panel, RadioInput, SelectInput, SwitchInput,
  TextInput, uploadImage, useAction, useHasGrant, useLoad, type AssetRef, t,
} from '@ui';
import { useEffect, useMemo, useState, type ReactNode } from 'react';

type Target = { kind: 'external'; url: string } | { kind: 'document'; documentId: string; blockId: null } | { kind: 'qa' };
type Config = {
  name: string; logo: AssetRef | null; favicon: AssetRef | null; themeColor: string; defaultShareImage: AssetRef | null;
  publicBaseURL: string; baseLanguage: string; titleTemplate: string; description: string; indexingEnabled: boolean;
  home: { mainTitle: string; subtitle: string; backgroundText: string; background: { kind: string; color?: string }; overlayPercent: number; textColor: string; buttons: { buttonId: string; text: string; style: 'primary' | 'secondary'; target: Target }[]; revision: number };
  footer: { copyright: string; links: { linkId: string; text: string; target: Target }[]; icpNumber: string | null; icpLink: string | null; sourceCodeURL: string };
  banner: { announcementId: string; revisionId: string; bodyHash: string; enabled: boolean } | null;
  resourceLimits: unknown; securityPolicy: unknown; revision: number;
};
type Button_ = { buttonId: string; text: string; style: 'primary' | 'secondary'; kind: 'external' | 'document' | 'qa'; value: string };

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
          [t('基准语言'), config.baseLanguage],
          [t('配置版本'), `#${config.revision}`],
        ]} />
      </Panel>
    </div>
  );
}

function BrandTab({ config, onSaved }: TabProps) {
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
            <Card.Footer className="flex gap-2">
              <FileButton accept="image/png,image/jpeg,image/webp" isPending={busy} size="sm" onFile={(file) => void update(item.field, item.label, file)}>{ref ? t('更换') : t('上传')}</FileButton>
              {ref && <Button isDisabled={busy} size="sm" variant="tertiary" onPress={() => update(item.field, item.label, null)}>{t('清除')}</Button>}
            </Card.Footer>
          </Card>
        );
      })}
    </div>
  );
}

function HomeTab({ config, onSaved }: TabProps) {
  const fromConfig = () => ({
    mainTitle: config.home.mainTitle, subtitle: config.home.subtitle, backgroundText: config.home.backgroundText,
    color: config.home.background?.kind === 'color' ? config.home.background.color ?? '' : '', overlayPercent: config.home.overlayPercent, textColor: config.home.textColor as 'light' | 'dark',
    buttons: config.home.buttons.map((b): Button_ => ({ buttonId: b.buttonId, text: b.text, style: b.style, kind: b.target.kind, value: b.target.kind === 'external' ? b.target.url : b.target.kind === 'document' ? b.target.documentId : '' })),
  });
  const [form, setForm] = useState(fromConfig);
  useEffect(() => setForm(fromConfig()), [config]); // eslint-disable-line
  const { busy, run } = useAction();
  const canReadDocs = useHasGrant('content.read');
  const docs = useLoad((signal) => canReadDocs ? api<{ documents: { documentId: string; title: string; state: string; pathSegment: string | null }[] }>('/v1/tree', { signal }).then((tree) => tree.documents.filter((doc) => doc.state === 'published')) : Promise.resolve(null), [canReadDocs]);
  // 同名文档用页面路径区分。
  const docOptions = useMemo(() => (docs.data ?? []).map((doc) => ({ value: doc.documentId, label: doc.title, description: doc.pathSegment ? `/${doc.pathSegment}` : undefined })), [docs.data]);
  const setButton = (index: number, patch: Partial<Button_>) => setForm((current) => ({ ...current, buttons: current.buttons.map((b, i) => i === index ? { ...b, ...patch } : b) }));

  const save = () => run(async () => {
    const target = (b: Button_): Target => b.kind === 'qa' ? { kind: 'qa' } : b.kind === 'document' ? { kind: 'document', documentId: b.value, blockId: null } : { kind: 'external', url: b.value };
    await api('/v1/home', {
      method: 'PUT', body: {
        home: {
          ...config.home, mainTitle: form.mainTitle, subtitle: form.subtitle, backgroundText: form.backgroundText,
          overlayPercent: form.overlayPercent, textColor: form.textColor,
          background: form.color ? { kind: 'color', color: form.color } : config.home.background,
          buttons: form.buttons.map((b) => ({ buttonId: b.buttonId, text: b.text, style: b.style, target: target(b) })),
        },
        expectedRevision: config.revision, expectedHomeRevision: config.home.revision,
      },
    });
    onSaved();
  }, t('首页已保存'));

  const dark = form.textColor === 'dark';
  return (
    <div className="grid gap-6 xl:grid-cols-5">
      <Panel className="xl:col-span-3" title={t('首页主视觉')} description={t('访客打开站点首页时看到的一屏。')}>
        <Form className="flex flex-col gap-5" onSubmit={(event) => { event.preventDefault(); void save(); }}>
          <TextInput isRequired label={t('主标题')} value={form.mainTitle} onChange={(mainTitle) => setForm({ ...form, mainTitle })} />
          <TextInput label={t('副标题')} value={form.subtitle} onChange={(subtitle) => setForm({ ...form, subtitle })} />
          <TextInput description={t('主标题背后淡淡的大号装饰文字，可以留空。')} label={t('背景文字')} value={form.backgroundText} onChange={(backgroundText) => setForm({ ...form, backgroundText })} />
          <FormGrid>
            <ColorInput label={t('背景颜色')} placeholder="#0F172A" value={form.color} onChange={(color) => setForm({ ...form, color })} />
            <NumberInput description={t('0 为不加遮罩，数值越大背景越暗。')} label={t('遮罩浓度（%）')} maxValue={100} minValue={0} value={form.overlayPercent} onChange={(overlayPercent) => setForm({ ...form, overlayPercent })} />
          </FormGrid>
          <RadioInput label={t('文字颜色')} options={[{ value: 'light', label: t('浅色（深色背景用）') }, { value: 'dark', label: t('深色（浅色背景用）') }]} value={form.textColor} onChange={(textColor) => setForm({ ...form, textColor })} />
          <div className="flex flex-col gap-3">
            <div className="flex items-center justify-between">
              <span className="text-sm font-medium text-foreground">{t('按钮（最多 3 个）')}</span>
              {form.buttons.length < 3 && <Button size="sm" variant="secondary" onPress={() => setForm({ ...form, buttons: [...form.buttons, { buttonId: newId(), text: '', style: 'primary', kind: 'document', value: '' }] })}><Plus />{t('添加按钮')}</Button>}
            </div>
            {!form.buttons.length && <p className="text-sm text-muted">{t('没有按钮时，首页显示“开始阅读”，指向第一篇文档。')}</p>}
            {form.buttons.map((b, index) => (
              <Card key={b.buttonId} className="gap-3 p-4" variant="secondary">
                <FormGrid columns={3}>
                  <TextInput isRequired label={t('文字')} value={b.text} onChange={(text) => setButton(index, { text })} />
                  <SelectInput label={t('样式')} options={[{ value: 'primary', label: t('主要') }, { value: 'secondary', label: t('次要') }]} value={b.style} onChange={(style) => setButton(index, { style })} />
                  <SelectInput label={t('指向')} options={[{ value: 'document', label: t('站内文档') }, { value: 'external', label: t('外部链接') }, { value: 'qa', label: t('AI 问答页') }]} value={b.kind} onChange={(kind) => setButton(index, { kind, value: '' })} />
                </FormGrid>
                {b.kind === 'external' && <TextInput isRequired label={t('链接地址')} placeholder="https://…" type="url" value={b.value} onChange={(value) => setButton(index, { value })} />}
                {b.kind === 'document' && (docOptions.length ? (
                  <SelectInput isRequired label={t('文档')} options={docOptions} placeholder={t('选择一篇已发布的文档')} value={b.value} onChange={(value) => setButton(index, { value })} />
                ) : (
                  <TextInput isRequired description={t('还没有可选的已发布文档时，可以先填文档 ID。')} label={t('文档 ID')} value={b.value} onChange={(value) => setButton(index, { value })} />
                ))}
                <div><Button size="sm" variant="tertiary" onPress={() => setForm({ ...form, buttons: form.buttons.filter((_, i) => i !== index) })}><TrashBin />{t('移除这个按钮')}</Button></div>
              </Card>
            ))}
          </div>
          <SaveBar busy={busy} />
        </Form>
      </Panel>
      <div className="xl:col-span-2">
        <div className="sticky top-20 flex flex-col gap-2">
          <span className="text-sm font-medium text-muted">{t('预览')}</span>
          <div className="relative flex aspect-video items-center justify-center overflow-hidden rounded-2xl border border-border" style={{ background: form.color || (dark ? '#F5F7FA' : '#172033') }}>
            <div className="absolute inset-0" style={{ background: `rgb(0 0 0 / ${form.overlayPercent}%)` }} />
            <div className={`relative px-6 text-center ${dark ? 'text-[#172033]' : 'text-white'}`}>
              {form.backgroundText && <div className="pointer-events-none absolute inset-x-0 -top-8 whitespace-nowrap text-5xl font-bold opacity-15">{form.backgroundText}</div>}
              <div className="text-2xl font-semibold tracking-tight">{form.mainTitle || t('主标题')}</div>
              {form.subtitle && <div className="mt-2 text-sm opacity-80">{form.subtitle}</div>}
              <div className="mt-5 flex flex-wrap justify-center gap-2">
                {(form.buttons.length ? form.buttons : [{ buttonId: 'start', text: t('开始阅读'), style: 'primary' as const }]).map((b) => (
                  <span key={b.buttonId} className={`rounded-full px-4 py-1.5 text-xs font-medium ${b.style === 'primary' ? (dark ? 'bg-[#172033] text-white' : 'bg-white text-[#172033]') : 'border border-current'}`}>{b.text || t('按钮')}</span>
                ))}
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
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
          links: form.links.filter((link) => link.text && link.url).map((link) => ({ linkId: link.linkId, text: link.text, target: { kind: 'external', url: link.url } })),
        },
        expectedRevision: config.revision,
      },
    });
    onSaved();
  }, t('页脚已保存'));
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
        <Group title={t('自定义链接')} action={form.links.length < 10 ? <Button size="sm" variant="secondary" onPress={() => setForm({ ...form, links: [...form.links, { linkId: newId(), text: '', url: '' }] })}><Plus />{t('添加链接')}</Button> : null}>
          {!form.links.length && <p className="text-sm text-muted">{t('例如“联系我们”“隐私政策”，只支持 https 外链。')}</p>}
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
