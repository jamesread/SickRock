<script setup lang="ts">
import { onMounted, ref, computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import Pagination from 'picocrank/vue/components/Pagination.vue'
import ColumnVisibilityDropdown from './ColumnVisibilityDropdown.vue'
import RowActionsDropdown from './RowActionsDropdown.vue'
import { createConnectTransport } from '@connectrpc/connect-web'
import { createClient } from '@connectrpc/connect'
import { SickRock } from '../gen/sickrock_pb'
import InsertRow from './InsertRow.vue'
import { GetTableStructureResponse } from '../gen/sickrock_pb'
import Section from 'picocrank/vue/components/Section.vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { ViewIcon, Edit03Icon, CheckListIcon, RefreshIcon } from '@hugeicons/core-free-icons'

const router = useRouter()
const tableStructure = ref<GetTableStructureResponse | null>(null)

// Foreign key lookup state
const foreignKeys = ref<Array<{
  constraintName: string
  tableName: string
  columnName: string
  referencedTable: string
  referencedColumn: string
  onDeleteAction: string
  onUpdateAction: string
}>>([])

const referencedTableData = ref<Record<string, any[]>>({})
const loadingForeignKeys = ref(false)

const props = defineProps<{
  tableId: string;
  fields?: Array<{ name: string; type: string }>;
  createButtonText?: string;
  items?: any[];
  showToolbar?: boolean;
  showPagination?: boolean;
  title?: string;
}>()

const emit = defineEmits<{
  'view-created': []
}>()

// View management state
const tableViews = ref<Array<{ id: number; tableName: string; viewName: string; isDefault: boolean; columns: Array<{ columnName: string; isVisible: boolean; columnOrder: number; sortOrder: string }> }>>([])
const selectedViewId = ref<number | null>(null)

// Computed property for the current view
const currentView = computed(() => {
  return tableViews.value.find(view => view.id === selectedViewId.value) || null
})
// Find default view for this table, if any
const defaultView = computed(() => tableViews.value.find(v => v.isDefault) || null)

// Computed property for the section title
const sectionTitle = computed(() => {
  return props.title || `Table: ${props.tableId}`
})

// Computed property for view options (including default)
const viewOptions = computed(() => {
  const options = [...tableViews.value]
  // Add a default option if no views exist or if we want to show "All Columns"
  if (options.length === 0 || !options.some(v => v.isDefault)) {
    options.unshift({
      id: -1,
      tableName: props.tableId,
      viewName: 'All Columns',
      isDefault: true,
      columns: []
    })
  }
  return options
})

type Item = Record<string, unknown>

const items = ref<Item[]>([])
const loading = ref(false)
const error = ref<string | null>(null)

const localFields = ref<string[]>([])
const localFieldDefs = ref<Array<{ name: string; type: string; required: boolean }>>([])

// Watch for changes in currentView to update column configuration
watch(
  () => currentView.value,
  (view) => {
    if (view && view.columns.length > 0) {
      // Use view configuration - include all visible columns as specified in the view
      console.log('[Table] Applying view columns:', view.viewName, view.columns
        .slice()
        .sort((a, b) => a.columnOrder - b.columnOrder)
        .map(c => ({ column: c.columnName, order: c.columnOrder, isVisible: c.isVisible, sort: c.sortOrder }))
      )
      const visibleColumns = view.columns
        .filter(col => col.isVisible)
        .sort((a, b) => a.columnOrder - b.columnOrder)
        .map(col => col.columnName)

      localFields.value = visibleColumns

      // Apply initial sort from view if provided
      const sortColumn = view.columns.find(c => c.sortOrder === 'asc' || c.sortOrder === 'desc')
      if (sortColumn) {
        sortBy.value = sortColumn.columnName
        sortDir.value = (sortColumn.sortOrder === 'desc' ? 'desc' : 'asc')
        console.log('[Table] Applied initial sort from view:', sortBy.value, sortDir.value)
      } else {
        // No sort specified by the view; leave any existing sort as-is
        console.log('[Table] View provides no initial sort order')
      }
    } else if (props.fields && props.fields.length) {
      // Fallback to all fields (default view) - include sr_created in default view
      localFields.value = props.fields.map(x => x.name)
    }
  },
  { immediate: true }
)

watch(
  () => props.fields,
  (f) => {
    if (f && f.length) {
      localFieldDefs.value = f.map(field => ({
        name: field.name,
        type: field.type,
        required: false
      }))

      // Only update localFields if no view is active
      if (!currentView.value || currentView.value.columns.length === 0) {
        localFields.value = f.map(x => x.name)
      }
    }
  },
  { immediate: true }
)
async function loadStructure() {
  tableStructure.value = await client.getTableStructure({ pageId: props.tableId })
  const defs = (tableStructure.value.fields ?? []).map(f => ({ name: f.name, type: f.type, required: !!f.required }))
  const names = defs.map(d => d.name)
  if (names.length) {
    localFieldDefs.value = defs

    // Only set localFields if no view is active
    if (!currentView.value || currentView.value.columns.length === 0) {
      localFields.value = names
      selectedColumns.value = [...names]
    }
  }
}

async function loadTableViews() {
  try {
    const response = await client.getTableViews({ tableName: props.tableId })
    tableViews.value = response.views.map(view => ({
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

    // Select the default view or first view
    const defaultView = tableViews.value.find(v => v.isDefault)
    if (defaultView) {
      selectedViewId.value = defaultView.id
    } else if (tableViews.value.length > 0) {
      selectedViewId.value = tableViews.value[0].id
    } else {
      // No views exist, use the default "All Columns" view
      selectedViewId.value = -1
    }
  } catch (error) {
    console.error('Failed to load table views:', error)
    // Fallback to default view
    selectedViewId.value = -1
  }
}

function onViewChange() {
  // This will trigger reactivity in the column configuration
  const view = currentView.value
  console.log('[Table] View changed to:', selectedViewId.value, view?.viewName)
  if (view) {
    const ordered = view.columns
      .slice()
      .sort((a, b) => a.columnOrder - b.columnOrder)
      .map(c => ({ column: c.columnName, order: c.columnOrder, isVisible: c.isVisible, sort: c.sortOrder }))
    console.log('[Table] View column order:', ordered)

    // Apply sort from the selected view on change
    const sortColumn = view.columns.find(c => c.sortOrder === 'asc' || c.sortOrder === 'desc')
    if (sortColumn) {
      sortBy.value = sortColumn.columnName
      sortDir.value = (sortColumn.sortOrder === 'desc' ? 'desc' : 'asc')
      console.log('[Table] Applied sort from changed view:', sortBy.value, sortDir.value)
    }
  }
}

function createTableView() {
  router.push({ name: 'create-table-view', params: { tableName: props.tableId } })
}

function editTableView() {
  if (currentView.value && currentView.value.id !== -1) {
    router.push({
      name: 'edit-table-view',
      params: {
        tableName: props.tableId,
        viewId: currentView.value.id.toString()
      }
    })
  }
}
const columns = computed(() => localFields.value.length ? localFields.value : ['id'])
const selectedColumns = ref<string[]>([])
watch(columns, (cols) => { selectedColumns.value = [...cols] }, { immediate: true })
const visibleColumns = computed(() =>
  columns.value
    .filter(c => selectedColumns.value.includes(c))
    .filter(c => getColumnType(c) !== 'unknown')
)

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
  const source = (props.items || items.value)
  if (!col) return source
  const dir = sortDir.value === 'asc' ? 1 : -1
  return [...source].sort((a, b) => {
    const avRaw = getItemValue(a, col)
    const bvRaw = getItemValue(b, col)
    if (avRaw == null && bvRaw == null) return 0
    if (avRaw == null) return 1
    if (bvRaw == null) return -1
    // Normalize values for comparison
    const av = typeof avRaw === 'bigint' ? Number(avRaw) : avRaw
    const bv = typeof bvRaw === 'bigint' ? Number(bvRaw) : bvRaw
    // Datetime: attempt numeric comparison if both numbers or parseable dates
    if (isDatetimeColumn(col)) {
      const an = typeof av === 'number' ? av : Date.parse(String(av))
      const bn = typeof bv === 'number' ? bv : Date.parse(String(bv))
      if (!isNaN(an) && !isNaN(bn)) return (an - bn) * dir
    }
    if (typeof av === 'number' && typeof bv === 'number') return (av - bv) * dir
    const as = String(av)
    const bs = String(bv)
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

// Use passed items or load from API
const displayItems = computed(() => {
  return props.items || items.value
})

// Show toolbar and pagination based on props
const showToolbar = computed(() => {
  return props.showToolbar !== false // Default to true unless explicitly false
})

const showPagination = computed(() => {
  return props.showPagination !== false // Default to true unless explicitly false
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
  // Check standard fields first (only id and sr_created are static now)
  if (column === 'id') {
    return item[column]
  }
  if (column === 'sr_created') {
    // The protobuf field sr_created becomes srCreated in TypeScript
    return item.srCreated
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

// Load foreign key information for the current table
async function loadForeignKeys() {
  try {
    loadingForeignKeys.value = true
    const response = await client.getForeignKeys({ tableName: props.tableId })
    foreignKeys.value = response.foreignKeys.map(fk => ({
      constraintName: fk.constraintName,
      tableName: fk.tableName,
      columnName: fk.columnName,
      referencedTable: fk.referencedTable,
      referencedColumn: fk.referencedColumn,
      onDeleteAction: fk.onDeleteAction,
      onUpdateAction: fk.onUpdateAction
    }))

    // Load referenced table data for each foreign key
    await loadReferencedTableData()
  } catch (err) {
    console.error('Error loading foreign keys:', err)
  } finally {
    loadingForeignKeys.value = false
  }
}

// Load data from referenced tables
async function loadReferencedTableData() {
  const data: Record<string, any[]> = {}

  for (const fk of foreignKeys.value) {
    try {
      const response = await client.listItems({ pageId: fk.referencedTable })
      data[fk.columnName] = response.items || []
    } catch (err) {
      console.error(`Error loading data for table ${fk.referencedTable}:`, err)
      data[fk.columnName] = []
    }
  }

  referencedTableData.value = data
}

// Check if a column is a foreign key
function isForeignKey(columnName: string): boolean {
  return foreignKeys.value.some(fk => fk.columnName === columnName)
}

// Get the foreign key info for a column
function getForeignKeyInfo(columnName: string) {
  return foreignKeys.value.find(fk => fk.columnName === columnName)
}

// Get the name field from a referenced item
function getReferencedItemName(item: any): string {
  if (!item) {
    return 'Unknown'
  }
  if (item.name) {
    return item.name
  }
  if (item.additionalFields && item.additionalFields.name) {
    return item.additionalFields.name
  }
  return `ID: ${item.id}`
}

// Get the referenced item for a foreign key value
function getReferencedItem(columnName: string, foreignKeyValue: any) {
  const fkInfo = getForeignKeyInfo(columnName)
  if (!fkInfo) {
    console.log(`No foreign key info found for column: ${columnName}`)
    return null
  }

  const referencedItems = referencedTableData.value[columnName] || []
  const foundItem = referencedItems.find(item => String(item.id) === String(foreignKeyValue))

  if (!foundItem) {
    console.log(`Referenced item not found for column: ${columnName}, value: ${foreignKeyValue}, available items:`, referencedItems.map(item => ({ id: item.id, name: item.name || item.additionalFields?.name })))
  }

  return foundItem
}

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
  // Don't allow editing of id, name, sr_created, or foreign key columns
  if (column === 'id' || column === 'name' || column === 'sr_created' || isForeignKey(column)) {
    return
  }

  const currentValue = getItemValue(item, column)
  editingCell.value = { rowId: String(item.id), column }

  // Handle datetime fields - convert ISO8601 to datetime-local format
  if (isDatetimeColumn(column) && currentValue != null) {
    try {
      const date = new Date(currentValue)
      if (!isNaN(date.getTime())) {
        // Convert to YYYY-MM-DDTHH:MM format for datetime-local input
        editingValue.value = date.toISOString().slice(0, 16)
      } else {
        editingValue.value = ''
      }
    } catch {
      editingValue.value = ''
    }
  } else {
    editingValue.value = currentValue == null ? '' : String(currentValue)
  }

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
      if (isDatetimeColumn(column)) {
        // Convert datetime-local input to MySQL datetime format
        try {
          const date = new Date(editingValue.value)
          if (!isNaN(date.getTime())) {
            // Convert to MySQL datetime format: YYYY-MM-DD HH:MM:SS
            const year = date.getFullYear()
            const month = String(date.getMonth() + 1).padStart(2, '0')
            const day = String(date.getDate()).padStart(2, '0')
            const hours = String(date.getHours()).padStart(2, '0')
            const minutes = String(date.getMinutes()).padStart(2, '0')
            const seconds = String(date.getSeconds()).padStart(2, '0')
            additionalFields[column] = `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`
          } else {
            additionalFields[column] = editingValue.value
          }
        } catch {
          additionalFields[column] = editingValue.value
        }
      } else {
        additionalFields[column] = editingValue.value
      }

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

// Helper function to check if a column is a datetime column
function isDatetimeColumn(column: string): boolean {
  const field = localFieldDefs.value.find(f => f.name === column)
  return field?.type === 'datetime'
}

// Helper function to get the SQL datatype for a column
function getColumnType(column: string): string {
  const field = localFieldDefs.value.find(f => f.name === column)
  if (field) {
    return field.type
  }

  // Fallback for standard columns
  if (column === 'id') return 'string'
  if (column === 'sr_created') return 'datetime'

  return 'unknown'
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
const isAllSelected = computed(() => pagedItems.value.length > 0 && selectedKeys.value.size === pagedItems.value.length)
const isIndeterminate = computed(() => selectedKeys.value.size > 0 && selectedKeys.value.size < pagedItems.value.length)

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

function toggleSelectAll() {
  if (isAllSelected.value) {
    selectNone()
  } else {
    selectAll()
  }
}

onMounted(load)
onMounted(loadStructure)
onMounted(loadTableViews)
onMounted(loadForeignKeys)
</script>

<template>
  <Section :title="sectionTitle" :padding="false">
    <template v-if="showToolbar" #toolbar>
      <div class="toolbar-group">
        <div class="view-selector">
          <label for="view-select">View:</label>
          <select
            id="view-select"
            v-model="selectedViewId"
            @change="onViewChange"
            class="view-dropdown"
          >
            <option
              v-for="view in viewOptions"
              :key="view.id"
              :value="view.id"
            >
              {{ view.viewName }}
            </option>
          </select>
        </div>
        <button
          v-if="currentView && currentView.id !== -1"
          @click="editTableView"
          class="button neutral"
        >
          <HugeiconsIcon :icon="Edit03Icon" />
          Edit View
        </button>
        <button @click="createTableView" class="button neutral">
          <HugeiconsIcon :icon="ViewIcon" />
          Create View
        </button>
        <router-link :to="`/table/${props.tableId}/column-types`" class="button neutral">Structure</router-link>
        <router-link :to="`/table/${props.tableId}/insert-row`" class="button neutral">
          {{ tableStructure?.CreateButtonText ?? 'Insert row' }}
        </router-link>
      </div>
    </template>
    <div v-if="error" class="error">{{ error }}</div>
    <div v-else-if="loading">Loading…</div>
    <div v-else-if="items.length === 0" class="empty-state">
      <div class="empty-state-content">
        <div class="empty-state-icon">📋</div>
        <h3>No items in this table</h3>
        <p>This table is empty. Get started by adding your first item.</p>
        <router-link class="button" :to="`/table/${props.tableId}/insert-row`">
          ➕ Insert First Item
        </router-link>
        <div class="empty-state-actions">
          <router-link class="button" :to="`/table/${props.tableId}/add-column`">
            Add Column
          </router-link>
        </div>
      </div>
    </div>
    <div v-else class="section-content">
      <!-- Selection controls -->
      <div v-if="pagedItems.length > 0 && hasSelectedItems" class="selection-controls padding">
        <button
          @click="confirmDeleteSelected"
          class="button delete-button"
          :disabled="deleting"
        >
          🗑️ Delete Selected ({{ selectedKeys.size }})
        </button>
      </div>
      <table class="table row-hover">
        <thead>
          <tr>
            <th class="small">
              <input
                type="checkbox"
                :checked="isAllSelected"
                :indeterminate="isIndeterminate"
                @change="toggleSelectAll"
              />
            </th>
            <th v-for="col in visibleColumns" :key="col" @click="toggleSort(col)" :title="getColumnType(col)" :class="{ small: col === 'id' }">
              {{ col }}<span v-if="sortBy === col"> {{ sortDir === 'asc' ? '▲' : '▼' }}</span>
            </th>
            <th>
              <ColumnVisibilityDropdown :columns="columns" v-model="selectedColumns" />
            </th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="it in pagedItems" :key="String((it as any).id ?? Math.random())"
            :class="{ selected: isSelected(it) }">
            <td class = "small">
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
                <!-- Datetime input for datetime columns -->
                <input
                  v-else-if="isDatetimeColumn(col)"
                  type="datetime-local"
                  v-model="editingValue"
                  @keyup.enter="saveEdit(it)"
                  @keyup.escape="cancelEdit"
                  @blur="saveEdit(it)"
                  :disabled="saving"
                  class="edit-input"
                  ref="editInput"
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
              <div v-else @click="startEdit(it, col)" class="cell-content" :class="{ 'editable': col !== 'id' && col !== 'name' && col !== 'sr_created' && !isForeignKey(col) }">
                <span v-if="col === 'sr_created' && getItemValue(it, col) != null">{{ new Date(Number(getItemValue(it, col)) * 1000).toLocaleString() }}</span>
                <span v-else-if="col === 'id'">
                  <router-link :to="`/table/${props.tableId}/${getItemValue(it, 'id')}`">{{ getItemValue(it, col) }}</router-link>
                </span>
                <span v-else-if="col === 'name'">
                  <router-link :to="`/table/${props.tableId}/${getItemValue(it, 'id')}`">{{ getItemValue(it, col) }}</router-link>
                </span>
                <span v-else-if="isForeignKey(col) && getItemValue(it, col) != null">
                  <template v-if="getReferencedItem(col, getItemValue(it, col))">
                    <router-link :to="`/table/${getForeignKeyInfo(col)?.referencedTable}/${getItemValue(it, col)}`">
                      {{ getReferencedItemName(getReferencedItem(col, getItemValue(it, col))) }}
                    </router-link>
                  </template>
                  <template v-else>
                    {{ getItemValue(it, col) }}
                  </template>
                </span>
                <span v-else-if="getItemValue(it, col) == null" class="subtle">NULL</span>
                <span v-else-if="isTinyintColumn(col)" class="boolean-display">
                  <span v-if="getBooleanValue(it, col)" class="boolean-true">✓</span>
                  <span v-else class="boolean-false">✗</span>
                </span>
                <span v-else-if="isDatetimeColumn(col) && getItemValue(it, col) != null">
                  {{ new Date(getItemValue(it, col)).toLocaleString() }}
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
	  <div v-if="showPagination" class = "padding">
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

.section-header .button {
  background: #fff;
}

.section-header .button:hover {
  background: #c9ccd4;
}

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

.button:active {
  transform: translateY(0);
}

.small {
  width: 1rem;
}

.small input {
  width: 1rem;
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

.insert-toolbar {
  display: flex;
  align-items: center;
}

.fg1 { flex-grow: 1 }

</style>
