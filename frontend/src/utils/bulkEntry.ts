import type { BulkEntryConfig, Field } from '../gen/sickrock_pb'
import { toSqlDateTime } from './dateFormatting'
import { applyViewToInsertFieldDefs, type InsertViewForFields } from './insertRowViewFields'

export type BulkEntryInputType = 'date' | 'number' | 'text'

/** The input used for a column on the bulk entry form (date-like columns take a date only). */
export function bulkEntryInputType(fieldType: string | undefined): BulkEntryInputType {
  const t = (fieldType ?? '').toLowerCase()
  if (t === 'datetime' || t === 'timestamp' || t === 'date') return 'date'
  if (t.includes('int') || t === 'float' || t === 'double' || t === 'decimal') return 'number'
  return 'text'
}

/** Converts a form value to what is stored: date inputs become SQL datetimes, blanks stay blank. */
export function bulkEntryStoredValue(field: Field | undefined, value: string): string {
  const trimmed = value.trim()
  if (!trimmed) return ''
  if (bulkEntryInputType(field?.type) === 'date') {
    return toSqlDateTime(trimmed) ?? ''
  }
  return trimmed
}

/**
 * Splits an entry into the columns that identify its row (the scanned code plus match columns; a blank
 * match value matches NULL) and the values used only when a new row is created.
 */
export function buildBulkEntryValues(
  config: Pick<BulkEntryConfig, 'scanColumn' | 'matchColumns' | 'fields'>,
  code: string,
  values: Record<string, string>,
  fieldDefs: Field[],
): { match: Record<string, string>; fields: Record<string, string> } {
  const field = (name: string) => fieldDefs.find(f => f.name === name)
  const match: Record<string, string> = { [config.scanColumn]: code.trim() }
  for (const column of config.matchColumns) {
    match[column] = bulkEntryStoredValue(field(column), values[column] ?? '')
  }
  const fields: Record<string, string> = {}
  for (const column of config.fields) {
    const stored = bulkEntryStoredValue(field(column), values[column] ?? '')
    if (stored) fields[column] = stored
  }
  return { match, fields }
}

/** The columns the bulk entry form needs, in config order: code, quantity, match columns, then fields. */
function bulkEntryColumns(config: Pick<BulkEntryConfig, 'scanColumn' | 'quantityColumn' | 'matchColumns' | 'fields'>): string[] {
  return [config.scanColumn, config.quantityColumn, ...config.matchColumns, ...config.fields].filter(Boolean)
}

/**
 * Orders the bulk entry form's columns like the insert form does for the selected view. Optional fields
 * the view hides are left out; the code, quantity and match columns are always shown, because entries
 * cannot be matched without them.
 */
export function orderBulkEntryColumns(
  config: Pick<BulkEntryConfig, 'scanColumn' | 'quantityColumn' | 'matchColumns' | 'fields'>,
  fieldDefs: Field[],
  views: InsertViewForFields[],
  selectedViewId: number | null,
): string[] {
  const columns = bulkEntryColumns(config)
  const required = new Set([config.scanColumn, config.quantityColumn, ...config.matchColumns])
  const defs = columns.map(name => {
    const def = fieldDefs.find(f => f.name === name)
    return { name, type: def?.type ?? '', required: required.has(name) }
  })
  const adjustedViews = views.map(view => ({
    ...view,
    columns: view.columns.map(col => (required.has(col.columnName) ? { ...col, isVisible: true } : col)),
  }))
  return applyViewToInsertFieldDefs(defs, adjustedViews, selectedViewId).map(def => def.name)
}
