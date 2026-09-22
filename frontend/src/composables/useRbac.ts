import { computed } from 'vue'
import { useAuthStore } from '../stores/auth'
import { hasPermission as checkPerm } from '../utils/rbacAccess'

export function useRbac() {
  const authStore = useAuthStore()

  const permissions = computed(() => {
    return authStore.user?.rbacPermissions
      ?? authStore.initResponse?.rbacPermissions
      ?? []
  })

  const isSuperuser = computed(() => {
    return authStore.user?.rbacIsSuperuser
      ?? authStore.initResponse?.rbacIsSuperuser
      ?? false
  })

  function hasPermission(permission: string): boolean {
    return checkPerm(permission, permissions.value, isSuperuser.value)
  }

  return { permissions, isSuperuser, hasPermission }
}
