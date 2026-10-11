/** Minimal FK shape for row-view related panel visibility. */
export type RowViewRelationFk = {
  tableTcName?: string
  tableName: string
  columnName: string
  referencedTableTcName?: string
  referencedTable: string
}

/** Table configuration name for the referencing (child) side of the relation. */
export function relationSourceTableTcName(fk: RowViewRelationFk): string {
  return (fk.tableTcName?.trim() || fk.tableName).trim()
}

/** Table configuration name for the referenced (parent) side of the relation. */
export function relationReferencedTableTcName(fk: RowViewRelationFk): string {
  return (fk.referencedTableTcName?.trim() || fk.referencedTable).trim()
}

/**
 * Show a related-rows panel only when viewing a row on the referenced (parent) table —
 * i.e. child rows that point at this row (e.g. opportunities on a customer row).
 * Hide when this row is on the source table and holds the FK (parent shown as a field link).
 */
export function shouldShowRelatedRelationPanel(currentTableTcName: string, fk: RowViewRelationFk): boolean {
  const current = currentTableTcName.trim()
  if (!current) return false
  return relationReferencedTableTcName(fk) === current
}

export function relatedRowsStorageKey(fk: RowViewRelationFk): string {
  return `${relationSourceTableTcName(fk)}.${fk.columnName}`
}
