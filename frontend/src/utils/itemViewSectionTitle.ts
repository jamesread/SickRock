import { ucFirst } from './rowInsertLabel'

/** Display value of the row's `name` column (top-level or additionalFields). */
export function getItemNameFieldValue(item: Record<string, unknown> | null | undefined): string {
  if (!item) return ''
  const top = item.name
  if (top != null && String(top).trim() !== '') {
    return String(top).trim()
  }
  const af = item.additionalFields
  if (af && typeof af === 'object' && 'name' in af) {
    const nested = (af as Record<string, unknown>).name
    if (nested != null && String(nested).trim() !== '') {
      return String(nested).trim()
    }
  }
  return ''
}

/** Section heading for the item row view page. */
export function itemViewSectionTitle(opts: {
  rowName?: string | null
  tableTitle?: string | null
  tableName: string
  item: Record<string, unknown> | null | undefined
}): string {
  const rawLabel = opts.rowName?.trim() || opts.tableTitle?.trim() || 'Row'
  const rowLabel = ucFirst(rawLabel)
  const nameValue = getItemNameFieldValue(opts.item)
  if (nameValue) {
    return `${rowLabel}: ${nameValue}`
  }
  return `Table: ${opts.tableName}`
}
