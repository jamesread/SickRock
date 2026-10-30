import { describe, expect, it } from 'vitest'
import { resolveRowActionRoute } from './rowActions'

describe('resolveRowActionRoute', () => {
  it('fills {id} and column placeholders and adds the return context', () => {
    const route = resolveRowActionRoute(
      { path: '/admin/table/create', query: { navigationAppId: '{id}', table: '{title}_items' } },
      { title: 'crm' },
      'table_apps',
      '7',
    )
    expect(route).toEqual({
      path: '/admin/table/create',
      query: { navigationAppId: '7', table: 'crm_items', fromTable: 'table_apps', fromRowId: '7' },
    })
  })

  it('leaves unknown placeholders empty and fills the path too', () => {
    const route = resolveRowActionRoute({ path: '/table/{missing}x', query: {} }, {}, 't', '1')
    expect(route.path).toBe('/table/x')
  })
})
