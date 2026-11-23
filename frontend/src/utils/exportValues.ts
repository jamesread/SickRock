/**
 * The value an export (ExportView) writes for a row's column: the id, the created time as ISO 8601,
 * or the column's value. A relation column is written as the referenced row's name, as the table
 * view shows it, falling back to the stored id when the row has no name or is not found.
 */
import type { Item } from '../gen/sickrock_pb'
import { foreignKeyReferencedDisplayLabel } from './fieldColumnDisplay'

/** Rows of the tables relation columns refer to, by relation column, then by row id. */
export type ReferencedRowsByColumn = Map<string, Map<string, Item>>

function createdIso(srCreated: unknown): string {
  const seconds = Number(srCreated)
  return Number.isFinite(seconds) && seconds > 0 ? new Date(seconds * 1000).toISOString() : ''
}

export function exportCellValue(item: Item, column: string, referenced: ReferencedRowsByColumn = new Map()): string {
  if (column === 'id') return String(item.id ?? '')
  if (column === 'sr_created') return createdIso(item.srCreated)
  const fields = item.additionalFields ?? {}
  const raw = column in fields ? fields[column] : (item as unknown as Record<string, unknown>)[column]
  if (raw === null || raw === undefined) return ''
  const rows = referenced.get(column)
  if (rows && String(raw) !== '') {
    return foreignKeyReferencedDisplayLabel(rows.get(String(raw)) as unknown as Record<string, unknown> | undefined, raw)
  }
  return String(raw)
}
