import { computed, ref } from 'vue'
import { createApiClient } from '../stores/api'
import { initialTableView } from '../utils/serverTableViews'
import type { InsertViewColumn } from '../utils/insertRowViewFields'

export type InsertFormView = {
  id: number
  tableName: string
  viewName: string
  isDefault: boolean
  columns: Array<InsertViewColumn & { sortOrder: string }>
}

/**
 * Table views for ordering an insert form's fields, with the selection starting on the caller's
 * default view (else the table's). When the table has no default view, "All Columns" (id -1) is offered.
 */
export function useInsertFormViews(tableName: string) {
  const client = createApiClient()
  const tableViews = ref<InsertFormView[]>([])
  const selectedViewId = ref<number | null>(null)

  const viewOptions = computed<InsertFormView[]>(() => {
    const options = [...tableViews.value]
    if (options.length === 0 || !options.some(v => v.isDefault)) {
      options.unshift({ id: -1, tableName, viewName: 'All Columns', isDefault: true, columns: [] })
    }
    return options
  })

  async function loadViews() {
    try {
      const res = await client.getTableViews({ tableName })
      tableViews.value = (res.views || []).map(view => ({
        id: view.id,
        tableName: view.tableName,
        viewName: view.viewName,
        isDefault: view.isDefault,
        columns: view.columns.map(col => ({
          columnName: col.columnName,
          isVisible: col.isVisible,
          columnOrder: col.columnOrder,
          sortOrder: col.sortOrder,
        })),
      }))
      selectedViewId.value = initialTableView(tableViews.value, res.userDefaultViewId)?.id ?? -1
    } catch {
      selectedViewId.value = -1
    }
  }

  return { tableViews, selectedViewId, viewOptions, loadViews }
}
