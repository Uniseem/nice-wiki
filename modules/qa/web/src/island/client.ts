import { solveChallenge } from '@ui/pow';

// 访客问答的同源接口：会话 ID 与准入令牌只放会话存储；需要人机验证时先做一次工作量证明再重试。
// 服务暂时不可用（发版重启、依赖还没起来）时自动退避重试，约 20 秒内恢复就不打扰访客。

export type Citation = { citationId?: string; title?: string; href?: string };
export type TurnInfo = { turnId: string; answerRevision?: number };
// 回答过程中的一步（读取当前页、理解问题、复用已有回答、检索、生成），给访客看 AI 在做什么。
export type Step = { key: 'understand' | 'page' | 'cache' | 'search' | 'answer'; status: 'running' | 'done' | 'skipped'; title: string; detail?: string; items?: string[] };
export type StreamEvent =
  | { kind: 'step'; step: Step }
  | { kind: 'delta'; text: string }
  | { kind: 'reset' }
  | { kind: 'citation'; citation: Citation }
  | { kind: 'done'; turn?: TurnInfo }
  | { kind: 'failed'; code: string }
  | { kind: 'cancelled' };

export class QaError extends Error {
  constructor(public code: string, message: string, public status = 0) { super(message); }
}

const store = window.sessionStorage;
const keys = { guard: 'nw-guard' };
// 会话按语言分开保存：切换站点语言后用新语言的会话，回答语言和检索都跟着变。
const conversationKey = (language: string) => `nw-qa-conv:${language}`;

function key(): string {
  const bytes = crypto.getRandomValues(new Uint8Array(16));
  return Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
}

function guard(): string {
  try { return store.getItem(keys.guard) ?? ''; } catch { return ''; }
}

// 暂时不可用：入口或依赖服务正在重启（503 DEPENDENCY_UNAVAILABLE、反向代理的 502/504、Cloudflare 的 520–524）或网络断开。
const busyStatus = (status: number) => status === 502 || status === 503 || status === 504 || (status >= 520 && status <= 524);
export function transient(problem: unknown): boolean {
  return problem instanceof QaError && (problem.code === 'DEPENDENCY_UNAVAILABLE' || problem.code === 'NETWORK' || busyStatus(problem.status));
}

// 退避间隔：1、2、4、6、8 秒，合计约 20 秒，盖住一次发版重启。
const backoff = [1000, 2000, 4000, 6000, 8000];
function wait(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    const timer = window.setTimeout(resolve, ms);
    signal?.addEventListener('abort', () => { window.clearTimeout(timer); reject(new DOMException('Aborted', 'AbortError')); }, { once: true });
  });
}

// 正文带幂等键的写操作重试时沿用它，服务端不会把同一次提问、同一次建会话算成两次；
// 不带的（人机验证、全文搜索这类只读或一次性的请求）每次重试换新键，入口会领新的准入。
async function raw<T>(method: string, path: string, body?: Record<string, unknown>): Promise<T> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (guard()) headers['X-Guard-Session'] = guard();
  for (let attempt = 0; ; attempt++) {
    if (method !== 'GET') headers['Idempotency-Key'] = (body?.idempotencyKey as string | undefined) ?? key();
    try {
      const response = await fetch(path, { method, headers, body: body ? JSON.stringify(body) : undefined }).catch(() => { throw new QaError('NETWORK', ''); });
      const text = await response.text();
      let json: Record<string, unknown> = {};
      try { json = JSON.parse(text); } catch { /* 非 JSON 响应 */ }
      if (!response.ok) throw new QaError(String(json.code ?? ''), String(json.detail ?? ''), response.status);
      return json as T;
    } catch (problem) {
      if (!transient(problem) || attempt >= backoff.length) throw problem;
      await wait(backoff[attempt]);
    }
  }
}

async function challenge(): Promise<void> {
  const issued = await raw<{ challengeId: string; nonce: string; difficulty: number }>('POST', '/v1/challenges', { requestId: key(), method: 'pow' });
  let proof: string;
  try { proof = await solveChallenge(issued.nonce, issued.difficulty); } catch (problem) {
    throw new QaError((problem as Error).message, '');
  }
  const passed = await raw<{ passedToken: string }>('POST', `/v1/challenges/${encodeURIComponent(issued.challengeId)}:verify`, { proof });
  try { store.setItem(keys.guard, passed.passedToken); } catch { /* 隐私模式 */ }
}

async function call<T>(method: string, path: string, body?: Record<string, unknown>): Promise<T> {
  try {
    return await raw<T>(method, path, body);
  } catch (problem) {
    if (!(problem instanceof QaError) || problem.code !== 'CHALLENGE_REQUIRED') throw problem;
    await challenge();
    return raw<T>(method, path, body);
  }
}

async function conversation(language: string): Promise<string> {
  try {
    const existing = store.getItem(conversationKey(language));
    if (existing) return existing;
  } catch { /* 隐私模式 */ }
  const created = await call<{ conversation: { conversationId: string } }>('POST', '/v1/conversations', { language, idempotencyKey: key() });
  try { store.setItem(conversationKey(language), created.conversation.conversationId); } catch { /* 隐私模式 */ }
  return created.conversation.conversationId;
}

export function resetConversation(language: string): void {
  try { store.removeItem(conversationKey(language)); } catch { /* 隐私模式 */ }
}

export type StartedTurn = { turn: TurnInfo; streamPath: string };

// 提问：带上访客所在页面（AI 会把这一页全文作为预读材料）；会话已过期（GONE）、已不存在（NOT_FOUND），
// 或已不属于当前访客（FORBIDDEN：访客身份由人机验证通过证明决定，证明过期、重新验证后身份就变了）时，换一个新会话再试一次。
export async function startTurn(question: string, language: string, documentId?: string): Promise<StartedTurn> {
  for (let attempt = 0; ; attempt++) {
    const id = await conversation(language);
    try {
      const body: Record<string, unknown> = { conversationId: id, question, idempotencyKey: key() };
      if (documentId && /^[A-Za-z0-9._:-]{1,128}$/.test(documentId)) body.currentDocumentId = documentId;
      return await call<StartedTurn>('POST', '/v1/turns', body);
    } catch (problem) {
      if (attempt === 0 && problem instanceof QaError && ['GONE', 'NOT_FOUND', 'FORBIDDEN'].includes(problem.code)) { resetConversation(language); continue; }
      throw problem;
    }
  }
}

// 跟随回答流（SSE）：断开后从最后一个序号续读，直到完成、失败或取消；服务暂时不可用或中途断网时退避后再续读。
export async function followTurn(started: StartedTurn, onEvent: (event: StreamEvent) => void, signal: AbortSignal): Promise<void> {
  let last = 0;
  let retries = 0;
  for (;;) {
    let ended = false;
    try {
      ended = await readStream(started, last, onEvent, signal, (sequence) => { last = sequence; retries = 0; });
    } catch (problem) {
      if (signal.aborted) throw problem;
      const failure = problem instanceof QaError ? problem : new QaError('NETWORK', '');
      if (!transient(failure) || retries >= backoff.length) throw failure;
      await wait(backoff[retries++], signal);
      continue;
    }
    if (ended || signal.aborted) return;
  }
}

// 读一次回答流，返回这一轮是否已经结束；每收到带序号的事件就回报序号，断线后从这里续读。
async function readStream(started: StartedTurn, after: number, onEvent: (event: StreamEvent) => void, signal: AbortSignal, onSequence: (sequence: number) => void): Promise<boolean> {
  let last = after;
  const headers: Record<string, string> = guard() ? { 'X-Guard-Session': guard() } : {};
  const response = await fetch(`${started.streamPath}?afterSequence=${last}&limit=100`, { headers, signal });
  if (!response.ok || !response.body) throw new QaError('STREAM', '', response.status);
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  let ended = false;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true }).replace(/\r\n/g, '\n');
    const parts = buffer.split('\n\n');
    buffer = parts.pop() ?? '';
    for (const part of parts) {
      let event = '', data = '', id = '';
      for (const line of part.split('\n')) {
        if (line.startsWith('event:')) event = line.slice(6).trim();
        else if (line.startsWith('data:')) data += line.slice(5).trim();
        else if (line.startsWith('id:')) id = line.slice(3).trim();
      }
      if (id) { last = Number.parseInt(id, 10) || last; onSequence(last); }
      if (!data) continue;
      let payload: { delta?: string; citation?: Citation; code?: string; turn?: TurnInfo; step?: Step };
      try { payload = JSON.parse(data); } catch { continue; }
      if (event === 'step' && payload.step) onEvent({ kind: 'step', step: payload.step });
      else if (event === 'delta' && payload.delta) onEvent({ kind: 'delta', text: payload.delta });
      else if (event === 'invalidate') onEvent({ kind: 'reset' });
      else if (event === 'citation' && payload.citation) onEvent({ kind: 'citation', citation: payload.citation });
      else if (event === 'failed') { onEvent({ kind: 'failed', code: payload.code ?? '' }); ended = true; }
      else if (event === 'cancelled') { onEvent({ kind: 'cancelled' }); ended = true; }
      else if (event === 'completed') { onEvent({ kind: 'done', turn: payload.turn }); ended = true; }
    }
  }
  return ended;
}

export type SearchHit = { title: string; href: string; excerpt: string };

// 全文搜索（⌘K 里按回车）：在知识索引上做关键词 + 向量混合检索，返回命中段落；每次算访客一次问答次数。
export async function searchDocuments(query: string, language: string): Promise<SearchHit[]> {
  const found = await call<{ results: SearchHit[] }>('POST', '/v1/search', { query, language });
  return found.results;
}

export async function cancelTurn(turnId: string): Promise<void> {
  await call('POST', `/v1/turns/${encodeURIComponent(turnId)}/cancel`, { turnId, reason: 'visitor-stopped', idempotencyKey: key() });
}

export async function sendFeedback(turn: TurnInfo, value: 'helpful' | 'unhelpful'): Promise<void> {
  const idempotencyKey = key();
  await call('PUT', `/v1/turns/${encodeURIComponent(turn.turnId)}/feedback`, { turnId: turn.turnId, answerRevision: turn.answerRevision ?? 0, value, idempotencyKey });
}
