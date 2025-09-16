<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { createConnectTransport } from '@connectrpc/connect-web'
import { createClient } from '@connectrpc/connect'
import { SickRock } from '../gen/sickrock_pb'
import InsertRow from '../components/InsertRow.vue'
import Section from 'picocrank/vue/components/Section.vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { ArrowLeft01Icon } from '@hugeicons/core-free-icons'

const route = useRoute()
const router = useRouter()
const tableId = route.params.tableName as string
const fieldDefs = ref<Array<{ name: string; type: string; required: boolean }>>([])
const displayFieldDefs = ref<Array<{ name: string; type: string; required: boolean }>>([])
const selectedDate = ref<string | null>(null)
const createButtonText = ref<string>('Insert Row')

const transport = createConnectTransport({ baseUrl: '/api' })
const client = createClient(SickRock, transport)

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
      .filter(f => f.name !== 'sr_created') // Hide sr_created field
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
  } catch (error) {
    console.error('Error loading table structure:', error)
  }
})

// Recompute display fields when selection or definitions change
watch([selectedViewId, fieldDefs], applyViewToFields)
function handleCreated() {
  router.push({ name: 'after-insert', params: { tableName: tableId } })
}

function goBack() {
  router.push(`/table/${tableId}`)
}
</script>

<template>
  <Section :title="createButtonText">
    <template #toolbar>
      <div class="toolbar-group">
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
      <button @click="goBack" class="button back-button">
        <HugeiconsIcon :icon="ArrowLeft01Icon" width="16" height="16" />
        Back to Table
      </button>
    </template>
    <InsertRow :table-id="tableId" :field-defs="displayFieldDefs" :selected-date="selectedDate" @created="handleCreated" />
  </Section>
</template>
