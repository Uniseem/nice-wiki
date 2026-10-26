import { Plus, TrashBin } from '@gravity-ui/icons';
import { Button, Chip, Form, Modal, Table } from '@heroui/react';
import { api, confirmAction, currentSite, EmptyBlock, ErrorBlock, fmtDate, fmtMoney, LoadingBlock, Notice, NumberInput, PageHeader, Panel, Stat, SwitchInput, TextInput, useAction, useCurrentSite, useLoad, t } from '@ui';
import { useState } from 'react';

type Entitlement = { entitled: boolean; capMicros: number; period?: 'once' | 'monthly'; tasks: string[]; redeemedAt?: string; periodStart?: string };
type Usage = { platformAllowed: boolean; platformOwner: boolean; capMicros: number; periodStart: string; spentMicros: number; heldMicros: number; unknownCount: number };
type Channel = { channelId: string; name: string; baseUrl: string; model: string; tasks: string[]; capabilities: string[]; keyLast4: string; createdAt: string; probe: { ok: boolean; chat: boolean; tools: boolean; latencyMs: number; message: string } };

// 任务所属服务的中文名：自带渠道可以只用于其中一部分。
const taskLabels: Record<string, string> = { qa: t('访客问答'), localization: t('翻译'), knowledge: t('知识库索引'), memory: t('长期记忆助手'), content: t('文档标题与摘要') };

export default function MyAiPage() {
  const site = useCurrentSite();
  const mine = useLoad((signal) => api<{ ai: Entitlement }>('/v1/me/ai', { signal }).then((result) => result.ai), []);
  const usage = useLoad((signal) => (site ? api<{ usage: Usage }>('/v1/ai/usage', { signal }).then((result) => result.usage).catch(() => null) : Promise.resolve(null)), [site]);
  const channels = useLoad((signal) => api<{ channels: Channel[] }>('/v1/ai/channels', { signal }).then((result) => result.channels), []);
  const [code, setCode] = useState('');
  const [adding, setAdding] = useState(false);
  const { busy, run } = useAction();

  const redeem = async () => {
    const done = await run(() => api('/v1/me/ai:redeem', { method: 'POST', body: { code: code.trim() } }), t('邀请码已兑换'));
    if (done !== undefined) { setCode(''); mine.reload(); usage.reload(); }
  };
  const remove = async (channel: Channel) => {
    const ok = await confirmAction({ title: t('删除渠道“{0}”？', channel.name), body: t('key 会被销毁，之后的 AI 请求不再走这个渠道。'), confirmLabel: t('删除'), tone: 'danger' });
    if (!ok) return;
    await run(() => api(`/v1/ai/channels/${channel.channelId}`, { method: 'DELETE' }), t('渠道已删除'));
    channels.reload();
  };

  const ai = mine.data;
  return (
    <div className="flex flex-col gap-6">
      <PageHeader title={t('AI 与额度')} description={t('访客问答、翻译、知识库索引这些 AI 功能，优先用你自己的 key；没有自带 key 或 key 不可用时，用平台额度。')} />
      <Panel title={t('平台额度')} description={t('兑换站主发的 AI 邀请码得到额度；新码会替换旧码，旧码剩余的额度作废。')}>
        <div className="flex flex-col gap-4">
          {mine.error ? <ErrorBlock error={mine.error} onRetry={mine.reload} /> : !ai ? <LoadingBlock lines={2} /> : (
            <div className="grid gap-3 sm:grid-cols-3">
              <Stat label={t('额度上限')} value={ai.entitled ? fmtMoney(ai.capMicros) : t('未兑换')} hint={ai.period === 'monthly' ? t('每月重置') : ai.entitled ? t('一次性') : undefined} />
              <Stat label={t('已用（当前站点所有者）')} value={usage.data ? fmtMoney(usage.data.spentMicros) : '—'} hint={usage.data?.heldMicros ? t('另有 {0} 在途', fmtMoney(usage.data.heldMicros)) : undefined} />
              <Stat label={t('周期起点')} value={ai.periodStart ? fmtDate(ai.periodStart) : '—'} />
            </div>
          )}
          {ai?.entitled && ai.tasks.length > 0 && <p className="text-sm text-muted">{t('平台渠道可用于：{0}', ai.tasks.map((task) => taskLabels[task] ?? task).join(' / '))}</p>}
          <Form className="flex items-end gap-2" onSubmit={(event) => { event.preventDefault(); void redeem(); }}>
            <TextInput className="max-w-sm" label={t('AI 邀请码')} placeholder="nwa-…" value={code} onChange={setCode} />
            <Button isDisabled={!code.trim()} isPending={busy} type="submit">{t('兑换')}</Button>
          </Form>
        </div>
      </Panel>
      <Panel
        title={t('自带渠道')}
        description={t('填 OpenAI 兼容的地址、key 和模型名（OpenRouter、DeepSeek 等都行）。添加时会试调用一次；key 保存后不再显示。')}
        actions={<Button size="sm" onPress={() => setAdding(true)}><Plus />{t('添加渠道')}</Button>}
      >
        {channels.error ? <ErrorBlock error={channels.error} onRetry={channels.reload} /> : !channels.data ? <LoadingBlock lines={2} /> : (
          <Table>
            <Table.ScrollContainer>
              <Table.Content aria-label={t('自带渠道')} className="min-w-[640px]">
                <Table.Header>
                  <Table.Column isRowHeader>{t('渠道')}</Table.Column>
                  <Table.Column>{t('模型')}</Table.Column>
                  <Table.Column>{t('能力')}</Table.Column>
                  <Table.Column className="w-16 text-end">{t('操作')}</Table.Column>
                </Table.Header>
                <Table.Body renderEmptyState={() => <EmptyBlock className="m-4" title={t('还没有自带渠道')} description={t('没有的话，AI 功能只用平台额度。')} />}>
                  {channels.data.map((channel) => (
                    <Table.Row key={channel.channelId} id={channel.channelId}>
                      <Table.Cell><span className="block font-medium text-foreground">{channel.name}</span><span className="block text-xs text-muted">{channel.baseUrl} · key …{channel.keyLast4}</span></Table.Cell>
                      <Table.Cell>{channel.model}</Table.Cell>
                      <Table.Cell>
                        <span className="flex flex-wrap gap-1">
                          <Chip size="sm" variant="soft">{t('对话')}</Chip>
                          {channel.probe.tools && <Chip size="sm" variant="soft">{t('工具调用')}</Chip>}
                          {channel.capabilities.includes('vision') && <Chip size="sm" variant="soft">{t('看图')}</Chip>}
                          {channel.tasks.length > 0 && <Chip size="sm" variant="soft">{channel.tasks.map((task) => taskLabels[task] ?? task).join(' / ')}</Chip>}
                        </span>
                      </Table.Cell>
                      <Table.Cell className="text-end"><Button isIconOnly aria-label={t('删除')} isDisabled={busy} size="sm" variant="ghost" onPress={() => void remove(channel)}><TrashBin /></Button></Table.Cell>
                    </Table.Row>
                  ))}
                </Table.Body>
              </Table.Content>
            </Table.ScrollContainer>
          </Table>
        )}
      </Panel>
      {!currentSite() && <Notice status="default">{t('选中一个站点后，这里会显示它的额度用量。')}</Notice>}
      {adding && <AddChannelDialog onClose={() => setAdding(false)} onSaved={() => { setAdding(false); channels.reload(); }} />}
    </div>
  );
}

function AddChannelDialog({ onClose, onSaved }: { onClose: () => void; onSaved: () => void }) {
  const [name, setName] = useState('');
  const [baseUrl, setBaseUrl] = useState('https://');
  const [apiKey, setApiKey] = useState('');
  const [model, setModel] = useState('');
  const [vision, setVision] = useState(false);
  const [maxOutput, setMaxOutput] = useState(4096);
  const { busy, run } = useAction();
  const submit = async () => {
    const done = await run(() => api('/v1/ai/channels', { method: 'POST', body: { name, baseUrl: baseUrl.trim(), apiKey: apiKey.trim(), model: model.trim(), vision, maxOutputTokens: maxOutput } }), t('渠道已添加'));
    if (done !== undefined) onSaved();
  };
  return (
    <Modal.Backdrop isOpen onOpenChange={(open) => { if (!open) onClose(); }}>
      <Modal.Container size="md">
        <Modal.Dialog>
          <Modal.CloseTrigger aria-label={t('关闭')} />
          <Modal.Header><Modal.Heading>{t('添加自带渠道')}</Modal.Heading></Modal.Header>
          <Form className="flex flex-col" onSubmit={(event) => { event.preventDefault(); void submit(); }}>
            <Modal.Body className="flex flex-col gap-4">
              <TextInput isRequired autoFocus label={t('名称')} maxLength={100} placeholder="DeepSeek" value={name} onChange={setName} />
              <TextInput isRequired label={t('接口地址')} placeholder="https://api.deepseek.com/v1" value={baseUrl} onChange={setBaseUrl} description={t('OpenAI 兼容的根地址，不能指向内网或本机。')} />
              <TextInput isRequired autoComplete="off" label="API key" type="password" value={apiKey} onChange={setApiKey} />
              <TextInput isRequired label={t('模型名')} placeholder="deepseek-chat" value={model} onChange={setModel} />
              <NumberInput label={t('单次输出上限（token）')} minValue={64} maxValue={200000} value={maxOutput} onChange={setMaxOutput} />
              <SwitchInput isSelected={vision} label={t('支持看图')} onChange={setVision} />
            </Modal.Body>
            <Modal.Footer>
              <Button slot="close" variant="secondary" onPress={onClose}>{t('取消')}</Button>
              <Button isPending={busy} type="submit">{t('测试并保存')}</Button>
            </Modal.Footer>
          </Form>
        </Modal.Dialog>
      </Modal.Container>
    </Modal.Backdrop>
  );
}
