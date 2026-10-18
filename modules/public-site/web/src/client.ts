export interface Problem { code: string; detail?: string }
export interface LoadPreviewInput {
  documentId: string;
  draftRevisionId: string;
  apiBaseURL: string;
  csrfToken: string;
  operationId: string;
  signal: AbortSignal;
}
export interface LoadPreviewResult {
  previewId: string;
  token: string;
  expiresAt: string;
  html: string;
}
export interface RequestPublicNavigationInput {
  locale: string;
  apiBaseURL: string;
  signal?: AbortSignal;
}
export interface NavigationNode {
  documentId: string;
  parentId: string | null;
  title: string;
  canonicalPath: string;
  sortKey: string;
  publication: { documentId: string; publishedRevisionId: string; revision: number; contentHash: string; schemaVersion: number; dataEpoch: string };
}
export interface RequestPublicNavigationResult {
  status: number;
  tree: NavigationNode[];
  etag: string;
}

function sameOriginURL(base: string, path: string): URL {
  const root = new URL(base, window.location.origin);
  if (root.origin !== window.location.origin || root.username || root.password || root.search || root.hash) {
    throw { code: 'VALIDATION', detail: '接口必须使用当前站点的同源地址。' } satisfies Problem;
  }
  return new URL(root.pathname.replace(/\/$/, '') + path, root.origin);
}

async function responseProblem(response: Response): Promise<Problem> {
  try {
    const value: unknown = await response.json();
    if (typeof value === 'object' && value !== null && 'code' in value && typeof value.code === 'string') {
      return { code: value.code };
    }
  } catch { /* 不回显响应正文，避免把敏感预览内容带进错误界面。 */ }
  return { code: response.status === 401 ? 'UNAUTHENTICATED' : response.status === 403 ? 'FORBIDDEN' : 'DEPENDENCY_UNAVAILABLE' };
}

// RequestPublicNavigation 不缓存private投影，503直接交还界面显示。
export async function RequestPublicNavigation(input: RequestPublicNavigationInput): Promise<RequestPublicNavigationResult> {
  const response = await fetch(sameOriginURL(input.apiBaseURL, '/' + encodeURIComponent(input.locale) + '/navigation'), {
    credentials: 'same-origin', cache: 'no-store', signal: input.signal,
  });
  if (!response.ok) throw await responseProblem(response);
  return await response.json() as RequestPublicNavigationResult;
}

// requestPreview 仅在当前页内存保留grant，不自动重试mutation。
export async function requestPreview(input: LoadPreviewInput): Promise<LoadPreviewResult> {
  const issued = await fetch(sameOriginURL(input.apiBaseURL, '/v1/preview/tokens'), {
    method: 'POST', credentials: 'same-origin', cache: 'no-store', signal: input.signal,
    headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': input.csrfToken, 'Idempotency-Key': input.operationId },
    body: JSON.stringify({ documentId: input.documentId, draftRevisionId: input.draftRevisionId }),
  });
  if (!issued.ok) throw await responseProblem(issued);
  const grant = await issued.json() as Omit<LoadPreviewResult, 'html'>;
  const previewURL = sameOriginURL(input.apiBaseURL, '/v1/preview/' + encodeURIComponent(grant.token));
  previewURL.searchParams.set('previewId', grant.previewId);
  const rendered = await fetch(previewURL, {
    credentials: 'same-origin', cache: 'no-store', referrerPolicy: 'no-referrer', signal: input.signal,
  });
  if (!rendered.ok) throw await responseProblem(rendered);
  return { ...grant, html: await rendered.text() };
}
