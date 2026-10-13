import { onMounted, reactive, ref } from 'vue'
import type { components } from '../../shared/api/schema.gen'
import { client, requireData } from '../../shared/api/client'
import { postSSE, SSERequestError } from '../../shared/api/sse'
export interface ChatMessage { role: 'user' | 'assistant', text: string, citations?: components['schemas']['QACitation'][], recordId?: string }
export function useChat() {
  const messages = ref<ChatMessage[]>([]); const progress = ref(''); const sending = ref(false); const error = ref(''); const verificationRequired=ref(false); const blocked=ref<{reason:string,until:string}>(); let controller: AbortController | undefined
  let conversation = localStorage.getItem('nw-conversation') ?? ''
  onMounted(async () => {
    if (!conversation) return
    try {
      const value = requireData(await client.GET('/api/public/conversation/{id}', { params: { path: { id: conversation } } }))
      messages.value = value.turns.flatMap(turn => turn.role === 'user' || turn.role === 'assistant' ? [{ role: turn.role, text: turn.text, recordId: turn.recordId } satisfies ChatMessage] : [])
    } catch {
      conversation = ''
      localStorage.removeItem('nw-conversation')
    }
  })
  async function send(question: string, page = '') {
    if (sending.value || !question.trim()) return; sending.value = true; error.value = ''; messages.value.push({ role: 'user', text: question }); const answer = reactive<ChatMessage>({ role: 'assistant', text: '' }); messages.value.push(answer); controller = new AbortController()
    try {
      await postSSE('/api/public/ask',{ question, conversationId: conversation, page, language: document.documentElement.lang }, event => { const data = event.data as components['schemas']['QAEvent']; if (event.event === 'error') throw new SSERequestError(data.code ?? 'qa_failed', data.message ?? '问答失败', 200); if (event.event === 'progress') progress.value = String(data.message ?? ''); if (event.event === 'text') answer.text += String(data.text ?? ''); if(event.event==='citations'&&Array.isArray(data.citations))answer.citations=data.citations as ChatMessage['citations']; if (event.event === 'done' && typeof data.conversationId === 'string') { conversation = data.conversationId; answer.recordId = data.recordId; localStorage.setItem('nw-conversation', conversation) } }, controller.signal)
    } catch (cause) { if (!controller.signal.aborted) { if(cause instanceof SSERequestError&&(cause.code==='verification_required'||cause.code==='qa_session_required')){verificationRequired.value=true;sessionStorage.removeItem('nw-qa-verified')}if(cause instanceof SSERequestError&&(cause.code==='qa_banned'||cause.code==='budget_exceeded')){const match=cause.message.match(/在\s+([^\s]+)\s+后/);blocked.value={reason:cause.message,until:match?.[1]??new Date(Date.now()+(cause.code==='qa_banned'?600000:3600000)).toLocaleString()}} error.value = cause instanceof Error ? cause.message : '问答失败'; if(!answer.text)messages.value.splice(-2) } }
    finally { sending.value = false; progress.value = '' }
  }
  function stop() { controller?.abort(); const last=messages.value.at(-1);if(last?.role==='assistant'&&!last.text)messages.value.pop();sending.value = false;progress.value='';error.value='已停止生成' }
  async function clear() { const id=conversation;messages.value=[];conversation='';localStorage.removeItem('nw-conversation');if(id)await client.DELETE('/api/public/conversation/{id}',{params:{path:{id}}}) }
  function verified(){verificationRequired.value=false;error.value=''}
  return { messages, progress, sending, error, verificationRequired, blocked, verified, send, stop, clear }
}
