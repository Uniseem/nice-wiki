import { ref } from 'vue'
import { client, requireData, errorMessage } from '../../shared/api/client'
import type { components } from '../../shared/api/schema.gen'

type Kind = 'images' | 'files' | 'videos' | 'drawings'
export function useUpload() {
  const uploading = ref(false)
  const error = ref('')
  async function upload(file: File, kind: Kind): Promise<components['schemas']['Asset']> {
    if (uploading.value) throw new Error('已有上传任务，请稍后')
    if (!file.size || file.size > 128 * 1024 * 1024) throw new Error('文件为空或超过 128 MiB')
    uploading.value = true; error.value = ''
    try {
      const form = new FormData(); form.set('kind', kind); form.set('file', file, file.name)
      return requireData(await client.POST('/api/admin/assets/upload', {
        body: { file: file.name, kind }, bodySerializer: () => form,
      }))
    } catch (cause) { error.value = errorMessage(cause); throw cause }
    finally { uploading.value = false }
  }
  return { uploading, error, upload }
}
