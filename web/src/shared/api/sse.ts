export interface SSEEvent<T = unknown> { event: string, data: T }
export class SSERequestError extends Error {
  constructor(public code: string, message: string, public status: number) { super(message) }
}
export async function readSSE(response: Response, receive: (event: SSEEvent) => void, signal?: AbortSignal): Promise<void> {
  if (!response.ok) {
    let code = 'request_failed'; let message = `流式请求失败：${response.status}`
    try { const value = await response.json() as { code?: string, message?: string }; code = value.code ?? code; message = value.message ?? message } catch { /* 使用通用错误。 */ }
    throw new SSERequestError(code, message, response.status)
  }
  if (!response.body) throw new SSERequestError('empty_response', '流式响应为空', response.status)
  const reader = response.body.getReader(); const decoder = new TextDecoder(); let buffer = ''
  while (!signal?.aborted) {
    const { value, done } = await reader.read(); if (done) break; buffer += decoder.decode(value, { stream: true })
    let boundary = buffer.indexOf('\n\n')
    while (boundary >= 0) {
      const frame = buffer.slice(0, boundary); buffer = buffer.slice(boundary + 2); let event = 'message'; const data: string[] = []
      for (const line of frame.split('\n')) { if (line.startsWith('event:')) event = line.slice(6).trim(); else if (line.startsWith('data:')) data.push(line.slice(5).trimStart()) }
      if (data.length) { let parsed: unknown = data.join('\n'); try { parsed = JSON.parse(String(parsed)) } catch { /* 文本事件保持原样。 */ } receive({ event, data: parsed }) }
      boundary = buffer.indexOf('\n\n')
    }
  }
}
export async function postSSE(path:string,body:unknown,receive:(event:SSEEvent)=>void,signal?:AbortSignal):Promise<void>{const response=await fetch(path,{method:'POST',headers:{'Content-Type':'application/json'},credentials:'same-origin',body:JSON.stringify(body),signal});return readSSE(response,receive,signal)}
