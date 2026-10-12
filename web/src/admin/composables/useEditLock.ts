import { onBeforeUnmount, ref } from 'vue'
import { client, requireData, errorMessage } from '../../shared/api/client'

export function useEditLock(documentId: string) {
  const token = ref('')
  const readonly = ref(true)
  const notice = ref('')
  const idleWarning = ref(false)
  let lastInput = Date.now()
  let timer: ReturnType<typeof setInterval> | undefined
  let pending = false
  let disposed = false

  function activity() { lastInput = Date.now(); idleWarning.value = false }
  async function release() {
    if (timer) { clearInterval(timer); timer = undefined }
    const previous = token.value
    token.value = ''; readonly.value = true
    if (previous) await client.DELETE('/api/admin/docs/{id}/lock', { params: { path: { id: documentId } }, body: { token: previous } })
  }
  async function acquire() {
    notice.value = ''
    try {
      const value = requireData(await client.POST('/api/admin/docs/{id}/lock', { params: { path: { id: documentId } } }))
      if (disposed) { await client.DELETE('/api/admin/docs/{id}/lock', { params: { path: { id: documentId } }, body: { token: value.token } }); return }
      token.value = value.token; readonly.value = false; activity()
      timer = setInterval(async () => {
        if (pending || disposed) return
        const idle = Date.now() - lastInput
        if (idle > 17 * 60_000) { notice.value = '长时间未输入，编辑锁已释放。'; await release(); return }
        idleWarning.value = idle > 15 * 60_000
        pending = true
        try { requireData(await client.PUT('/api/admin/docs/{id}/lock', { params: { path: { id: documentId } }, body: { token: token.value } })) }
        catch (cause) { readonly.value = true; notice.value = errorMessage(cause); if (timer) clearInterval(timer) }
        finally { pending = false }
      }, 30_000)
    } catch (cause) { notice.value = errorMessage(cause) }
  }
  onBeforeUnmount(() => { disposed = true; void release().catch(() => {}) })
  return { token, readonly, notice, idleWarning, activity, acquire, release }
}
