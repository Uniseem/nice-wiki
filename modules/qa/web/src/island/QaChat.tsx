import { ArrowUp, Plus, Sparkles, Stop, ThumbsDown, ThumbsUp } from '@gravity-ui/icons';
import { Button, cn, ScrollShadow, Spinner, TextArea, Tooltip } from '@heroui/react';
import { useEffect, useRef, useState } from 'react';
import { cancelTurn, followTurn, QaError, resetConversation, sendFeedback, startTurn, type Citation, type TurnInfo } from './client';
import { Markdown } from './markdown';

export type Translate = (key: string) => string;

type Message = {
  id: string;
  role: 'user' | 'assistant';
  text: string;
  status: 'streaming' | 'done' | 'error' | 'cancelled';
  citations: Citation[];
  turn?: TurnInfo;
  feedback?: 'helpful' | 'unhelpful';
};

// 对话记录和会话一样按语言分开保存。
const logKey = (locale: string) => `nw-qa-log:${locale}`;

function loadLog(locale: string): Message[] {
  try {
    const saved = JSON.parse(sessionStorage.getItem(logKey(locale)) ?? '[]') as Message[];
    return saved.map((message) => message.status === 'streaming' ? { ...message, status: 'cancelled' } : message);
  } catch { return []; }
}

function failure(t: Translate, problem: unknown): string {
  if (problem instanceof QaError) {
    if (problem.code === 'challenge-failed') return t('人机验证没有完成');
    if (problem.code === 'challenge-unsupported') return t('人机验证无法在此浏览器运行');
    if (problem.code === 'STREAM') return `${t('回答连接失败（')}${problem.status}${t('），请稍后再试。')}`;
    return `${t('请求失败（')}${problem.code || problem.status}${t('），请稍后再试。')}`;
  }
  return (problem as Error)?.message || t('请求失败（');
}

// 同一浏览器标签里，同一语言的文档页右下角面板和独立问答页共用一个会话和对话记录。
export function QaChat({ locale, t, autoFocus, className }: { locale: string; t: Translate; autoFocus?: boolean; className?: string }) {
  const [messages, setMessages] = useState<Message[]>(() => loadLog(locale));
  const [question, setQuestion] = useState('');
  const [running, setRunning] = useState<{ turnId?: string; controller: AbortController } | null>(null);
  const bottom = useRef<HTMLDivElement>(null);
  const input = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    try { sessionStorage.setItem(logKey(locale), JSON.stringify(messages.slice(-30))); } catch { /* 隐私模式 */ }
    bottom.current?.scrollIntoView({ block: 'end' });
  }, [messages, locale]);

  useEffect(() => { if (autoFocus) input.current?.focus(); }, [autoFocus]);

  const patch = (id: string, update: (message: Message) => Message) => setMessages((current) => current.map((message) => message.id === id ? update(message) : message));

  async function ask() {
    const text = question.trim();
    if (!text || running) return;
    const id = crypto.randomUUID();
    const controller = new AbortController();
    setQuestion('');
    setRunning({ controller });
    setMessages((current) => [...current, { id: `${id}-q`, role: 'user', text, status: 'done', citations: [] }, { id, role: 'assistant', text: '', status: 'streaming', citations: [] }]);
    try {
      const started = await startTurn(text, locale);
      setRunning({ controller, turnId: started.turn.turnId });
      patch(id, (message) => ({ ...message, turn: started.turn }));
      await followTurn(started, (event) => {
        if (event.kind === 'delta') patch(id, (message) => ({ ...message, text: message.text + event.text }));
        else if (event.kind === 'citation') patch(id, (message) => ({ ...message, citations: [...message.citations, event.citation] }));
        else if (event.kind === 'done') patch(id, (message) => ({ ...message, status: 'done', turn: event.turn?.turnId ? event.turn : message.turn }));
        else if (event.kind === 'cancelled') patch(id, (message) => ({ ...message, status: 'cancelled' }));
        else patch(id, (message) => ({ ...message, status: 'error', text: `${t('这次没有回答成功（')}${event.code || 'error'}${t('），请稍后再试。')}` }));
      }, controller.signal);
    } catch (problem) {
      if (!controller.signal.aborted) patch(id, (message) => ({ ...message, status: 'error', text: failure(t, problem) }));
      else patch(id, (message) => ({ ...message, status: 'cancelled' }));
    } finally {
      setRunning(null);
      input.current?.focus();
    }
  }

  async function stop() {
    if (!running) return;
    running.controller.abort();
    if (running.turnId) await cancelTurn(running.turnId).catch(() => {});
  }

  function restart() {
    running?.controller.abort();
    resetConversation(locale);
    setMessages([]);
    input.current?.focus();
  }

  async function rate(message: Message, value: 'helpful' | 'unhelpful') {
    if (!message.turn || message.feedback) return;
    patch(message.id, (current) => ({ ...current, feedback: value }));
    await sendFeedback(message.turn, value).catch(() => patch(message.id, (current) => ({ ...current, feedback: undefined })));
  }

  return (
    <div className={cn('flex min-h-0 flex-1 flex-col', className)}>
      <ScrollShadow className="min-h-0 flex-1 px-4 py-4" hideScrollBar>
        {!messages.length ? (
          <div className="flex h-full flex-col items-center justify-center gap-3 py-10 text-center">
            <span className="flex size-11 items-center justify-center rounded-2xl bg-accent-soft text-accent-soft-foreground"><Sparkles className="size-5" /></span>
            <p className="max-w-64 text-sm text-muted">{t('问问这个站点的文档…')}</p>
          </div>
        ) : (
          <ol className="flex flex-col gap-4">
            {messages.map((message) => message.role === 'user' ? (
              <li key={message.id} className="ms-10 self-end whitespace-pre-wrap break-words rounded-2xl rounded-ee-md bg-accent px-3.5 py-2 text-sm leading-6 text-accent-foreground">{message.text}</li>
            ) : (
              <li key={message.id} className="me-6 flex flex-col gap-2">
                <div className={cn('rounded-2xl rounded-es-md px-3.5 py-2.5 text-sm', message.status === 'error' ? 'bg-danger-soft text-danger-soft-foreground' : 'bg-surface-secondary text-foreground')}>
                  {message.status === 'streaming' && !message.text ? (
                    <span className="flex items-center gap-2 text-muted"><Spinner color="current" size="sm" />{t('思考中…')}</span>
                  ) : message.status === 'error' ? message.text : <Markdown citations={message.citations} text={message.text} />}
                </div>
                {message.citations.length > 0 && (
                  <div className="flex flex-wrap gap-1.5 ps-1">
                    {unique(message.citations).map((citation) => (
                      <a key={citation.href} className="chip chip--default chip--soft chip--sm max-w-full truncate no-underline hover:bg-default-hover" href={citation.href}>{citation.title || citation.href}</a>
                    ))}
                  </div>
                )}
                {message.status === 'done' && message.turn && (
                  <div className="flex items-center gap-1 ps-1 text-xs text-muted">
                    {message.feedback ? <span>{t('感谢反馈')}</span> : (
                      <>
                        <Tooltip delay={300}>
                          <Button isIconOnly aria-label={t('有帮助')} size="sm" variant="ghost" onPress={() => rate(message, 'helpful')}><ThumbsUp /></Button>
                          <Tooltip.Content>{t('有帮助')}</Tooltip.Content>
                        </Tooltip>
                        <Tooltip delay={300}>
                          <Button isIconOnly aria-label={t('没帮助')} size="sm" variant="ghost" onPress={() => rate(message, 'unhelpful')}><ThumbsDown /></Button>
                          <Tooltip.Content>{t('没帮助')}</Tooltip.Content>
                        </Tooltip>
                      </>
                    )}
                  </div>
                )}
              </li>
            ))}
          </ol>
        )}
        <div ref={bottom} />
      </ScrollShadow>
      <form className="border-t border-separator p-3" onSubmit={(event) => { event.preventDefault(); void ask(); }}>
        <div className="flex items-end gap-2 rounded-2xl border border-border bg-field p-1.5 shadow-field focus-within:border-field-border-focus">
          <TextArea
            ref={input}
            aria-label={t('问问这个站点的文档…')}
            className="max-h-40 min-h-9 flex-1 resize-none border-0 bg-transparent px-2 py-1.5 shadow-none outline-none focus:ring-0"
            maxLength={500}
            placeholder={t('问问这个站点的文档…')}
            rows={1}
            value={question}
            onChange={(event) => setQuestion(event.target.value)}
            onKeyDown={(event) => { if (event.key === 'Enter' && !event.shiftKey && !event.nativeEvent.isComposing) { event.preventDefault(); void ask(); } }}
          />
          {running ? (
            <Button isIconOnly aria-label={t('停止')} size="sm" variant="secondary" onPress={stop}><Stop /></Button>
          ) : (
            <Button isIconOnly aria-label={t('发送')} isDisabled={!question.trim()} size="sm" type="submit"><ArrowUp /></Button>
          )}
        </div>
        <div className="mt-2 flex items-center justify-between gap-2 px-1">
          <span className="text-[11px] leading-4 text-muted">{t('回答由 AI 生成，可能有误，请以文档原文为准。')}</span>
          {messages.length > 0 && <Button className="shrink-0" size="sm" variant="ghost" onPress={restart}><Plus />{t('新对话')}</Button>}
        </div>
      </form>
    </div>
  );
}

function unique(citations: Citation[]): Citation[] {
  const seen = new Set<string>();
  return citations.filter((citation) => {
    const key = (citation.href ?? '').split('#')[0];
    if (!key || seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}
