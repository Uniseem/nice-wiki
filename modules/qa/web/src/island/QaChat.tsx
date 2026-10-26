import { ArrowDownIcon as ArrowDown, ArrowUpIcon as ArrowUp, FileTextIcon as FileText, PlusIcon as Plus, SquareIcon as Stop, ThumbsDownIcon as ThumbsDown, ThumbsUpIcon as ThumbsUp } from 'lucide-react';
import { Button, cn, ScrollShadow, TextArea, Tooltip } from '@heroui/react';
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
  replacing?: boolean; // 服务端要求整段重发：下一段文字替换现有正文，不先清空
  reconnecting?: boolean;
  error?: string;
};

// 触屏设备（手机、平板）上从不主动聚焦输入框：聚焦会直接弹出软键盘、挡住大半屏，等访客自己点输入框。
// 发送后收起键盘，让出地方看回答。
const touch = matchMedia('(pointer: coarse)').matches;

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

// 对话不保留：刷新、重新进入问答页或在文档之间换页，都从空白开始，服务端也另起一个会话。
// documentId 是访客所在页面：每次提问都带上，AI 会先读这一页全文再回答。
// 滚动只发生在对话区里，从不滚动页面：提问后把这一轮滚到对话区顶部，回答逐字写出时不自动往下滚，
// 访客可以从头读；最新内容在可视范围外时显示“回到最新”。
// 独立页直接使用整页空间；文档中的浮窗按当前文档各自从头开始。
export function QaChat({ locale, t, autoFocus, className, documentId, pageTitle, fullPage, welcome }: { locale: string; t: Translate; autoFocus?: boolean; className?: string; documentId?: string; pageTitle?: string; fullPage?: boolean; welcome?: QaWelcome }) {
  const [messages, setMessages] = useState<Message[]>([]);
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
  const inputMeasure = useRef<HTMLTextAreaElement>(null);
  const inputBox = useRef<HTMLDivElement>(null);
  const [inputSize, setInputSize] = useState<{ height: number; overflow: boolean } | null>(null);

  // 隐藏副本按相同宽度测量换行，不重置可见输入框的高度、光标和滚动位置。
  const resizeInput = () => {
    const measure = inputMeasure.current;
    if (!measure) return;
    const line = parseFloat(getComputedStyle(measure).lineHeight);
    const height = Math.max(line, Math.min(measure.scrollHeight, line * 6));
    const overflow = measure.scrollHeight > line * 6;
    setInputSize(current => current?.height === height && current.overflow === overflow ? current : { height, overflow });
  };
  useLayoutEffect(resizeInput, [question]);
  useLayoutEffect(() => {
    const box = inputBox.current;
    if (!box) return;
    let width = 0;
    const observer = new ResizeObserver(([entry]) => {
      if (entry.contentRect.width === width) return;
      width = entry.contentRect.width;
      resizeInput();
    });
    observer.observe(box);
    return () => observer.disconnect();
  }, []);

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

  // 模型每一轮的文字都实时流出；带工具调用的一轮结束时，它的过程说明随步骤进入时间线，从正文末尾移走，
  // 正文只留最终回答。服务端收尾时若要求整段重发，等新文字到了再替换，已经写出的部分不会清空重打。
  function receive(id: string, event: StreamEvent, allowChallenge = false): boolean {
    if (event.kind === 'step') patch(id, (message) => {
      const narration = event.step.key === 'answer' && event.step.status === 'done' && !event.step.tool ? event.step.detail?.trim() : undefined;
      const text = narration && message.text.trimEnd().endsWith(narration) ? message.text.trimEnd().slice(0, -narration.length) : message.text;
      return { ...message, text, steps: mergeStep(message.steps ?? [], event.step) };
    });
    else if (event.kind === 'delta') patch(id, (message) => ({ ...message, text: message.replacing ? event.text : message.text + event.text, replacing: false }));
    else if (event.kind === 'reset') patch(id, (message) => ({ ...message, replacing: true }));
    else if (event.kind === 'citation') patch(id, (message) => ({ ...message, citations: [...message.citations, event.citation] }));
    else if (event.kind === 'done') patch(id, (message) => ({ ...message, status: 'done', text: message.replacing ? '' : message.text, replacing: false, steps: finishSteps(message.steps, 'done'), reconnecting: false, turn: event.turn?.turnId ? event.turn : message.turn }));
    else if (event.kind === 'cancelled') patch(id, (message) => ({ ...message, status: 'cancelled', steps: finishSteps(message.steps, 'cancelled'), reconnecting: false }));
    else if (event.kind === 'failed') {
      if (event.code === 'CHALLENGE_REQUIRED' && allowChallenge) return true;
      patch(id, (message) => ({ ...message, status: 'error', steps: finishSteps(message.steps, 'error'), reconnecting: false, error: event.code === 'DEPENDENCY_UNAVAILABLE' ? busy(t) : event.code === 'CHALLENGE_REQUIRED' ? t('人机验证没有完成') : `${t('这次没有回答成功（')}${event.code || 'error'}${t('），请稍后再试。')}` }));
    }
    return false;
  }

  // 断网或服务重启时 followTurn 自己从最后一个序号续读，这里只把“正在重新连接”显示出来。
  const cursor = (id: string) => ({ onConnection: (reconnecting: boolean) => patch(id, (message) => ({ ...message, reconnecting })) });

  // 每次挂载都另起服务端会话；离开（换页、换语言、关掉）时停掉还在写的回答，服务端也不再继续花钱。
  useEffect(() => {
    resetConversation(locale);
    return () => {
      const run = activeRun.current;
      run?.controller.abort();
      if (run?.turnId) void cancelTurn(run.turnId).catch(() => {});
    };
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
        patch(id, (message) => ({ ...message, turn: started.turn }));
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
    <div className={cn('flex min-h-0 min-w-0 flex-1 flex-col', fullPage && 'nw-qa-page', className)} data-empty={!messages.length}>
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
              <li key={round.id} className="flex flex-col gap-4" data-turn={round.id} data-live={round.answer?.live || undefined} style={round.id === pinned && view ? { minHeight: view - 32 } : undefined}>
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
          <div className="nw-qa-composer flex items-center gap-2 border border-border focus-within:border-foreground/50" style={inputSize ? { '--nw-input-height': `${inputSize.height}px` } as CSSProperties : undefined}>
            {messages.length > 0 && <Tooltip delay={300}><Button isIconOnly aria-label={t('新对话')} size="sm" variant="ghost" onPress={restart}><Plus /></Button><Tooltip.Content>{t('新对话')}</Tooltip.Content></Tooltip>}
            <div ref={inputBox} className="nw-qa-input relative min-w-0 flex-1">
              <TextArea
                ref={input}
                aria-label={t('问问这个站点的文档…')}
                className="nw-qa-input-field resize-none border-0 bg-transparent shadow-none outline-none focus:ring-0"
                data-overflow={inputSize?.overflow || undefined}
                maxLength={500}
                placeholder={t('问问这个站点的文档…')}
                rows={1}
                value={question}
                onChange={(event) => setQuestion(event.target.value)}
                onKeyDown={(event) => { if (event.key === 'Enter' && !event.shiftKey && !event.nativeEvent.isComposing) { event.preventDefault(); void ask(); } }}
              />
              <textarea ref={inputMeasure} className="nw-qa-input-measure" aria-hidden tabIndex={-1} readOnly rows={1} value={question} />
            </div>
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
      {/* 过程标题行从提问那一刻起就在原位：先是“思考中…”，再换成当前动作，不在占位和过程之间来回切换。 */}
      {(message.status === 'streaming' || !!message.steps?.length) && <Activity answerText={message.text} reconnecting={message.reconnecting} status={message.status} steps={message.steps ?? []} t={t} />}
      {/* 回答直接呈现为正文，出错时才使用提示底色。 */}
      <div className={cn('text-base', message.status === 'error' && !message.error ? 'rounded-xl bg-danger-soft px-3.5 py-2.5 text-danger-soft-foreground' : 'text-foreground', message.status === 'streaming' && !message.text && 'hidden')}>
        {message.status === 'error' && !message.error ? message.text : <Typed message={message} t={t} />}
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
