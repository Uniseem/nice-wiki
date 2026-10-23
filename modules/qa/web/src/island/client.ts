import { solveChallenge } from '@ui/pow';

// 访客问答的同源接口：会话 ID 与准入令牌只放会话存储；需要人机验证时先做一次工作量证明再重试。

export type Citation = { citationId?: string; title?: string; href?: string };
export type TurnInfo = { turnId: string; answerRevision?: number };
// 回答过程中的一步（读取当前页、检索、生成），给访客看 AI 在做什么。
export type Step = { key: 'understand' | 'page' | 'search' | 'answer'; status: 'running' | 'done' | 'skipped'; title: string; detail?: string; items?: string[] };
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

async function raw<T>(method: string, path: string, body?: Record<string, unknown>): Promise<T> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (method !== 'GET') headers['Idempotency-Key'] = (body?.idempotencyKey as string | undefined) ?? key();
  if (guard()) headers['X-Guard-Session'] = guard();
  const response = await fetch(path, { method, headers, body: body ? JSON.stringify(body) : undefined });
  const text = await response.text();
  let json: Record<string, unknown> = {};
  try { json = JSON.parse(text); } catch { /* 非 JSON 响应 */ }
  if (!response.ok) throw new QaError(String(json.code ?? ''), String(json.detail ?? ''), response.status);
  return json as T;
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

// 跟随回答流（SSE）：断开后从最后一个序号续读，直到完成、失败或取消。
export async function followTurn(started: StartedTurn, onEvent: (event: StreamEvent) => void, signal: AbortSignal): Promise<void> {
  let last = 0;
  for (;;) {
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
        if (id) last = Number.parseInt(id, 10) || last;
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
    if (ended || signal.aborted) return;
  }
}

export async function cancelTurn(turnId: string): Promise<void> {
  await call('POST', `/v1/turns/${encodeURIComponent(turnId)}/cancel`, { turnId, reason: 'visitor-stopped', idempotencyKey: key() });
}

export async function sendFeedback(turn: TurnInfo, value: 'helpful' | 'unhelpful'): Promise<void> {
  const idempotencyKey = key();
  await call('PUT', `/v1/turns/${encodeURIComponent(turn.turnId)}/feedback`, { turnId: turn.turnId, answerRevision: turn.answerRevision ?? 0, value, idempotencyKey });
}
