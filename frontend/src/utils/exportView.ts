/**
 * Which view a table's Export page starts on. table_configurations.default_export_view_id holds a
 * view id, ALL_COLUMNS_VIEW_ID, or nothing (0 / unset) for the table's default view.
 */

/** The built-in "All Columns" view, which has no table_views row. */
export const ALL_COLUMNS_VIEW_ID = -1

type ExportViewChoice = { id: number; viewName: string; isDefault: boolean }

/**
 * The view to start on: the chosen default export view when it is All Columns or still one of the
 * table's views, else the table's default view, else its first view, else All Columns.
 */
export function initialExportViewId(views: ExportViewChoice[], defaultExportViewId: number | undefined): number {
  if (defaultExportViewId === ALL_COLUMNS_VIEW_ID) return ALL_COLUMNS_VIEW_ID
  if (defaultExportViewId && views.some(view => view.id === defaultExportViewId)) return defaultExportViewId
  return views.find(view => view.isDefault)?.id ?? views[0]?.id ?? ALL_COLUMNS_VIEW_ID
}

/**
 * The choices for a table's default export view: automatic (0, naming the view it starts on), All
 * Columns, then each view.
 */
export function defaultExportViewOptions(views: ExportViewChoice[]): Array<{ value: number; label: string }> {
  const automatic = views.find(view => view.id === initialExportViewId(views, 0))
  return [
    { value: 0, label: `Table's default view (${automatic?.viewName ?? 'All Columns'})` },
    { value: ALL_COLUMNS_VIEW_ID, label: 'All Columns' },
    ...views.map(view => ({ value: view.id, label: view.viewName })),
  ]
}
