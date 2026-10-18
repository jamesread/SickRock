<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute } from 'vue-router'

import { createApiClient } from '../stores/api'
import type { GetTableStructureResponse } from '../gen/sickrock_pb'
import TableComponent from '../components/TableComponent.vue'
import CalendarComponent from '../components/CalendarComponent.vue'
import TickListComponent from '../components/TickListComponent.vue'
import Section from 'picocrank/vue/components/Section.vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { DatabaseIcon } from '@hugeicons/core-free-icons'
import { useTableViewManager } from '../composables/useTableViewManager'
import { useTableAccess } from '../composables/useTableAccess'
import { useRbac } from '../composables/useRbac'
import { tableViewSectionTitle } from '../utils/rowInsertLabel'

const route = useRoute()
const props = defineProps<{ tableName?: string }>()
const tableId = computed(() => (props.tableName ?? (route.params.tableName as string)))
const { access: tableAccess } = useTableAccess(tableId)
const { hasPermission } = useRbac()
const showStructure = computed(() => hasPermission('system.settings'))

// Optional ?where={"column":"value"} (same format as export links), e.g. from
// "view all related …" links. Invalid values are ignored.
const whereFilter = computed<Record<string, string> | undefined>(() => {
  const raw = route.query.where
  if (typeof raw !== 'string' || !raw.trim()) return undefined
  try {
    const parsed = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return undefined
    const entries = Object.entries(parsed).map(([column, value]) => [column, String(value)])
    return entries.length > 0 ? Object.fromEntries(entries) : undefined
  } catch {
    return undefined
  }
})
const whereDescription = computed(() => Object.entries(whereFilter.value ?? {})
  .map(([column, value]) => `${column} = ${value}`)
  .join(', '))

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

const tableSectionTitle = computed(() =>
  tableViewSectionTitle(
    tableStructure.value?.rowNamePlural,
    tableStructure.value?.rowName,
    tableStructure.value?.title,
    tableId.value,
  ),
)

function handleViewChanged(viewType: string) {
  currentViewType.value = viewType
  viewTypeInitialized.value = true // Mark as initialized by user interaction
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
    <Section
      v-else
      :padding="false"
      classes="table-list-section"
      subtitle="Browse, filter, and manage rows in this table."
    >
      <template #title>
        <span class="section-title-with-icon">
          <HugeiconsIcon :icon="DatabaseIcon" width="22" height="22" aria-hidden="true" />
          <span>{{ tableSectionTitle }}</span>
        </span>
      </template>
      <p v-if="whereFilter" class="section-content padding table-where-notice">
        Showing only rows where <code>{{ whereDescription }}</code>.
        <router-link :to="{ path: route.path }">Show all</router-link>
      </p>
      <TableComponent
        :table-id="tableId"
        :table-structure="tableStructure"
        :where="whereFilter"
        :wrap-in-section="false"
        toolbar-teleport=".table-list-section [role='toolbar']"
        :show-toolbar="true"
        :show-view-switcher="true"
        :show-export="tableAccess.canView"
        :show-structure="showStructure"
        :show-insert="tableAccess.canInsert"
        :show-pagination="true"
        :show-view-create="tableAccess.canEdit"
        :show-view-edit="tableAccess.canEdit"
        @view-changed="handleViewChanged"
      />
    </Section>
  </template>
</template>

<style scoped>
.section-title-with-icon {
  display: inline-flex;
  align-items: center;
  gap: 0.45em;
  vertical-align: middle;
}
</style>
