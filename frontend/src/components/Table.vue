<script setup lang="ts">
import { onMounted, ref, computed, watch } from 'vue'
import Pagination from 'picocrank/vue/components/Pagination.vue'
import ColumnVisibilityDropdown from './ColumnVisibilityDropdown.vue'
import RowActionsDropdown from './RowActionsDropdown.vue'
import { createConnectTransport } from '@connectrpc/connect-web'
import { createClient } from '@connectrpc/connect'
import { SickRock } from '../gen/sickrock_pb'
import InsertRow from './InsertRow.vue'
import { GetTableStructureResponse } from '../gen/sickrock_pb'

const tableStructure = ref<GetTableStructureResponse | null>(null)

const props = defineProps<{ tableId: string; fields?: Array<{ name: string; type: string }>; createButtonText?: string }>()

type Item = Record<string, unknown>

const items = ref<Item[]>([])
const loading = ref(false)
const error = ref<string | null>(null)

const localFields = ref<string[]>([])
const localFieldDefs = ref<Array<{ name: string; type: string; required: boolean }>>([])
watch(
  () => props.fields,
  (f) => {
    if (f && f.length) localFields.value = f.map(x => x.name)
  },
  { immediate: true }
)
async function loadStructure() {
  tableStructure.value = await client.getTableStructure({ pageId: props.tableId })
  const defs = (tableStructure.value.fields ?? []).map(f => ({ name: f.name, type: f.type, required: !!f.required }))
  const names = defs.map(d => d.name)
  if (names.length) {
    localFieldDefs.value = defs
    localFields.value = names
    selectedColumns.value = [...names]
  }
}
const columns = computed(() => localFields.value.length ? localFields.value : ['id', 'created_at_unix'])
const selectedColumns = ref<string[]>([])
watch(columns, (cols) => { selectedColumns.value = [...cols] }, { immediate: true })
const visibleColumns = computed(() => selectedColumns.value.filter(c => columns.value.includes(c)))

const sortBy = ref<string | null>(null)
const sortDir = ref<'asc' | 'desc'>('asc')
function toggleSort(col: string) {
  if (sortBy.value === col) {
    sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
  } else {
    sortBy.value = col
    sortDir.value = 'asc'
  }
}
const sortedItems = computed(() => {
  const col = sortBy.value
  if (!col) return items.value
  const dir = sortDir.value === 'asc' ? 1 : -1
  return [...items.value].sort((a, b) => {
    const av = (a as any)[col]
    const bv = (b as any)[col]
    if (av == null && bv == null) return 0
    if (av == null) return 1
    if (bv == null) return -1
    const an = typeof av === 'bigint' ? Number(av) : av
    const bn = typeof bv === 'bigint' ? Number(bv) : bv
    if (typeof an === 'number' && typeof bn === 'number') return (an - bn) * dir
    const as = String(an)
    const bs = String(bn)
    return as.localeCompare(bs) * dir
  })
})

const page = ref(1)
const pageSize = ref(10)
const total = computed(() => sortedItems.value.length)
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))
watch([sortedItems, pageSize], () => { page.value = 1 })
const pagedItems = computed(() => {
  const start = (page.value - 1) * pageSize.value
  return sortedItems.value.slice(start, start + pageSize.value)
})

const selectedKeys = ref<Set<string>>(new Set())

// Inline editing state
const editingCell = ref<{ rowId: string; column: string } | null>(null)
const editingValue = ref<string>('')
const saving = ref(false)
const editInput = ref<HTMLInputElement | null>(null)

// Bulk delete state
const showDeleteConfirm = ref(false)
const deleting = ref(false)

// Helper function to get item value for a column, handling both standard and dynamic fields
function getItemValue(item: any, column: string): any {
  // Check standard fields first (only id and created_at_unix are static now)
  if (column === 'id' || column === 'created_at_unix') {
    return item[column]
  }
  // Check additional fields from protobuf (all other fields including name are dynamic)
  if (item.additionalFields && item.additionalFields[column] !== undefined) {
    return item.additionalFields[column]
  }
  // Fallback to direct property access
  return item[column]
}

function keyOf(it: any): string {
  const k = getItemValue(it, 'id')
  return k == null ? '' : String(k)
}
function isSelected(it: any): boolean {
  const k = keyOf(it)
  return k !== '' && selectedKeys.value.has(k)
}
function toggleSelected(it: any, ev: Event) {
  const k = keyOf(it)
  if (k === '') return
  const checked = (ev.target as HTMLInputElement).checked
  if (checked) selectedKeys.value.add(k)
  else selectedKeys.value.delete(k)
}

const transport = createConnectTransport({ baseUrl: '/api' })
const client = createClient(SickRock, transport)

async function load() {
  loading.value = true
  error.value = null
  try {
    const res = await client.listItems({ pageId: props.tableId })
    items.value = Array.isArray(res.items) ? (res.items as Item[]) : []
  } catch (e) {
    error.value = String(e)
  } finally {
    loading.value = false
  }
}

// Inline editing functions
function startEdit(item: any, column: string) {
  // Don't allow editing of id or created_at_unix columns
  if (column === 'id' || column === 'created_at_unix') {
    return
  }

  const currentValue = getItemValue(item, column)
  editingCell.value = { rowId: String(item.id), column }
  editingValue.value = currentValue == null ? '' : String(currentValue)

  // Focus the input after the DOM updates
  setTimeout(() => {
    if (editInput.value) {
      editInput.value.focus()
      editInput.value.select()
    }
  }, 0)
}

function cancelEdit() {
  editingCell.value = null
  editingValue.value = ''
}

async function saveEdit(item: any) {
  if (!editingCell.value) return

  saving.value = true
  try {
    const { rowId, column } = editingCell.value

    // Prepare the update data - all fields go into additionalFields
    const additionalFields: Record<string, string> = {}

    // Get all current values and update just the one being edited
    const currentItem = items.value.find(it => String(it.id) === rowId)
    if (currentItem) {
      // Get all additional fields from the current item
      if (currentItem.additionalFields) {
        Object.entries(currentItem.additionalFields).forEach(([key, value]) => {
          additionalFields[key] = String(value)
        })
      }
      // Update the specific field being edited
      additionalFields[column] = editingValue.value

      await client.editItem({
        id: rowId,
        additionalFields: additionalFields,
        pageId: props.tableId
      })
    }

    // Reload the data to reflect changes
    await load()
    cancelEdit()
  } catch (e) {
    console.error('Failed to save edit:', e)
    // You might want to show an error message to the user here
  } finally {
    saving.value = false
  }
}

function isEditing(item: any, column: string): boolean {
  return editingCell.value?.rowId === String(item.id) && editingCell.value?.column === column
}

// Helper function to check if a column is a tinyint (boolean) column
function isTinyintColumn(column: string): boolean {
  const field = localFieldDefs.value.find(f => f.name === column)
  return field?.type.startsWith('tinyint')
}

// Helper function to get boolean value from tinyint column
function getBooleanValue(item: any, column: string): boolean {
  const value = getItemValue(item, column)
  if (value === null || value === undefined) return false
  // Convert to number first, then to boolean
  const numValue = Number(value)
  return numValue === 1
}

// Bulk delete functionality
const selectedItems = computed(() => {
  return items.value.filter(item => {
    const key = keyOf(item)
    return key !== '' && selectedKeys.value.has(key)
  })
})

const hasSelectedItems = computed(() => selectedKeys.value.size > 0)

async function deleteSelectedItems() {
  if (selectedItems.value.length === 0) return

  deleting.value = true
  error.value = null

  try {
    // Delete items one by one
    for (const item of selectedItems.value) {
      const key = keyOf(item)
      if (key !== '') {
        await client.deleteItem({ pageId: props.tableId, id: key })
      }
    }

    // Clear selection and reload data
    selectedKeys.value.clear()
    await load()
    showDeleteConfirm.value = false
  } catch (e) {
    error.value = String(e)
  } finally {
    deleting.value = false
  }
}

function confirmDeleteSelected() {
  if (selectedItems.value.length > 0) {
    showDeleteConfirm.value = true
  }
}

function cancelDeleteSelected() {
  showDeleteConfirm.value = false
}

function selectAll() {
  pagedItems.value.forEach(item => {
    const key = keyOf(item)
    if (key !== '') {
      selectedKeys.value.add(key)
    }
  })
}

function selectNone() {
  selectedKeys.value.clear()
}

onMounted(load)
onMounted(loadStructure)
</script>

<template>
  <section class="with-header-and-content">
    <div class="section-header">
      <h2 class="table-title">{{ tableId }}</h2>
      <button @click="load" :disabled="loading">Reload</button>
    </div>
    <div v-if="error" class="error">{{ error }}</div>
    <div v-else-if="loading">Loading…</div>
    <div v-else-if="items.length === 0" class="empty-state">
      <div class="empty-state-content">
        <div class="empty-state-icon">📋</div>
        <h3>No items in this table</h3>
        <p>This table is empty. Get started by adding your first item.</p>
        <router-link class="button primary" :to="`/table/${props.tableId}/insert-row`">
          ➕ Insert First Item
        </router-link>
        <div class="empty-state-actions">
          <router-link class="button secondary" :to="`/table/${props.tableId}/add-column`">
            Add Column
          </router-link>
          <router-link class="button secondary" :to="`/table/${props.tableId}/calendar`">
            Calendar View
          </router-link>
        </div>
      </div>
    </div>
    <div v-else class="section-content">
      <div role="toolbar" class = "padding" v-if = "tableStructure">
        <router-link class="button" :to="`/table/${props.tableId}/insert-row`">{{ tableStructure.CreateButtonText ?? 'Insert row' }}</router-link>
        <router-link class="button" :to="`/table/${props.tableId}/add-column`">Add column</router-link>
        <router-link class="button" :to="`/table/${props.tableId}/calendar`">Calendar view</router-link>
        <ColumnVisibilityDropdown :columns="columns" v-model="selectedColumns" />

        <!-- Selection controls -->
        <div v-if="pagedItems.length > 0" class="selection-controls">
          <button @click="selectAll" class="button small">Select All</button>
          <button @click="selectNone" class="button small">Select None</button>
          <button
            v-if="hasSelectedItems"
            @click="confirmDeleteSelected"
            class="button small delete-button"
            :disabled="deleting"
          >
            🗑️ Delete Selected ({{ selectedKeys.size }})
          </button>
        </div>
      </div>
      <table class="table row-hover">
        <thead>
          <tr>
            <th></th>
            <th v-for="col in visibleColumns" :key="col" @click="toggleSort(col)">
              {{ col }}<span v-if="sortBy === col"> {{ sortDir === 'asc' ? '▲' : '▼' }}</span>
            </th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="it in pagedItems" :key="String((it as any).id ?? Math.random())"
            :class="{ selected: isSelected(it) }">
            <td>
              <input type="checkbox" :checked="isSelected(it)" @change="(e) => toggleSelected(it, e)" />
            </td>
            <td v-for="col in visibleColumns" :key="col">
              <!-- Inline editing input -->
              <div v-if="isEditing(it, col)" class="inline-edit">
                <!-- Checkbox for tinyint columns -->
                <input
                  v-if="isTinyintColumn(col)"
                  type="checkbox"
                  :checked="getBooleanValue(it, col)"
                  @change="(e) => { editingValue = (e.target as HTMLInputElement).checked ? '1' : '0'; saveEdit(it); }"
                  :disabled="saving"
                  class="edit-checkbox"
                />
                <!-- Text input for other columns -->
                <input
                  v-else
                  v-model="editingValue"
                  @keyup.enter="saveEdit(it)"
                  @keyup.escape="cancelEdit"
                  @blur="saveEdit(it)"
                  :disabled="saving"
                  class="edit-input"
                  ref="editInput"
                />
              </div>
              <!-- Display values -->
              <div v-else @click="startEdit(it, col)" class="cell-content" :class="{ 'editable': col !== 'id' && col !== 'created_at_unix' }">
                <span v-if="col === 'created_at_unix' && getItemValue(it, col) != null">{{ new Date(Number(getItemValue(it, col)) *
                  1000).toLocaleString() }}</span>
                <span v-else-if="col === 'id'">
                  <router-link :to="`/table/${props.tableId}/${getItemValue(it, 'id')}`">{{ getItemValue(it, col) }}</router-link>
                </span>
                <span v-else-if="getItemValue(it, col) == null" class="subtle">NULL</span>
                <span v-else-if="isTinyintColumn(col)" class="boolean-display">
                  <span v-if="getBooleanValue(it, col)" class="boolean-true">✓</span>
                  <span v-else class="boolean-false">✗</span>
                </span>
                <span v-else>{{ getItemValue(it, col) }}</span>
              </div>
            </td>
            <td style = "width: 5%">
              <RowActionsDropdown :table-id="props.tableId" :row-id="getItemValue(it, 'id')" @deleted="load" />
            </td>
          </tr>
        </tbody>
      </table>
	  <div class = "padding">
		  <Pagination :total="total" v-model:page="page" v-model:page-size="pageSize" />
	  </div>
    </div>

    <!-- Bulk Delete Confirmation Dialog -->
    <div v-if="showDeleteConfirm" class="modal-overlay" @click="cancelDeleteSelected">
      <div class="modal-content" @click.stop>
        <h3>Confirm Delete</h3>
        <p>Are you sure you want to delete {{ selectedKeys.size }} selected row(s)? This action cannot be undone.</p>
        <div class="modal-actions">
          <button @click="cancelDeleteSelected" class="button cancel-button" :disabled="deleting">
            Cancel
          </button>
          <button @click="deleteSelectedItems" class="button confirm-delete-button" :disabled="deleting">
            {{ deleting ? 'Deleting...' : `Delete ${selectedKeys.size} Row(s)` }}
          </button>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.cell-content {
  min-height: 1.5em;
  padding: 0.25rem;
  border-radius: 3px;
  transition: background-color 0.2s;
}

.cell-content.editable {
  cursor: pointer;
}

.cell-content.editable:hover {
  background-color: #f8f9fa;
}

.inline-edit {
  padding: 0;
}

.edit-input {
  width: 100%;
  border: 2px solid #007bff;
  border-radius: 3px;
  padding: 0.25rem;
  font-size: inherit;
  background: white;
  outline: none;
}

.edit-input:focus {
  box-shadow: 0 0 0 2px rgba(0, 123, 255, 0.25);
}

.edit-input:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.boolean-display {
  display: flex;
  align-items: center;
  min-height: 1.5em;
}

.boolean-true {
  color: #28a745;
  font-weight: bold;
  font-size: 1.2em;
}

.boolean-false {
  color: #dc3545;
  font-weight: bold;
  font-size: 1.2em;
}

.edit-checkbox {
  transform: scale(1.2);
  cursor: pointer;
}

.edit-checkbox:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.table-title {
  margin: 0;
}

.error {
  color: #b00020;
}

.table {
  width: 100%;
  border-collapse: collapse;
}

.table thead th {
  text-align: left;
  border-bottom: 1px solid #ddd;
  padding: .5rem;
}

.table thead th {
  cursor: pointer;
  transition: color .15s ease-in-out;
  background-color: #fff;
}

.table thead th:hover {
  color: #0366d6;
}

.table tbody td {
  border-bottom: 1px solid #eee;
  padding: .5rem;
}

.no-items {
  padding: .75rem;
  color: #666;
}

.selected {
  background: #f0f7ff;
}

.dropdown-menu {
  position: absolute;
  z-index: 10;
  background: #fff;
  border: 1px solid #ddd;
  padding: .5rem;
  box-shadow: 0 2px 6px rgba(0, 0, 0, .08);
  min-width: 200px;
}

/* Selection controls */
.selection-controls {
  display: flex;
  gap: 0.5rem;
  align-items: center;
  margin-left: auto;
  flex-wrap: wrap;
}

.button.small {
  padding: 0.25rem 0.5rem;
  font-size: 0.8rem;
  border-radius: 3px;
}

.delete-button {
  background: #dc3545;
  color: white;
  border: none;
  cursor: pointer;
  transition: background-color 0.2s;
}

.delete-button:hover:not(:disabled) {
  background: #c82333;
}

.delete-button:disabled {
  background: #6c757d;
  cursor: not-allowed;
  opacity: 0.6;
}

/* Modal Dialog Styles */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal-content {
  background: white;
  border-radius: 8px;
  padding: 2rem;
  max-width: 400px;
  width: 90%;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.3);
}

.modal-content h3 {
  margin: 0 0 1rem 0;
  color: #dc3545;
  font-size: 1.25rem;
}

.modal-content p {
  margin: 0 0 1.5rem 0;
  color: #666;
  line-height: 1.5;
}

.modal-actions {
  display: flex;
  gap: 1rem;
  justify-content: flex-end;
}

.cancel-button {
  background: #6c757d;
  color: white;
  border: none;
  padding: 0.5rem 1rem;
  border-radius: 4px;
  font-size: 0.9rem;
  font-weight: 500;
  cursor: pointer;
  transition: background-color 0.2s;
}

.cancel-button:hover:not(:disabled) {
  background: #545b62;
}

.cancel-button:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.confirm-delete-button {
  background: #dc3545;
  color: white;
  border: none;
  padding: 0.5rem 1rem;
  border-radius: 4px;
  font-size: 0.9rem;
  font-weight: 500;
  cursor: pointer;
  transition: background-color 0.2s;
}

.confirm-delete-button:hover:not(:disabled) {
  background: #c82333;
}

.confirm-delete-button:disabled {
  background: #6c757d;
  cursor: not-allowed;
  opacity: 0.6;
}

/* Empty State Styles */
.empty-state {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 400px;
  padding: 2rem;
}

.empty-state-content {
  text-align: center;
  max-width: 500px;
}

.empty-state-icon {
  font-size: 4rem;
  margin-bottom: 1rem;
  opacity: 0.6;
}

.empty-state-content h3 {
  margin: 0 0 0.5rem 0;
  color: #333;
  font-size: 1.5rem;
  font-weight: 600;
}

.empty-state-content p {
  margin: 0 0 2rem 0;
  color: #666;
  font-size: 1rem;
  line-height: 1.5;
}

.empty-state-actions {
  display: flex;
  gap: 1rem;
  justify-content: center;
  flex-wrap: wrap;
  margin-top: 1.5rem;
}

.button.primary {
  background: #007bff;
  color: white;
  text-decoration: none;
  padding: 0.75rem 1.5rem;
  border-radius: 6px;
  font-size: 1rem;
  font-weight: 500;
  transition: all 0.2s ease;
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
}

.button.primary:hover {
  background: #0056b3;
  color: white;
  text-decoration: none;
  transform: translateY(-1px);
  box-shadow: 0 4px 8px rgba(0, 123, 255, 0.3);
}

.button.secondary {
  background: #6c757d;
  color: white;
  text-decoration: none;
  padding: 0.5rem 1rem;
  border-radius: 4px;
  font-size: 0.9rem;
  font-weight: 500;
  transition: all 0.2s ease;
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
}

.button.secondary:hover {
  background: #545b62;
  color: white;
  text-decoration: none;
  transform: translateY(-1px);
  box-shadow: 0 2px 4px rgba(108, 117, 125, 0.3);
}

.button:active {
  transform: translateY(0);
}

/* Responsive design for selection controls */
@media (max-width: 768px) {
  .selection-controls {
    margin-left: 0;
    margin-top: 0.5rem;
    width: 100%;
    justify-content: flex-start;
  }

  .modal-actions {
    flex-direction: column;
  }

  .modal-actions .button {
    width: 100%;
  }

  .empty-state {
    min-height: 300px;
    padding: 1rem;
  }

  .empty-state-icon {
    font-size: 3rem;
  }

  .empty-state-content h3 {
    font-size: 1.25rem;
  }

  .empty-state-actions {
    flex-direction: column;
    align-items: center;
  }

  .empty-state-actions .button {
    width: 100%;
    max-width: 250px;
  }
}
</style>
