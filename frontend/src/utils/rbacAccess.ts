import type { InitResponse } from '../gen/sickrock_pb'

const CONTROL_PANEL_PERMISSIONS = [
  'users.view',
  'rbac.view',
  'usergroups.view',
  'users.reset-password',
  'system.settings',
  'audit.view',
  'exports.manage',
]

export const IAM_PERMISSIONS = [
  'users.view',
  'usergroups.view',
  'rbac.view',
]

/** System table_configuration names that require system.settings (not table share grants). */
export const SYSTEM_SETTINGS_TABLE_CONFIGURATIONS = new Set([
  'table_settings',
  'table_navigation',
  'table_configurations',
  'table_workflows',
  'table_dashboards',
  'table_dashboard_components',
  'device_codes',
])

export const READ_ONLY_EXPORTS_TABLE_CONFIGURATION = 'table_read_only_exports'
export const RSS_CALENDAR_FEEDS_TABLE_CONFIGURATION = 'table_rss_calendar_feeds'
export const AUDIT_LOGS_TABLE_CONFIGURATION = 'table_logs'

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

export function canManageSystemSettings(perms: string[], superuser: boolean): boolean {
  return hasPermission('system.settings', perms, superuser)
}

export function canManageExports(perms: string[], superuser: boolean): boolean {
  return hasPermission('exports.manage', perms, superuser)
}

export function canAccessControlPanel(perms: string[], superuser: boolean): boolean {
  if (superuser) return true
  return CONTROL_PANEL_PERMISSIONS.some((p) => perms.includes(p))
}

export function canAccessIam(perms: string[], superuser: boolean): boolean {
  if (superuser) return true
  return IAM_PERMISSIONS.some((p) => perms.includes(p))
}

export function canViewAuditLogs(perms: string[], superuser: boolean): boolean {
  if (superuser) return true
  return perms.includes('audit.view')
}

/** Route guard for /table/:tableName admin system tables. */
export function canAccessAdminTableConfiguration(
  tableName: string,
  perms: string[],
  superuser: boolean,
): boolean {
  if (tableName === AUDIT_LOGS_TABLE_CONFIGURATION) {
    return canViewAuditLogs(perms, superuser)
  }
  if (tableName === READ_ONLY_EXPORTS_TABLE_CONFIGURATION
    || tableName === RSS_CALENDAR_FEEDS_TABLE_CONFIGURATION) {
    return canManageExports(perms, superuser)
  }
  if (SYSTEM_SETTINGS_TABLE_CONFIGURATIONS.has(tableName)) {
    return canManageSystemSettings(perms, superuser)
  }
  return true
}

export const SYSTEM_ROLES = new Set(['superuser', 'member', 'viewer', 'auditor'])

export const SYSTEM_GROUPS = new Set(['Everyone', 'Administrators'])

export function isSystemRole(name: string): boolean {
  return SYSTEM_ROLES.has(name)
}

export function isSystemGroup(name: string): boolean {
  return SYSTEM_GROUPS.has(name)
}
