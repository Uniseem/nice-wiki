import type { ModuleAPIClient } from '../../../../libs/platform/web/module-api';
import { t } from '@ui';

export interface StatisticRow { periodStart: string; documentId: string; locale: string; views: number; visits: number }
export interface PageStatistics { rows: StatisticRow[]; watermark: number; dropCount: number; granularity: string; fromUTC: string; toUTC: string }
export interface AuditResult { auditJSON: { auditId: string; action: string; actorId: string; resourceId: string; resultCode: string; occurredAt: string; hash: string; previousHash: string; [key: string]: unknown }; chainValid: boolean }
export interface LoadStatisticsInput { from: string; to: string }
export interface LoadStatisticsResult { rows: StatisticRow[]; watermark: number; dropCount: number; fromUTC: string; toUTC: string }
export interface LoadAuditPageInput { cursor: string; limit: number; action?: string; from?: string; to?: string }
export interface LoadAuditPageResult { auditIds: string[]; nextCursor: string }
// 计费暂停期间不导出费用。
export type ExportKind = 'pages' | 'audits';
export interface ExportResult { exportId: string; kind: ExportKind; expiresAt: string; watermark: number; objectRef: { byteSize: number; sha256: string } }
export interface DownloadAuthorizedExportInput { exportId: string; kind?: ExportKind }
export interface DownloadAuthorizedExportResult { downloaded: boolean }
export interface AnalyticsClient {
  LoadStatistics(input: LoadStatisticsInput, signal: AbortSignal): Promise<LoadStatisticsResult>;
  LoadAuditPage(input: LoadAuditPageInput, signal: AbortSignal): Promise<LoadAuditPageResult>;
  ReadAudit(id: string, signal: AbortSignal): Promise<AuditResult>;
  RequestExport(input: { kind: ExportKind; from: string; to: string; operationId: string }, signal: AbortSignal): Promise<ExportResult>;
  DownloadAuthorizedExport(input: DownloadAuthorizedExportInput, signal: AbortSignal): Promise<DownloadAuthorizedExportResult>;
}

export function problemCode(error: unknown): string { const candidate = error as { code?: unknown; problem?: { code?: unknown } }; const code = candidate?.code ?? candidate?.problem?.code; return typeof code === 'string' && /^[A-Z_]{1,64}$/.test(code) ? code : 'DEPENDENCY_UNAVAILABLE'; }
export function isAccessFailure(error: unknown): boolean { const code = problemCode(error); return code === 'UNAUTHENTICATED' || code === 'FORBIDDEN'; }
export function problemText(error: unknown): string { switch (problemCode(error)) { case 'UNAUTHENTICATED': return t('登录已失效，请重新登录。'); case 'FORBIDDEN': return t('当前权限不足，已清空已加载内容。'); case 'STALE_RESULT': case 'VERSION_MISMATCH': return t('数据或权限版本已变化，请重新载入。'); case 'GONE': return t('导出已过期，请重新创建。'); case 'VALIDATION': return t('请检查类型、日期与完整时间边界。'); case 'BUDGET_EXCEEDED': return t('结果超过预算，请缩小日期范围。'); case 'INTEGRITY_ERROR': return t('完整性校验未通过，未提供下载。'); default: return t('当前能力暂不可用，请稍后重试。'); } }
export function utcInput(value: string): string { const date = new Date(`${value}Z`); if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/.test(value) || !Number.isFinite(date.getTime()) || date.toISOString().slice(0, 16) !== value) throw { code: 'VALIDATION' }; return date.toISOString(); }

// 每个挂载实例持有自己的同源端口与真实导出kind；不保存凭据或猜测缺失kind。
export function createAnalyticsClient(api: ModuleAPIClient): AnalyticsClient {
  const exportKinds = new Map<string, ExportKind>();
  async function LoadStatistics(input: LoadStatisticsInput, signal: AbortSignal): Promise<LoadStatisticsResult> {
    const query = new URLSearchParams({ from: input.from, to: input.to, granularity: 'hour' });
    // 计费暂停：只读访问统计，不再读模型费用投影。
    const pages = await api.request<PageStatistics>(`/v1/statistics/pages?${query}`, { signal });
    if (signal.aborted) throw new DOMException(t('已取消'), 'AbortError');
    return { rows: pages.rows, watermark: pages.watermark, dropCount: pages.dropCount, fromUTC: pages.fromUTC, toUTC: pages.toUTC };
  }
  async function LoadAuditPage(input: LoadAuditPageInput, signal: AbortSignal): Promise<LoadAuditPageResult> {
    const query = new URLSearchParams({ limit: String(input.limit) }); if (input.cursor) query.set('cursor', input.cursor); if (input.action) query.set('action', input.action); if (input.from) query.set('from', input.from); if (input.to) query.set('to', input.to);
    return api.request<LoadAuditPageResult>(`/v1/audits?${query}`, { signal });
  }
  async function FetchAuditRecord(id: string, signal: AbortSignal): Promise<AuditResult> { return api.request<AuditResult>(`/v1/audits/${encodeURIComponent(id)}`, { signal }); }
  async function RequestExport(input: { kind: ExportKind; from: string; to: string; operationId: string }, signal: AbortSignal): Promise<ExportResult> {
    const result = await api.request<ExportResult>(input.kind === 'audits' ? '/v1/audit-exports' : '/v1/statistics:export', { method: 'POST', body: { kind: input.kind, from: input.from, to: input.to }, idempotencyKey: input.operationId, signal }); exportKinds.set(result.exportId, result.kind); return result;
  }
  async function DownloadAuthorizedExport(input: DownloadAuthorizedExportInput, signal: AbortSignal): Promise<DownloadAuthorizedExportResult> {
    const kind = input.kind ?? exportKinds.get(input.exportId); if (!kind) throw { code: 'VALIDATION' };
    const response = await api.request<Response>(`${kind === 'audits' ? '/v1/audit-exports/' : '/v1/statistics/exports/'}${encodeURIComponent(input.exportId)}`, { responseType: 'stream', signal });
    const reader = response.body?.getReader(); if (!reader) throw { code: 'DEPENDENCY_UNAVAILABLE' };
    const chunks: ArrayBuffer[] = []; let bytes = 0;
    try { for (;;) { const result = await reader.read(); if (signal.aborted) throw new DOMException(t('已取消'), 'AbortError'); if (result.done) break; bytes += result.value.byteLength; if (bytes > 32 * 1024 * 1024) throw { code: 'BUDGET_EXCEEDED' }; chunks.push(result.value.slice().buffer); } } finally { await reader.cancel().catch(() => {}); reader.releaseLock(); }
    if (signal.aborted) throw new DOMException(t('已取消'), 'AbortError');
    const length = response.headers.get('Content-Length'); if (length !== null && Number(length) !== bytes) throw { code: 'INTEGRITY_ERROR' };
    const url = URL.createObjectURL(new Blob(chunks, { type: 'text/csv;charset=utf-8' })); const link = document.createElement('a'); link.href = url; link.download = 'analytics.csv'; link.hidden = true; document.body.appendChild(link); link.click(); link.remove(); setTimeout(() => URL.revokeObjectURL(url), 0); return { downloaded: true };
  }
  return { LoadStatistics, LoadAuditPage, ReadAudit: FetchAuditRecord, RequestExport, DownloadAuthorizedExport };
}
