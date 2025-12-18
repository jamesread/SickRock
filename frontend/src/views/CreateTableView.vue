<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { createApiClient } from '../stores/api'
import { SickRock } from '../gen/sickrock_pb'
import Section from 'picocrank/vue/components/Section.vue'
import { HugeiconsIcon, ArrowLeft01Icon } from '@hugeicons/core-free-icons'

const route = useRoute()
const router = useRouter()
const tableId = route.params.tableName as string
const viewId = route.params.viewId as string

// Transport handled by authenticated client
const client = createApiClient()

const viewName = ref('')
const viewType = ref<'table' | 'calendar' | 'ticklist'>('table')
const availableColumns = ref<Array<{ name: string; type: string }>>([])
const selectedColumns = ref<Array<{ name: string; isVisible: boolean; columnOrder: number; sortOrder: string }>>([])
const loading = ref(false)
const error = ref<string | null>(null)
const showDeleteConfirm = ref(false)
const deleting = ref(false)

// Determine if we're in edit mode
const isEditMode = computed(() => !!viewId && viewId !== 'new')
const pageTitle = computed(() => isEditMode.value ? 'Edit Table View' : 'Create Table View')
const submitButtonText = computed(() => loading.value ? 'Saving...' : (isEditMode.value ? 'Update View' : 'Save View'))

// Drag-and-drop state
const dragIndex = ref<number | null>(null)
const overIndex = ref<number | null>(null)

function onDragStart(index: number, ev: DragEvent) {
  dragIndex.value = index
  ev.dataTransfer?.setData('text/plain', String(index))
  ev.dataTransfer?.setDragImage?.(new Image(), 0, 0)
}

function onDragOverAt(index: number, ev: DragEvent) {
  ev.preventDefault()
  const el = ev.currentTarget as HTMLElement | null
  if (el) {
    const rect = el.getBoundingClientRect()
    const offsetY = ev.clientY - rect.top
    const half = rect.height / 2
    overIndex.value = offsetY < half ? index : index + 1
  } else {
    overIndex.value = index
  }
  if (ev.dataTransfer) ev.dataTransfer.dropEffect = 'move'
}

function onDragEnterAt(index: number, ev: DragEvent) {
  ev.preventDefault()
  overIndex.value = index
}

function onDrop(targetIndex: number, ev: DragEvent) {
  ev.preventDefault()
  const from = dragIndex.value ?? parseInt(ev.dataTransfer?.getData('text/plain') || '-1')
  const to = overIndex.value ?? targetIndex
  if (isNaN(from) || from < 0 || from === to) return
  const list = [...selectedColumns.value]
  const [moved] = list.splice(from, 1)
  const boundedTo = Math.max(0, Math.min(list.length, to))
  list.splice(boundedTo, 0, moved)
  list.forEach((c, i) => { c.columnOrder = i })
  selectedColumns.value = list
  dragIndex.value = null
  overIndex.value = null
}
function onListDragOver(ev: DragEvent) {
  ev.preventDefault()
}


function onDragEnd() {
  dragIndex.value = null
  overIndex.value = null
}

// Load available columns from table structure
onMounted(async () => {
  try {
    const res = await client.getTableStructure({ pageId: tableId })
    availableColumns.value = res.fields?.map(field => ({
      name: field.name,
      type: field.type
    })) || []

    // Load current view type from existing view if editing
    if (isEditMode.value) {
      // Will be loaded in loadExistingView()
    } else {
      // Default to "table" for new views
      viewType.value = 'table'
    }

    if (isEditMode.value) {
      // Load existing view data
      await loadExistingView()
    } else {
      // Initialize selected columns (all visible by default)
      selectedColumns.value = availableColumns.value.map((col, index) => ({
        name: col.name,
        isVisible: true,
        columnOrder: index,
        sortOrder: ''
      }))
    }
  } catch (err) {
    error.value = String(err)
  }
})

async function loadExistingView() {
  try {
    const response = await client.getTableViews({ tableName: tableId })
    const view = response.views.find(v => v.id === parseInt(viewId))

    if (view) {
      viewName.value = view.viewName

      // Initialize selected columns based on existing view
      selectedColumns.value = availableColumns.value.map((col, index) => {
        const existingColumn = view.columns.find(c => c.columnName === col.name)
        return {
          name: col.name,
          isVisible: existingColumn ? existingColumn.isVisible : false,
          columnOrder: existingColumn ? existingColumn.columnOrder : index,
          sortOrder: existingColumn ? existingColumn.sortOrder : ''
        }
      })

      // Sort by column order
      selectedColumns.value.sort((a, b) => a.columnOrder - b.columnOrder)
    } else {
      error.value = 'View not found'
    }
  } catch (err) {
    error.value = String(err)
  }
}

function updateColumnOrder(index: number, direction: 'up' | 'down') {
  const newOrder = [...selectedColumns.value]
  const item = newOrder[index]

  if (direction === 'up' && index > 0) {
    newOrder[index] = newOrder[index - 1]
    newOrder[index - 1] = item
    // Update column orders
    newOrder.forEach((col, i) => {
      col.columnOrder = i
    })
    selectedColumns.value = newOrder
  } else if (direction === 'down' && index < newOrder.length - 1) {
    newOrder[index] = newOrder[index + 1]
    newOrder[index + 1] = item
    // Update column orders
    newOrder.forEach((col, i) => {
      col.columnOrder = i
    })
    selectedColumns.value = newOrder
  }
}

function updateSortOrder(index: number, sortOrder: string) {
  selectedColumns.value[index].sortOrder = sortOrder
}

async function saveTableView() {
  if (!viewName.value.trim()) {
    error.value = 'View name is required'
    return
  }

  loading.value = true
  error.value = null

  try {
    let response

    // Only save table view columns if view type is table
    if (viewType.value === 'table') {
      const visibleColumns = selectedColumns.value.filter(col => col.isVisible)

      if (isEditMode.value) {
        // Update existing view
        response = await client.updateTableView({
          viewId: parseInt(viewId),
          tableName: tableId,
          viewName: viewName.value,
          columns: visibleColumns.map(col => ({
            columnName: col.name,
            isVisible: col.isVisible,
            columnOrder: col.columnOrder,
            sortOrder: col.sortOrder
          })),
          viewType: viewType.value
        })
      } else {
      // Create new view
      response = await client.createTableView({
        tableName: tableId,
        viewName: viewName.value,
        columns: visibleColumns.map(col => ({
          columnName: col.name,
          isVisible: col.isVisible,
          columnOrder: col.columnOrder,
          sortOrder: col.sortOrder
        })),
        viewType: viewType.value
      })
      }
    } else {
      // For calendar and ticklist views, we still create/update the view record
      // Column configurations are not used for these views, but the view record is still needed
      if (isEditMode.value) {
        response = await client.updateTableView({
          viewId: parseInt(viewId),
          tableName: tableId,
          viewName: viewName.value,
          columns: [], // No columns for calendar/ticklist views
          viewType: viewType.value
        })
      } else {
        response = await client.createTableView({
          tableName: tableId,
          viewName: viewName.value,
          columns: [], // No columns for calendar/ticklist views
          viewType: viewType.value
        })
      }
    }

    if (response.success) {
      // Redirect back to table on success
      router.push({ name: 'table', params: { tableName: tableId } })
    } else {
      error.value = response.message || `Failed to ${isEditMode.value ? 'update' : 'create'} table view`
    }
  } catch (err) {
    error.value = String(err)
  } finally {
    loading.value = false
  }
}

function goBack() {
  router.push({ name: 'table', params: { tableName: tableId } })
}

async function deleteTableView() {
  if (!isEditMode.value || !viewId) {
    return
  }

  deleting.value = true
  error.value = null

  try {
    const response = await client.deleteTableView({
      viewId: parseInt(viewId)
    })

    if (response.success) {
      // Navigate back to table view
      router.push({ name: 'table', params: { tableName: tableId } })
    } else {
      error.value = response.message || 'Failed to delete view'
    }
  } catch (err) {
    error.value = `Error deleting view: ${err}`
  } finally {
    deleting.value = false
    showDeleteConfirm.value = false
  }
}
</script>

<template>
  <Section :title="pageTitle">
    <template #toolbar>
      <router-link
        :to="`/table/${tableId}`"
        class="button"
      >
        <HugeiconsIcon :icon="ArrowLeft01Icon" width="16" height="16" />
        Back to Table
      </router-link>
    </template>

    <div v-if="error" class="error-message">
      {{ error }}
    </div>

    <form @submit.prevent="saveTableView">
        <label for="view-name">View Name</label>
        <input
          id="view-name"
          v-model="viewName"
          type="text"
          placeholder="Enter view name (e.g., Compact, Detailed)"
          required
        />

        <label for="view-type">View Type</label>
        <select
          id="view-type"
          v-model="viewType"
          class="view-type-select"
        >
          <option value="table">Table</option>
          <option value="calendar">Calendar</option>
          <option value="ticklist">Tick List</option>
        </select>
        <small class="view-type-help">
          <span v-if="viewType === 'table'">Table view displays data in rows and columns with sorting and filtering.</span>
          <span v-else-if="viewType === 'calendar'">Calendar view displays data as events on a calendar based on date fields.</span>
          <span v-else-if="viewType === 'ticklist'">Tick List view displays items as large tiles that can be checked off as completed.</span>
        </small>

        <div v-if="viewType === 'table'">
          <label>Column Configuration</label>
          <div class="columns-list" @dragover="onListDragOver">
          <!-- Drag indicators -->
          <div v-if="dragIndex !== null" class="drag-status">
            Dragging: <code>{{ selectedColumns[dragIndex].name }}</code>
            <span v-if="overIndex !== null" class="drop-pos">→ Drop at position {{ overIndex + 1 }}</span>
          </div>
          <div
            v-for="(column, index) in selectedColumns"
            :key="column.name"
            class="column-item"
            :data-index="index"
            @dragover="onDragOverAt(index, $event)"
            @dragenter="onDragEnterAt(index, $event)"
            @drop="onDrop(index, $event)"
            @dragend="onDragEnd"
          >
            <div
              v-if="overIndex !== null && (overIndex === index || overIndex === index + 1)"
              class="drop-indicator"
              :style="{
                top: (overIndex === index ? '0' : 'calc(100% - 2px)')
              }"
            />
            <div class="column-info">
              <span
                class="drag-handle"
                title="Drag to reorder"
                aria-label="Drag to reorder"
                draggable="true"
                @dragstart="onDragStart(index, $event)"
              >⠿</span>
              <input
                type="checkbox"
                :id="`col-${index}`"
                v-model="column.isVisible"
              />
              <label :for="`col-${index}`" class="column-name">
                {{ column.name }}
                <span class="column-type">({{ availableColumns.find(c => c.name === column.name)?.type }})</span>
              </label>
            </div>

            <div class="column-controls">
              <div class="order-controls">
                <button
                  type="button"
                  @click="updateColumnOrder(index, 'up')"
                  :disabled="index === 0"
                  class="button small"
                >
                  ↑
                </button>
                <button
                  type="button"
                  @click="updateColumnOrder(index, 'down')"
                  :disabled="index === selectedColumns.length - 1"
                  class="button small"
                >
                  ↓
                </button>
              </div>

              <select
                v-model="column.sortOrder"
                @change="updateSortOrder(index, column.sortOrder)"
                class="sort-select"
              >
                <option value="">No Sort</option>
                <option value="asc">Ascending</option>
                <option value="desc">Descending</option>
              </select>
            </div>
          </div>
          </div>
        </div>
        <div v-else class="calendar-view-info">
          <p>Calendar views display data as events on a calendar. The calendar automatically detects date fields (calendar_date, starts, finishes, or sr_created) to display events.</p>
          <p>No column configuration is needed for calendar views.</p>
        </div>

      <div class="form-actions" style = "grid-column: 1 / -1;">
        <button type="button" @click="goBack" class="button neutral">
          Cancel
        </button>
        <button type="submit" :disabled="loading || !viewName.trim()" class="button good">
          {{ submitButtonText }}
        </button>
        <div class = "fg1" />
        <button
          v-if="isEditMode"
          type="button"
          @click="showDeleteConfirm = true"
          :disabled="deleting"
          class="button bad"
        >
          {{ deleting ? 'Deleting...' : 'Delete View' }}
        </button>
      </div>
    </form>

    <!-- Delete Confirmation Dialog -->
    <div v-if="showDeleteConfirm" class="modal-overlay" @click="showDeleteConfirm = false">
      <div class="modal" @click.stop>
        <h3>Delete View</h3>
        <p>Are you sure you want to delete the view "{{ viewName }}"? This action cannot be undone.</p>
        <div class="modal-actions">
          <button @click="showDeleteConfirm = false" class="button neutral">
            Cancel
          </button>
          <button @click="deleteTableView" :disabled="deleting" class="button bad">
            {{ deleting ? 'Deleting...' : 'Delete View' }}
          </button>
        </div>
      </div>
    </div>
  </Section>
</template>

<style scoped>
.error-message {
  background: #f8d7da;
  color: #721c24;
  padding: 1rem;
  border-radius: 4px;
  margin-bottom: 1rem;
}

.form-group {
  margin-bottom: 2rem;
}

.form-group label {
  display: block;
  margin-bottom: 0.5rem;
  font-weight: 600;
}

.form-group input[type="text"] {
  width: 100%;
  padding: 0.75rem;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 1rem;
}

.view-type-select {
  width: 100%;
  padding: 0.75rem;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 1rem;
  background: white;
  cursor: pointer;
}

.view-type-select:focus {
  outline: none;
  border-color: #007bff;
  box-shadow: 0 0 0 2px rgba(0, 123, 255, 0.25);
}

.view-type-help {
  display: block;
  margin-top: 0.5rem;
  color: #666;
  font-size: 0.875rem;
}

.calendar-view-info {
  padding: 1rem;
  background: #f8f9fa;
  border: 1px solid #dee2e6;
  border-radius: 4px;
  color: #495057;
}

.calendar-view-info p {
  margin: 0.5rem 0;
}

.calendar-view-info p:first-child {
  margin-top: 0;
}

.calendar-view-info p:last-child {
  margin-bottom: 0;
}

.columns-list {
  border: 1px solid #ddd;
  border-radius: 4px;
  max-height: 750px;
  overflow-y: auto;
}

.drag-status {
  position: sticky;
  top: 0;
  background: #fffbe6;
  border-bottom: 1px solid #f1e8b8;
  color: #6b5d00;
  padding: 0.5rem 1rem;
  font-size: 0.9rem;
  z-index: 0;
  pointer-events: none;
}

.drag-status .drop-pos {
  margin-left: .5rem;
  font-weight: 600;
}

.column-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem;
  border-bottom: 1px solid #eee;
  background: #fff;
  position: relative; /* for drop indicator positioning */
}

.column-item:last-child {
  border-bottom: none;
}

/* Drag initiated only from handle */

.drop-indicator {
  position: absolute;
  left: 0;
  right: 0;
  height: 4px;
  background: #007bff;
  border-radius: 2px;
  z-index: 5;
  pointer-events: none;
}

.column-info {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.drag-handle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2rem;
  height: 2rem;
  color: #888;
  cursor: grab;
  user-select: none;
  font-size: 1.4rem;
  border-radius: 4px;
  transition: color .15s ease, background-color .15s ease, transform .1s ease;
}

.drag-handle:active {
  cursor: grabbing;
}

.drag-handle:hover {
  color: #333;
  background: #f2f4f7;
  transform: scale(1.05);
}

.column-name {
  font-weight: 500;
  margin: 0;
  cursor: pointer;
}

.column-type {
  color: #666;
  font-weight: normal;
  font-size: 0.9rem;
}

.column-controls {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.order-controls {
  display: flex;
  gap: 0.25rem;
}

.button.small {
  padding: 0.25rem 0.5rem;
  font-size: 0.8rem;
  min-width: auto;
}

.sort-select {
  padding: 0.5rem;
  border: 1px solid #ddd;
  border-radius: 4px;
  background: white;
}

.form-actions {
  display: flex;
  gap: 1rem;
  justify-content: flex-end;
  margin-top: 2rem;
}

.button:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.button.neutral {
  background: #6c757d;
  color: white;
}

.button.neutral:hover:not(:disabled) {
  background: #5a6268;
}

.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal {
  background: white;
  padding: 2rem;
  border-radius: 8px;
  box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
  max-width: 400px;
  width: 90%;
}

.modal h3 {
  margin: 0 0 1rem 0;
  color: #333;
}

.modal p {
  margin: 0 0 1.5rem 0;
  color: #666;
}

.modal-actions {
  display: flex;
  gap: 1rem;
  justify-content: flex-end;
}
</style>
