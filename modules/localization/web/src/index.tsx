import { Globe, Plus } from '@gravity-ui/icons';
import { Button, Card, Chip, Form, Modal, Table } from '@heroui/react';
import {
  adminLanguage, api, confirmAction, EmptyBlock, ErrorBlock, findSiteLanguage, LoadingBlock, newKey, Notice, PageHeader, Panel, MiniStat, SelectInput, siteLanguages, SwitchInput,
  useAction, useHasGrant, useLoad, t,
} from '@ui';
import { useEffect, useMemo, useState } from 'react';

type Language = { tag: string; version: number; enabled: boolean; isDefault: boolean; automaticTranslation: boolean; direction: 'ltr' | 'rtl'; reflectionEnabled: boolean; batchOutputTokens: number; translationMemoryMinimumCharacters: number };
type DocState = 'translated' | 'translating' | 'retrying' | 'stopped' | 'waiting';
type Progress = { documents: { documentId: string; language: string; state: DocState; previous: boolean }[]; modelReady: boolean };
type NavNode = { documentId: string; title?: string; publication: unknown };

const names = new Intl.DisplayNames([adminLanguage()], { type: 'language' });
const nameOf = (tag: string) => { try { return names.of(tag) ?? tag; } catch { return tag; } };
// 站点要求：基准语言之外的英文始终启用并在发布时自动翻译（AI 决策模型只读英文），不能关。
const requiredEnglish = (language: Language) => !language.isDefault && /^en(-|$)/i.test(language.tag);

export default function LanguagesPage() {
  const canLanguages = useHasGrant('localization.languages');
  const canTranslate = useHasGrant('localization.manage');
  // 平台模型由站主在“模型接入”里配；站点所有者用“AI 与额度”里的自带渠道或邀请码额度。
  const operator = useHasGrant('model.manage');
  const languages = useLoad((signal) => api<{ languages: Language[] }>('/v1/languages?includeDisabled=true', { signal }).then((result) => result.languages), []);
  const [adding, setAdding] = useState(false);
  const { busy, run } = useAction();

  const setEnabled = (language: Language, enabled: boolean) => run(async () => {
    const idempotencyKey = newKey();
    await api(`/v1/languages/${encodeURIComponent(language.tag)}/enabled`, { method: 'POST', idempotencyKey, body: { tag: language.tag, enabled, expectedVersion: language.version, idempotencyKey } });
    languages.reload();
  }, enabled ? t('{0}已启用', nameOf(language.tag)) : t('{0}已停用', nameOf(language.tag)));

  if (languages.error && !languages.data) return <ErrorBlock error={languages.error} onRetry={languages.reload} />;
  const list = languages.data ?? [];
  const base = list.find((language) => language.isDefault);
  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title={t('多语言')}
        description={operator ? t('基准语言的页面直接来自文档正文；其他语言通过翻译生成。翻译会调用模型，需要先在“模型接入”里配好模型。') : t('基准语言的页面直接来自文档正文；其他语言通过翻译生成。翻译会调用模型：在“AI 与额度”里添加自带渠道或兑换 AI 邀请码即可。')}
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
                  <SwitchInput description={t('启用后访客可以切换到这个语言，所有已发布文档会自动翻译。')} isDisabled={busy || !canLanguages || requiredEnglish(language)} isSelected={language.enabled} label={t('对访客启用')} onChange={(value) => void setEnabled(language, value)} />
                  {requiredEnglish(language) && <p className="text-xs leading-5 text-muted">{t('英文是站点必需语言：始终启用，发布时自动翻译；AI 判断也只读英文。')}</p>}
                </div>
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

// 文档译文全自动：启用的语言会自动翻译每篇已发布文档，重新发布后自动更新，失败的稍后自动重试。这里只看状态。
function Translations({ base, languages }: { base: string; languages: Language[] }) {
  const [target, setTarget] = useState(languages[0]?.tag ?? '');
  useEffect(() => { if (!languages.some((language) => language.tag === target)) setTarget(languages[0]?.tag ?? ''); }, [languages, target]);
  const nodes = useLoad(async (signal) => {
    const response = await fetch(`/${base}/navigation`, { signal });
    return response.ok ? (await response.json()) as NavNode[] : [];
  }, [base]);
  const progress = useLoad((signal) => api<Progress>('/v1/translations/progress', { signal }), []);
  const operator = useHasGrant('model.manage');
  const titles = useMemo(() => new Map((nodes.data ?? []).map((node) => [node.documentId, node.title ?? node.documentId])), [nodes.data]);
  const rows = (progress.data?.documents ?? []).filter((item) => item.language === target);
  const pending = rows.filter((item) => item.state !== 'translated').length;

  useEffect(() => {
    if (!pending) return;
    const timer = setInterval(progress.reload, 8000);
    return () => clearInterval(timer);
  }, [pending, progress.reload]);

  if (!languages.length) {
    return <Panel title={t('文档译文')}><EmptyBlock icon={<Globe />} title={t('还没有启用的其他语言')} description={t('先在上面启用一个非基准语言，才能翻译文档。')} /></Panel>;
  }
  const count = (state: DocState) => rows.filter((item) => item.state === state).length;
  const labels: Record<DocState, [string, 'success' | 'accent' | 'warning' | 'danger' | 'default']> = {
    translated: [t('已翻译'), 'success'], translating: [t('翻译中'), 'accent'], retrying: [t('稍后自动重试'), 'warning'], stopped: [t('已停止，等下次发布'), 'danger'], waiting: [t('等待翻译'), 'default'],
  };
  return (
    <Panel
      title={t('文档译文')}
      description={t('启用的语言会自动翻译每篇已发布文档，重新发布后自动更新，不需要手动操作。')}
      actions={<SelectInput aria-label={t('目标语言')} className="w-48" options={languages.map((language) => ({ value: language.tag, label: t('{0}（{1}）', nameOf(language.tag), language.tag) }))} value={target} onChange={setTarget} variant="primary" />}
    >
      {progress.error ? <ErrorBlock error={progress.error} onRetry={progress.reload} /> : !progress.data ? <LoadingBlock lines={3} /> : (
        <>
          {!progress.data.modelReady && <Notice status="warning" title={t('还没有可用的翻译模型')}>{operator ? t('到“模型接入”填入 OpenRouter 密钥一键配置后，会自动开始翻译。') : t('到“AI 与额度”添加自带渠道或兑换 AI 邀请码后，会自动开始翻译。')}</Notice>}
          <div className="grid gap-3 sm:grid-cols-3">
            <MiniStat label={t('已翻译')} value={count('translated')} tone="success" />
            <MiniStat label={t('翻译中')} value={count('translating')} tone="accent" />
            <MiniStat label={t('等待或重试')} value={count('waiting') + count('retrying')} tone="warning" />
          </div>
          <Table variant="secondary">
            <Table.ScrollContainer>
              <Table.Content aria-label={t('已发布文档')} className="min-w-[480px]">
                <Table.Header>
                  <Table.Column isRowHeader>{t('已发布文档')}</Table.Column>
                  <Table.Column className="w-48 text-end">{t('状态')}</Table.Column>
                </Table.Header>
                <Table.Body renderEmptyState={() => <p className="py-6 text-center text-sm text-muted">{t('还没有已发布文档。')}</p>}>
                  {rows.map((item) => (
                    <Table.Row key={item.documentId} id={item.documentId}>
                      <Table.Cell>{titles.get(item.documentId) ?? item.documentId}</Table.Cell>
                      <Table.Cell className="text-end">
                        <span className="inline-flex flex-wrap justify-end gap-1">
                          <Chip color={labels[item.state][1]} size="sm" variant="soft">{labels[item.state][0]}</Chip>
                          {item.previous && <Chip size="sm" variant="soft">{t('暂时显示旧译文')}</Chip>}
                        </span>
                      </Table.Cell>
                    </Table.Row>
                  ))}
                </Table.Body>
              </Table.Content>
            </Table.ScrollContainer>
          </Table>
        </>
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
