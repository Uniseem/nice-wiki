import createClient from 'openapi-fetch'
import type { paths } from './schema.gen'

export class APIError extends Error {
  constructor(public readonly code: string, message: string, public readonly status: number) {
    super(message)
    this.name = 'APIError'
  }
}

export const client = createClient<paths>({ credentials: 'same-origin' })
client.use({
  async onResponse({ response, request }) {
    if (response.ok) return response
    const data = await response.clone().json().catch(() => ({ code: 'network_error', message: '请求失败' }))
    const requestPath = new URL(request.url, location.href).pathname
    if (response.status === 401 && requestPath.startsWith('/api/admin/') && !requestPath.endsWith('/api/admin/auth/me') && !requestPath.endsWith('/api/admin/auth/login') && !location.pathname.endsWith('/login') && !location.pathname.endsWith('/register')) {
      location.assign('/admin/login')
    }
    if (data.code === 'password_change_required' && !location.pathname.endsWith('/password')) {
      location.assign('/admin/password')
    }
    return response
  },
})

export function requireData<T>(result: { data?: T; error?: unknown; response: Response }): T {
  if (result.error || result.data === undefined) {
    const error = result.error as { code?: string; message?: string } | undefined
    throw new APIError(error?.code ?? 'request_failed', error?.message ?? '请求失败', result.response.status)
  }
  return result.data
}

export function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : '操作失败，请重试'
}
