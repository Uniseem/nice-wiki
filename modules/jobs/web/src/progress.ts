import type { ModuleAPIClient } from '../../../../libs/platform/web/module-api';
import { t } from '@ui';

export interface ProgressEvent {
  jobId: string;
  seq: number;
  phase: string;
  completedUnits: number;
  totalUnits: number;
}

export interface ReconnectProgressInput {
  jobId: string;
  lastEventId: number;
}

export interface ReconnectProgressResult {
  events: ProgressEvent[];
}

type ProgressListener = (event: ProgressEvent) => void;
type ProblemCode = { code?: unknown; problem?: { code?: unknown } };

let apiClient: ModuleAPIClient | undefined;

const terminalStates = new Set(['succeeded', 'cancelled', 'dead-letter']);
const retryDelays = [1000, 2000, 4000, 8000, 16000];

export function ConfigureProgressAPI(client: ModuleAPIClient): void {
  apiClient = client;
}

function problemCode(error: unknown): string {
  const problem = error as ProblemCode | null;
  const code = problem?.code ?? problem?.problem?.code;
  return typeof code === 'string' ? code : 'DEPENDENCY_UNAVAILABLE';
}

function delay(milliseconds: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal.aborted) {
      reject(new DOMException(t('已取消'), 'AbortError'));
      return;
    }
    let timer: ReturnType<typeof setTimeout>;
    const onAbort = () => {
      clearTimeout(timer);
      reject(new DOMException(t('已取消'), 'AbortError'));
    };
    timer = setTimeout(() => {
      signal.removeEventListener('abort', onAbort);
      resolve();
    }, milliseconds);
    signal.addEventListener('abort', onAbort, { once: true });
  });
}

function parseFrame(frame: string): { event: string; id: string; data: string } {
  let event = 'message';
  let id = '';
  const data: string[] = [];
  for (const line of frame.split('\n')) {
    if (line === '' || line.startsWith(':')) continue;
    const separator = line.indexOf(':');
    const field = separator < 0 ? line : line.slice(0, separator);
    const value = separator < 0 ? '' : line.slice(separator + 1).replace(/^ /, '');
    if (field === 'event') event = value;
    else if (field === 'id') id = value;
    else if (field === 'data') data.push(value);
  }
  return { event, id, data: data.join('\n') };
}

async function readProgressResponse(
  response: Response,
  signal: AbortSignal,
  lastEventId: number,
  onEvent: ProgressListener,
): Promise<number> {
  if (!response.body) throw { code: 'DEPENDENCY_UNAVAILABLE' };
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  let current = lastEventId;
  try {
    for (;;) {
      const part = await reader.read();
      if (part.done) break;
      if (signal.aborted) throw new DOMException(t('已取消'), 'AbortError');
      buffer += decoder.decode(part.value, { stream: true }).replace(/\r\n/g, '\n');
      for (;;) {
        const end = buffer.indexOf('\n\n');
        if (end < 0) break;
        const frame = parseFrame(buffer.slice(0, end));
        buffer = buffer.slice(end + 2);
        if (frame.event !== 'progress' || frame.data === '') continue;
        let event: ProgressEvent;
        try {
          event = JSON.parse(frame.data) as ProgressEvent;
        } catch {
          throw { code: 'INTEGRITY_ERROR' };
        }
        if (
          typeof event.jobId !== 'string' ||
          !Number.isSafeInteger(event.seq) ||
          event.seq < 0 ||
          typeof event.phase !== 'string' ||
          !Number.isSafeInteger(event.completedUnits) ||
          !Number.isSafeInteger(event.totalUnits) ||
          event.completedUnits < 0 ||
          event.totalUnits < 0
        ) {
          throw { code: 'INTEGRITY_ERROR' };
        }
        const transportSequence = frame.id === '' ? event.seq : Number(frame.id);
        if (!Number.isSafeInteger(transportSequence) || transportSequence !== event.seq || event.seq <= current) {
          throw { code: 'INTEGRITY_ERROR' };
        }
        current = Math.max(current, event.seq);
        onEvent(event);
      }
    }
  } finally {
    await reader.cancel().catch(() => undefined);
    reader.releaseLock();
  }
  return current;
}

async function jobIsTerminal(jobId: string, signal: AbortSignal): Promise<boolean> {
  if (!apiClient) throw { code: 'DEPENDENCY_UNAVAILABLE' };
  const job = await apiClient.request<{ state: string }>(`/v1/jobs/${encodeURIComponent(jobId)}`, { signal });
  return terminalStates.has(job.state);
}

// 进度使用portal同源流端口；连接断开按1/2/4/8/16秒退避，终局追平EOF后停止。
export async function ReconnectProgress(
  input: ReconnectProgressInput,
  signal: AbortSignal,
  onEvent: ProgressListener,
): Promise<ReconnectProgressResult> {
  if (!apiClient || !input.jobId || !Number.isSafeInteger(input.lastEventId) || input.lastEventId < 0) {
    throw { code: 'VALIDATION' };
  }
  const events: ProgressEvent[] = [];
  let cursor = input.lastEventId;
  for (let attempt = 0; attempt <= retryDelays.length; attempt += 1) {
    try {
      const query = new URLSearchParams({ lastEventId: String(cursor) });
      const response = await apiClient.request<Response>(
        `/v1/jobs/${encodeURIComponent(input.jobId)}/progress?${query}`,
        { responseType: 'stream', signal },
      );
      cursor = await readProgressResponse(response, signal, cursor, (event) => {
        if (events.length >= 10000) throw { code: 'BUDGET_EXCEEDED' };
        if (event.jobId !== input.jobId) throw { code: 'INTEGRITY_ERROR' };
        cursor = event.seq;
        events.push(event);
        onEvent(event);
      });
    } catch (error) {
      if (signal.aborted) throw new DOMException(t('已取消'), 'AbortError');
      const code = problemCode(error);
      if (code === 'UNAUTHENTICATED' || code === 'FORBIDDEN') throw error;
      if (code === 'VALIDATION' || code === 'VERSION_MISMATCH' || code === 'INTEGRITY_ERROR' || code === 'BUDGET_EXCEEDED') throw error;
      if (code === 'GONE' || code === 'NOT_FOUND') cursor = 0;
      if (attempt === retryDelays.length) throw error;
    }
    if (await jobIsTerminal(input.jobId, signal)) return { events };
    if (attempt === retryDelays.length) throw { code: 'DEPENDENCY_UNAVAILABLE' };
    await delay(retryDelays[attempt], signal);
  }
  throw { code: 'DEPENDENCY_UNAVAILABLE' };
}
