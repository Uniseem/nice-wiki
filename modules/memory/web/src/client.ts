import type { ModuleAPIClient, ModuleAPIRequest } from '../../../../libs/platform/web/module-api';

let client: ModuleAPIClient | null = null;

export function configureAPI(next: ModuleAPIClient): void { client = next; }

export function api<T>(path: string, options?: ModuleAPIRequest): Promise<T> {
  if (!client) throw new Error('接口客户端未装配');
  return client.request<T>(path, options);
}

export function messageOf(error: unknown): string {
  const e = error as { message?: string; code?: string };
  return e?.message ? (e.code ? `${e.message}（${e.code}）` : e.message) : '操作失败';
}
