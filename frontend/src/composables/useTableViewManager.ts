import { ref, computed, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { createApiClient } from '../stores/api'

export type TableView = {
  id: number
  tableName: string
  viewName: string
  isDefault: boolean
  viewType: string
  columns: Array<{
    columnName: string
    isVisible: boolean
    columnOrder: number
    sortOrder: string
  }>
}

export function useTableViewManager(tableId: string, onViewChange?: (view: TableView | null) => void) {
  const router = useRouter()
  const client = createApiClient()

  const tableViews = ref<TableView[]>([])
  const selectedViewId = ref<number | null>(null)
  const showViewsDialog = ref(false)
  const isInitialLoad = ref(true) // Track if this is the first load

  // Computed property for the current view
  const currentView = computed(() => {
    return tableViews.value.find(view => view.id === selectedViewId.value) || null
  })

  // Find default view for this table, if any
  const defaultView = computed(() => tableViews.value.find(v => v.isDefault) || null)

  // Dedicated list for the Views dialog: always show "All Columns" plus every saved view
  const viewsForDialog = computed(() => {
    return [
      {
        id: -1,
        tableName: tableId,
        viewName: 'All Columns',
        isDefault: !tableViews.value.length || !!defaultView.value,
        viewType: 'table',
        columns: []
      },
      ...tableViews.value
    ]
  })

  async function loadTableViews() {
    try {
      const response = await client.getTableViews({ tableName: tableId })
      tableViews.value = response.views.map(view => ({
        id: view.id,
        tableName: view.tableName,
        viewName: view.viewName,
        isDefault: view.isDefault,
        viewType: view.viewType || 'table', // Default to "table" if not set
        columns: view.columns.map(col => ({
          columnName: col.columnName,
          isVisible: col.isVisible,
          columnOrder: col.columnOrder,
          sortOrder: col.sortOrder
        }))
      }))

      // Find default view or first view
      const defaultView = tableViews.value.find(v => v.isDefault) || (tableViews.value.length > 0 ? tableViews.value[0] : null)

      // Only select default view if no view is currently selected
      // This prevents overriding user's selection when component remounts
      if (selectedViewId.value === null) {
        // Select the default view or first view
        if (defaultView) {
          selectedViewId.value = defaultView.id
          // Only emit view-changed on initial load if we're not already showing a specific view type
          // This prevents TickListComponent/CalendarComponent from overriding the view type when they mount
          if (onViewChange && !isInitialLoad.value) {
            onViewChange(defaultView)
          }
        } else {
          // No views exist, use the default "All Columns" view
          selectedViewId.value = -1
          if (onViewChange && !isInitialLoad.value) {
            onViewChange(null)
          }
        }
      } else {
        // A view is already selected, verify it still exists in the loaded views
        const currentViewExists = selectedViewId.value === -1 || tableViews.value.some(v => v.id === selectedViewId.value)
        if (!currentViewExists) {
          // Current selection no longer exists, fall back to default
          if (defaultView) {
            selectedViewId.value = defaultView.id
            if (onViewChange) {
              onViewChange(defaultView)
            }
          } else {
            selectedViewId.value = -1
            if (onViewChange) {
              onViewChange(null)
            }
          }
        }
        // If current selection still exists, don't change anything
      }

      // Mark that initial load is complete
      isInitialLoad.value = false
    } catch (error) {
      console.error('Failed to load table views:', error)
      // Only set fallback if no view is currently selected
      if (selectedViewId.value === null) {
        selectedViewId.value = -1
        if (onViewChange && !isInitialLoad.value) {
          onViewChange(null)
        }
      }
      // Mark that initial load is complete even on error
      isInitialLoad.value = false
    }
  }

  function openViewsDialog() {
    showViewsDialog.value = true
  }

  function closeViewsDialog() {
    showViewsDialog.value = false
  }

  function selectView(viewId: number) {
    selectedViewId.value = viewId
    nextTick(() => {
      const view = currentView.value
      if (onViewChange) {
        onViewChange(view)
      }
    })
    closeViewsDialog()
  }

  function createTableView() {
    router.push({ name: 'create-table-view', params: { tableName: tableId } })
  }

  function editTableView() {
    if (currentView.value && currentView.value.id !== -1) {
      router.push({
        name: 'edit-table-view',
        params: {
          tableName: tableId,
          viewId: currentView.value.id.toString()
        }
      })
    }
  }

  return {
    tableViews,
    selectedViewId,
    currentView,
    defaultView,
    viewsForDialog,
    showViewsDialog,
    loadTableViews,
    openViewsDialog,
    closeViewsDialog,
    selectView,
    createTableView,
    editTableView
  }
}
