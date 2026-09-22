import type { InitResponse } from '../gen/sickrock_pb'

const CONTROL_PANEL_PERMISSIONS = [
  'users.view',
  'rbac.view',
  'usergroups.view',
  'users.reset-password',
  'system.settings',
]

export const IAM_PERMISSIONS = [
  'users.view',
  'usergroups.view',
  'rbac.view',
]

export function permissionsFromStatus(st: InitResponse | null | undefined): string[] {
  return st?.rbacPermissions ?? []
}

export function isSuperuserFromStatus(st: InitResponse | null | undefined): boolean {
  return Boolean(st?.rbacIsSuperuser)
}

export function hasPermission(
  permission: string,
  perms: string[],
  superuser: boolean,
): boolean {
  if (superuser) return true
  return perms.includes(permission)
}

export function canAccessControlPanel(perms: string[], superuser: boolean): boolean {
  if (superuser) return true
  return CONTROL_PANEL_PERMISSIONS.some((p) => perms.includes(p))
}

export function canAccessIam(perms: string[], superuser: boolean): boolean {
  if (superuser) return true
  return IAM_PERMISSIONS.some((p) => perms.includes(p))
}

export const SYSTEM_ROLES = new Set(['superuser', 'member'])
export const SYSTEM_GROUPS = new Set(['Everyone', 'Administrators'])

export function isSystemRole(name: string): boolean {
  return SYSTEM_ROLES.has(name)
}

export function isSystemGroup(name: string): boolean {
  return SYSTEM_GROUPS.has(name)
}
