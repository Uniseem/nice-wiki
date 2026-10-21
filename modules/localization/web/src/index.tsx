import { Globe, Plus } from '@gravity-ui/icons';
import { Button, Card, Chip, Form, Modal, Table } from '@heroui/react';
import {
  adminLanguage, api, confirmAction, EmptyBlock, ErrorBlock, findSiteLanguage, LoadingBlock, newKey, Notice, PageHeader, Panel, MiniStat, SelectInput, siteLanguages, SwitchInput,
  useAction, useHasGrant, useLoad, t,
} from '@ui';
import { useEffect, useMemo, useState } from 'react';

type Language = { tag: string; version: number; enabled: boolean; isDefault: boolean; automaticTranslation: boolean; direction: 'ltr' | 'rtl'; reflectionEnabled: boolean; batchOutputTokens: number; translationMemoryMinimumCharacters: number };
type Progress = { language: string; completedUnits: number; queuedUnits: number; reviewUnits: number };
type Active = { translationId: string; documentId: string; language: string; state: string };
type NavNode = { documentId: string; title?: string; publication: unknown };

const names = new Intl.DisplayNames([adminLanguage()], { type: 'language' });
const nameOf = (tag: string) => { try { return names.of(tag) ?? tag; } catch { return tag; } };
// 站点要求：基准语言之外的英文始终启用并在发布时自动翻译（AI 决策模型只读英文），不能关。
const requiredEnglish = (language: Language) => !language.isDefault && /^en(-|$)/i.test(language.tag);

export default function LanguagesPage() {
  const canLanguages = useHasGrant('localization.languages');
  const canTranslate = useHasGrant('localization.manage');
  const languages = useLoad((signal) => api<{ languages: Language[] }>('/v1/languages?includeDisabled=true', { signal }).then((result) => result.languages), []);
  const [adding, setAdding] = useState(false);
  const { busy, run } = useAction();

  const setEnabled = (language: Language, enabled: boolean) => run(async () => {
    const idempotencyKey = newKey();
    await api(`/v1/languages/${encodeURIComponent(language.tag)}/enabled`, { method: 'POST', idempotencyKey, body: { tag: language.tag, enabled, expectedVersion: language.version, idempotencyKey } });
    languages.reload();
  }, enabled ? t('{0}已启用', nameOf(language.tag)) : t('{0}已停用', nameOf(language.tag)));

  const setAuto = async (language: Language, on: boolean) => {
    if (on && !(await confirmAction({ title: t('开启自动翻译？'), body: t('开启后，每篇文档发布新版本时都会自动调用模型翻译到该语言。'), confirmLabel: t('开启'), tone: 'warning' }))) return;
    await run(async () => {
      const idempotencyKey = newKey();
      await api(`/v1/languages/${encodeURIComponent(language.tag)}`, { method: 'PUT', idempotencyKey, body: { expectedVersion: language.version, idempotencyKey, language: { ...language, version: language.version + 1, automaticTranslation: on } } });
      languages.reload();
    }, on ? t('已开启自动翻译') : t('已关闭自动翻译'));
  };

  const translateUI = async (language: Language) => {
    const ok = await confirmAction({ title: t('把界面文字翻译成{0}？', nameOf(language.tag)), body: t('公开页上的固定文字（如“本页目录”“上一篇”）会调用一次模型翻译。约一分钟后生效，没翻译的部分仍显示英文。'), confirmLabel: t('开始翻译'), tone: 'warning' });
    if (!ok) return;
    await run(async () => {
      const idempotencyKey = newKey();
      await api(`/v1/interface-strings/${encodeURIComponent(language.tag)}:translate`, { method: 'POST', idempotencyKey, body: { language: language.tag } });
    }, t('已开始翻译界面文字'));
  };

  if (languages.error && !languages.data) return <ErrorBlock error={languages.error} onRetry={languages.reload} />;
  const list = languages.data ?? [];
  const base = list.find((language) => language.isDefault);
  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title={t('多语言')}
        description={t('基准语言的页面直接来自文档正文；其他语言通过翻译生成。翻译会调用模型，需要先在“模型接入”里配好模型。')}
        actions={canLanguages ? <Button onPress={() => setAdding(true)}><Plus />{t('添加语言')}</Button> : undefined}
      />
      {!languages.data ? <LoadingBlock lines={4} /> : (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {list.map((language) => (
            <Card key={language.tag} className="gap-4 p-5">
              <div className="flex items-start justify-between gap-3">
                <Card.Header className="gap-0.5">
                  <Card.Title className="text-base font-semibold">{nameOf(language.tag)}</Card.Title>
                  <Card.Description className="font-mono text-xs">{language.tag}</Card.Description>
                </Card.Header>
                <div className="flex flex-wrap justify-end gap-1">
                  {language.isDefault && <Chip color="accent" size="sm" variant="soft">{t('基准语言')}</Chip>}
                  {!language.isDefault && <Chip color={language.enabled ? 'success' : 'default'} size="sm" variant="soft">{language.enabled ? t('已启用') : t('未启用')}</Chip>}
                  {language.direction === 'rtl' && <Chip size="sm" variant="soft">{t('从右到左')}</Chip>}
                </div>
              </div>
              {language.isDefault ? (
                <p className="text-sm leading-6 text-muted">{t('访客默认看到这个语言。文档正文就是这个语言的内容，不需要翻译。')}</p>
              ) : (
                <div className="flex flex-col gap-3">
                  <SwitchInput description={t('启用后访客才能切换到这个语言。')} isDisabled={busy || !canLanguages || requiredEnglish(language)} isSelected={language.enabled} label={t('对访客启用')} onChange={(value) => void setEnabled(language, value)} />
                  <SwitchInput description={t('文档发布新版本时自动调用模型翻译。')} isDisabled={busy || !language.enabled || !canLanguages || requiredEnglish(language)} isSelected={language.automaticTranslation} label={t('发布时自动翻译')} onChange={(value) => void setAuto(language, value)} />
                  {requiredEnglish(language) && <p className="text-xs leading-5 text-muted">{t('英文是站点必需语言：始终启用，发布时自动翻译；AI 判断也只读英文。')}</p>}
                </div>
              )}
              {!language.isDefault && language.enabled && canTranslate && (
                <Card.Footer><Button isDisabled={busy} size="sm" variant="secondary" onPress={() => translateUI(language)}><Globe />{t('翻译界面文字')}</Button></Card.Footer>
              )}
            </Card>
          ))}
        </div>
      )}
      {canTranslate && list.filter((language) => language.enabled).length > 1 && <SuggestionPanel />}
      {canTranslate && base && <Translations base={base.tag} languages={list.filter((language) => language.enabled && !language.isDefault)} />}
      {adding && <AddLanguage existing={list.map((language) => language.tag)} onClose={() => setAdding(false)} onAdded={() => { setAdding(false); languages.reload(); }} />}
    </div>
  );
}

// 访客语言建议：关闭时只按浏览器语言给页首提示条；开启后由“模型接入”里的决策模型判断，用居中双语弹窗问访客一次。
function SuggestionPanel() {
  const policy = useLoad((signal) => api<{ enabled: boolean; version: number }>('/v1/language-suggestion-policy', { signal }), []);
  const { busy, run } = useAction();
  const toggle = async (enabled: boolean) => {
    if (enabled && !(await confirmAction({ title: t('开启 AI 语言建议？'), body: t('访客打开页面时，会把浏览器语言、时区和 IP 所在国家（不含地址）交给决策模型判断一次，同样的组合只判断一次。需要先在“模型接入”里添加决策模型（如 OpenRouter 的 typesafe/jev-1.14）并生成决策档位。'), confirmLabel: t('开启'), tone: 'warning' }))) return;
    await run(async () => {
      const idempotencyKey = newKey();
      await api('/v1/language-suggestion-policy', { method: 'PUT', idempotencyKey, body: { enabled, expectedVersion: policy.data!.version, idempotencyKey } });
      policy.reload();
    }, enabled ? t('已开启 AI 语言建议') : t('已关闭 AI 语言建议'));
  };
  return (
    <Panel title={t('访客语言建议')} description={t('访客没手动选过语言时，判断他更可能想读哪种语言并提示切换；访客选过一次就不再打扰。')}>
      {policy.error ? <ErrorBlock error={policy.error} onRetry={policy.reload} /> : !policy.data ? <LoadingBlock lines={1} /> : (
        <SwitchInput
          description={t('关闭时只按浏览器语言在页首给提示条；开启后由决策模型判断，浏览器是简体中文时基本不看 IP 国家（常挂代理）。')}
          isDisabled={busy}
          isSelected={policy.data.enabled}
          label={t('AI 判断访客语言')}
          onChange={(value) => void toggle(value)}
        />
      )}
    </Panel>
  );
}

function Translations({ base, languages }: { base: string; languages: Language[] }) {
  const [target, setTarget] = useState(languages[0]?.tag ?? '');
  useEffect(() => { if (!languages.some((language) => language.tag === target)) setTarget(languages[0]?.tag ?? ''); }, [languages, target]);
  const nodes = useLoad(async (signal) => {
    const response = await fetch(`/${base}/navigation`, { signal });
    return response.ok ? (await response.json()) as NavNode[] : [];
  }, [base]);
  const progress = useLoad((signal) => api<{ languages: Progress[]; active?: Active[] }>('/v1/translations/progress', { signal }), []);
  const { busy, run } = useAction();
  const titles = useMemo(() => new Map((nodes.data ?? []).map((node) => [node.documentId, node.title ?? node.documentId])), [nodes.data]);

  useEffect(() => {
    if (!progress.data?.active?.length) return;
    const timer = setInterval(progress.reload, 8000);
    return () => clearInterval(timer);
  }, [progress.data, progress.reload]);

  const translate = async (node: NavNode) => {
    const ok = await confirmAction({ title: t('把“{0}”翻译成{1}？', node.title ?? node.documentId, nameOf(target)), body: t('按块调用模型翻译。完成后访客就能看到该语言的页面。'), confirmLabel: t('开始翻译'), tone: 'warning' });
    if (!ok) return;
    await run(async () => {
      const idempotencyKey = newKey();
      await api('/v1/translations', { method: 'POST', idempotencyKey, body: { document: node.publication, targetLanguage: target, idempotencyKey } });
      progress.reload();
    }, t('已开始翻译，稍后刷新查看进度'));
  };

  const cancel = async (item: Active) => {
    const ok = await confirmAction({ title: t('取消这次翻译？'), body: t('取消后不再继续调用模型；已经发出的那一批调用无法撤回。'), confirmLabel: t('取消翻译'), tone: 'danger', cancelLabel: t('继续翻译') });
    if (!ok) return;
    await run(async () => {
      const idempotencyKey = newKey();
      await api(`/v1/translations/${item.translationId}/cancel`, { method: 'POST', idempotencyKey, body: { translationId: item.translationId, reason: t('管理员在后台取消'), idempotencyKey } });
      progress.reload();
    }, t('已取消该翻译'));
  };

  if (!languages.length) {
    return <Panel title={t('文档译文')}><EmptyBlock icon={<Globe />} title={t('还没有启用的其他语言')} description={t('先在上面启用一个非基准语言，才能翻译文档。')} /></Panel>;
  }
  const stats = progress.data?.languages.find((item) => item.language === target);
  const active = (progress.data?.active ?? []).filter((item) => item.language === target);
  return (
    <Panel
      title={t('文档译文')}
      description={t('选择目标语言，为已发布的文档发起翻译。')}
      actions={<SelectInput aria-label={t('目标语言')} className="w-48" options={languages.map((language) => ({ value: language.tag, label: t('{0}（{1}）', nameOf(language.tag), language.tag) }))} value={target} onChange={setTarget} variant="primary" />}
    >
      {stats && (
        <div className="grid gap-3 sm:grid-cols-3">
          <MiniStat label={t('已完成')} value={stats.completedUnits} tone="success" />
          <MiniStat label={t('进行中')} value={stats.queuedUnits} tone="accent" />
          <MiniStat label={t('待复核')} value={stats.reviewUnits} tone="warning" />
        </div>
      )}
      {active.length > 0 && (
        <Notice status="accent" title={t('有 {0} 篇正在翻译', active.length)}>
          <ul className="mt-1 flex flex-col gap-1">
            {active.map((item) => (
              <li key={item.translationId} className="flex flex-wrap items-center gap-2">
                <span>{titles.get(item.documentId) ?? item.documentId}</span>
                <Chip size="sm" variant="soft">{item.state === 'running' ? t('进行中') : t('排队中')}</Chip>
                <Button isDisabled={busy} size="sm" variant="tertiary" onPress={() => cancel(item)}>{t('取消')}</Button>
              </li>
            ))}
          </ul>
        </Notice>
      )}
      {nodes.loading && !nodes.data ? <LoadingBlock lines={3} /> : (
        <Table variant="secondary">
          <Table.ScrollContainer>
            <Table.Content aria-label={t('已发布文档')} className="min-w-[480px]">
              <Table.Header>
                <Table.Column isRowHeader>{t('已发布文档')}</Table.Column>
                <Table.Column className="w-32 text-end">{t('操作')}</Table.Column>
              </Table.Header>
              <Table.Body renderEmptyState={() => <p className="py-6 text-center text-sm text-muted">{t('还没有已发布文档。')}</p>}>
                {(nodes.data ?? []).map((node) => (
                  <Table.Row key={node.documentId} id={node.documentId}>
                    <Table.Cell>{node.title ?? node.documentId}</Table.Cell>
                    <Table.Cell className="text-end"><Button isDisabled={busy} size="sm" variant="secondary" onPress={() => translate(node)}>{t('翻译')}</Button></Table.Cell>
                  </Table.Row>
                ))}
              </Table.Body>
            </Table.Content>
          </Table.ScrollContainer>
        </Table>
      )}
    </Panel>
  );
}

// 添加语言只能从预设的站点语言里选（服务端同样只接受这些）。
function AddLanguage({ existing, onClose, onAdded }: { existing: string[]; onClose: () => void; onAdded: () => void }) {
  const available = siteLanguages.filter((language) => !existing.includes(language.tag));
  const [tag, setTag] = useState(available[0]?.tag ?? '');
  const { busy, run } = useAction();
  const add = async () => {
    const done = await run(async () => {
      const idempotencyKey = newKey();
      await api(`/v1/languages/${encodeURIComponent(tag)}`, {
        method: 'PUT', idempotencyKey, body: {
          expectedVersion: 0, idempotencyKey, language: {
            tag, version: 1, enabled: false, isDefault: false, automaticTranslation: false, direction: findSiteLanguage(tag)?.direction ?? 'ltr',
            reflectionEnabled: false, batchOutputTokens: 1000, translationMemoryMinimumCharacters: 20,
          },
        },
      });
      return true;
    }, t('语言已添加（默认未启用）'));
    if (done) onAdded();
  };
  return (
    <Modal.Backdrop isOpen onOpenChange={(open) => { if (!open) onClose(); }}>
      <Modal.Container size="sm">
        <Modal.Dialog>
          <Modal.CloseTrigger aria-label={t('关闭')} />
          <Modal.Header><Modal.Heading>{t('添加语言')}</Modal.Heading></Modal.Header>
          <Form className="flex flex-col" onSubmit={(event) => { event.preventDefault(); if (tag) void add(); }}>
            <Modal.Body className="flex flex-col gap-4">
              {available.length ? (
                <div className="grid grid-cols-2 gap-1.5">
                  {available.map((language) => (
                    <Button key={language.tag} size="sm" variant={tag === language.tag ? 'primary' : 'tertiary'} onPress={() => setTag(language.tag)}>
                      <span lang={language.tag}>{language.native}</span>
                    </Button>
                  ))}
                </div>
              ) : <p className="text-sm text-muted">{t('预设的语言都已添加。')}</p>}
              {tag && <p className="text-sm text-muted">{t('将添加：{0}', nameOf(tag))}</p>}
            </Modal.Body>
            <Modal.Footer>
              <Button variant="tertiary" onPress={onClose}>{t('取消')}</Button>
              <Button isDisabled={!tag} isPending={busy} type="submit">{t('添加')}</Button>
            </Modal.Footer>
          </Form>
        </Modal.Dialog>
      </Modal.Container>
    </Modal.Backdrop>
  );
}
