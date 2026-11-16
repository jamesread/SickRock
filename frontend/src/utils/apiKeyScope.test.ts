import { describe, expect, it } from 'vitest'

import {
  dateTimeLocalToUnix,
  emptyScope,
  permissionOptions,
  resourceOptions,
  scopeFromApi,
  scopeSummary,
  scopeWarnings,
  selectedResources,
  unixToDateTimeLocal,
  withResources,
} from './apiKeyScope'

describe('scopeFromApi', () => {
  it('fills in missing fields', () => {
    expect(scopeFromApi(undefined)).toEqual(emptyScope())
    expect(scopeFromApi({ restrictPermissions: true, permissions: ['app.read'] })).toEqual({
      ...emptyScope(), restrictPermissions: true, permissions: ['app.read'],
    })
  })
})

describe('permissionOptions', () => {
  it('offers the caller\'s permissions and any already on the key, without key-only ones', () => {
    const scope = { ...emptyScope(), permissions: ['users.view'] }
    const values = permissionOptions(['table.view', 'apikeys.use', 'app.read'], scope).map((o) => o.value)
    expect(values).toEqual(['app.read', 'table.view', 'users.view'])
  })

  it('labels known permissions and shows unknown ones as-is', () => {
    const [known, unknown] = permissionOptions(['app.read', 'zz.custom'], emptyScope())
    expect(known.label).toContain('(app.read)')
    expect(unknown.label).toBe('zz.custom')
  })
})

describe('resources', () => {
  const available = [
    { type: 'table', key: 'recipes', label: 'Recipes' },
    { type: 'dashboard', key: 'home', label: 'home' },
  ]

  it('builds options per type, keeping resources already on the key', () => {
    const scope = { ...emptyScope(), resources: [{ type: 'table', key: 'gone' }] }
    expect(resourceOptions('table', available, scope)).toEqual([
      { value: 'table:recipes', label: 'Recipes' },
      { value: 'table:gone', label: 'gone' },
    ])
    expect(resourceOptions('workflow', available, scope)).toEqual([])
  })

  it('replaces only the chosen type', () => {
    let scope = { ...emptyScope(), resources: [{ type: 'dashboard', key: 'home' }] }
    scope = withResources(scope, 'table', ['table:recipes', 'table:a:b'])
    expect(selectedResources('table', scope)).toEqual(['table:recipes', 'table:a:b'])
    expect(scope.resources).toContainEqual({ type: 'table', key: 'a:b' })
    expect(selectedResources('dashboard', scope)).toEqual(['dashboard:home'])
    expect(withResources(scope, 'table', []).resources).toEqual([{ type: 'dashboard', key: 'home' }])
  })
})

describe('scopeSummary', () => {
  it('describes the access', () => {
    expect(scopeSummary(emptyScope(), false)).toBe('Read-write · all your access')
    expect(scopeSummary({
      restrictPermissions: true, permissions: ['app.read'],
      restrictResources: true, resources: [{ type: 'table', key: 'a' }, { type: 'table', key: 'b' }, { type: 'workflow', key: '1' }],
    }, true)).toBe('Read-only · 1 permission · 2 tables, 1 workflow')
    expect(scopeSummary({ ...emptyScope(), restrictResources: true }, false)).toBe('Read-write · no tables, dashboards or workflows')
  })
})

describe('scopeWarnings', () => {
  it('warns about keys that cannot do anything', () => {
    expect(scopeWarnings(emptyScope())).toEqual([])
    expect(scopeWarnings({ ...emptyScope(), restrictPermissions: true, permissions: ['table.view'] })).toHaveLength(1)
    expect(scopeWarnings({ ...emptyScope(), restrictResources: true })).toHaveLength(1)
  })
})

describe('datetime-local conversion', () => {
  it('round-trips to the minute', () => {
    const unix = Math.floor(new Date(2030, 4, 6, 7, 8).getTime() / 1000)
    expect(unixToDateTimeLocal(unix)).toBe('2030-05-06T07:08')
    expect(dateTimeLocalToUnix('2030-05-06T07:08')).toBe(unix)
    expect(unixToDateTimeLocal(0)).toBe('')
    expect(dateTimeLocalToUnix('')).toBe(0)
  })
})
