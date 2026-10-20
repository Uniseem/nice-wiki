import type { ModuleAPIClient, ModuleAPIRequest } from '../../../../libs/platform/web/module-api';

// 浏览器端的同源 API 客户端：CSRF 令牌只存会话存储，HttpOnly 的会话 cookie 由浏览器自带。
const csrfKey = 'nw_csrf';

export class ApiError extends Error {
  constructor(public status: number, public code: string, message: string, public body: unknown) {
    super(message);
  }
}

export function setCSRF(token: string | null, sessionId?: string | null, grants?: string[]): void {
  try {
    if (token) sessionStorage.setItem(csrfKey, token);
    else sessionStorage.removeItem(csrfKey);
    if (grants) sessionStorage.setItem('nw_grants', JSON.stringify(grants));
    if (sessionId !== undefined) {
      if (sessionId) sessionStorage.setItem('nw_session_id', sessionId);
      else sessionStorage.removeItem('nw_session_id');
    }
  } catch { /* 隐私模式下忽略 */ }
}

export function currentGrants(): string[] {
  try { return JSON.parse(sessionStorage.getItem('nw_grants') ?? '[]'); } catch { return []; }
}

export function currentSessionId(): string {
  try { return sessionStorage.getItem('nw_session_id') ?? ''; } catch { return ''; }
}

function csrf(): string {
  try { return sessionStorage.getItem(csrfKey) ?? ''; } catch { return ''; }
}

function newKey(): string {
  const bytes = crypto.getRandomValues(new Uint8Array(16));
  return Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
}

let refreshing: Promise<boolean> | null = null;

// 访问令牌过期时静默刷新一次；刷新失败说明需要重新登录。
export async function refresh(): Promise<boolean> {
  refreshing ??= (async () => {
    try {
      const response = await fetch('/v1/sessions:refresh', {
        method: 'POST', credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json', 'Idempotency-Key': newKey(), 'X-CSRF-Token': csrf() },
        body: '{}',
      });
      if (!response.ok) return false;
      const body = await response.json();
      setCSRF(body?.session?.csrfToken ?? body?.tokens?.csrfToken ?? null, body?.session?.sessionId, body?.session?.auth?.grants);
      return true;
    } catch { return false; } finally { setTimeout(() => { refreshing = null; }, 0); }
  })();
  return refreshing;
}

export async function request<T>(path: string, options: ModuleAPIRequest = {}, retried = false): Promise<T> {
  const method = options.method ?? 'GET';
  const headers: Record<string, string> = {};
  let body: BodyInit | undefined;
  if (options.body !== undefined) {
    if (options.body instanceof Blob || options.body instanceof ArrayBuffer) {
      body = options.body as BodyInit;
    } else {
      headers['Content-Type'] = 'application/json';
      body = JSON.stringify(options.body);
    }
  }
  if (options.uploadChunk) {
    headers['Content-Type'] = 'application/octet-stream';
    headers['X-Upload-Offset'] = String(options.uploadChunk.offset);
    headers['X-Chunk-SHA256'] = options.uploadChunk.sha256;
    headers['If-Match'] = String(options.uploadChunk.revision);
  }
  if (method !== 'GET') {
    headers['Idempotency-Key'] = options.idempotencyKey ?? newKey();
    headers['X-CSRF-Token'] = csrf();
  }
  const response = await fetch(path, { method, headers, body, credentials: 'same-origin', signal: options.signal });
  if (response.status === 401 && !retried && !path.startsWith('/v1/sessions') && (await refresh())) {
    return request<T>(path, options, true);
  }
  if (!response.ok) {
    let payload: any = null;
    try { payload = await response.json(); } catch { /* 非 JSON 错误体 */ }
    throw new ApiError(response.status, payload?.code ?? 'UNKNOWN', payload?.detail ?? `请求失败（${response.status}）`, payload);
  }
  switch (options.responseType) {
    case 'blob': return (await response.blob()) as T;
    case 'text': return (await response.text()) as T;
    default: return (response.status === 204 ? undefined : await response.json()) as T;
  }
}

export const apiClient: ModuleAPIClient = { request };

export function errorText(error: unknown): string {
  if (error instanceof ApiError) return `${error.message}（${error.code}）`;
  return error instanceof Error ? error.message : '操作失败';
}
