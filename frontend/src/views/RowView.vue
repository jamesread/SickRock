<script setup lang="ts">
import { ref, onMounted, computed, inject } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { SickRock } from '../gen/sickrock_pb'
import { HugeiconsIcon } from '@hugeicons/vue'
import { ArrowLeft01Icon, Edit01Icon, Delete01Icon } from '@hugeicons/core-free-icons'
import Table from '../components/TableComponent.vue'
import Section from 'picocrank/vue/components/Section.vue'
import type { createApiClient } from '../stores/api'
import { formatUnixTimestamp } from '../utils/dateFormatting'

// Helper function to format relative time values
function formatRelativeTime(seconds: number): string {
  if (seconds < 60) {
    return `${seconds}s ago`
  } else if (seconds < 3600) {
    const minutes = Math.floor(seconds / 60)
    return `${minutes}m ago`
  } else if (seconds < 86400) {
    const hours = Math.floor(seconds / 3600)
    return `${hours}h ago`
  } else {
    const days = Math.floor(seconds / 86400)
    return `${days}d ago`
  }
}

const route = useRoute()
const router = useRouter()
const tableName = route.params.tableName as string
const rowId = route.params.rowId as string

// Use global API client
const client = inject<ReturnType<typeof createApiClient>>('apiClient')

const item = ref<Record<string, unknown> | null>(null)
const loading = ref(false)
const error = ref<string | null>(null)
const showDeleteConfirm = ref(false)
const deleting = ref(false)

// Foreign key and related rows state
const foreignKeys = ref<Array<{
  constraintName: string
  tableName: string
  columnName: string
  referencedTable: string
  referencedColumn: string
  onDeleteAction: string
  onUpdateAction: string
}>>([])
const relatedRows = ref<Record<string, any[]>>({})
const loadingRelated = ref(false)

// Index of referenced item names for quick lookup: table -> id -> name
const referencedNameIndex = computed(() => {
  const index: Record<string, Record<string, string>> = {}
  for (const [key, rows] of Object.entries(relatedRows.value)) {
    // Only build index for referenced tables (Direction 2 keys use referencedTable.referencedColumn)
    const [table] = key.split('.')
    if (!index[table]) index[table] = {}
    for (const row of rows) {
      const id = String((row && (row.id || (row.additionalFields && row.additionalFields.id))) ?? '')
      if (!id) continue
      const nameCandidate = (row && (row.name || (row.additionalFields && (row.additionalFields.name || row.additionalFields.title))))
      const name = String(nameCandidate ?? id)
      index[table][id] = name
    }
  }
  return index
})

// Quick FK map: columnName -> { referencedTable, referencedColumn }
const foreignKeyByColumn = computed(() => {
  const map: Record<string, { referencedTable: string; referencedColumn: string }> = {}
  for (const fk of foreignKeys.value) {
    map[fk.columnName] = { referencedTable: fk.referencedTable, referencedColumn: fk.referencedColumn }
  }
  return map
})

function resolveRawFieldValue(key: string): any {
  if (!item.value) return undefined
  // check top-level first
  if (Object.prototype.hasOwnProperty.call(item.value, key)) {
    return (item.value as any)[key]
  }
  // then additionalFields
  const af = (item.value as any).additionalFields
  if (af && Object.prototype.hasOwnProperty.call(af, key)) {
    return af[key]
  }
  return undefined
}

function getFkDisplay(key: string): { to?: string; label?: string } {
  const fk = foreignKeyByColumn.value[key]
  if (!fk) return {}
  const raw = resolveRawFieldValue(key)
  if (raw === null || raw === undefined || raw === '') return {}
  const id = String(raw)
  const table = fk.referencedTable
  const label = (referencedNameIndex.value[table] && referencedNameIndex.value[table][id]) || id
  return { to: `/table/${table}/${id}`, label }
}

// Load foreign keys for the current table
async function loadForeignKeys() {
  try {
    const response = await client.getForeignKeys({ tableName })
    foreignKeys.value = response.foreignKeys.map(fk => ({
      constraintName: fk.constraintName,
      tableName: fk.tableName,
      columnName: fk.columnName,
      referencedTable: fk.referencedTable,
      referencedColumn: fk.referencedColumn,
      onDeleteAction: fk.onDeleteAction,
      onUpdateAction: fk.onUpdateAction
    }))
    console.log('Loaded foreign keys for table:', tableName, foreignKeys.value)
  } catch (err) {
    console.error('Error loading foreign keys:', err)
  }
}

// Load related rows based on foreign key relationships (bidirectional)
async function loadRelatedRows() {
  if (!item.value || foreignKeys.value.length === 0) {
    console.log('No item or foreign keys:', { item: item.value, foreignKeys: foreignKeys.value })
    return
  }

  loadingRelated.value = true
  try {
    const relatedData: Record<string, any[]> = {}
    console.log('Loading related rows for item:', item.value)
    console.log('Item value keys:', Object.keys(item.value))

    for (const fk of foreignKeys.value) {
      console.log('Processing foreign key:', fk)
      // Direction 1: Find rows in the referencing table that point to the current row
      // (e.g., tasks that reference this task_list)
      const referencedValue = item.value[fk.referencedColumn]
      console.log(`Direction 1 - Looking for ${fk.referencedColumn} = ${referencedValue} in table ${fk.tableName}`)

      try {
        let matchingItems: any[] = []
        if (referencedValue !== null && referencedValue !== undefined && referencedValue !== '') {
          const res = await client.listItems({ tcName: fk.tableName, where: { [fk.columnName]: String(referencedValue) } })
          matchingItems = res.items || []
        }
        relatedData[`${fk.tableName}.${fk.columnName}`] = matchingItems
      } catch (err) {
        console.error(`Error loading related rows for ${fk.tableName}:`, err)
        relatedData[`${fk.tableName}.${fk.columnName}`] = []
      }

      // Direction 2: Find rows in the referenced table that this row points to
      // (e.g., task_list that this task references)
      // Only process if the current table is the referencing table (has the foreign key column)
      const currentValue = item.value[fk.columnName] || (item.value.additionalFields && item.value.additionalFields[fk.columnName])
      console.log(`Direction 2 - Looking for ${fk.columnName} = ${currentValue} in table ${fk.referencedTable}`)

      if (currentValue !== null && currentValue !== undefined) {
        try {
          const res = await client.listItems({ tcName: fk.referencedTable, where: { [fk.referencedColumn]: String(currentValue) } })
          relatedData[`${fk.referencedTable}.${fk.referencedColumn}`] = res.items || []
        } catch (err) {
          console.error(`Error loading referenced rows for ${fk.referencedTable}:`, err)
          relatedData[`${fk.referencedTable}.${fk.referencedColumn}`] = []
        }
      } else {
        // Include empty array for referenced table even if current value is null
        relatedData[`${fk.referencedTable}.${fk.referencedColumn}`] = []
      }
    }

    console.log('Final related data:', relatedData)
    relatedRows.value = relatedData
  } catch (err) {
    console.error('Error loading related rows:', err)
  } finally {
    loadingRelated.value = false
  }
}

onMounted(async () => {
  loading.value = true
  try {
    const res = await client.getItem({ pageId: tableName, id: rowId })
    item.value = res.item as any ?? null

    // Load foreign keys and related rows after getting the main item
    if (item.value) {
      await loadForeignKeys()
      await loadRelatedRows()
    }
  } catch (e) {
    error.value = String(e)
  } finally {
    loading.value = false
  }
})

const entries = computed(() => {
  if (!item.value) return []

  const entries: Array<[string, any, string]> = []

  // Handle regular fields (id, srCreated, etc.)
  for (const [key, value] of Object.entries(item.value)) {
    // Skip additionalFields - we'll handle it separately
    if (key === 'additionalFields') continue
    // Remove internal/system fields
    if (key === '$typeName') continue
    // Skip relative time fields - they're displayed inline with srCreated/srUpdated
    if (key === 'srCreatedRelative' || key === 'srUpdatedRelative') continue

    let displayValue = value
    let valueClass = ''

    // Handle different data types
    if (value === null || value === undefined) {
      displayValue = '(empty)'
      valueClass = 'empty'
    } else if (key === 'srCreated' || key === 'sr_created' || key === 'srUpdated' || key === 'sr_updated') {
      // Format sr_created/sr_updated timestamp as locale-aware date
      if (typeof value === 'number' || typeof value === 'string' || typeof value === 'bigint') {
        displayValue = formatUnixTimestamp(value)
        // Add relative time if available
        if (key === 'srCreated' || key === 'sr_created') {
          const relativeValue = item.value.srCreatedRelative
          if (relativeValue != null) {
            displayValue += ` (${formatRelativeTime(Number(relativeValue))})`
          }
        } else if (key === 'srUpdated' || key === 'sr_updated') {
          const relativeValue = item.value.srUpdatedRelative
          if (relativeValue != null) {
            displayValue += ` (${formatRelativeTime(Number(relativeValue))})`
          }
        }
        valueClass = 'date'
      } else {
        displayValue = String(value)
      }
    } else if (typeof value === 'bigint') {
      displayValue = Number(value)
    } else if (typeof value === 'object') {
      displayValue = JSON.stringify(value, null, 2)
      valueClass = 'json'
    } else if (typeof value === 'boolean') {
      displayValue = value ? 'Yes' : 'No'
      valueClass = value ? 'boolean-true' : 'boolean-false'
    }

    entries.push([key, displayValue, valueClass])
  }

  // Handle additionalFields by flattening them
  if (item.value.additionalFields && typeof item.value.additionalFields === 'object') {
    for (const [key, value] of Object.entries(item.value.additionalFields)) {
      // Remove internal/system fields
      if (key === '$typeName') continue
      // Skip markdown fields - they're handled by their base field
      if (key.endsWith('Markdown')) continue
      
      let displayValue = value
      let valueClass = ''

      // Check if there's a corresponding markdown field
      const markdownFieldName = key + 'Markdown'
      const markdownValue = item.value.additionalFields[markdownFieldName]
      
      if (markdownValue) {
        // Use markdown content instead of original value
        displayValue = markdownValue
        valueClass = 'markdown-content'
      } else {
        // Handle different data types for additional fields
        if (value === null || value === undefined || value === '') {
          displayValue = '(empty)'
          valueClass = 'empty'
        } else if (key === 'srCreated' || key === 'sr_created' || key === 'srUpdated' || key === 'sr_updated') {
          // Format sr_created/sr_updated timestamp as locale-aware date
          if (typeof value === 'number' || typeof value === 'string' || typeof value === 'bigint') {
            displayValue = formatUnixTimestamp(value)
            // Add relative time if available
            if (key === 'srCreated' || key === 'sr_created') {
              const relativeValue = item.value.srCreatedRelative
              if (relativeValue != null) {
                displayValue += ` (${formatRelativeTime(Number(relativeValue))})`
              }
            } else if (key === 'srUpdated' || key === 'sr_updated') {
              const relativeValue = item.value.srUpdatedRelative
              if (relativeValue != null) {
                displayValue += ` (${formatRelativeTime(Number(relativeValue))})`
              }
            }
            valueClass = 'date'
          } else {
            displayValue = String(value)
          }
        } else if (typeof value === 'string') {
          // Try to detect if it's a JSON string
          try {
            const parsed = JSON.parse(value)
            if (typeof parsed === 'object' && parsed !== null) {
              displayValue = JSON.stringify(parsed, null, 2)
              valueClass = 'json'
            } else {
              displayValue = value
            }
          } catch {
            // Not JSON, treat as regular string
            displayValue = value
          }
        } else {
          displayValue = String(value)
        }
      }

      entries.push([key, displayValue, valueClass])
    }
  }

  return entries
})

// Computed property to format related tables data for display
const relatedTables = computed(() => {
  const tables: Array<{
    tableName: string
    columnName: string
    rows: any[]
    title: string
    direction: 'referencing' | 'referenced'
    filterColumn: string
    filterValue: string
    relationName: string
    rowCountText: string
  }> = []

  for (const [key, rows] of Object.entries(relatedRows.value)) {
    const [tableName, columnName] = key.split('.')

    // Determine the direction based on the foreign key relationship
    const fk = foreignKeys.value.find(fk =>
      (fk.tableName === tableName && fk.columnName === columnName) ||
      (fk.referencedTable === tableName && fk.referencedColumn === columnName)
    )

    const direction = fk && fk.tableName === tableName ? 'referencing' : 'referenced'

    // Create a descriptive title including relation name (constraint or column)
    const rowCount = `${rows.length} related row${rows.length !== 1 ? 's' : ''}`
    const relationName = fk?.constraintName || columnName
    const title = `${tableName} (${rowCount})`
    const rowCountText = `(${rowCount})`

    // Determine filter column (on the related table) and value (from the current item)
    let filterColumn = columnName
    let filterValue = ''
    if (fk) {
      if (direction === 'referencing') {
        // Related table column is fk.columnName, value on current item is fk.referencedColumn
        filterColumn = fk.columnName
        const v = resolveRawFieldValue(fk.referencedColumn)
        filterValue = v == null ? '' : String(v)
      } else {
        // Related table column is fk.referencedColumn, value on current item is fk.columnName
        filterColumn = fk.referencedColumn
        const v = resolveRawFieldValue(fk.columnName)
        filterValue = v == null ? '' : String(v)
      }
    }

    tables.push({
      tableName,
      columnName,
      rows,
      title,
      direction,
      filterColumn,
      filterValue,
      relationName,
      rowCountText
    })
  }

  return tables
})

// Section title matching Table component style
const sectionTitle = computed(() => `Table: ${tableName}`)

function displayFieldLabel(key: string): string {
  if (key === 'id') return 'ID'
  if (key === 'srCreated') return 'Created'
  if (key === 'srUpdated') return 'Updated'
  return key
}

async function deleteItem() {
  deleting.value = true
  error.value = null
  try {
    await client.deleteItem({ pageId: tableName, id: rowId })
    // Navigate back to the table after successful deletion
    router.push({ name: 'table', params: { tableName } })
  } catch (e) {
    error.value = String(e)
  } finally {
    deleting.value = false
    showDeleteConfirm.value = false
  }
}

function confirmDelete() {
  showDeleteConfirm.value = true
}

function cancelDelete() {
  showDeleteConfirm.value = false
}
</script>

<template>
  <div>
    <!-- Main Row Section -->
    <Section :title="sectionTitle">
      <template #toolbar>
        <router-link
          :to="`/table/${tableName}`"
          class="button"
        >
          <HugeiconsIcon :icon="ArrowLeft01Icon" width="16" height="16" />
          Back to Table
        </router-link>
        <router-link
          :to="`/table/${tableName}/${rowId}/edit`"
          class="button"
        >
          <HugeiconsIcon :icon="Edit01Icon" width="16" height="16" />
          Edit Row
        </router-link>
        <button
          @click="confirmDelete"
          class="button bad"
          :disabled="deleting"
        >
          <HugeiconsIcon :icon="Delete01Icon" width="16" height="16" />
          Delete Row
        </button>
      </template>
      
      <div v-if="error">{{ error }}</div>
      <div v-else-if="loading">Loading…</div>
      <dl v-else-if="entries.length > 0">
        <template v-for="[k, v, valueClass] in entries" :key="k">
          <dt>{{ displayFieldLabel(k) }}</dt>
          <dd :class="valueClass">
            <template v-if="getFkDisplay(k).to">
              <router-link :to="getFkDisplay(k).to">{{ getFkDisplay(k).label }}</router-link>
            </template>
            <template v-else-if="valueClass === 'markdown-content'">
              <div v-html="v"></div>
            </template>
            <template v-else>
              {{ v }}
            </template>
          </dd>
        </template>
      </dl>
      <div v-else class="no-data">
        <p>No data available for this row.</p>
      </div>
    </Section>

    <!-- Related Tables as Top-Level Sections -->
    <template v-if="foreignKeys.length > 0">
      <template v-for="table in relatedTables" :key="`${table.tableName}.${table.columnName}`">
        <Section :title="`${table.tableName} ${table.rowCountText}`">
          <template #toolbar>
            <router-link :to="`/table/${table.tableName}`" class="button">Open Table</router-link>
            <router-link :to="{ path: `/table/${table.tableName}/export`, query: { where: JSON.stringify(table.filterValue ? { [table.filterColumn]: table.filterValue } : {}) } }" class="button">Export</router-link>
            <router-link :to="{ path: `/table/${table.tableName}/insert-row`, query: Object.assign({ fromTable: tableName, fromRowId: rowId }, table.filterValue ? { [table.filterColumn]: table.filterValue } : {}) }" class="button neutral">Insert</router-link>
          </template>
          
          <div v-if="loadingRelated" class="loading-related">
            Loading related rows...
          </div>
          <div v-else-if="table.rows.length > 0">
            <Table
              :key="`${table.tableName}.${table.columnName}.${table.rows.length}`"
              :title="''"
              :table-id="table.tableName"
              :items="table.rows"
              :show-toolbar="true"
              :show-view-edit="false"
              :show-view-create="false"
              :show-pagination="false"
              :show-view-switcher="true"
              @rows-updated="loadRelatedRows"
              @row-deleted="(id) => {
                // Optimistically remove the deleted row from this related section
                const key = `${table.tableName}.${table.columnName}`
                if (relatedRows[key]) {
                  relatedRows[key] = relatedRows[key].filter((r: any) => String(r.id) !== String(id))
                }
              }"
            />
          </div>
          <div v-else>
            <p>No related rows found in this table.</p>
          </div>
        </Section>
      </template>
    </template>

    <!-- Delete Confirmation Dialog -->
    <div v-if="showDeleteConfirm" class="modal-overlay" @click="cancelDelete">
      <div class="modal-content" @click.stop>
        <h3>Confirm Delete</h3>
        <p>Are you sure you want to delete this row? This action cannot be undone.</p>
        <div class="modal-actions">
          <button @click="cancelDelete" class="button" :disabled="deleting">
            Cancel
          </button>
          <button @click="deleteItem" class="button bad" :disabled="deleting">
            {{ deleting ? 'Deleting...' : 'Delete' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
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

@media (max-width: 768px) {
  .modal-actions {
    flex-direction: column;
  }

  .modal-actions .button {
    width: 100%;
  }
}

.loading-related {
  text-align: center;
  padding: 2rem;
  color: #666;
  font-style: italic;
}

.empty-related-table {
  padding: 2rem;
  text-align: center;
  color: #666;
  font-style: italic;
}

</style>
