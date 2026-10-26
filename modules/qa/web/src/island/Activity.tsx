// 时间线排布与折叠交互改编自 ZCode v3.16.2 ToolSummaryRow / ToolLayout（Apache-2.0）。
// Copyright 2026 Z.AI Co., Ltd；来源 https://github.com/zai-org/ZCode/tree/main/packages/ui/src/ToolCallBlocks
// 修改：接入本站只读文档工具事件，使用 HeroUI 主题；整段过程收进一个可折叠的标题行，步骤排成带节点的时间线。
import { BookOpenIcon, ChevronRightIcon, CircleAlertIcon, CompassIcon, EllipsisIcon, FileTextIcon, SearchIcon, ShieldAlertIcon, SparklesIcon, SquareIcon, WrenchIcon } from 'lucide-react';
import { cn } from '@heroui/react';
import { memo, useId, useState } from 'react';
import type { Step } from './client';
import { Matrix } from './loader';
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

function startOf(step: StepView): number | undefined {
  const start = typeof step.startedAt === 'number' ? step.startedAt : Date.parse(step.startedAt ?? '');
  return Number.isFinite(start) ? start : undefined;
}

function elapsed(step: StepView, now: number): number | undefined {
  if (step.durationMs != null) return step.durationMs;
  if (step.ms != null) return step.ms;
  const start = startOf(step);
  return step.status === 'running' && start != null ? Math.max(0, now - start) : undefined;
}

// 整段用时：第一步开始到最后一步结束，都按服务端时间算。
function totalTime(steps: StepView[]): number | undefined {
  let first = Infinity, last = -Infinity;
  for (const step of steps) {
    const start = startOf(step);
    if (start == null) continue;
    first = Math.min(first, start);
    last = Math.max(last, start + (elapsed(step, start) ?? 0));
  }
  return Number.isFinite(first) ? last - first : undefined;
}

function seconds(ms?: number): string {
  return ms == null ? '' : ms < 100 ? '<0.1s' : `${(ms / 1000).toFixed(1)}s`;
}

const toolIcons: Record<string, typeof SearchIcon> = { search_documents: SearchIcon, read_document: BookOpenIcon, read_guide: CompassIcon, report_abuse: ShieldAlertIcon };

function StepIcon({ step }: { step: StepView }) {
  const Icon = step.tool ? toolIcons[step.tool] ?? WrenchIcon : SparklesIcon;
  return <Icon aria-hidden />;
}

function ToolRow({ step, expanded, onToggle, t }: { step: StepView; expanded?: boolean; onToggle: () => void; t: Translate }) {
  const end = step.nextOffset ?? step.totalCharacters;
  const detailsId = useId();
  const hasRange = step.offset != null && end != null && step.totalCharacters != null;
  const hasDetails = !!(step.detail || step.query || step.items?.length || hasRange);
  const open = expanded ?? step.status === 'failed';
  const primary = step.query || step.items?.[0];
  const time = step.status === 'running' ? undefined : elapsed(step, Date.now());
  const labels: Record<Step['status'], string> = { running: t('进行中'), done: t('已完成'), skipped: t('已跳过'), failed: t('失败'), cancelled: t('已停止') };
  const summary = <>
    <span className={cn('shrink-0 font-medium', step.status === 'running' ? 'nw-qa-working' : 'text-foreground/80')}>{t(step.title)}</span>
    {primary && <span className="min-w-0 truncate" title={primary}>{primary}</span>}
    {step.status === 'failed' ? <span className="inline-flex shrink-0 items-center gap-1 text-danger"><CircleAlertIcon aria-hidden className="size-3.5" />{labels.failed}</span>
      : step.status === 'cancelled' || step.status === 'skipped' ? <span className="shrink-0">{labels[step.status]}</span>
      : <span className="sr-only">{labels[step.status]}</span>}
    {time != null && <span className="nw-qa-trace-time shrink-0 tabular-nums">{seconds(time)}</span>}
    {hasDetails && <ChevronRightIcon aria-hidden className={cn('nw-qa-tool-chevron size-3.5 shrink-0 transition-[transform,opacity] duration-200 motion-reduce:transition-none rtl:-scale-x-100', open && 'rotate-90 opacity-100 rtl:-rotate-90')} />}
  </>;
  return <>
    {hasDetails ? <button type="button" aria-controls={detailsId} aria-expanded={open} className="nw-qa-tool-summary flex max-w-full items-center gap-2 rounded-md text-start outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-foreground/20" onClick={onToggle}>{summary}</button> : <div className="flex max-w-full items-center gap-2">{summary}</div>}
    {hasDetails && <div id={detailsId} className="nw-qa-tool-collapse" data-open={open} aria-hidden={!open} inert={!open}><div className="overflow-hidden"><div className="nw-qa-trace-details">
      {step.query && <p className="whitespace-pre-wrap break-words leading-6 text-foreground/80">{step.query}</p>}
      {step.detail && <p className={cn('whitespace-pre-wrap break-words leading-6', step.status === 'failed' ? 'text-danger' : 'text-muted')}>{step.detail}</p>}
      {!!step.items?.length && <ul className="flex flex-wrap gap-1.5">
        {step.items.map((item) => <li key={item} className="inline-flex min-w-0 max-w-full items-start gap-1.5 rounded-md bg-default/60 px-2 py-1 text-foreground/80"><FileTextIcon aria-hidden className="mt-0.5 size-3.5 shrink-0" /><span className="break-words leading-5">{item}</span></li>)}
      </ul>}
      {hasRange && <div>
        <div className="mb-1 flex items-center justify-between gap-2"><span>{t('阅读进度')}</span><span className="tabular-nums">{Math.min(step.offset! + 1, end!)}–{end} / {step.totalCharacters} {t('字')}</span></div>
        <progress className="nw-qa-read-progress block h-0.5 w-full" aria-label={t('阅读进度')} value={end} max={Math.max(1, step.totalCharacters!)} />
      </div>}
    </div></div></div>}
  </>;
}

// 进行中只露出最近几步，更早的收成一行，时间线不会随工具次数无限变长。
const recent = 4;

// 回答上方的过程：标题行在进行中显示当前动作（点阵加载和扫光文字），完成后概括为“思考过程 · 文档数 · 用时”。
// 进行中默认展开，回答完成后自动收起；访客点过标题行就以访客为准。工具按调用顺序排成时间线，参数与结果逐行展开。
export const Activity = memo(function Activity({ steps, status, reconnecting, answerText, t }: { steps: StepView[]; status: 'streaming' | 'done' | 'error' | 'cancelled'; reconnecting?: boolean; answerText?: string; t: Translate }) {
  const active = status === 'streaming';
  const [open, setOpen] = useState<boolean>();
  const [all, setAll] = useState(false);
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const panelId = useId();
  const answered = !!answerText?.trim();
  const rows = steps.map((step, index) => ({ step, id: step.id ?? `${step.key}.${index}` }))
    .filter(({ step }) => step.tool || (step.detail && step.status !== 'running' && step.detail.trim() !== answerText?.trim()));
  if (!active && !rows.length) return null;
  const shown = open ?? (active || !answered);
  const hidden = active && !all ? Math.max(0, rows.length - recent) : 0;
  const current = active ? steps.findLast((step) => step.status === 'running') : undefined;
  const label = !active ? t('思考过程') : reconnecting ? t('正在重新连接…') : !current ? t('思考中…') : !current.tool && answered ? t('整理回答') : t(current.title);
  const documents = new Set(steps.filter((step) => step.tool === 'read_document').flatMap((step) => step.items ?? [])).size;
  const total = active ? undefined : totalTime(steps);
  const toggle = (id: string, automatic: boolean) => setExpanded((values) => ({ ...values, [id]: !(values[id] ?? automatic) }));
  return (
    <section className="nw-qa-trace" data-state={status} aria-label={t('思考与工具')}>
      <button type="button" className="nw-qa-trace-head" aria-controls={panelId} aria-expanded={rows.length ? shown : undefined} disabled={!rows.length} onClick={() => setOpen(!shown)}>
        {active ? <Matrix className="nw-qa-trace-glyph" /> : status === 'cancelled' ? <SquareIcon aria-hidden className="size-3" /> : status === 'error' ? <CircleAlertIcon aria-hidden className="size-3.5 text-danger" /> : null}
        <span key={label} role={active ? 'status' : undefined} className={cn('nw-qa-trace-label', active && 'nw-qa-working')}>{label}</span>
        {!active && <span className="nw-qa-trace-meta">
          {documents > 0 && <span className="inline-flex items-center gap-1" title={`${documents} ${t('篇文档')}`}><FileTextIcon aria-hidden className="size-3.5" />{documents}</span>}
          {total != null && <span>{seconds(total)}</span>}
        </span>}
        {!!rows.length && <ChevronRightIcon aria-hidden className="nw-qa-trace-chevron size-3.5" />}
      </button>
      {!!rows.length && <div id={panelId} className="nw-qa-tool-collapse" data-open={shown} inert={!shown}><div className="overflow-hidden">
        <ol className="nw-qa-trace-list">
          {hidden > 0 && <li className="nw-qa-trace-item">
            <span className="nw-qa-trace-node"><EllipsisIcon aria-hidden /></span>
            <button type="button" className="nw-qa-trace-more" onClick={() => setAll(true)}>+{hidden}</button>
          </li>}
          {rows.slice(hidden).map(({ step, id }) => (
            <li key={id} className="nw-qa-trace-item" data-status={step.status} data-live={active || undefined}>
              <span className="nw-qa-trace-node"><StepIcon step={step} /></span>
              <div className="min-w-0">
                {step.tool ? <ToolRow step={step} expanded={expanded[id]} t={t} onToggle={() => toggle(id, step.status === 'failed')} />
                  : <button type="button" aria-expanded={!!expanded[id]} className={cn('nw-qa-trace-thought', !expanded[id] && 'line-clamp-3')} onClick={() => toggle(id, false)}>{step.detail}</button>}
              </div>
            </li>
          ))}
        </ol>
      </div></div>}
    </section>
  );
});
