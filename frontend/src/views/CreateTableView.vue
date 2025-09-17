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

// Load available columns from table structure
onMounted(async () => {
  try {
    const res = await client.getTableStructure({ pageId: tableId })
    availableColumns.value = res.fields?.map(field => ({
      name: field.name,
      type: field.type
    })) || []

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
    const visibleColumns = selectedColumns.value.filter(col => col.isVisible)

    let response
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
        }))
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
        }))
      })
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
      <button @click="goBack" class="button neutral">
        <HugeiconsIcon :icon="ArrowLeft01Icon" />
        Back to Table
      </button>
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

        <label>Column Configuration</label>
        <div class="columns-list">
          <div
            v-for="(column, index) in selectedColumns"
            :key="column.name"
            class="column-item"
          >
            <div class="column-info">
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

.columns-list {
  border: 1px solid #ddd;
  border-radius: 4px;
  max-height: 400px;
  overflow-y: auto;
}

.column-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem;
  border-bottom: 1px solid #eee;
}

.column-item:last-child {
  border-bottom: none;
}

.column-info {
  display: flex;
  align-items: center;
  gap: 0.5rem;
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
