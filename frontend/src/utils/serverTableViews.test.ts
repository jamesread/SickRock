import { describe, expect, it } from 'vitest'

import type { TableView } from '../composables/useTableViewManager'
import {
  initialTableView,
  parseViewLayout,
  presetStateFromView,
  viewPayloadFromState,
  type LayoutState,
} from './serverTableViews'

const fields = ['id', 'name', 'status', 'created']

function view(overrides: Partial<TableView> = {}): TableView {
  return {
    id: 7,
    tableName: 'customers',
    viewName: 'Mine',
    isDefault: false,
    viewType: 'table',
    layoutJson: '',
    columns: [],
    ...overrides,
  }
}

describe('server table views <-> PicoCrank layout state', () => {
  it('round-trips columns, sort and layout extras through a saved view', () => {
    const state: LayoutState = {
      columnOrder: ['status', 'name', '__actions', 'pt__customer__region', 'id', 'created'],
      columnVisibility: { status: true, name: true, id: false, created: false, __actions: true },
      sortBy: 'name',
      sortDir: 'desc',
      filters: { status: [{ operator: 'eq', value: 'open' }] },
      groupBy: 'status',
      pageSize: 25,
      columnPriorities: { name: 1 },
    }

    const payload = viewPayloadFromState(state, fields)
    expect(payload.columns.map(column => column.columnName)).toEqual(['status', 'name', 'id', 'created'])
    expect(payload.columns.filter(column => column.isVisible).map(column => column.columnName)).toEqual(['status', 'name'])
    expect(payload.columns.find(column => column.columnName === 'name')?.sortOrder).toBe('desc')

    // The server keeps only visible columns.
    const saved = view({
      columns: payload.columns.filter(column => column.isVisible),
      layoutJson: payload.layoutJson,
    })
    expect(presetStateFromView(saved, fields)).toEqual({
      columnOrder: ['status', 'name', 'id', 'created'],
      columnVisibility: { id: false, name: true, status: true, created: false },
      sortBy: 'name',
      sortDir: 'desc',
      filters: { status: [{ operator: 'eq', value: 'open' }] },
      groupBy: 'status',
      pageSize: 25,
      columnPriorities: { name: 1 },
    })
  })

  it('stores no layout_json when there is nothing beyond columns and sort', () => {
    expect(viewPayloadFromState({ columnOrder: fields, filters: {}, groupBy: null, columnPriorities: {} }, fields).layoutJson).toBe('')
  })

  it('appends fields missing from the state and treats them as visible', () => {
    const payload = viewPayloadFromState({ columnOrder: ['name'] }, fields)
    expect(payload.columns.map(column => column.columnName)).toEqual(['name', 'id', 'status', 'created'])
    expect(payload.columns.every(column => column.isVisible)).toBe(true)
  })

  it('resets filters, group-by and priorities for views without a layout', () => {
    const state = presetStateFromView(view({ columns: [{ columnName: 'name', isVisible: true, columnOrder: 0, sortOrder: '' }] }), fields)
    expect(state.filters).toEqual({})
    expect(state.groupBy).toBeNull()
    expect(state.columnPriorities).toEqual({})
    expect(state.sortBy).toBeNull()
    expect(state.pageSize).toBeUndefined()
  })
})

describe('parseViewLayout', () => {
  it('ignores invalid JSON and non-objects', () => {
    expect(parseViewLayout('{not json')).toEqual({})
    expect(parseViewLayout('[1,2]')).toEqual({})
    expect(parseViewLayout('')).toEqual({})
    expect(parseViewLayout(undefined)).toEqual({})
  })

  it('keeps only valid values', () => {
    expect(parseViewLayout(JSON.stringify({
      filters: 'nope',
      groupBy: 42,
      pageSize: 33,
      columnPriorities: { name: 2, status: 'high', created: null },
      unknown: true,
    }))).toEqual({ columnPriorities: { name: 2, created: null } })
  })
})

describe('initialTableView', () => {
  const views = [view({ id: 1 }), view({ id: 2, isDefault: true }), view({ id: 3 })]

  it("prefers the user's own default, then the table default, then the first view", () => {
    expect(initialTableView(views, 3)?.id).toBe(3)
    expect(initialTableView(views, 0)?.id).toBe(2)
    expect(initialTableView(views, 99)?.id).toBe(2)
    expect(initialTableView([view({ id: 5 })], 0)?.id).toBe(5)
    expect(initialTableView([], 0)).toBeNull()
  })
})
