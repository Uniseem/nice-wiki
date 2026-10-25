// 摘要排布与折叠交互改编自 ZCode v3.16.2 ToolSummaryRow / ToolLayout（Apache-2.0）。
// Copyright 2026 Z.AI Co., Ltd；来源 https://github.com/zai-org/ZCode/tree/main/packages/ui/src/ToolCallBlocks
// 修改：接入本站只读文档工具事件，使用 HeroUI 主题，移除与阅读无关的状态装饰。
import { ChevronDownIcon, ChevronRightIcon, CircleAlertIcon, FileTextIcon, SearchIcon, SquareIcon, WrenchIcon } from 'lucide-react';
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
  // ZCode 的 read、search、explore 共用 Lucide SearchIcon，运行时保持静态。
  const Icon = step.tool === 'search_documents' || step.tool === 'read_document' || step.key === 'page' || step.key === 'search' ? SearchIcon : WrenchIcon;
  return <Icon aria-hidden className="size-4 shrink-0 text-muted" />;
}

function ActivityRow({ step, now, expanded, onToggle, t }: { step: StepView; now: number; expanded?: boolean; onToggle: (row: HTMLLIElement | null) => void; t: Translate }) {
  const end = step.nextOffset ?? step.totalCharacters;
  const detailsId = useId();
  const hasRange = step.offset != null && end != null && step.totalCharacters != null;
  const hasDetails = !!(step.detail || step.query || step.items?.length || hasRange);
  const open = expanded ?? step.status === 'failed';
  const primary = step.query || step.items?.[0];
  const labels: Record<Step['status'], string> = { running: t('进行中'), done: t('已完成'), skipped: t('已跳过'), failed: t('失败'), cancelled: t('已停止') };
  const summary = <>
    <ActionIcon step={step} />
    <span className={cn('shrink-0 font-medium', step.status === 'running' ? 'nw-qa-working' : 'text-muted')}>{t(step.title)}</span>
    {primary && <span className="min-w-0 truncate text-muted" title={primary}>{primary}</span>}
    {step.status === 'failed' ? <span className="inline-flex shrink-0 items-center gap-1 text-danger"><CircleAlertIcon aria-hidden className="size-3.5" />{labels.failed}</span> : step.status === 'cancelled' ? <span className="inline-flex shrink-0 items-center gap-1"><SquareIcon aria-hidden className="size-3" />{labels.cancelled}</span> : <span className="sr-only">{labels[step.status]}</span>}
    {hasDetails && <ChevronRightIcon aria-hidden className={cn('nw-qa-tool-chevron size-4 shrink-0 text-muted transition-[transform,opacity] duration-200 motion-reduce:transition-none rtl:-scale-x-100', open && 'rotate-90 opacity-100 rtl:-rotate-90')} />}
  </>;
  return <>
    {hasDetails ? <button type="button" aria-controls={detailsId} aria-expanded={open} className="nw-qa-tool-summary flex max-w-full items-center gap-2 rounded-md py-1.5 text-start outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-foreground/20" onClick={(event) => onToggle(event.currentTarget.closest('li'))}>{summary}</button> : <div className="flex max-w-full items-center gap-2 py-1.5">{summary}</div>}
    {hasDetails && <div id={detailsId} className="nw-qa-tool-collapse" data-open={open} aria-hidden={!open} inert={!open}><div className="overflow-hidden"><div className="mb-2 ms-2 mt-1 border-s border-border py-1 ps-4 text-sm">
      {step.query && <p className="whitespace-pre-wrap break-words leading-6 text-foreground/80">{step.query}</p>}
      {step.detail && <p className={cn('whitespace-pre-wrap break-words leading-6', step.status === 'failed' ? 'text-danger' : 'text-muted')}>{step.detail}</p>}
      {!!step.items?.length && <ul className={cn('flex flex-wrap gap-1.5', (step.query || step.detail) && 'mt-2')}>
        {step.items.map((item) => <li key={item} className="inline-flex min-w-0 max-w-full items-start gap-1.5 rounded-md bg-default/60 px-2 py-1 text-foreground/80"><FileTextIcon aria-hidden className="mt-0.5 size-4 shrink-0" /><span className="break-words leading-5">{item}</span></li>)}
      </ul>}
      {hasRange && <div className="mt-3 text-muted">
        <div className="mb-1 flex items-center justify-between gap-2"><span>{t('阅读进度')}</span><span className="tabular-nums">{Math.min(step.offset! + 1, end!)}–{end} / {step.totalCharacters} {t('字')}</span></div>
        <progress className="nw-qa-read-progress block h-0.5 w-full" aria-label={t('阅读进度')} value={end} max={Math.max(1, step.totalCharacters!)} />
      </div>}
      {elapsed(step, now) != null && <p className="mt-2 tabular-nums text-muted">{seconds(elapsed(step, now))}</p>}
    </div></div></div>}
  </>;
}

// 工具直接排列在回答中，参数与结果按行展开；访客向上查看时保持滚动位置。
export const Activity = memo(function Activity({ steps, active, reconnecting, answerText, t }: { steps: StepView[]; active: boolean; reconnecting?: boolean; answerText?: string; t: Translate }) {
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const [now, setNow] = useState(Date.now);
  const [following, setFollowing] = useState(true);
  const follow = useRef(true);
  const scroller = useRef<HTMLOListElement>(null);
  const inspecting = useRef<HTMLLIElement | null>(null);
  useEffect(() => {
    if (!active) return;
    setNow(Date.now());
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [active]);
  useLayoutEffect(() => {
    if (follow.current && scroller.current) scroller.current.scrollTop = scroller.current.scrollHeight;
  }, [steps]);
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
  const jump = () => {
    follow.current = true;
    setFollowing(true);
    scroller.current?.scrollTo({ top: scroller.current.scrollHeight, behavior: 'smooth' });
  };
  return (
    <section className="nw-qa-activity min-w-0 py-1 text-[13px] text-muted" aria-label={t('思考与工具')}>
      {active && reconnecting && <p role="status" className="nw-qa-working py-1.5">{t('正在重新连接…')}</p>}
        <ol ref={scroller} className="nw-qa-activity-list max-h-[min(18rem,var(--nw-qa-activity-height,18rem))] overflow-y-auto overscroll-contain pe-1" onScroll={() => {
          const box = scroller.current;
          if (!box) return;
          follow.current = box.scrollHeight - box.scrollTop - box.clientHeight < 32;
          setFollowing(follow.current);
        }}>
          {steps.map((step, index) => {
            if (!step.tool && !step.detail && step.status !== 'running') return null;
            const id = step.id ?? `${step.key}.${index}`;
            if (!step.tool && step.key === 'answer' && step.detail) {
              if (step.detail.trim() === answerText?.trim()) return null;
              return <li key={id} className="whitespace-pre-wrap break-words py-2 text-sm leading-6 text-foreground/80">{step.detail}</li>;
            }
            return <li key={id} data-activity-id={step.id}><ActivityRow step={step} now={now} expanded={expanded[id]} onToggle={(row) => {
              follow.current = false;
              setFollowing(false);
              inspecting.current = row;
              setExpanded((values) => {
                const automatic = step.status === 'failed';
                return { ...values, [id]: !(values[id] ?? automatic) };
              });
            }} t={t} /></li>;
          })}
        </ol>
        {active && !following && <button className="mt-1 flex w-full items-center justify-center gap-1 py-1 text-muted hover:text-foreground" type="button" onClick={jump}>{t('回到最新')}<ChevronDownIcon aria-hidden className="size-3" /></button>}
    </section>
  );
});
