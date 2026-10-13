import { reactive, ref } from 'vue'
import type { components } from '../../shared/api/schema.gen'
import { postSSE } from '../../shared/api/sse'
export interface ChatMessage { role: 'user' | 'assistant', text: string, citations?: components['schemas']['QACitation'][] }
export function useChat() {
  const messages = ref<ChatMessage[]>([]); const progress = ref(''); const sending = ref(false); const error = ref(''); let controller: AbortController | undefined
  let conversation = localStorage.getItem('nw-conversation') ?? ''
  async function send(question: string, page = '') {
    if (sending.value || !question.trim()) return; sending.value = true; error.value = ''; messages.value.push({ role: 'user', text: question }); const answer = reactive<ChatMessage>({ role: 'assistant', text: '' }); messages.value.push(answer); controller = new AbortController()
    try {
      await postSSE('/api/public/ask',{ question, conversationId: conversation, page, language: document.documentElement.lang }, event => { const data = event.data as components['schemas']['QAEvent']; if (event.event === 'error') throw new Error(data.message ?? '问答失败'); if (event.event === 'progress') progress.value = String(data.message ?? ''); if (event.event === 'text') answer.text += String(data.text ?? ''); if(event.event==='citations'&&Array.isArray(data.citations))answer.citations=data.citations as ChatMessage['citations']; if (event.event === 'done' && typeof data.conversationId === 'string') { conversation = data.conversationId; localStorage.setItem('nw-conversation', conversation) } }, controller.signal)
    } catch (cause) { if (!controller.signal.aborted) error.value = cause instanceof Error ? cause.message : '问答失败' }
    finally { sending.value = false; progress.value = '' }
  }
  function stop() { controller?.abort(); sending.value = false }
  function clear() { messages.value = []; conversation = ''; localStorage.removeItem('nw-conversation') }
  return { messages, progress, sending, error, send, stop, clear }
}
