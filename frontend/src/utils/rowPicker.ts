/**
 * Choosing a row of another table (a relation's target) in RowPickerDialog: the options it shows,
 * whether as image tiles or a list, and the rows picked recently.
 *
 * A table with a Cards view is shown as tiles with that view's image and detail columns; one with an
 * image column as tiles without details; any other as a list showing a few columns of its default
 * table view (or its first plain columns).
 */
import type { Item } from '../gen/sickrock_pb'
import { cardDetails, chooseCardsView, filterByText, parseCardsConfig } from './cardsView'
import { getItemNameFieldValue } from './itemViewSectionTitle'
import { hasImageValue, isImageFieldType } from './fieldTypes'

export type RowPickerLayout = 'tiles' | 'list'

export type RowPickerOption = {
  value: string
  label: string
  imageId: string
  /** Extra fields shown with the option, e.g. the detail columns of the table's Cards view. */
  details?: Array<{ label: string; value: string }>
}

export type RowPickerColumns = {
  layout: RowPickerLayout
  imageColumn: string
  detailColumns: string[]
}

/** How many columns a list shows besides the name and id. */
export const MAX_LIST_DETAIL_COLUMNS = 3

/** How many recently picked rows are remembered per table. */
export const MAX_RECENT_PICKS = 8

type PickerField = { name: string; type: string }
type PickerView = {
  id: number
  viewType: string
  isDefault: boolean
  layoutJson?: string
  columns?: Array<{ columnName: string; isVisible: boolean; columnOrder: number }>
}

// Columns that already make up an option's label or id, or are bookkeeping.
const LABEL_COLUMNS = new Set(['id', 'name', 'title', 'sr_created', 'sr_updated'])

/**
 * Options whose label and details together contain every word of query, ignoring case and accents.
 */
export function filterPickerOptions<T extends { label: string; details?: Array<{ value: string }> }>(
  options: T[],
  query: string,
): T[] {
  return filterByText(options, query, option => [option.label, ...(option.details ?? []).map(detail => detail.value)])
}

/** What a row is called in a picker: its name, else its title, else "#id". */
export function pickerLabel(row: Item): string {
  return getItemNameFieldValue(row as unknown as Record<string, unknown>)
    || row.additionalFields?.title?.trim()
    || `#${row.id}`
}

/** The image a row shows in a picker: its value in imageColumn, or "" for none. */
export function pickerImageId(row: Item | undefined, imageColumn: string): string {
  const image = row && imageColumn ? row.additionalFields?.[imageColumn] : ''
  return hasImageValue(image) ? String(image) : ''
}

/** The table view to take list columns from: the caller's default table view, the table's, or none. */
function defaultTableView<T extends PickerView>(views: T[], userDefaultViewId?: number): T | undefined {
  const tables = views.filter(view => !view.viewType || view.viewType === 'table')
  return tables.find(view => view.id === userDefaultViewId) ?? tables.find(view => view.isDefault)
}

/** Whether the picker shows tiles or a list for a table, and which columns it shows. */
export function pickerColumns(
  fields: PickerField[],
  views: PickerView[],
  userDefaultViewId?: number,
): RowPickerColumns {
  const known = new Set(fields.map(field => field.name))
  const imageField = fields.find(field => isImageFieldType(field.type))
  const cardsView = chooseCardsView(views, userDefaultViewId)
  if (cardsView) {
    const cards = parseCardsConfig(cardsView.layoutJson)
    return {
      layout: 'tiles',
      // As the weekly plan always has: a Cards view without an image column falls back to the table's.
      imageColumn: cards.imageColumn || imageField?.name || '',
      detailColumns: cards.detailColumns.filter(column => known.has(column) || LABEL_COLUMNS.has(column)),
    }
  }
  if (imageField) {
    return { layout: 'tiles', imageColumn: imageField.name, detailColumns: [] }
  }
  const usable = (column: string) => known.has(column) && !LABEL_COLUMNS.has(column)
  const view = defaultTableView(views, userDefaultViewId)
  const fromView = [...(view?.columns ?? [])]
    .filter(column => column.isVisible && usable(column.columnName))
    .sort((a, b) => a.columnOrder - b.columnOrder)
    .map(column => column.columnName)
  const detailColumns = (fromView.length ? fromView : fields.map(field => field.name).filter(usable))
    .slice(0, MAX_LIST_DETAIL_COLUMNS)
  return { layout: 'list', imageColumn: '', detailColumns }
}

/** One option per row, sorted by label. */
export function pickerOptions(
  rows: Item[],
  columns: RowPickerColumns,
  fieldTypes: Record<string, string> = {},
  labels: Record<string, string> = {},
): RowPickerOption[] {
  return rows
    .map(row => ({
      value: String(row.id),
      label: pickerLabel(row),
      imageId: pickerImageId(row, columns.imageColumn),
      details: cardDetails(row, columns.detailColumns, fieldTypes, labels)
        .map(detail => ({ label: detail.label, value: detail.value })),
    }))
    .sort((a, b) => a.label.localeCompare(b.label, undefined, { numeric: true }))
}

/** The options for recentValues, in that order (values with no option are left out). */
export function recentOptions<T extends { value: string }>(options: T[], recentValues: string[]): T[] {
  const byValue = new Map(options.map(option => [option.value, option]))
  return recentValues.flatMap(value => {
    const option = byValue.get(value)
    return option ? [option] : []
  })
}

/** The localStorage key for a user's recent picks from one table (or "users" for user fields). */
export function recentPicksKey(username: string, source: string): string {
  return `sickrock.recentPicks.${username || 'anonymous'}.${source}`
}

/** The values picked recently, newest first; empty when nothing is stored or storage is unavailable. */
export function recentPicks(key: string, storage: Pick<Storage, 'getItem'> | undefined = globalThis.localStorage): string[] {
  try {
    const parsed: unknown = JSON.parse(storage?.getItem(key) ?? '[]')
    return Array.isArray(parsed) ? parsed.filter((value): value is string => typeof value === 'string').slice(0, MAX_RECENT_PICKS) : []
  } catch {
    return []
  }
}

/** Puts value first in the recent picks (once), keeping at most MAX_RECENT_PICKS; returns the new list. */
export function rememberPick(
  key: string,
  value: string,
  storage: Pick<Storage, 'getItem' | 'setItem'> | undefined = globalThis.localStorage,
): string[] {
  if (!value) return recentPicks(key, storage)
  const picks = [value, ...recentPicks(key, storage).filter(existing => existing !== value)].slice(0, MAX_RECENT_PICKS)
  try {
    storage?.setItem(key, JSON.stringify(picks))
  } catch {
    // Private browsing or full storage: recents are a convenience, so carry on without them.
  }
  return picks
}
