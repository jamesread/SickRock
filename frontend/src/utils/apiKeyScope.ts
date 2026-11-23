// API key scopes: what a key may do, within its owner's own access. The server enforces them;
// these helpers build the editor's choices and the key list's summary.

export type ApiKeyResourceType = 'table' | 'dashboard' | 'workflow'

export const API_KEY_RESOURCE_TYPES: { type: ApiKeyResourceType; label: string; plural: string }[] = [
  { type: 'table', label: 'table', plural: 'Tables' },
  { type: 'dashboard', label: 'dashboard', plural: 'Dashboards' },
  { type: 'workflow', label: 'workflow', plural: 'Workflows' },
]

export type ApiKeyResource = { type: string; key: string }

export type ApiKeyResourceOption = ApiKeyResource & { label: string }

export type ApiKeyScope = {
  restrictPermissions: boolean
  permissions: string[]
  restrictResources: boolean
  resources: ApiKeyResource[]
}

export type CheckOption = { value: string; label: string }

export function emptyScope(): ApiKeyScope {
  return { restrictPermissions: false, permissions: [], restrictResources: false, resources: [] }
}

/** Copies a scope from the API (fields may be missing on older servers). */
export function scopeFromApi(scope?: Partial<ApiKeyScope> | null): ApiKeyScope {
  return {
    restrictPermissions: Boolean(scope?.restrictPermissions),
    permissions: [...(scope?.permissions ?? [])],
    restrictResources: Boolean(scope?.restrictResources),
    resources: (scope?.resources ?? []).map((r) => ({ type: r.type, key: r.key })),
  }
}

/** What each permission means, for the people choosing them. Unknown names show as-is. */
const PERMISSION_LABELS: Record<string, string> = {
  'app.read': 'View data (needed for almost every read)',
  'app.access': 'Change data (needed for almost every write)',
  'table.view': 'View rows in tables',
  'table.insert': 'Add rows to tables',
  'table.edit': 'Edit rows in tables',
  'table.delete': 'Delete rows from tables',
  'dashboard.view': 'View dashboards',
  'workflow.view': 'View workflows',
  'workflow.start': 'Use workflow actions',
  'exports.view': 'View read-only calendar exports',
  'exports.manage': 'Manage read-only calendar exports',
  'rollup.run': 'Run rollups (replaces the rows of their target table)',
  'audit.view': 'View audit logs',
  'system.settings': 'View and change system settings, tables and columns',
  'system.logs': 'View logs and run maintenance',
  'users.view': 'View users',
  'users.create': 'Create users',
  'users.delete': 'Delete users',
  'users.reset-password': "Reset other users' passwords",
  'usergroups.view': 'View user groups',
  'usergroups.manage': 'Manage user groups',
  'rbac.view': 'View roles and permissions',
  'rbac.manage': 'Manage roles and permissions',
}

/**
 * Permissions that make no sense on a key: keys cannot manage keys, claim device codes or
 * impersonate (the server refuses those calls from API keys).
 */
const NOT_FOR_KEYS = new Set(['apikeys.use', 'devicecode.claim', 'system.impersonate'])

/** The caller's own permissions, plus any already on the key (eg ones the caller has since lost). */
export function permissionOptions(available: string[], scope: ApiKeyScope): CheckOption[] {
  const names = new Set([...available.filter((p) => !NOT_FOR_KEYS.has(p)), ...scope.permissions])
  return [...names].sort().map((name) => ({
    value: name,
    label: PERMISSION_LABELS[name] ? `${PERMISSION_LABELS[name]} (${name})` : name,
  }))
}

/** A resource as a single CheckGroup value. */
export function resourceValue(r: ApiKeyResource): string {
  return `${r.type}:${r.key}`
}

/** The options for one resource type: those the caller can view, plus any already on the key. */
export function resourceOptions(type: string, available: ApiKeyResourceOption[], scope: ApiKeyScope): CheckOption[] {
  const out = new Map<string, string>()
  for (const r of available) {
    if (r.type === type) out.set(r.key, r.label || r.key)
  }
  for (const r of scope.resources) {
    if (r.type === type && !out.has(r.key)) out.set(r.key, r.key)
  }
  return [...out].map(([key, label]) => ({ value: resourceValue({ type, key }), label }))
}

/** The selected values of one resource type. */
export function selectedResources(type: string, scope: ApiKeyScope): string[] {
  return scope.resources.filter((r) => r.type === type).map(resourceValue)
}

/** Replaces the resources of one type with the selected CheckGroup values. */
export function withResources(scope: ApiKeyScope, type: string, values: string[]): ApiKeyScope {
  const prefix = `${type}:`
  const chosen = values
    .filter((v) => v.startsWith(prefix))
    .map((v) => ({ type, key: v.slice(prefix.length) }))
  return { ...scope, resources: [...scope.resources.filter((r) => r.type !== type), ...chosen] }
}

function plural(n: number, word: string): string {
  return `${n} ${word}${n === 1 ? '' : 's'}`
}

/** A short description of a key's access for the key list, eg "2 permissions · 1 table". */
export function scopeSummary(scope: ApiKeyScope, readOnly: boolean): string {
  const parts: string[] = [readOnly ? 'Read-only' : 'Read-write']
  if (scope.restrictPermissions) parts.push(plural(scope.permissions.length, 'permission'))
  if (scope.restrictResources) {
    const counts = API_KEY_RESOURCE_TYPES
      .map(({ type, label }) => [scope.resources.filter((r) => r.type === type).length, label] as const)
      .filter(([n]) => n > 0)
      .map(([n, label]) => plural(n, label))
    parts.push(counts.length ? counts.join(', ') : 'no tables, dashboards or workflows')
  }
  if (!scope.restrictPermissions && !scope.restrictResources) parts.push('all your access')
  return parts.join(' · ')
}

/** Problems that would make a scoped key useless, to show before saving. */
export function scopeWarnings(scope: ApiKeyScope): string[] {
  const warnings: string[] = []
  if (scope.restrictPermissions && !scope.permissions.includes('app.read')) {
    warnings.push('Without "View data (app.read)" the key cannot read anything.')
  }
  if (scope.restrictResources && scope.resources.length === 0) {
    warnings.push('No tables, dashboards or workflows are chosen, so the key cannot use any.')
  }
  return warnings
}

/** A datetime-local input value (local time) for a Unix timestamp; empty for none. */
export function unixToDateTimeLocal(unix: number): string {
  if (!unix) return ''
  const d = new Date(unix * 1000)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/** The Unix timestamp of a datetime-local input value; 0 for empty. */
export function dateTimeLocalToUnix(value: string): number {
  if (!value.trim()) return 0
  const ms = new Date(value).getTime()
  return Number.isNaN(ms) ? 0 : Math.floor(ms / 1000)
}
