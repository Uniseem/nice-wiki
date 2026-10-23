import { ArrowDown, ArrowUp, ChevronDown, FileText, Plus, Sparkles, Stop, ThumbsDown, ThumbsUp } from '@gravity-ui/icons';
import { Button, cn, ScrollShadow, Spinner, TextArea, Tooltip } from '@heroui/react';
import { useEffect, useLayoutEffect, useRef, useState, type RefObject } from 'react';
import { cancelTurn, followTurn, QaError, resetConversation, sendFeedback, startTurn, transient, type Citation, type Step, type TurnInfo } from './client';
import { Markdown } from './markdown';

export type Translate = (key: string) => string;

type Message = {
  id: string;
  role: 'user' | 'assistant';
  text: string;
  status: 'streaming' | 'done' | 'error' | 'cancelled';
  citations: Citation[];
  steps?: StepView[];
  turn?: TurnInfo;
  feedback?: 'helpful' | 'unhelpful';
  live?: boolean; // 本次打开后新提问得到的回答：逐字写出（命中缓存时整段答案一次到达，也照样逐字写）
};

// 触屏设备（手机、平板）上从不主动聚焦输入框：聚焦会直接弹出软键盘、挡住大半屏，等访客自己点输入框。
// 发送后收起键盘，让出地方看回答。
const touch = matchMedia('(pointer: coarse)').matches;

// 对话记录和会话一样按语言分开保存。
const logKey = (locale: string) => `nw-qa-log:${locale}`;

function loadLog(locale: string): Message[] {
  try {
    const saved = JSON.parse(sessionStorage.getItem(logKey(locale)) ?? '[]') as Message[];
    return saved.map((message) => ({ ...message, live: false, status: message.status === 'streaming' ? 'cancelled' : message.status }));
  } catch { return []; }
}

// 服务暂时不可用（发版重启等）时客户端已经自动重试过一阵，仍不行就给一句友好提示，不显示错误码。
const busy = (t: Translate) => t('服务正在更新，请稍后再试。');

function failure(t: Translate, problem: unknown): string {
  if (transient(problem)) return busy(t);
  if (problem instanceof QaError) {
    if (problem.code === 'challenge-failed') return t('人机验证没有完成');
    if (problem.code === 'challenge-unsupported') return t('人机验证无法在此浏览器运行');
    if (problem.code === 'STREAM') return `${t('回答连接失败（')}${problem.status}${t('），请稍后再试。')}`;
    return `${t('请求失败（')}${problem.code || problem.status}${t('），请稍后再试。')}`;
  }
  return (problem as Error)?.message || t('请求失败（');
}

// 同一浏览器标签里，同一语言的文档页右下角面板和独立问答页共用一个会话和对话记录。
// documentId 是访客所在页面：每次提问都带上，AI 会先读这一页全文再回答。
// 滚动只发生在对话区里，从不滚动页面：提问后把这一轮滚到对话区顶部，回答逐字写出时不自动往下滚，
// 访客可以从头读；最新内容在可视范围外时显示“回到最新”。
// compact（首页的“问问 AI”）：还没有对话时只显示一个输入框；开始提问后和面板一样显示对话区，由外层给出高度。
export function QaChat({ locale, t, autoFocus, className, documentId, pageTitle, compact }: { locale: string; t: Translate; autoFocus?: boolean; className?: string; documentId?: string; pageTitle?: string; compact?: boolean }) {
  const [messages, setMessages] = useState<Message[]>(() => loadLog(locale));
  const [question, setQuestion] = useState('');
  const [running, setRunning] = useState<{ turnId?: string; controller: AbortController } | null>(null);
  // pinned 是本次打开后最新提问的那一轮：它至少占满对话区可视高度，问题才能滚到顶部、回答在下方展开。
  const [pinned, setPinned] = useState('');
  const [view, setView] = useState(0);
  const [behind, setBehind] = useState(false);
  const scroller = useRef<HTMLDivElement>(null);
  const list = useRef<HTMLOListElement>(null);
  const end = useRef<HTMLDivElement>(null);
  const input = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    try { sessionStorage.setItem(logKey(locale), JSON.stringify(messages.slice(-30))); } catch { /* 隐私模式 */ }
  }, [messages, locale]);

  // 最新回答的末尾在可视范围之下时显示“回到最新”。
  const check = () => {
    const box = scroller.current;
    const mark = end.current;
    setBehind(!!box && !!mark && mark.getBoundingClientRect().top > box.getBoundingClientRect().bottom + 1);
  };

  const idle = !!compact && !messages.length;

  // 打开时停在对话末尾；之后跟踪对话区高度（新一轮的最小高度）和内容变化（逐字输出时更新“回到最新”）。
  // compact 形态的对话区在第一次提问后才出现，那时再挂上。
  useLayoutEffect(() => {
    const box = scroller.current;
    if (!box) return;
    box.scrollTop = box.scrollHeight;
    const observer = new ResizeObserver(() => { setView(box.clientHeight); check(); });
    observer.observe(box);
    if (list.current) observer.observe(list.current);
    return () => observer.disconnect();
  }, [idle]);

  // 提问后把这一轮平滑滚到对话区顶部。
  useLayoutEffect(() => {
    const box = scroller.current;
    const turn = pinned ? box?.querySelector<HTMLElement>(`[data-turn="${pinned}"]`) : null;
    if (!box || !turn) return;
    box.scrollTo({ top: box.scrollTop + turn.getBoundingClientRect().top - box.getBoundingClientRect().top - 16, behavior: 'smooth' });
  }, [pinned]);

  const focus = () => { if (!touch) input.current?.focus({ preventScroll: true }); };
  useEffect(() => { if (autoFocus) focus(); }, [autoFocus]);

  const patch = (id: string, update: (message: Message) => Message) => setMessages((current) => current.map((message) => message.id === id ? update(message) : message));

  function latest() {
    const box = scroller.current;
    const mark = end.current;
    if (box && mark) box.scrollTo({ top: box.scrollTop + mark.getBoundingClientRect().top - box.getBoundingClientRect().bottom + 24, behavior: 'smooth' });
  }

  async function ask() {
    const text = question.trim();
    if (!text || running) return;
    const id = crypto.randomUUID();
    const controller = new AbortController();
    setQuestion('');
    if (touch) input.current?.blur();
    setRunning({ controller });
    setPinned(id);
    setMessages((current) => [...current, { id: `${id}-q`, role: 'user', text, status: 'done', citations: [] }, { id, role: 'assistant', text: '', status: 'streaming', citations: [], live: true }]);
    try {
      const started = await startTurn(text, locale, documentId);
      setRunning({ controller, turnId: started.turn.turnId });
      patch(id, (message) => ({ ...message, turn: started.turn }));
      await followTurn(started, (event) => {
        if (event.kind === 'step') patch(id, (message) => ({ ...message, steps: mergeStep(message.steps ?? [], event.step) }));
        else if (event.kind === 'delta') patch(id, (message) => ({ ...message, text: message.text + event.text }));
        else if (event.kind === 'reset') patch(id, (message) => ({ ...message, text: '' }));
        else if (event.kind === 'citation') patch(id, (message) => ({ ...message, citations: [...message.citations, event.citation] }));
        else if (event.kind === 'done') patch(id, (message) => ({ ...message, status: 'done', turn: event.turn?.turnId ? event.turn : message.turn }));
        else if (event.kind === 'cancelled') patch(id, (message) => ({ ...message, status: 'cancelled' }));
        else patch(id, (message) => ({ ...message, status: 'error', text: event.code === 'DEPENDENCY_UNAVAILABLE' ? busy(t) : `${t('这次没有回答成功（')}${event.code || 'error'}${t('），请稍后再试。')}` }));
      }, controller.signal);
    } catch (problem) {
      if (!controller.signal.aborted) patch(id, (message) => ({ ...message, status: 'error', text: failure(t, problem) }));
      else patch(id, (message) => ({ ...message, status: 'cancelled' }));
    } finally {
      setRunning(null);
      focus();
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
    setPinned('');
    focus();
  }

  async function rate(message: Message, value: 'helpful' | 'unhelpful') {
    if (!message.turn || message.feedback) return;
    patch(message.id, (current) => ({ ...current, feedback: value }));
    await sendFeedback(message.turn, value).catch(() => patch(message.id, (current) => ({ ...current, feedback: undefined })));
  }

  const rounds = group(messages);
  return (
    <div className={cn('flex min-h-0 flex-1 flex-col', className)}>
      {!idle && <div className="relative flex min-h-0 flex-1 flex-col">
        <ScrollShadow ref={scroller} className="min-h-0 flex-1 px-4 py-4" onScroll={check}>
          {!messages.length && (
            <div className="flex h-full flex-col items-center justify-center gap-3 py-10 text-center">
              <span className="flex size-11 items-center justify-center rounded-2xl bg-accent-soft text-accent-soft-foreground"><Sparkles className="size-5" /></span>
              <p className="max-w-64 text-sm text-muted">{t('问问这个站点的文档…')}</p>
            </div>
          )}
          <ol ref={list} className="mx-auto flex w-full max-w-3xl flex-col gap-6">
            {rounds.map((round, index) => (
              <li key={round.id} className="flex flex-col gap-4" data-turn={round.id} style={round.id === pinned && view ? { minHeight: view - 32 } : undefined}>
                {round.question && <div className="ms-10 self-end whitespace-pre-wrap break-words rounded-2xl rounded-ee-md bg-accent px-3.5 py-2 text-sm leading-6 text-accent-foreground">{round.question.text}</div>}
                {round.answer && <Answer last={index === rounds.length - 1} message={round.answer} t={t} end={end} onRate={rate} />}
              </li>
            ))}
          </ol>
        </ScrollShadow>
        {behind && (
          <Button className="absolute bottom-3 left-1/2 -translate-x-1/2 rounded-full shadow-overlay" size="sm" variant="tertiary" onPress={latest}>
            <ArrowDown />{t('回到最新')}
          </Button>
        )}
      </div>}
      <form className={idle ? '' : 'border-t border-separator p-3'} onSubmit={(event) => { event.preventDefault(); void ask(); }}>
        <div className="mx-auto w-full max-w-3xl">
          {documentId && pageTitle && (
            <div className="mb-2 flex min-w-0 items-center gap-1.5 px-1 text-xs text-muted">
              <FileText className="size-3.5 shrink-0" />
              <span className="shrink-0">{t('当前页面')}</span>
              <span className="truncate font-medium text-foreground/80">{pageTitle}</span>
            </div>
          )}
          <div className={cn('flex items-end gap-2 border border-border bg-field shadow-field focus-within:border-field-border-focus', idle ? 'items-center rounded-full py-2 pe-2 ps-4' : 'rounded-2xl p-1.5')}>
            {idle && <Sparkles className="size-5 shrink-0 text-accent" />}
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
              <Button isIconOnly aria-label={t('发送')} className={cn(idle && 'rounded-full')} isDisabled={!question.trim()} size={idle ? 'md' : 'sm'} type="submit"><ArrowUp /></Button>
            )}
          </div>
          <div className={cn('mt-2 flex items-center justify-between gap-2 px-1', idle && 'hidden')}>
            <span className="text-[11px] leading-4 text-muted">{t('回答由 AI 生成，可能有误，请以文档原文为准。')}</span>
            {messages.length > 0 && <Button className="shrink-0" size="sm" variant="ghost" onPress={restart}><Plus />{t('新对话')}</Button>}
          </div>
        </div>
      </form>
    </div>
  );
}

// 一轮对话：访客的问题（ID 为回答 ID 加 -q）和对应的回答。
function group(messages: Message[]): { id: string; question?: Message; answer?: Message }[] {
  const rounds: { id: string; question?: Message; answer?: Message }[] = [];
  for (const message of messages) {
    const last = rounds[rounds.length - 1];
    if (message.role === 'assistant' && last && !last.answer) last.answer = message;
    else rounds.push(message.role === 'user' ? { id: message.id.replace(/-q$/, ''), question: message } : { id: message.id, answer: message });
  }
  return rounds;
}

// 一条回答：思考过程、正文（流式时逐字写出）、引用和反馈；最后一条回答末尾放一个标记，用来判断最新内容是否可见。
function Answer({ message, last, end, t, onRate }: { message: Message; last: boolean; end: RefObject<HTMLDivElement | null>; t: Translate; onRate: (message: Message, value: 'helpful' | 'unhelpful') => void }) {
  return (
    <div className="me-6 flex flex-col gap-2">
      {!!message.steps?.length && <Steps active={message.status === 'streaming'} steps={message.steps} t={t} />}
      <div className={cn('rounded-2xl rounded-es-md px-3.5 py-2.5 text-sm', message.status === 'error' ? 'bg-danger-soft text-danger-soft-foreground' : 'bg-surface-secondary text-foreground', message.status === 'streaming' && !message.text && message.steps?.length && 'hidden')}>
        {message.status === 'streaming' && !message.text ? (
          <span className="flex items-center gap-2 text-muted"><Spinner color="current" size="sm" />{t('思考中…')}</span>
        ) : message.status === 'error' ? message.text : <Typed message={message} />}
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
                <Button isIconOnly aria-label={t('有帮助')} size="sm" variant="ghost" onPress={() => onRate(message, 'helpful')}><ThumbsUp /></Button>
                <Tooltip.Content>{t('有帮助')}</Tooltip.Content>
              </Tooltip>
              <Tooltip delay={300}>
                <Button isIconOnly aria-label={t('没帮助')} size="sm" variant="ghost" onPress={() => onRate(message, 'unhelpful')}><ThumbsDown /></Button>
                <Tooltip.Content>{t('没帮助')}</Tooltip.Content>
              </Tooltip>
            </>
          )}
        </div>
      )}
      {last && <div ref={end} />}
    </div>
  );
}

// 打字效果：流式收到的文字先进缓冲，每 32 毫秒放出一段，积压越多放得越快（大约落后 0.3 秒，最快每秒约 500 字），
// 命中缓存时整段答案一次到达，也照样逐字写出；之前的对话记录直接整段显示。
function Typed({ message }: { message: Message }) {
  const [shown, setShown] = useState(message.live ? 0 : message.text.length);
  const length = message.text.length;
  useEffect(() => {
    if (shown > length) { setShown(length); return; }
    if (shown === length) return;
    const timer = window.setTimeout(() => setShown((count) => Math.min(length, count + Math.min(16, Math.max(2, Math.ceil((length - count) / 10))))), 32);
    return () => window.clearTimeout(timer);
  }, [shown, length]);
  const typing = message.status === 'streaming' || shown < length;
  return (
    <div className={cn(typing && 'nw-typing')}>
      <Markdown citations={message.citations} text={message.text.slice(0, shown)} />
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

// StepView 在服务端步骤上加本地计时：第一次收到时记下开始时间，完成或跳过时算出耗时（毫秒）。
type StepView = Step & { startedAt?: number; ms?: number };

// mergeStep 按步骤类型更新：同一步从“进行中”变成“完成/跳过”，新步骤接在后面。
function mergeStep(steps: StepView[], step: Step): StepView[] {
  const now = Date.now();
  const index = steps.findIndex((item) => item.key === step.key);
  if (index < 0) return [...steps, { ...step, startedAt: now, ms: step.status === 'running' ? undefined : 0 }];
  const next = [...steps];
  const previous = steps[index];
  next[index] = { ...previous, ...step, ms: step.status !== 'running' && previous.startedAt ? now - previous.startedAt : previous.ms };
  return next;
}

// 耗时太短（例如刷新后重放历史事件）不显示。
function seconds(ms?: number): string {
  return ms && ms >= 100 ? `${(ms / 1000).toFixed(1)}s` : '';
}

// Steps 是一条回答的思考过程：理解问题、阅读当前页、检索、生成。回答进行中展开，结束后自动收起，可再点开。
function Steps({ steps, active, t }: { steps: StepView[]; active: boolean; t: Translate }) {
  const [open, setOpen] = useState(active);
  const [wasActive, setWasActive] = useState(active);
  if (wasActive !== active) {
    setWasActive(active);
    if (!active) setOpen(false);
  }
  const labels: Record<Step['key'], string> = { understand: t('理解问题'), page: t('阅读当前页面'), cache: t('查找已有回答'), search: t('检索相关文档'), answer: t('整理回答') };
  // 复用已有回答这一步命中时改叫“命中缓存”，答案直接来自之前的回答。
  const label = (step: StepView) => (step.key === 'cache' && step.status === 'done' ? t('命中缓存') : labels[step.key]);
  return (
    <div className="nw-qa-steps rounded-xl border border-border/70 bg-surface px-3 py-2 text-xs">
      <button aria-expanded={open} className="flex w-full items-center gap-2 text-start text-muted" type="button" onClick={() => setOpen(!open)}>
        {active ? <Spinner color="current" size="sm" /> : <Sparkles className="size-3.5 text-accent" />}
        <span className="flex-1 font-medium">{active ? label(steps[steps.length - 1]) : t('思考过程')}</span>
        {!active && <span className="tabular-nums text-muted">{seconds(steps.reduce((total, step) => total + (step.ms ?? 0), 0))}</span>}
        <ChevronDown className={cn('size-3.5 transition-transform', open && 'rotate-180')} />
      </button>
      {open && (
        <ol className="mt-2 flex flex-col gap-2 border-s border-border ps-3">
          {steps.map((step) => (
            <li key={step.key} className="nw-qa-step relative flex flex-col gap-1">
              <span className={cn('absolute -start-[17px] top-1 size-2 rounded-full ring-2 ring-surface', step.status === 'running' ? 'animate-pulse bg-accent' : step.status === 'done' ? 'bg-success' : 'bg-default')} />
              <span className="flex items-baseline gap-2">
                <span className={cn('flex-1 font-medium', step.status === 'skipped' ? 'text-muted line-through' : 'text-foreground')}>{label(step)}</span>
                <span className="tabular-nums text-muted">{seconds(step.ms)}</span>
              </span>
              {step.detail && step.key !== 'answer' && <span className="line-clamp-2 break-words text-muted">{step.detail}</span>}
              {!!step.items?.length && (
                <span className="flex flex-wrap gap-1">
                  {step.items.map((item) => <span key={item} className="max-w-full truncate rounded-md bg-default px-1.5 py-0.5 text-[11px] text-foreground/80">{item}</span>)}
                </span>
              )}
            </li>
          ))}
        </ol>
      )}
    </div>
  );
}
