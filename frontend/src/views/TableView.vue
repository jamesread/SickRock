<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute } from 'vue-router'

import { createApiClient } from '../stores/api'
import type { GetTableStructureResponse } from '../gen/sickrock_pb'
import TableComponent from '../components/TableComponent.vue'
import CalendarComponent from '../components/CalendarComponent.vue'
import TickListComponent from '../components/TickListComponent.vue'
import { useTableViewManager } from '../composables/useTableViewManager'

const route = useRoute()
const props = defineProps<{ tableName?: string }>()
const tableId = computed(() => (props.tableName ?? (route.params.tableName as string)))

// Transport handled by authenticated client
const client = createApiClient()

const tableStructure = ref<GetTableStructureResponse | null>(null)
const loading = ref(true)
const currentViewType = ref<string>('table') // Default to "table"
const viewTypeInitialized = ref(false) // Track if view type has been set by user interaction

// Use table view manager to track current view
const viewManager = useTableViewManager(tableId.value, (view) => {
  if (view) {
    currentViewType.value = view.viewType || 'table'
    viewTypeInitialized.value = true
  }
})

// Computed property for current view name
const currentViewName = computed(() => {
  const view = viewManager.currentView.value
  return view && view.id !== -1 ? view.viewName : null
})

function handleViewChanged(viewType: string) {
  console.log('[TableView] View changed to:', viewType)
  currentViewType.value = viewType
  viewTypeInitialized.value = true // Mark as initialized by user interaction
  console.log('[TableView] currentViewType is now:', currentViewType.value)
}

onMounted(async () => {
  try {
    const res = await client.getTableStructure({ pageId: tableId.value })
    tableStructure.value = res

    // Load views using the view manager
    await viewManager.loadTableViews()

    // Determine the current view type (only if not already set by user)
    if (!viewTypeInitialized.value) {
      const currentView = viewManager.currentView.value
      if (currentView && currentView.viewType) {
        currentViewType.value = currentView.viewType
      }
    }
  } catch (error) {
    console.error('Failed to get table structure:', error)
    tableStructure.value = null
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div v-if="loading">Loading...</div>
  <template v-else>
    <CalendarComponent v-if="currentViewType === 'calendar'" :table-id="tableId" :view-name="currentViewName" @view-changed="handleViewChanged" />
    <TickListComponent v-else-if="currentViewType === 'ticklist'" :table-id="tableId" @view-changed="handleViewChanged" />
    <TableComponent v-else :table-id="tableId" :table-structure="tableStructure" :show-toolbar="true" :show-view-switcher="true" :show-export="true" :show-structure="true" :show-insert="true" :show-pagination="true" :show-view-create="true" :show-view-edit="true" @view-changed="handleViewChanged"/>
  </template>
</template>
