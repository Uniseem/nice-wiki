import { useAuthStore } from '../stores/auth'
import type { components } from '../../shared/api/schema.gen'

type Permission = components['schemas']['Permission']
export function usePermission() {
  const auth = useAuthStore()
  function has(permission: Permission): boolean {
    const account = auth.account
    return !!account && !account.disabled && !account.mustChangePassword && (account.superadmin || account.permissions.includes(permission))
  }
  return { has }
}
