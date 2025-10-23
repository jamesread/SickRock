<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { createApiClient } from '../stores/api'
import { SickRock } from '../gen/sickrock_pb'
import InsertRow from '../components/InsertRow.vue'
import Section from 'picocrank/vue/components/Section.vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { ArrowLeft01Icon } from '@hugeicons/core-free-icons'

const route = useRoute()
const router = useRouter()
const tableId = route.params.tableName as string
const fieldDefs = ref<Array<{ name: string; type: string; required: boolean }>>([])
const initialValues = ref<Record<string, string> | null>(null)
const displayFieldDefs = ref<Array<{ name: string; type: string; required: boolean }>>([])
const selectedDate = ref<string | null>(null)
const createButtonText = ref<string>('Insert Row')

// Transport handled by authenticated client
const client = createApiClient()

// Views state for controlling insert form ordering/visibility
const tableViews = ref<Array<{ id: number; tableName: string; viewName: string; isDefault: boolean; columns: Array<{ columnName: string; isVisible: boolean; columnOrder: number; sortOrder: string }> }>>([])
const selectedViewId = ref<number | null>(null)
const viewOptions = computed(() => {
  const options = [...tableViews.value]
  if (options.length === 0 || !options.some(v => v.isDefault)) {
    options.unshift({
      id: -1,
      tableName: tableId,
      viewName: 'All Columns',
      isDefault: true,
      columns: []
    })
  }
  return options
})

function applyViewToFields() {
  const defs = [...fieldDefs.value]
  const currentView = tableViews.value.find(v => v.id === selectedViewId.value) || null
  if (!currentView || currentView.id === -1 || !currentView.columns || currentView.columns.length === 0) {
    displayFieldDefs.value = defs
    return
  }

  const orderMap: Record<string, number> = {}
  const visibilityMap: Record<string, boolean> = {}
  currentView.columns.forEach(col => {
    orderMap[col.columnName] = col.columnOrder
    visibilityMap[col.columnName] = col.isVisible
  })

  const inView: typeof defs = []
  const notInView: typeof defs = []
  for (const d of defs) {
    if (orderMap[d.name] != null) inView.push(d)
    else notInView.push(d)
  }
  inView.sort((a, b) => (orderMap[a.name] ?? 0) - (orderMap[b.name] ?? 0))
  const visibleInView = inView.filter(d => visibilityMap[d.name] !== false)
  displayFieldDefs.value = [...visibleInView, ...notInView]
}

onMounted(async () => {
  try {
    const res = await client.getTableStructure({ pageId: tableId })
    const defs = (res.fields ?? [])
      .filter(f => f.name !== 'sr_created' && f.name !== 'sr_updated') // Hide sr_created and sr_updated fields
      .map(f => ({ name: f.name, type: f.type, required: !!f.required }))

    fieldDefs.value = defs

    // Load views for selector and initialize selection
    try {
      const viewsRes = await client.getTableViews({ tableName: tableId })
      tableViews.value = (viewsRes.views || []).map(view => ({
        id: view.id,
        tableName: view.tableName,
        viewName: view.viewName,
        isDefault: view.isDefault,
        columns: view.columns.map(col => ({
          columnName: col.columnName,
          isVisible: col.isVisible,
          columnOrder: col.columnOrder,
          sortOrder: col.sortOrder
        }))
      }))

      const defaultView = tableViews.value.find(v => v.isDefault)
      selectedViewId.value = defaultView ? defaultView.id : (tableViews.value[0]?.id ?? -1)
    } catch (e) {
      selectedViewId.value = -1
    }

    applyViewToFields()

    // Set the create button text as the title
    if (res.CreateButtonText) {
      createButtonText.value = res.CreateButtonText
    }

    // Get date parameter from URL
    const dateParam = route.query.date as string
    if (dateParam) {
      selectedDate.value = dateParam
    }

    // Prepopulate initial values from query params (e.g. fk=value)
    const iv: Record<string, string> = {}
    Object.entries(route.query).forEach(([k, v]) => {
      if (k === 'date') return
      if (typeof v === 'string') {
        iv[k] = v
      }
    })
    initialValues.value = Object.keys(iv).length ? iv : null
  } catch (error) {
    console.error('Error loading table structure:', error)
  }
})

// Recompute display fields when selection or definitions change
watch([selectedViewId, fieldDefs], applyViewToFields)
function handleCreated() {
  // If this insert was initiated from a related row context, propagate origin for return link
  const fromTable = route.query.fromTable as string | undefined
  const fromRowId = route.query.fromRowId as string | undefined
  const dashboard = route.query.dashboard as string | undefined
  const dashboardName = route.query.dashboardName as string | undefined
  const q: Record<string, string> = {}
  if (fromTable) q.fromTable = fromTable
  if (fromRowId) q.fromRowId = fromRowId
  if (dashboard) q.dashboard = dashboard
  if (dashboardName) q.dashboardName = dashboardName
  router.push({ name: 'after-insert', params: { tableName: tableId }, query: q })
}

</script>

<template>
  <Section :title="createButtonText">
    <template #toolbar>
      <div class="toolbar-group">
        <div class="view-selector">
          <label for="insert-view-select">View:</label>
          <select
            id="insert-view-select"
            v-model="selectedViewId"
            class="view-dropdown"
          >
            <option v-for="view in viewOptions" :key="view.id" :value="view.id">
              {{ view.viewName }}
            </option>
          </select>
        </div>
      </div>
      <router-link
        :to="`/table/${tableId}`"
        class="button"
      >
        <HugeiconsIcon :icon="ArrowLeft01Icon" width="16" height="16" />
        Back to Table
      </router-link>
    </template>
    <InsertRow :table-id="tableId" :field-defs="displayFieldDefs" :selected-date="selectedDate" :initial-values="initialValues" @created="handleCreated" />
  </Section>
</template>

<style scoped>
.toolbar-group {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.view-selector {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.view-selector label {
  font-weight: 600;
  color: #333;
}

.view-dropdown {
  padding: 0.5rem 0.75rem;
  border: 1px solid #ddd;
  border-radius: 4px;
  background: white;
  font-size: 1rem;
  cursor: pointer;
  min-width: 150px;
}

.view-dropdown:focus {
  outline: none;
  border-color: #007bff;
  box-shadow: 0 0 0 2px rgba(0, 123, 255, 0.25);
}
</style>
