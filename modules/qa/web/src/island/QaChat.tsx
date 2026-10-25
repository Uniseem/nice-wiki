import { ArrowDownIcon as ArrowDown, ArrowUpIcon as ArrowUp, FileTextIcon as FileText, PlusIcon as Plus, SquareIcon as Stop, ThumbsDownIcon as ThumbsDown, ThumbsUpIcon as ThumbsUp } from 'lucide-react';
import { Button, cn, ScrollShadow, Spinner, TextArea, Tooltip } from '@heroui/react';
import { useEffect, useLayoutEffect, useRef, useState, type CSSProperties, type RefObject } from 'react';
import { cancelTurn, followTurn, QaError, resetConversation, sendFeedback, startTurn, transient, verifyHuman, type Citation, type StreamEvent, type TurnInfo } from './client';
import { Activity, finishSteps, mergeStep, type StepView } from './Activity';
import { Markdown } from './markdown';

export type QaWelcome = { name: string; description: string; topics: {title: string; href: string}[] };

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
  live?: boolean; // 本次打开后新提问得到的回答：逐字写出
  streamPath?: string;
  sequence?: number;
  reconnecting?: boolean;
  error?: string;
};

// 触屏设备（手机、平板）上从不主动聚焦输入框：聚焦会直接弹出软键盘、挡住大半屏，等访客自己点输入框。
// 发送后收起键盘，让出地方看回答。
const touch = matchMedia('(pointer: coarse)').matches;

// 对话记录和会话一样按语言分开保存。
const logKey = (locale: string) => `nw-qa-log:${locale}`;

function loadLog(locale: string): Message[] {
  try {
    const saved = JSON.parse(sessionStorage.getItem(logKey(locale)) ?? '[]') as Message[];
    return saved.map((message) => {
      const interrupted = message.status === 'streaming' && !(message.turn && message.streamPath);
      return { ...message, live: false, status: interrupted ? 'cancelled' : message.status, steps: interrupted ? finishSteps(message.steps, 'cancelled') : message.steps };
    });
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
// 独立页直接使用整页空间；文档中的浮窗沿用同一套会话。
export function QaChat({ locale, t, autoFocus, className, documentId, pageTitle, fullPage, welcome }: { locale: string; t: Translate; autoFocus?: boolean; className?: string; documentId?: string; pageTitle?: string; fullPage?: boolean; welcome?: QaWelcome }) {
  const [messages, setMessages] = useState<Message[]>(() => loadLog(locale));
  const [question, setQuestion] = useState('');
  const [running, setRunning] = useState<{ turnId?: string; controller: AbortController } | null>(null);
  const activeRun = useRef<typeof running>(null);
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

  // 打开时停在对话末尾；之后跟踪对话区高度（新一轮的最小高度）和内容变化（逐字输出时更新“回到最新”）。
  useLayoutEffect(() => {
    const box = scroller.current;
    if (!box) return;
    box.scrollTop = messages.length ? box.scrollHeight : 0;
    const observer = new ResizeObserver(() => { setView(box.clientHeight); check(); });
    observer.observe(box);
    if (list.current) observer.observe(list.current);
    return () => observer.disconnect();
  }, [!messages.length]);

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

  function trackRun(value: typeof running) { activeRun.current = value; setRunning(value); }
  function endRun(controller: AbortController) { if (activeRun.current?.controller === controller) trackRun(null); }

  function receive(id: string, event: StreamEvent, allowChallenge = false): boolean {
    if (event.kind === 'step') patch(id, (message) => ({ ...message, steps: mergeStep(message.steps ?? [], event.step) }));
    else if (event.kind === 'delta') patch(id, (message) => ({ ...message, text: message.text + event.text }));
    else if (event.kind === 'reset') patch(id, (message) => ({ ...message, text: '' }));
    else if (event.kind === 'citation') patch(id, (message) => ({ ...message, citations: [...message.citations, event.citation] }));
    else if (event.kind === 'done') patch(id, (message) => ({ ...message, status: 'done', steps: finishSteps(message.steps, 'done'), reconnecting: false, turn: event.turn?.turnId ? event.turn : message.turn }));
    else if (event.kind === 'cancelled') patch(id, (message) => ({ ...message, status: 'cancelled', steps: finishSteps(message.steps, 'cancelled'), reconnecting: false }));
    else if (event.kind === 'failed') {
      if (event.code === 'CHALLENGE_REQUIRED' && allowChallenge) return true;
      patch(id, (message) => ({ ...message, status: 'error', steps: finishSteps(message.steps, 'error'), reconnecting: false, error: event.code === 'DEPENDENCY_UNAVAILABLE' ? busy(t) : event.code === 'CHALLENGE_REQUIRED' ? t('人机验证没有完成') : `${t('这次没有回答成功（')}${event.code || 'error'}${t('），请稍后再试。')}` }));
    }
    return false;
  }

  const cursor = (id: string, afterSequence = 0) => ({
    afterSequence,
    onSequence: (sequence: number) => patch(id, (message) => ({ ...message, sequence })),
    onConnection: (reconnecting: boolean) => patch(id, (message) => ({ ...message, reconnecting })),
  });

  // 刷新或在文档之间跳转后续读同一轮持久事件，不重新提问或重复调用模型。
  useEffect(() => {
    const pending = messages.find((message) => message.role === 'assistant' && message.status === 'streaming' && message.turn && message.streamPath);
    if (pending?.turn && pending.streamPath) {
      const controller = new AbortController();
      trackRun({ controller, turnId: pending.turn.turnId });
      void followTurn({ turn: pending.turn, streamPath: pending.streamPath }, (event) => { receive(pending.id, event); }, controller.signal, cursor(pending.id, pending.sequence)).catch((problem) => {
        if (!controller.signal.aborted) patch(pending.id, (message) => ({ ...message, status: 'error', error: failure(t, problem), steps: finishSteps(message.steps, 'error'), reconnecting: false }));
      }).finally(() => endRun(controller));
    }
    return () => activeRun.current?.controller.abort();
  }, [locale]);

  function latest() {
    const box = scroller.current;
    const mark = end.current;
    if (box && mark) box.scrollTo({ top: box.scrollTop + mark.getBoundingClientRect().top - box.getBoundingClientRect().bottom + 24, behavior: 'smooth' });
  }

  async function ask(suggestion?: string) {
    const text = (suggestion ?? question).trim();
    if (!text || running) return;
    const id = crypto.randomUUID();
    const controller = new AbortController();
    setQuestion('');
    if (touch) input.current?.blur();
    trackRun({ controller });
    setPinned(id);
    setMessages((current) => [...current, { id: `${id}-q`, role: 'user', text, status: 'done', citations: [] }, { id, role: 'assistant', text: '', status: 'streaming', citations: [], live: true }]);
    try {
      // 风控在回答途中要求人机验证（例如判断这个问题可疑）时，先做完验证，再把同一个问题重新问一次；第二次还要求就如实提示。
      for (let attempt = 0; attempt < 2; attempt++) {
        let challenged = false;
        const started = await startTurn(text, locale, documentId);
        if (controller.signal.aborted) {
          await cancelTurn(started.turn.turnId).catch(() => {});
          throw new DOMException('Aborted', 'AbortError');
        }
        trackRun({ controller, turnId: started.turn.turnId });
        patch(id, (message) => ({ ...message, turn: started.turn, streamPath: started.streamPath, sequence: 0 }));
        await followTurn(started, (event) => {
          if (receive(id, event, attempt === 0)) challenged = true;
        }, controller.signal, cursor(id));
        if (!challenged) break;
        patch(id, (message) => ({ ...message, text: '', steps: [], citations: [] }));
        await verifyHuman();
      }
    } catch (problem) {
      if (!controller.signal.aborted) patch(id, (message) => ({ ...message, status: 'error', error: failure(t, problem), steps: finishSteps(message.steps, 'error'), reconnecting: false }));
      else patch(id, (message) => ({ ...message, status: 'cancelled', steps: finishSteps(message.steps, 'cancelled'), reconnecting: false }));
    } finally {
      endRun(controller);
      focus();
    }
  }

  async function stop() {
    if (!running) return;
    running.controller.abort();
    setMessages((current) => current.map((message) => message.status === 'streaming' ? { ...message, status: 'cancelled', steps: finishSteps(message.steps, 'cancelled'), reconnecting: false } : message));
    if (running.turnId) await cancelTurn(running.turnId).catch(() => {});
  }

  function restart() {
    running?.controller.abort();
    if (running?.turnId) void cancelTurn(running.turnId).catch(() => {});
    trackRun(null);
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
    <div className={cn('flex min-h-0 min-w-0 flex-1 flex-col', fullPage && 'nw-qa-page', className)} data-empty={!messages.length} style={view ? { '--nw-qa-activity-height': `${Math.max(128, view - 176)}px` } as CSSProperties : undefined}>
      <div className="nw-qa-thread relative flex min-h-0 flex-1 flex-col">
        <ScrollShadow ref={scroller} className="nw-qa-scroll min-h-0 flex-1 px-4 py-4" onScroll={check}>
          {!messages.length && (
            fullPage ? <div className="nw-qa-empty">
              <h1>{t('从一个问题开始')}</h1>
              {welcome?.description && <p className="nw-qa-intro">{welcome.description}</p>}
              <div className="nw-qa-starters">
                {['这个站点有哪些内容？', '我应该从哪里开始？'].map(text => <button key={text} type="button" onClick={() => { void ask(t(text)); }}>{t(text)}<ArrowUp size={16} aria-hidden /></button>)}
              </div>
              {!!welcome?.topics.length && <nav className="nw-qa-topics" aria-label={t('文档')}>
                {welcome.topics.map(topic => <a key={topic.href} href={topic.href}><FileText size={18} aria-hidden /><span>{topic.title}</span></a>)}
              </nav>}
            </div> : <div className="flex h-full items-center justify-center py-10 text-center"><p className="max-w-64 text-[15px] text-muted">{t('问问这个站点的文档…')}</p></div>
          )}
          <ol ref={list} className="nw-qa-content mx-auto flex w-full flex-col gap-8">
            {rounds.map((round, index) => (
              <li key={round.id} className="flex flex-col gap-4" data-turn={round.id} style={round.id === pinned && view ? { minHeight: view - 32 } : undefined}>
                {round.question && <div className="nw-qa-question ms-10 self-end whitespace-pre-wrap break-words rounded-xl bg-default px-4 py-2.5 text-base leading-7 text-foreground">{round.question.text}</div>}
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
      </div>
      <form className={cn('nw-qa-form p-3', fullPage ? 'sm:px-6 sm:pb-6' : 'border-t border-separator')} onSubmit={(event) => { event.preventDefault(); void ask(); }}>
        <div className="nw-qa-content mx-auto w-full">
          {documentId && pageTitle && (
            <div className="mb-2 flex min-w-0 items-center gap-1.5 px-1 text-xs text-muted">
              <FileText className="size-3.5 shrink-0" />
              <span className="shrink-0">{t('当前页面')}</span>
              <span className="truncate font-medium text-foreground/80">{pageTitle}</span>
            </div>
          )}
          <div className="nw-qa-composer flex items-end gap-2 border border-border focus-within:border-foreground/50">
            {messages.length > 0 && <Tooltip delay={300}><Button isIconOnly aria-label={t('新对话')} size="sm" variant="ghost" onPress={restart}><Plus /></Button><Tooltip.Content>{t('新对话')}</Tooltip.Content></Tooltip>}
            <TextArea
              ref={input}
              aria-label={t('问问这个站点的文档…')}
              className="max-h-40 min-h-9 min-w-0 flex-1 resize-none border-0 bg-transparent px-1 py-1.5 shadow-none outline-none focus:ring-0"
              maxLength={500}
              placeholder={t('问问这个站点的文档…')}
              rows={fullPage ? 2 : 1}
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
    <div className="nw-qa-answer flex min-w-0 flex-col gap-3">
      {!!message.steps?.length && <Activity active={message.status === 'streaming'} answerText={message.text} reconnecting={message.reconnecting} steps={message.steps} t={t} />}
      {/* 回答直接呈现为正文，出错时才使用提示底色。 */}
      <div className={cn('text-base', message.status === 'error' && !message.error ? 'rounded-xl bg-danger-soft px-3.5 py-2.5 text-danger-soft-foreground' : 'text-foreground', message.status === 'streaming' && !message.text && message.steps?.length && 'hidden')}>
        {message.status === 'streaming' && !message.text ? (
          <span className="flex items-center gap-2 text-muted"><Spinner color="current" size="sm" />{t('思考中…')}</span>
        ) : message.status === 'error' && !message.error ? message.text : <Typed message={message} t={t} />}
      </div>
      {message.error && <p role="alert" className="rounded-2xl bg-danger-soft px-3.5 py-2.5 text-sm text-danger-soft-foreground">{message.error}</p>}
      {message.status === 'cancelled' && <p className="px-1 text-xs text-muted">{t('已停止')}</p>}
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
function Typed({ message, t }: { message: Message; t: Translate }) {
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
      <Markdown citations={message.citations} referenceLabel={t('参考文献')} text={message.text.slice(0, shown)} />
    </div>
  );
}
