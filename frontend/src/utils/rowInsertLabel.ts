/** Capitalize the first character for labels at the start of a sentence or phrase. */
export function ucFirst(label: string): string {
  if (!label) return label
  return label.charAt(0).toUpperCase() + label.slice(1)
}

/** Display form of table_configurations.row_name. */
export function rowNameLabel(rowName?: string | null): string {
  const name = rowName?.trim()
  return name ? ucFirst(name) : ''
}

/** Toolbar / empty-state label for adding a row (uses table_configurations.row_name). */
export function rowInsertActionLabel(rowName?: string | null): string {
  const name = rowNameLabel(rowName)
  return name ? `Create ${name}` : 'Insert Row'
}

/** Toolbar label for full-page edit (uses table_configurations.row_name). */
export function rowEditActionLabel(rowName?: string | null): string {
  const name = rowNameLabel(rowName)
  return name ? `Edit ${name}` : 'Edit Row'
}

/** Plural row label (table_configurations.row_name_plural, else simple fallback from row_name). */
export function rowNamePluralLabel(
  rowNamePlural?: string | null,
  rowName?: string | null,
): string {
  const plural = rowNamePlural?.trim()
  if (plural) return ucFirst(plural)
  const singular = rowName?.trim()
  if (singular) return ucFirst(`${singular}s`)
  return 'Items'
}

/** Main section heading on the table list view (prefers row_name_plural / row_name). */
export function tableViewSectionTitle(
  rowNamePlural?: string | null,
  rowName?: string | null,
  tableTitle?: string | null,
  fallbackId?: string | null,
): string {
  if (rowNamePlural?.trim() || rowName?.trim()) {
    return rowNamePluralLabel(rowNamePlural, rowName)
  }
  const title = tableTitle?.trim()
  if (title) return ucFirst(title)
  const id = fallbackId?.trim()
  return id ? ucFirst(id) : 'Table'
}

/** Toolbar link back to the table list (e.g. "Opportunities table"). */
export function tableListLinkLabel(
  rowNamePlural?: string | null,
  rowName?: string | null,
): string {
  return `${rowNamePluralLabel(rowNamePlural, rowName)} table`
}

/** Empty-state heading for the main table view. */
export function tableEmptyHeading(
  rowNamePlural: string | null | undefined,
  rowName: string | null | undefined,
): string {
  const plural = rowNamePluralLabel(rowNamePlural, rowName)
  return `No ${plural} in this table`
}

/** Empty-state heading for a related table on an item view. */
export function relatedTableEmptyHeading(
  rowNamePlural: string | null | undefined,
  rowName: string | null | undefined,
  parentRowLabel: string,
): string {
  const plural = rowNamePluralLabel(rowNamePlural, rowName)
  const parent = parentRowLabel.trim() || 'this record'
  return `No ${plural} for ${parent}`
}
