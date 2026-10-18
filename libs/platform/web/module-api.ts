// 浏览器内的非序列化端口：由portal注入同源API调用能力，不传递HttpOnly凭据。
export interface ModuleAPIRequest {
  method?: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  body?: unknown;
  signal?: AbortSignal;
  responseType?: "json" | "blob" | "text" | "stream";
  idempotencyKey?: string;
  // 只由portal映射至data-protection对应固定操作的单个头；不开放任意headers。
  restoreProgressToken?: string;
  restoreControlGrant?: string;
  backupDownloadToken?: string;
}

export interface ModuleAPIClient {
  // path是本域版本化路径与可选查询，不允许URL、其他域前缀或调用方自定义认证头。
  // portal负责前缀、同源cookies、CSRF、单次会话刷新、错误分类与用户权限变化。
  request<T>(path: string, options?: ModuleAPIRequest): Promise<T>;
}

export interface ModuleUIBootstrap {
  module: string;
  apiBaseURL: string;
  locale: string;
  mountSelector: string;
  capabilities: readonly string[];
  buildRevision: string;
  apiClient: ModuleAPIClient;
}
