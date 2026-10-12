import { defineStore } from 'pinia'
import { ref } from 'vue'
import { client, requireData } from '../../shared/api/client'
import type { components } from '../../shared/api/schema.gen'

export const useDocTreeStore = defineStore('docTree', () => {
  const documents = ref<components['schemas']['DocMeta'][]>([])
  const loading = ref(false)
  async function refresh() {
    loading.value = true
    try { documents.value = requireData(await client.GET('/api/admin/docs')) }
    finally { loading.value = false }
  }
  return { documents, loading, refresh }
})
