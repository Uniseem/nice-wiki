import { onBeforeUnmount, ref } from 'vue'
import type { components } from '../../shared/api/schema.gen'
import { client, requireData, errorMessage } from '../../shared/api/client'

type Content = components['schemas']['DocumentContent']
export function useAutosave(documentId: string, getToken: () => string, canWrite: () => boolean) {
  const version = ref(0)
  const dirty = ref(false)
  const saving = ref(false)
  const error = ref('')
  let pending: Content | undefined
  let timer: ReturnType<typeof setTimeout> | undefined
  let flight: Promise<void> | undefined
  let sequence = 0

  function schedule(content: Content) {
    if (!canWrite()) return
    pending = structuredClone(content); sequence++; dirty.value = true
    if (timer) clearTimeout(timer)
    timer = setTimeout(() => { void flush().catch(() => {}) }, 800)
  }
  async function flush() {
    if (timer) { clearTimeout(timer); timer = undefined }
    if (flight) { await flight; if (dirty.value && pending) await flush(); return }
    if (!dirty.value || !pending) return
    if (!canWrite()) throw new Error('编辑锁已失效，内容保留在本页。')
    const content = pending
    const savedSequence = sequence
    saving.value = true; error.value = ''
    flight = (async () => {
      try {
        const value = requireData(await client.PUT('/api/admin/docs/{id}/draft', {
          params: { path: { id: documentId } }, body: { version: version.value, document: content, lockToken: getToken() },
        }))
        version.value = value.version
        if (savedSequence === sequence) { dirty.value = false; pending = undefined }
      } catch (cause) { error.value = errorMessage(cause); throw cause }
      finally { saving.value = false; flight = undefined }
    })()
    await flight
    if (dirty.value && pending && savedSequence !== sequence) await flush()
  }
  const beforeUnload = (event: BeforeUnloadEvent) => { if (dirty.value) event.preventDefault() }
  window.addEventListener('beforeunload', beforeUnload)
  onBeforeUnmount(() => { if (timer) clearTimeout(timer); window.removeEventListener('beforeunload', beforeUnload) })
  return { version, dirty, saving, error, schedule, flush }
}
