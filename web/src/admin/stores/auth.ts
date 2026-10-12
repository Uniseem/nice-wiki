import { defineStore } from 'pinia'
import { ref } from 'vue'
import { client, requireData } from '../../shared/api/client'
import type { components } from '../../shared/api/schema.gen'

type Account = components['schemas']['Account']
type SetupState = components['schemas']['SetupState']
export const useAuthStore = defineStore('auth', () => {
  const account = ref<Account | null>(null)
  const setup = ref<SetupState | null>(null)
  const loaded = ref(false)
  let loading: Promise<void> | undefined

  async function refresh() {
    setup.value = requireData(await client.GET('/api/admin/setup'))
    const response = await client.GET('/api/admin/auth/me')
    account.value = response.data ?? null
    loaded.value = true
  }
  async function initialize() {
    if (loaded.value) return
    loading ??= refresh().finally(() => { loading = undefined })
    await loading
  }
  async function login(username: string, password: string) {
    account.value = requireData(await client.POST('/api/admin/auth/login', { body: { username, password } }))
    setup.value = requireData(await client.GET('/api/admin/setup'))
    loaded.value = true
  }
  async function register(username: string, password: string) {
    account.value = requireData(await client.POST('/api/admin/setup/register', { body: { username, password } }))
    setup.value = requireData(await client.GET('/api/admin/setup'))
    loaded.value = true
  }
  async function logout() {
    requireData(await client.POST('/api/admin/auth/logout'))
    account.value = null
  }
  return { account, setup, loaded, initialize, refresh, login, register, logout }
})
