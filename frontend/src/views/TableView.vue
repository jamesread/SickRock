<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute } from 'vue-router'

import { createApiClient } from '../stores/api'
import type { GetTableStructureResponse } from '../gen/sickrock_pb'
import TableComponent from '../components/TableComponent.vue'
import CalendarComponent from '../components/CalendarComponent.vue'
import TickListComponent from '../components/TickListComponent.vue'

const route = useRoute()
const props = defineProps<{ tableName?: string }>()
const tableId = computed(() => (props.tableName ?? (route.params.tableName as string)))

// Transport handled by authenticated client
const client = createApiClient()

const tableStructure = ref<GetTableStructureResponse | null>(null)
const loading = ref(true)
const currentViewType = ref<string>('table') // Default to "table"
const viewTypeInitialized = ref(false) // Track if view type has been set by user interaction

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

    // Load views to determine the current view type (only if not already set by user)
    if (!viewTypeInitialized.value) {
      try {
        const viewsRes = await client.getTableViews({ tableName: tableId.value })
        if (viewsRes.views && viewsRes.views.length > 0) {
          // Find default view or first view
          const defaultView = viewsRes.views.find(v => v.isDefault) || viewsRes.views[0]
          if (defaultView.viewType) {
            currentViewType.value = defaultView.viewType
          }
        }
      } catch (e) {
        console.warn('Failed to load views for view type:', e)
        // Default to "table" if views can't be loaded
        currentViewType.value = 'table'
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
    <CalendarComponent v-if="currentViewType === 'calendar'" :table-id="tableId" @view-changed="handleViewChanged" />
    <TickListComponent v-else-if="currentViewType === 'ticklist'" :table-id="tableId" @view-changed="handleViewChanged" />
    <TableComponent v-else :table-id="tableId" :table-structure="tableStructure" :show-toolbar="true" :show-view-switcher="true" :show-export="true" :show-structure="true" :show-insert="true" :show-pagination="true" :show-view-create="true" :show-view-edit="true" @view-changed="handleViewChanged"/>
  </template>
</template>
