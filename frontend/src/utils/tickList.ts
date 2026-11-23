/**
 * The Tick List view type: rows as tiles that are ticked off (shared tick state on the server).
 * Settings live in the view's layout_json under "tickList": the column each tile is labelled with,
 * detail columns shown under it, sort columns and an optional group-by column.
 *
 * Every column setting is a column path: a column of the table ("quantity"), or for a relation
 * column, a column of the row it references ("ingredient.aisle"). A relation column on its own
 * shows the referenced row's name.
 */
import type { Item } from '../gen/sickrock_pb'
import { cardDetailValue } from './cardsView'
import { getItemNameFieldValue } from './itemViewSectionTitle'

export const TICK_LIST_VIEW_TYPE = 'ticklist'

export type TickListConfig = {
  /** Path the tile is labelled with; empty for the row's name or title. */
  labelColumn: string
  /** Paths shown under the label, joined with spaces (e.g. quantity and unit: "550 g"). */
  detailColumns: string[]
  /** Paths tiles are sorted by, in order. */
  sortColumns: string[]
  /** Path tiles are grouped under headings by; empty for no groups. */
  groupColumn: string
}

export const MAX_TICK_LIST_DETAILS = 4
export const MAX_TICK_LIST_SORTS = 3

/** Rows of referenced tables by relation column, then by row id. */
export type ReferencedRows = Map<string, Map<string, Item>>

export function defaultTickListConfig(): TickListConfig {
  return { labelColumn: '', detailColumns: [], sortColumns: [], groupColumn: '' }
}

function parseLayout(layoutJson: string | undefined): Record<string, unknown> {
  if (!layoutJson?.trim()) return {}
  try {
    const parsed = JSON.parse(layoutJson)
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : {}
  } catch {
    return {}
  }
}

const text = (value: unknown) => (typeof value === 'string' ? value.trim() : '')

function pathList(value: unknown, max: number): string[] {
  return Array.isArray(value) ? [...new Set(value.map(text).filter(Boolean))].slice(0, max) : []
}

/** Reads the tick list settings from a view's layout_json, filling in defaults for anything missing. */
export function parseTickListConfig(layoutJson: string | undefined): TickListConfig {
  const raw = parseLayout(layoutJson).tickList
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) return defaultTickListConfig()
  const stored = raw as Record<string, unknown>
  return {
    labelColumn: text(stored.labelColumn),
    detailColumns: pathList(stored.detailColumns, MAX_TICK_LIST_DETAILS),
    sortColumns: pathList(stored.sortColumns, MAX_TICK_LIST_SORTS),
    groupColumn: text(stored.groupColumn),
  }
}

/** Returns layoutJson with its tick list settings replaced, keeping every other key. */
export function withTickListConfig(layoutJson: string | undefined, config: TickListConfig): string {
  const layout = parseLayout(layoutJson)
  layout.tickList = {
    labelColumn: config.labelColumn.trim(),
    detailColumns: pathList(config.detailColumns, MAX_TICK_LIST_DETAILS),
    sortColumns: pathList(config.sortColumns, MAX_TICK_LIST_SORTS),
    groupColumn: config.groupColumn.trim(),
  }
  return JSON.stringify(layout)
}

/** The relation columns a config's paths need the referenced rows of. */
export function relationColumnsUsed(config: TickListConfig, relationColumns: string[]): string[] {
  const paths = [config.labelColumn, ...config.detailColumns, ...config.sortColumns, config.groupColumn]
  const used = new Set(paths.filter(Boolean).map(path => path.split('.')[0]))
  return relationColumns.filter(column => used.has(column))
}

/** A row's own name: its name or title column, else "#id". */
export function rowName(row: Item): string {
  return getItemNameFieldValue(row as unknown as Record<string, unknown>)
    || row.additionalFields?.title?.trim()
    || `#${row.id}`
}

/**
 * A row's value for a path, as text: a column's value, a referenced row's column, or (for a
 * relation column alone) the referenced row's name. Empty when there is none.
 */
export function pathValue(row: Item, path: string, refs: ReferencedRows): string {
  const [column, field] = splitPath(path)
  const ownValue = cardDetailValue(row, column)
  const referenced = refs.get(column)
  if (!referenced) return field ? '' : ownValue
  const target = ownValue ? referenced.get(ownValue) : undefined
  if (!target) return field ? '' : ownValue
  return field ? cardDetailValue(target, field) : rowName(target)
}

function splitPath(path: string): [string, string] {
  const dot = path.indexOf('.')
  return dot < 0 ? [path, ''] : [path.slice(0, dot), path.slice(dot + 1)]
}

/** A tile's label: the label column's value, else the row's name. */
export function tileLabel(row: Item, config: TickListConfig, refs: ReferencedRows): string {
  return (config.labelColumn && pathValue(row, config.labelColumn, refs)) || rowName(row)
}

/** A tile's detail line, e.g. "550 g". */
export function tileDetails(row: Item, config: TickListConfig, refs: ReferencedRows): string {
  return config.detailColumns.map(path => pathValue(row, path, refs)).filter(Boolean).join(' ')
}

const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: 'base' })

/** Compares values: empty last, numbers by value, text naturally ("Aisle 2" before "Aisle 10"). */
export function compareValues(a: string, b: string): number {
  if (a === '' || b === '') return Number(a === '') - Number(b === '')
  const x = Number(a)
  const y = Number(b)
  if (Number.isFinite(x) && Number.isFinite(y)) return x - y
  return collator.compare(a, b)
}

/** Rows sorted by the config's group column, then its sort columns (stable). */
export function sortTickListRows(rows: Item[], config: TickListConfig, refs: ReferencedRows): Item[] {
  const paths = [config.groupColumn, ...config.sortColumns].filter(Boolean)
  if (paths.length === 0) return rows
  const keyed = rows.map(row => ({ row, keys: paths.map(path => pathValue(row, path, refs)) }))
  keyed.sort((a, b) => {
    for (let i = 0; i < paths.length; i++) {
      const order = compareValues(a.keys[i], b.keys[i])
      if (order !== 0) return order
    }
    return 0
  })
  return keyed.map(entry => entry.row)
}

export type TickListGroup = { label: string; rows: Item[] }

/** Sorted rows under headings by the group column; one unlabelled group when there is none. */
export function groupTickListRows(rows: Item[], config: TickListConfig, refs: ReferencedRows): TickListGroup[] {
  const sorted = sortTickListRows(rows, config, refs)
  if (!config.groupColumn) return [{ label: '', rows: sorted }]
  const groups: TickListGroup[] = []
  for (const row of sorted) {
    const label = pathValue(row, config.groupColumn, refs) || 'Other'
    const last = groups[groups.length - 1]
    if (last?.label === label) last.rows.push(row)
    else groups.push({ label, rows: [row] })
  }
  return groups
}
