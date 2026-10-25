import { BookOpen, ChevronDown, CircleExclamation, FaceRobot, FileText, Magnifier, Stop } from '@gravity-ui/icons';
import { cn } from '@heroui/react';
import { memo, useEffect, useId, useLayoutEffect, useRef, useState } from 'react';
import type { Step } from './client';
import type { Translate } from './QaChat';

// 数字时间和ms保留对本标签页旧记录的兼容；新活动以服务端id配对、服务端耗时计时。
export type StepView = Omit<Step, 'startedAt'> & { startedAt?: string | number; ms?: number };

export function mergeStep(steps: StepView[], step: Step): StepView[] {
  let index = -1;
  for (let position = steps.length - 1; position >= 0; position--) {
    if (step.id ? steps[position].id === step.id : steps[position].key === step.key && steps[position].status === 'running') { index = position; break; }
  }
  const previous = steps[index];
  const next: StepView = { ...previous, ...step, startedAt: step.startedAt ?? previous?.startedAt ?? Date.now() };
  if (step.durationMs == null && step.status !== 'running' && previous?.status === 'running') next.ms = elapsed(previous, Date.now());
  if (index < 0) return [...steps, next];
  return steps.map((item, position) => position === index ? next : item);
}

export function finishSteps(steps: StepView[] | undefined, status: 'done' | 'error' | 'cancelled'): StepView[] | undefined {
  return steps?.map((step) => step.status === 'running' ? { ...step, status: status === 'error' ? 'failed' : status } : step);
}

function elapsed(step: StepView, now: number): number | undefined {
  if (step.durationMs != null) return step.durationMs;
  if (step.ms != null) return step.ms;
  if (step.status !== 'running' || step.startedAt == null) return undefined;
  const start = typeof step.startedAt === 'number' ? step.startedAt : Date.parse(step.startedAt);
  return Number.isFinite(start) ? Math.max(0, now - start) : undefined;
}

function seconds(ms?: number): string {
  return ms == null ? '' : ms < 100 ? '<0.1s' : `${(ms / 1000).toFixed(1)}s`;
}

function ActionIcon({ step }: { step: StepView }) {
  const Icon = step.tool === 'search_documents' ? Magnifier : step.tool === 'read_document' || step.key === 'page' ? FileText : step.key === 'search' ? BookOpen : FaceRobot;
  return <Icon aria-hidden className="size-3.5 shrink-0 text-muted" />;
}

function ActivityRow({ step, now, expanded, onToggle, t }: { step: StepView; now: number; expanded?: boolean; onToggle: (row: HTMLLIElement | null) => void; t: Translate }) {
  const end = step.nextOffset ?? step.totalCharacters;
  const hasRange = step.offset != null && end != null && step.totalCharacters != null;
  const hasDetails = !!(step.detail || step.query || step.items?.length || hasRange);
  // 过程说明和失败原因直接展开；工具参数与结果从摘要行展开，手动选择始终优先。
  const open = expanded ?? (step.status === 'failed' || (!step.tool && step.key === 'answer' && !!step.detail));
  const primary = step.query || step.items?.[0] || (!step.tool && step.key !== 'answer' ? step.detail?.split('\n')[0] : '');
  const labels: Record<Step['status'], string> = { running: t('进行中'), done: t('已完成'), skipped: t('已跳过'), failed: t('失败'), cancelled: t('已停止') };
  const summary = <>
    <ActionIcon step={step} />
    <span className={cn('shrink-0 font-medium', step.status === 'running' ? 'nw-qa-working' : 'text-muted')}>{t(step.title)}</span>
    {step.round && !step.tool && <span className="shrink-0 text-[10px] tabular-nums text-muted">#{step.round}</span>}
    {primary && <span className="min-w-0 flex-1 truncate text-foreground/80" title={primary}>{primary}</span>}
    <span className="ms-auto inline-flex shrink-0 items-center gap-1.5 ps-1 text-[10px] text-muted">
      {hasRange && <span className="tabular-nums">{Math.round(end! * 100 / Math.max(1, step.totalCharacters!))}%</span>}
      {step.status === 'failed' ? <span className="inline-flex items-center gap-1 text-danger"><CircleExclamation aria-hidden className="size-3" />{labels.failed}</span> : step.status === 'cancelled' ? <span className="inline-flex items-center gap-1"><Stop aria-hidden className="size-3" />{labels.cancelled}</span> : step.status === 'skipped' ? <span>{labels.skipped}</span> : <span className="sr-only">{labels[step.status]}</span>}
      <span className="tabular-nums">{seconds(elapsed(step, now))}</span>
      {hasDetails && <ChevronDown aria-hidden className={cn('size-3 transition-transform motion-reduce:transition-none', open && 'rotate-180')} />}
    </span>
  </>;
  return <>
    {hasDetails ? <button type="button" aria-expanded={open} className="flex w-full items-center gap-2 rounded-md py-1 text-start text-xs leading-5 outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-foreground/20" onClick={(event) => onToggle(event.currentTarget.closest('li'))}>{summary}</button> : <div className="flex w-full items-center gap-2 py-1 text-xs leading-5">{summary}</div>}
    {open && hasDetails && <div className="mb-2 ms-1.5 mt-1 border-s border-border/70 py-0.5 ps-4 text-xs">
      {step.query && <p className="whitespace-pre-wrap break-words leading-6 text-foreground/80">{step.query}</p>}
      {step.detail && <p className={cn('whitespace-pre-wrap break-words leading-6', step.status === 'failed' ? 'text-danger' : 'text-muted')}>{step.detail}</p>}
      {!!step.items?.length && <ul className={cn('flex flex-wrap gap-1.5', (step.query || step.detail) && 'mt-2')}>
        {step.items.map((item) => <li key={item} className="inline-flex min-w-0 max-w-full items-start gap-1 rounded-md bg-default/60 px-1.5 py-0.5 text-[11px] text-foreground/80"><FileText aria-hidden className="mt-1 size-3 shrink-0" /><span className="break-words leading-5">{item}</span></li>)}
      </ul>}
      {hasRange && <div className="mt-2 text-[10px] text-muted">
        <div className="mb-1 flex items-center justify-between gap-2"><span>{t('阅读进度')}</span><span className="tabular-nums">{Math.min(step.offset! + 1, end!)}–{end} / {step.totalCharacters} {t('字')}</span></div>
        <progress className="nw-qa-read-progress block h-0.5 w-full" aria-label={t('阅读进度')} value={end} max={Math.max(1, step.totalCharacters!)} />
      </div>}
    </div>}
  </>;
}

// 交互参考 ZCode v3.16.2 的工具摘要与详情层级，使用本站组件独立实现。
// https://github.com/zai-org/ZCode/blob/main/packages/ui/src/ToolCallBlocks/ToolLayout.tsx
// 整组过程默认展开，滚动跟随新活动；访客向上查看时保持其位置。
export const Activity = memo(function Activity({ steps, active, reconnecting, t }: { steps: StepView[]; active: boolean; reconnecting?: boolean; t: Translate }) {
  const [open, setOpen] = useState(true);
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const [now, setNow] = useState(Date.now);
  const [following, setFollowing] = useState(true);
  const follow = useRef(true);
  const scroller = useRef<HTMLOListElement>(null);
  const inspecting = useRef<HTMLLIElement | null>(null);
  const listId = useId();
  useEffect(() => {
    if (!active || !open) return;
    setNow(Date.now());
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [active, open]);
  useLayoutEffect(() => {
    if (open && follow.current && scroller.current) scroller.current.scrollTop = scroller.current.scrollHeight;
  }, [steps, open]);
  useLayoutEffect(() => {
    const box = scroller.current;
    const row = inspecting.current;
    if (!box || !row) return;
    const viewport = box.getBoundingClientRect();
    const item = row.getBoundingClientRect();
    // 只滚动过程列表，把刚展开的内容带进视野，保持聊天和页面的位置。
    if (item.height > viewport.height || item.top < viewport.top) box.scrollTop += item.top - viewport.top;
    else if (item.bottom > viewport.bottom) box.scrollTop += item.bottom - viewport.bottom;
    inspecting.current = null;
  }, [expanded]);
  const current = [...steps].reverse().find((step) => step.status === 'running') ?? steps[steps.length - 1];
  const toolCount = steps.filter((step) => step.tool).length;
  const duration = steps.reduce((total, step) => total + (elapsed(step, now) ?? 0), 0);
  const jump = () => {
    follow.current = true;
    setFollowing(true);
    scroller.current?.scrollTo({ top: scroller.current.scrollHeight, behavior: 'smooth' });
  };
  return (
    <section className="nw-qa-activity min-w-0 py-1 text-xs" aria-label={t('思考与工具')}>
      <button aria-label={t('思考与工具')} aria-controls={listId} aria-expanded={open} className="flex w-full items-center gap-2 py-1.5 text-start text-muted outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-foreground/20" type="button" onClick={() => setOpen(!open)}>
        <FaceRobot aria-hidden className="size-3.5 shrink-0" />
        <span className={cn('min-w-0 truncate font-medium', active && 'nw-qa-working')} role="status">{active ? reconnecting ? t('正在重新连接…') : t(current.title) : t('思考与工具')}</span>
        {!!toolCount && <span className="shrink-0 text-[10px]">· {t('工具调用')} {toolCount}</span>}
        <span aria-hidden className="ms-auto shrink-0 text-[10px] tabular-nums">{(!active || open) && seconds(duration)}</span>
        <ChevronDown aria-hidden className={cn('size-3 shrink-0 transition-transform motion-reduce:transition-none', open && 'rotate-180')} />
      </button>
      {open && <>
        <ol id={listId} ref={scroller} className="nw-qa-activity-list mt-1 max-h-[min(18rem,var(--nw-qa-activity-height,18rem))] space-y-1 overflow-y-auto overscroll-contain pe-1" onScroll={() => {
          const box = scroller.current;
          if (!box) return;
          follow.current = box.scrollHeight - box.scrollTop - box.clientHeight < 32;
          setFollowing(follow.current);
        }}>
          {steps.map((step, index) => {
            const id = step.id ?? `${step.key}.${index}`;
            return <li key={id} data-activity-id={step.id}><ActivityRow step={step} now={now} expanded={expanded[id]} onToggle={(row) => {
              follow.current = false;
              setFollowing(false);
              inspecting.current = row;
              setExpanded((values) => {
                const automatic = step.status === 'failed' || (!step.tool && step.key === 'answer' && !!step.detail);
                return { ...values, [id]: !(values[id] ?? automatic) };
              });
            }} t={t} /></li>;
          })}
        </ol>
        {active && !following && <button className="mt-1 flex w-full items-center justify-center gap-1 py-1 text-[10px] text-muted hover:text-foreground" type="button" onClick={jump}>{t('回到最新')}<ChevronDown aria-hidden className="size-3" /></button>}
      </>}
    </section>
  );
});
