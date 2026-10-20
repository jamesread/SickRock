// Mapping between SickRock server table views (table_views + table_view_columns +
// layout_json) and PicoCrank Table layout state, so views saved from PicoCrank's view
// menu are stored on the server instead of in browser localStorage.
import type { TableView } from '../composables/useTableViewManager'

/** Layout state that server view columns cannot hold; stored in table_views.layout_json. */
export type ViewLayout = {
  filters?: Record<string, unknown>
  groupBy?: string | null
  pageSize?: number
  columnPriorities?: Record<string, number | null>
}

/** The PicoCrank Table layout state (see picocrank getCurrentLayoutState). */
export type LayoutState = ViewLayout & {
  columnOrder?: string[]
  columnVisibility?: Record<string, boolean>
  sortBy?: string | null
  sortDir?: 'asc' | 'desc'
}

export type ViewColumnInput = {
  columnName: string
  isVisible: boolean
  columnOrder: number
  sortOrder: string
}

type ProtoTableViewLike = {
  id: number
  tableName: string
  viewName: string
  isDefault: boolean
  viewType: string
  layoutJson?: string
  columns: Array<{ columnName: string; isVisible: boolean; columnOrder: number; sortOrder: string }>
}

const PAGE_SIZES = [10, 25, 50, 100]

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

export function tableViewFromProto(view: ProtoTableViewLike): TableView {
  return {
    id: view.id,
    tableName: view.tableName,
    viewName: view.viewName,
    isDefault: view.isDefault,
    viewType: view.viewType || 'table',
    layoutJson: view.layoutJson ?? '',
    columns: view.columns.map(column => ({
      columnName: column.columnName,
      isVisible: column.isVisible,
      columnOrder: column.columnOrder,
      sortOrder: column.sortOrder,
    })),
  }
}

/** The view to show first: the user's own default, then the table default, then the first view. */
export function initialTableView<T extends Pick<TableView, 'id' | 'isDefault'>>(views: T[], userDefaultViewId: number): T | null {
  return views.find(view => userDefaultViewId > 0 && view.id === userDefaultViewId)
    ?? views.find(view => view.isDefault)
    ?? views[0]
    ?? null
}

/** Parses table_views.layout_json, ignoring anything invalid. */
export function parseViewLayout(json: string | undefined | null): ViewLayout {
  if (!json?.trim()) return {}
  let raw: unknown
  try {
    raw = JSON.parse(json)
  } catch {
    return {}
  }
  if (!isPlainObject(raw)) return {}

  const layout: ViewLayout = {}
  if (isPlainObject(raw.filters)) layout.filters = raw.filters
  if (typeof raw.groupBy === 'string' && raw.groupBy) layout.groupBy = raw.groupBy
  if (typeof raw.pageSize === 'number' && PAGE_SIZES.includes(raw.pageSize)) layout.pageSize = raw.pageSize
  if (isPlainObject(raw.columnPriorities)) {
    layout.columnPriorities = Object.fromEntries(
      Object.entries(raw.columnPriorities).filter(([, value]) => value === null || Number.isInteger(value)),
    ) as Record<string, number | null>
  }
  return layout
}

/** Builds PicoCrank layout state from a server view (only views store visible columns). */
export function presetStateFromView(view: TableView, fieldNames: string[]): LayoutState {
  const known = new Set(fieldNames)
  const configured = [...view.columns]
    .filter(column => known.has(column.columnName) && column.isVisible)
    .sort((left, right) => left.columnOrder - right.columnOrder)
  const visible = new Set(configured.map(column => column.columnName))
  const columnOrder = [
    ...configured.map(column => column.columnName),
    ...fieldNames.filter(name => !visible.has(name)),
  ]
  const sorted = configured.find(column => column.sortOrder === 'asc' || column.sortOrder === 'desc')

  return {
    columnOrder,
    columnVisibility: Object.fromEntries(fieldNames.map(name => [name, visible.has(name)])),
    sortBy: sorted?.columnName ?? null,
    sortDir: sorted?.sortOrder === 'desc' ? 'desc' : 'asc',
    filters: {},
    groupBy: null,
    columnPriorities: {},
    ...parseViewLayout(view.layoutJson),
  }
}

/**
 * Converts PicoCrank layout state into server view columns (for the real table fields only,
 * not the actions column or synthetic pull-through columns) plus layout_json for the rest.
 */
export function viewPayloadFromState(
  state: LayoutState,
  fieldNames: string[],
): { columns: ViewColumnInput[]; layoutJson: string } {
  const known = new Set(fieldNames)
  const fromState = (state.columnOrder ?? []).filter(name => known.has(name))
  const order = [...new Set([...fromState, ...fieldNames])]

  const columns = order.map((columnName, index) => ({
    columnName,
    isVisible: state.columnVisibility?.[columnName] !== false,
    columnOrder: index,
    sortOrder: state.sortBy === columnName ? (state.sortDir === 'desc' ? 'desc' : 'asc') : '',
  }))

  const layout: ViewLayout = {}
  if (state.filters && Object.keys(state.filters).length > 0) layout.filters = state.filters
  if (state.groupBy) layout.groupBy = state.groupBy
  if (state.pageSize) layout.pageSize = state.pageSize
  if (state.columnPriorities && Object.keys(state.columnPriorities).length > 0) {
    layout.columnPriorities = state.columnPriorities
  }

  return {
    columns,
    layoutJson: Object.keys(layout).length > 0 ? JSON.stringify(layout) : '',
  }
}
