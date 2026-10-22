import type { ModuleAPIClient, ModuleAPIRequest } from '../module-api';
import { t } from './i18n';

// 浏览器端的同源 API 客户端：HttpOnly 的会话 cookie 由浏览器自带，CSRF 令牌、会话 ID 和授权项存本地存储。
// cookie 是同一浏览器所有标签页共用的，配套的 CSRF 也必须共用：任一标签页刷新会话后，其他标签页立即读到新令牌。
// 各域页面只用它请求本站版本化路径，不接触凭据，也不拼接其他来源的地址。

export class ApiError extends Error {
  constructor(public status: number, public code: string, public detail: string, public body: unknown) {
    super(detail || t('请求失败（{0}）', status));
  }
}

const keys = { csrf: 'nw_csrf', grants: 'nw_grants', session: 'nw_session_id' };

function read(key: string): string {
  try { return localStorage.getItem(key) ?? ''; } catch { return ''; }
}

function write(key: string, value: string | null): void {
  try {
    if (value === null) localStorage.removeItem(key);
    else localStorage.setItem(key, value);
  } catch { /* 隐私模式下忽略 */ }
}

type SessionReply = { csrfToken?: string; sessionId?: string; auth?: { grants?: string[] } };
const grantListeners = new Set<() => void>();

// 保存登录、刷新或改口令后拿到的新会话；传 null 表示退出。
export function storeSession(session: SessionReply | null): void {
  if (!session) {
    write(keys.csrf, null);
    write(keys.session, null);
    write(keys.grants, null);
  } else {
    if (session.csrfToken) write(keys.csrf, session.csrfToken);
    if (session.sessionId) write(keys.session, session.sessionId);
    if (session.auth?.grants) write(keys.grants, JSON.stringify(session.auth.grants));
  }
  grantsCache = null;
  grantListeners.forEach((listener) => listener());
}

let grantsCache: string[] | null = null;
export function currentGrants(): string[] {
  if (!grantsCache) {
    try { grantsCache = JSON.parse(read(keys.grants) || '[]') as string[]; } catch { grantsCache = []; }
  }
  return grantsCache;
}

// 其他标签页登录、刷新或退出：授权项跟着变；退出了就回登录页。
if (typeof window !== 'undefined') {
  addEventListener('storage', (event) => {
    if (event.key !== keys.grants && event.key !== keys.csrf && event.key !== null) return;
    grantsCache = null;
    grantListeners.forEach((listener) => listener());
    if (!read(keys.csrf)) unauthorizedListeners.forEach((listener) => listener());
  });
}

export function subscribeGrants(listener: () => void): () => void {
  grantListeners.add(listener);
  return () => grantListeners.delete(listener);
}

export function currentSessionId(): string {
  return read(keys.session);
}

export function newKey(): string {
  const bytes = crypto.getRandomValues(new Uint8Array(16));
  return Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
}

let refreshing: Promise<boolean> | null = null;

// 访问令牌过期时静默刷新一次；刷新失败说明需要重新登录。
// seen 是发现需要刷新时手里的 CSRF：多个标签页排队刷新，轮到时令牌已被别的标签页换新就直接用新的，
// 不再拿同一个已消费的刷新凭据去刷新（那会被当成凭据被盗而撤销整个会话）。
export function refresh(seen = read(keys.csrf)): Promise<boolean> {
  refreshing ??= withRefreshLock(async () => {
    if (read(keys.csrf) && read(keys.csrf) !== seen) return true;
    try {
      const response = await fetch('/v1/sessions:refresh', {
        method: 'POST', credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json', 'Idempotency-Key': newKey(), 'X-CSRF-Token': read(keys.csrf) },
        body: '{}',
      });
      if (!response.ok) return false;
      const body = await response.json() as { session?: SessionReply };
      storeSession(body.session ?? null);
      return !!body.session;
    } catch { return false; }
  }).finally(() => { setTimeout(() => { refreshing = null; }, 0); });
  return refreshing;
}

// 同一浏览器的标签页依次刷新。Web Locks 只在安全上下文（HTTPS 或本机）可用，没有时退回本标签页内去重。
function withRefreshLock(run: () => Promise<boolean>): Promise<boolean> {
  const locks = (navigator as Navigator & { locks?: LockManager }).locks;
  return locks ? locks.request('nw-session-refresh', run) : run();
}

const unauthorizedListeners = new Set<() => void>();
// 刷新后仍是 401：通知外壳回到登录页。
export function onUnauthorized(listener: () => void): () => void {
  unauthorizedListeners.add(listener);
  return () => unauthorizedListeners.delete(listener);
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
  if (options.backupDownloadToken) headers['X-Backup-Download-Token'] = options.backupDownloadToken;
  if (options.backupPassphrase) headers['X-Backup-Passphrase'] = options.backupPassphrase;
  const csrf = read(keys.csrf);
  if (method !== 'GET') {
    headers['Idempotency-Key'] = options.idempotencyKey ?? newKey();
    headers['X-CSRF-Token'] = csrf;
  }
  const response = await fetch(path, { method, headers, body, credentials: 'same-origin', signal: options.signal });
  if (response.status === 401 && !retried && !path.startsWith('/v1/sessions') && path !== '/v1/registration') {
    if (await refresh(csrf)) return request<T>(path, options, true);
    unauthorizedListeners.forEach((listener) => listener());
  }
  // 请求在途时别的标签页刷新了会话，带去的 CSRF 已作废：换新令牌重发一次。其他 403 照常报错。
  if (response.status === 403 && !retried && method !== 'GET' && read(keys.csrf) && read(keys.csrf) !== csrf) {
    return request<T>(path, options, true);
  }
  if (!response.ok) {
    let payload: { code?: string; detail?: string } | null = null;
    try { payload = await response.json(); } catch { /* 非 JSON 错误体 */ }
    throw new ApiError(response.status, payload?.code ?? 'UNKNOWN', payload?.detail ?? '', payload);
  }
  switch (options.responseType) {
    case 'stream': return response as T;
    case 'blob': return (await response.blob()) as T;
    case 'text': return (await response.text()) as T;
    default: return (response.status === 204 ? undefined : await response.json()) as T;
  }
}

export const api = request;
export const apiClient: ModuleAPIClient = { request };

const codeText: Record<string, string> = {
  VALIDATION: t('提交的内容不符合要求'),
  UNAUTHENTICATED: t('登录已失效，请重新登录'),
  FORBIDDEN: t('当前账号没有这项操作的权限'),
  NOT_FOUND: t('对象不存在或已被删除'),
  GONE: t('对象已过期或已被删除'),
  CONFLICT: t('数据已被其他操作修改，请刷新后重试'),
  VERSION_MISMATCH: t('数据已被其他人修改，请刷新后重试'),
  LOCKED: t('对象正被占用，请稍后再试'),
  UNSUPPORTED_SCHEMA: t('内容格式暂不支持'),
  UNSUPPORTED_CAPABILITY: t('这项能力尚未配置或暂不支持'),
  RATE_LIMITED: t('操作太频繁，请稍后再试'),
  BUDGET_EXCEEDED: t('超出数量或大小上限'),
  CANCELLED: t('操作已取消'),
  DEADLINE_EXCEEDED: t('服务响应超时，请稍后再试'),
  DEPENDENCY_UNAVAILABLE: t('依赖的服务暂不可用，请稍后再试'),
  STALE_RESULT: t('数据已更新，请刷新后重试'),
  INTEGRITY_ERROR: t('完整性校验未通过'),
  INTERNAL: t('服务内部错误'),
  CHALLENGE_REQUIRED: t('需要先完成人机验证'),
  INVOCATION_NOT_ACCEPTED: t('调用未被受理'),
};

// 任务失败码这类只有错误码的场合：给出可读含义，认不出的码原样显示。
export function codeMeaning(code: string): string {
  return codeText[code] ?? code;
}

export function problemCode(error: unknown): string {
  if (error instanceof ApiError) return error.code;
  const code = (error as { code?: unknown } | null)?.code;
  return typeof code === 'string' ? code : '';
}

// 给用户看的错误说明：先给出可操作的中文含义，服务端的细节放在括号里。
export function errorText(error: unknown): string {
  if (error instanceof ApiError) {
    const base = codeText[error.code] ?? t('请求失败（{0}）', error.status);
    return error.detail && error.detail !== base ? t('{0}（{1}）', base, error.detail) : base;
  }
  if (error instanceof DOMException && error.name === 'AbortError') return t('操作已取消');
  if (error instanceof Error) return error.message;
  const code = problemCode(error);
  return codeText[code] ?? t('操作失败');
}
