<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'

import { createApiClient } from '../stores/api'
import { SickRock } from '../gen/sickrock_pb'

const props = defineProps<{
  tableId: string,
  fieldDefs: Array<{ name: string; type: string; required: boolean }>,
  selectedDate?: string | null,
  // Edit mode props
  editMode?: boolean,
  itemId?: string,
  existingItem?: Record<string, unknown> | null,
  initialValues?: Record<string, string> | null
}>()
const emit = defineEmits<{
  created: [],
  updated: [],
  cancelled: []
}>()


const form = ref<Record<string, any>>({})
const loading = ref(false)
const error = ref<string | null>(null)

// Edit mode state
const isEditMode = computed(() => props.editMode || false)
const saving = ref(false)

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
const initialApplied = ref(false)
// Queue for FK values that should apply once FK options are loaded
const pendingFkInitials = ref<Record<string, string>>({})

// Search state for each foreign key field
const searchQueries = ref<Record<string, string>>({})
const filteredData = ref<Record<string, any[]>>({})
const showDropdowns = ref<Record<string, boolean>>({})

// Watch for changes to selectedDate prop and update form
watch(() => props.selectedDate, (newDate) => {
  if (newDate) {
    try {
      const date = new Date(newDate)
      if (!isNaN(date.getTime())) {
        // Convert to datetime-local format (YYYY-MM-DDTHH:MM)
        const year = date.getFullYear()
        const month = String(date.getMonth() + 1).padStart(2, '0')
        const day = String(date.getDate()).padStart(2, '0')
        const hours = String(date.getHours()).padStart(2, '0')
        const minutes = String(date.getMinutes()).padStart(2, '0')
        const datetimeLocalValue = `${year}-${month}-${day}T${hours}:${minutes}`

        // Check if there's a 'starts' field in the field definitions
        const startsField = props.fieldDefs.find(f => f.name === 'starts' && f.type === 'datetime')
        if (startsField) {
          // Use the 'starts' field if it exists and is datetime type
          form.value.starts = datetimeLocalValue
        } else {
          // Fallback to sr_created for backward compatibility
          form.value.sr_created = Math.floor(date.getTime() / 1000)
        }
      }
    } catch {
      // Ignore invalid dates
    }
  }
}, { immediate: true })


// Watch for changes to any datetime field and update unix timestamp
watch(() => {
  const datetimeFields: Record<string, any> = {}
  for (const field of props.fieldDefs) {
    if ((field.type === 'datetime' || field.type === 'timestamp') && form.value[field.name]) {
      datetimeFields[field.name] = form.value[field.name]
    }
  }
  return datetimeFields
}, (newDateTimeFields) => {
  for (const [fieldName, value] of Object.entries(newDateTimeFields)) {
    if (value) {
      try {
        const date = new Date(value)
        if (!isNaN(date.getTime())) {
          // Store the unix timestamp for this datetime field
          form.value[`${fieldName}_unix`] = Math.floor(date.getTime() / 1000)
        }
      } catch {
        // Ignore invalid dates
      }
    }
  }
}, { deep: true })

// Format the selected date for display
const formattedDate = computed(() => {
  if (!props.selectedDate) return null
  try {
    const date = new Date(props.selectedDate)
    return date.toLocaleDateString('en-US', {
      weekday: 'long',
      year: 'numeric',
      month: 'long',
      day: 'numeric'
    })
  } catch {
    return null
  }
})

// Transport handled by authenticated client
const client = createApiClient()

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
      const response = await client.listItems({ tcName: fk.referencedTable })
      data[fk.columnName] = response.items || []
    } catch (err) {
      console.error(`Error loading data for table ${fk.referencedTable}:`, err)
      data[fk.columnName] = []
    }
  }

  referencedTableData.value = data
}

// Check if a field is a foreign key
function isForeignKey(fieldName: string): boolean {
  return foreignKeys.value.some(fk => fk.columnName === fieldName)
}

// Get the foreign key info for a field
function getForeignKeyInfo(fieldName: string) {
  return foreignKeys.value.find(fk => fk.columnName === fieldName)
}

// Get display text for a referenced table item
function getDisplayText(item: any, fkInfo: any): string {
  // Check if the item has a name field at top level
  if (item.name) {
    return `${item.name} (ID: ${item.id})`
  }

  // Check if the item has a name field in additionalFields
  if (item.additionalFields && item.additionalFields.name) {
    return `${item.additionalFields.name} (ID: ${item.id})`
  }

  // Fallback to other meaningful display fields at top level
  const displayFields = ['title', 'label', 'description']
  for (const field of displayFields) {
    if (item[field]) {
      return `${item[field]} (ID: ${item.id})`
    }
  }

  // Fallback to other meaningful display fields in additionalFields
  for (const field of displayFields) {
    if (item.additionalFields && item.additionalFields[field]) {
      return `${item.additionalFields[field]} (ID: ${item.id})`
    }
  }

  // Final fallback to just the ID
  return `ID: ${item.id}`
}

// Get the name field from an item (checks both top level and additionalFields)
function getItemName(item: any): string {
  if (item.name) {
    return item.name
  }
  if (item.additionalFields && item.additionalFields.name) {
    return item.additionalFields.name
  }
  return ''
}

// Filter items based on search query
function filterItems(fieldName: string, query: string) {
  const allItems = referencedTableData.value[fieldName] || []
  if (!query.trim()) {
    filteredData.value[fieldName] = allItems
    return
  }

  const searchTerm = query.toLowerCase()
  filteredData.value[fieldName] = allItems.filter(item => {
    const name = getItemName(item).toLowerCase()
    return name.includes(searchTerm)
  })
}

// Handle search input
function onSearchInput(fieldName: string, query: string) {
  searchQueries.value[fieldName] = query
  filterItems(fieldName, query)
}

// Handle dropdown toggle
function toggleDropdown(fieldName: string) {
  showDropdowns.value[fieldName] = !showDropdowns.value[fieldName]
  if (showDropdowns.value[fieldName]) {
    // Initialize filtered data when opening dropdown
    filterItems(fieldName, searchQueries.value[fieldName] || '')
  }
}

// Handle item selection
function selectItem(fieldName: string, item: any) {
  form.value[fieldName] = item.id
  searchQueries.value[fieldName] = getItemName(item)
  showDropdowns.value[fieldName] = false
}

// Clear selection
function clearSelection(fieldName: string) {
  form.value[fieldName] = ''
  searchQueries.value[fieldName] = ''
  showDropdowns.value[fieldName] = false
}

// Handle blur with delay
function handleBlur(fieldName: string) {
  setTimeout(() => {
    showDropdowns.value[fieldName] = false
  }, 200)
}

// Initialize form with existing data for edit mode
function initializeFormWithExistingData() {
  if (!isEditMode.value || !props.existingItem) return

  const initialData: Record<string, any> = {}

  // Handle standard fields
  if (props.existingItem.name !== undefined) {
    initialData.name = String(props.existingItem.name)
  }

  // Handle additional fields
  if (props.existingItem.additionalFields) {
    Object.entries(props.existingItem.additionalFields).forEach(([key, value]) => {
      const fieldDef = props.fieldDefs.find(f => f.name === key)
      if (fieldDef && isDatetimeField(fieldDef.type)) {
        // Convert ISO8601 to datetime-local format for datetime fields
        initialData[key] = isoToDatetimeLocal(String(value))
      } else {
        initialData[key] = String(value)
      }

      // Handle foreign key display text
      if (isForeignKey(key)) {
        const list = referencedTableData.value[key] || []
        const found = list.find((it: any) => String(it.id) === String(value))
        if (found) {
          const display = (found.name) ? found.name : (found.additionalFields && (found.additionalFields.name || found.additionalFields.title))
          searchQueries.value[key] = display ? String(display) : String(value)
        } else if (!searchQueries.value[key]) {
          searchQueries.value[key] = String(value)
        }
      }
    })
  }

  form.value = initialData
}

// Helper function to check if a field is datetime type
function isDatetimeField(fieldType: string): boolean {
  return fieldType === 'datetime' || fieldType === 'timestamp'
}

// Helper function to convert ISO8601 to datetime-local format
function isoToDatetimeLocal(isoString: string): string {
  try {
    const date = new Date(isoString)
    if (!isNaN(date.getTime())) {
      return date.toISOString().slice(0, 16)
    }
  } catch {
    // Invalid date
  }
  return ''
}

// Set default values for datetime fields when not in edit mode
function setDefaultDatetimeValues() {
  if (isEditMode.value) return // Don't set defaults in edit mode

  for (const field of props.fieldDefs) {
    if ((field.type === 'datetime' || field.type === 'timestamp') && !form.value[field.name]) {
      // Set current timestamp as default value in datetime-local format
      const now = new Date()
      const year = now.getFullYear()
      const month = String(now.getMonth() + 1).padStart(2, '0')
      const day = String(now.getDate()).padStart(2, '0')
      const hours = String(now.getHours()).padStart(2, '0')
      const minutes = String(now.getMinutes()).padStart(2, '0')
      const datetimeLocalValue = `${year}-${month}-${day}T${hours}:${minutes}`

      form.value[field.name] = datetimeLocalValue
    }
  }
}

// Load foreign keys when component is mounted
onMounted(() => {
  loadForeignKeys()
  initializeFormWithExistingData()
  setDefaultDatetimeValues()
  // Apply initialValues for create mode
  if (!isEditMode.value && props.initialValues) {
    // Defer applying FK display names until FK data is loaded
    applyInitialValues()
  }
})

function applyInitialValues() {
  if (isEditMode.value || !props.initialValues) return
  let appliedAny = false
  for (const [k, v] of Object.entries(props.initialValues)) {
    // Only set if not already filled by user or prior application
    const current = form.value[k]
    if (current == null || current === '') {
      if (isForeignKey(k)) {
        // If FK options for this field aren't loaded yet, queue it
        const options = referencedTableData.value[k]
        if (!options || options.length === 0 || loadingForeignKeys.value) {
          pendingFkInitials.value[k] = String(v)
        } else {
          form.value[k] = v
          appliedAny = true
        }
      } else {
        form.value[k] = v
        appliedAny = true
      }
    }
    if (isForeignKey(k)) {
      // Try to find the referenced item to show its name
      const list = referencedTableData.value[k] || []
      const found = list.find((it: any) => String(it.id) === String(v))
      if (found) {
        const display = (found.name) ? found.name : (found.additionalFields && (found.additionalFields.name || found.additionalFields.title))
        searchQueries.value[k] = display ? String(display) : String(v)
      } else if (!searchQueries.value[k]) {
        searchQueries.value[k] = String(v)
      }
    }
  }
  if (appliedAny) initialApplied.value = true
}

// When FK referenced data finishes loading, attempt to apply initial values for FK display
watch(referencedTableData, () => {
  if (!initialApplied.value && !isEditMode.value && props.initialValues) {
    applyInitialValues()
  }

  // Handle edit mode: update search queries for existing foreign key values
  if (isEditMode.value && props.existingItem && props.existingItem.additionalFields) {
    Object.entries(props.existingItem.additionalFields).forEach(([key, value]) => {
      if (isForeignKey(key) && form.value[key] && !searchQueries.value[key]) {
        const list = referencedTableData.value[key] || []
        const found = list.find((it: any) => String(it.id) === String(value))
        if (found) {
          const display = (found.name) ? found.name : (found.additionalFields && (found.additionalFields.name || found.additionalFields.title))
          searchQueries.value[key] = display ? String(display) : String(value)
        } else {
          searchQueries.value[key] = String(value)
        }
      }
    })
  }

  // Apply queued FK initials when their options arrive
  if (!isEditMode.value && Object.keys(pendingFkInitials.value).length > 0) {
    const queue = { ...pendingFkInitials.value }
    let applied = false
    for (const [field, val] of Object.entries(queue)) {
      const opts = referencedTableData.value[field]
      if (opts && opts.length) {
        if (!form.value[field]) {
          form.value[field] = val
          applied = true
        }
        const found = opts.find((it: any) => String(it.id) === String(val))
        if (found) {
          const display = (found.name) ? found.name : (found.additionalFields && (found.additionalFields.name || found.additionalFields.title))
          searchQueries.value[field] = display ? String(display) : String(val)
        } else if (!searchQueries.value[field]) {
          searchQueries.value[field] = String(val)
        }
        delete pendingFkInitials.value[field]
      }
    }
    if (applied) initialApplied.value = true
  }
})

// Also react to initialValues arriving after mount (race with parent loading)
watch(() => props.initialValues, (val) => {
  if (val && !isEditMode.value && !initialApplied.value) {
    applyInitialValues()
  }
}, { immediate: true })

// Re-apply when field definitions are ready (some parents compute these async)
watch(() => props.fieldDefs, (defs) => {
  if ((defs && defs.length) && props.initialValues && !isEditMode.value && !initialApplied.value) {
    applyInitialValues()
  }
  // Set default datetime values when field definitions are ready
  if (defs && defs.length && !isEditMode.value) {
    setDefaultDatetimeValues()
  }
}, { immediate: true })

async function submit() {
  const payload: Record<string, any> = {}
  for (const f of props.fieldDefs) {
    if (f.name === 'id') continue
    const v = form.value[f.name]
    if (v == null || v === '') continue

    if (f.type === 'datetime' || f.type === 'timestamp') {
      // For datetime fields, convert to MySQL-compatible format (YYYY-MM-DD HH:MM:SS)
      try {
        const date = new Date(v)
        if (!isNaN(date.getTime())) {
          // Convert to MySQL datetime format: YYYY-MM-DD HH:MM:SS
          const year = date.getFullYear()
          const month = String(date.getMonth() + 1).padStart(2, '0')
          const day = String(date.getDate()).padStart(2, '0')
          const hours = String(date.getHours()).padStart(2, '0')
          const minutes = String(date.getMinutes()).padStart(2, '0')
          const seconds = String(date.getSeconds()).padStart(2, '0')
          payload[f.name] = `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`
        }
      } catch {
        // Skip invalid dates
        continue
      }
    } else if (f.type === 'int64' || f.type === 'int' || f.type === 'bigint' || f.type === 'integer') {
      payload[f.name] = String(Number(v))
    } else if (f.type === 'float') {
      payload[f.name] = String(Number(v))
    } else {
      payload[f.name] = String(v)
    }
  }

  loading.value = true
  saving.value = true
  error.value = null

  try {
    if (isEditMode.value) {
      // Edit mode - update existing item
      await client.editItem({
        id: props.itemId!,
        additionalFields: payload,
        pageId: props.tableId
      })
      emit('updated')
    } else {
      // Create mode - create new item
      const createRequest: any = {
        pageId: props.tableId,
        additionalFields: payload
      }
      await client.createItem(createRequest)
      form.value = {}
      emit('created')
    }
  } catch (e) {
    error.value = String(e)
  } finally {
    loading.value = false
    saving.value = false
  }
}

// Cancel function for edit mode
function cancel() {
  emit('cancelled')
}
</script>

<template>
  <div>
    <form @submit.prevent="submit">
      <template v-for="f in fieldDefs" :key="f.name">
        <template v-if="f.name !== 'id'">
            <label :for="'field-' + f.name">{{ f.name }}<span v-if="f.required" aria-label="required" title="required">*</span></label>

            <!-- Foreign key searchable dropdown -->
            <div v-if="isForeignKey(f.name)" class="foreign-key-dropdown">
              <div class="search-input-container">
                <input
                  :id="'field-' + f.name"
                  v-model="searchQueries[f.name]"
                  @input="onSearchInput(f.name, ($event.target as HTMLInputElement).value)"
                  @focus="toggleDropdown(f.name)"
                  @blur="handleBlur(f.name)"
                  :placeholder="`Search ${f.name}...`"
                  :disabled="loadingForeignKeys"
                  class="search-input"
                />
                <button
                  v-if="form[f.name]"
                  @click="clearSelection(f.name)"
                  type="button"
                  class="clear-button"
                  title="Clear selection"
                >
                  ×
                </button>
                <button
                  @click="toggleDropdown(f.name)"
                  type="button"
                  class="dropdown-toggle"
                  :class="{ 'open': showDropdowns[f.name] }"
                >
                  ▼
                </button>
              </div>

              <!-- Dropdown results -->
              <div
                v-if="showDropdowns[f.name]"
                class="dropdown-results"
              >
                <div
                  v-if="(filteredData[f.name] || []).length === 0"
                  class="no-results"
                >
                  {{ searchQueries[f.name] ? 'No results found' : 'No items available' }}
                </div>
                <div
                  v-for="item in filteredData[f.name] || []"
                  :key="item.id"
                  @click="selectItem(f.name, item)"
                  class="dropdown-item"
                  :class="{ 'selected': form[f.name] === item.id }"
                >
                  {{ getDisplayText(item, getForeignKeyInfo(f.name)) }}
                </div>
              </div>
            </div>

            <!-- Regular text input -->
            <input
              v-else-if="f.type === 'string' || f.type === 'varchar' || f.type === 'text'"
              v-model="form[f.name]"
              :placeholder="f.name"
              :id="'field-' + f.name"
              type="text"
              @keyup.enter="submit"
            />
            <!-- Number input -->
            <input
              v-else-if="f.type === 'int64' || f.type === 'int' || f.type === 'bigint' || f.type === 'integer' || f.type === 'float'"
              v-model.number="form[f.name]"
              :placeholder="f.name"
              :id="'field-' + f.name"
              type="number"
              :step="f.type === 'float' ? '0.01' : '1'"
              @keyup.enter="submit"
            />
            <!-- DateTime input -->
            <input
              v-else-if="f.type === 'datetime' || f.type === 'timestamp'"
              v-model="form[f.name]"
              :placeholder="f.name"
              :id="'field-' + f.name"
              type="datetime-local"
              @keyup.enter="submit"
            />
            <!-- Fallback text input -->
            <input
              v-else
              v-model="form[f.name]"
              :placeholder="f.name"
              :id="'field-' + f.name"
              type="text"
              @keyup.enter="submit"
            />
        </template>
      </template>

      <div class="form-actions">
        <button
          v-if="isEditMode"
          type="button"
          @click="cancel"
          :disabled="loading || saving"
        >
          Cancel
        </button>
        <button
          type="submit"
          :disabled="loading || saving"
          class="primary"
        >
          {{ isEditMode ? (saving ? 'Saving...' : 'Save Changes') : (loading ? 'Adding...' : 'Add') }}
        </button>
      </div>
    </form>
    <div v-if="error" class="error">{{ error }}</div>
  </div>
</template>



<style scoped>
.selected-date-info {
  background: #e3f2fd;
  border: 1px solid #2196f3;
  border-radius: 4px;
  padding: 1rem;
  margin-bottom: 1rem;
}

.selected-date-info h3 {
  margin: 0 0 0.5rem 0;
  color: #1976d2;
  font-size: 1.1rem;
}

.date-note {
  margin: 0;
  font-size: 0.9rem;
  color: #666;
  font-style: italic;
}

form {
  grid-template-columns: max-content 1fr;
  gap: 1em;
}

input[type="datetime-local"] {
  padding: 0.5rem;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 1rem;
  background-color: white;
  width: 100%;
  box-sizing: border-box;
}

input[type="datetime-local"]:focus {
  outline: none;
  border-color: #007bff;
  box-shadow: 0 0 0 2px rgba(0, 123, 255, 0.25);
}

input[type="datetime-local"]:hover {
  border-color: #999;
}

/* Ensure datetime inputs have consistent styling with other inputs */
input[type="datetime-local"]::-webkit-calendar-picker-indicator {
  cursor: pointer;
  border-radius: 4px;
  margin-right: 2px;
  opacity: 0.6;
  transition: opacity 0.2s;
}

input[type="datetime-local"]::-webkit-calendar-picker-indicator:hover {
  opacity: 1;
}

/* Foreign key searchable dropdown styling */
.foreign-key-dropdown {
  position: relative;
  width: 100%;
}

.search-input-container {
  position: relative;
  display: flex;
  align-items: center;
}

.search-input {
  padding: 0.5rem 2.5rem 0.5rem 0.5rem;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 1rem;
  background-color: white;
  width: 100%;
  box-sizing: border-box;
  cursor: text;
}

.search-input:focus {
  outline: none;
  border-color: #007bff;
  box-shadow: 0 0 0 2px rgba(0, 123, 255, 0.25);
}

.search-input:hover {
  border-color: #999;
}

.search-input:disabled {
  background-color: #f5f5f5;
  color: #666;
  cursor: not-allowed;
}

.clear-button {
  position: absolute;
  right: 2rem;
  top: 50%;
  transform: translateY(-50%);
  background: none;
  border: none;
  font-size: 1.2rem;
  color: #999;
  cursor: pointer;
  padding: 0;
  width: 1.5rem;
  height: 1.5rem;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
}

.clear-button:hover {
  background-color: #f0f0f0;
  color: #666;
}

.dropdown-toggle {
  position: absolute;
  right: 0.5rem;
  top: 50%;
  transform: translateY(-50%);
  background: none;
  border: none;
  font-size: 0.8rem;
  color: #666;
  cursor: pointer;
  padding: 0;
  width: 1.5rem;
  height: 1.5rem;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: transform 0.2s;
}

.dropdown-toggle.open {
  transform: translateY(-50%) rotate(180deg);
}

.dropdown-toggle:hover {
  color: #333;
}

.dropdown-results {
  position: absolute;
  top: 100%;
  left: 0;
  right: 0;
  background: white;
  border: 1px solid #ddd;
  border-top: none;
  border-radius: 0 0 4px 4px;
  max-height: 200px;
  overflow-y: auto;
  z-index: 1000;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.dropdown-item {
  padding: 0.5rem;
  cursor: pointer;
  border-bottom: 1px solid #f0f0f0;
  transition: background-color 0.2s;
}

.dropdown-item:hover {
  background-color: #f8f9fa;
}

.dropdown-item.selected {
  background-color: #e3f2fd;
  color: #1976d2;
}

.dropdown-item:last-child {
  border-bottom: none;
}

.no-results {
  padding: 0.5rem;
  color: #666;
  font-style: italic;
  text-align: center;
}

/* Select dropdown styling (fallback) */
select {
  padding: 0.5rem;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 1rem;
  background-color: white;
  width: 100%;
  box-sizing: border-box;
  cursor: pointer;
}

select:focus {
  outline: none;
  border-color: #007bff;
  box-shadow: 0 0 0 2px rgba(0, 123, 255, 0.25);
}

select:hover {
  border-color: #999;
}

select:disabled {
  background-color: #f5f5f5;
  color: #666;
  cursor: not-allowed;
}

.readonly-field {
  background-color: #f5f5f5;
  color: #666;
  cursor: not-allowed;
}

.field-help {
  display: block;
  margin-top: 0.25rem;
  color: #666;
  font-size: 0.85rem;
  font-style: italic;
}

/* Form actions styling */
.form-actions {
  margin-top: 2rem;
  display: flex;
  gap: 1rem;
  justify-content: flex-end;
}

.form-actions button:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

/* Error styling */
.error {
  background-color: #f8d7da;
  color: #721c24;
  padding: 0.75rem;
  border: 1px solid #f5c6cb;
  border-radius: 4px;
  margin-top: 1rem;
}

@media (max-width: 768px) {
  form {
    grid-template-columns: 1fr;
  }
}

</style>
