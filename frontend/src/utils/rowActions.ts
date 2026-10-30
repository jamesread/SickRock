import type { RowAction } from '../gen/sickrock_pb'

export type RowActionRoute = { path: string; query: Record<string, string> }

/**
 * Fills a table configuration's row action in from a row: `{id}` is the row's primary key and
 * `{column}` any of its fields (unknown placeholders become empty). `fromTable`/`fromRowId` are
 * always added so the target page can return to the row.
 */
export function resolveRowActionRoute(
  action: Pick<RowAction, 'path' | 'query'>,
  rowFields: Record<string, unknown>,
  tableName: string,
  rowId: string,
): RowActionRoute {
  const fill = (template: string) =>
    template.replace(/\{([^{}]+)\}/g, (_, key: string) => {
      if (key === 'id') return rowId
      const value = rowFields[key]
      return value == null ? '' : String(value)
    })

  const query: Record<string, string> = {}
  for (const [key, template] of Object.entries(action.query ?? {})) {
    query[key] = fill(template)
  }
  query.fromTable = tableName
  query.fromRowId = rowId
  return { path: fill(action.path), query }
}
