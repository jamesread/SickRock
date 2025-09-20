<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { createApiClient } from '../stores/api'
import { SickRock } from '../gen/sickrock_pb'
import Section from 'picocrank/vue/components/Section.vue'
import { HugeiconsIcon, ArrowLeft01Icon, Edit03Icon, CheckmarkSquare03Icon, Delete01Icon } from '@hugeicons/core-free-icons'

const route = useRoute()
const router = useRouter()
const tableId = route.params.tableName as string

// Transport handled by authenticated client
const client = createApiClient()

// State
const columns = ref<Array<{ name: string; type: string; required: boolean }>>([])
const loading = ref(false)
const error = ref<string | null>(null)
const editingColumn = ref<string | null>(null)
const newType = ref('')
const customType = ref('')
const useCustomType = ref(false)
const showDropConfirm = ref<string | null>(null)
const dropping = ref(false)

// Rename column state
const renamingColumn = ref<string | null>(null)
const newColumnName = ref('')

// Sorted view of columns: id, name, system (sr_*), then remaining
const sortedColumns = computed(() => {
  const cols = [...columns.value]
  const isSystem = (n: string) => n?.startsWith('sr_')

  const idCol = cols.find(c => c.name === 'id')
  const nameCol = cols.find(c => c.name === 'name')
  const systemCols = cols.filter(c => c.name !== 'id' && c.name !== 'name' && isSystem(c.name))
  const otherCols = cols.filter(c => c.name !== 'id' && c.name !== 'name' && !isSystem(c.name))

  // Preserve original relative order within groups
  const out: Array<{ name: string; type: string; required: boolean }> = []
  if (idCol) out.push(idCol)
  if (nameCol) out.push(nameCol)
  out.push(...systemCols)
  out.push(...otherCols)
  return out
})

// Foreign keys for this table
const foreignKeys = ref<Array<{
  constraintName: string
  tableName: string
  columnName: string
  referencedTable: string
  referencedColumn: string
  onDeleteAction: string
  onUpdateAction: string
}>>([])

async function loadForeignKeys() {
  try {
    const res = await client.getForeignKeys({ tableName: tableId })
    foreignKeys.value = res.foreignKeys.map(fk => ({
      constraintName: fk.constraintName,
      tableName: fk.tableName,
      columnName: fk.columnName,
      referencedTable: fk.referencedTable,
      referencedColumn: fk.referencedColumn,
      onDeleteAction: fk.onDeleteAction,
      onUpdateAction: fk.onUpdateAction,
    }))
  } catch (err) {
    // non-fatal
    console.error('Error loading foreign keys:', err)
  }
}

// Available column types - mapping native database types to display names
const columnTypes = [
  { value: 'TEXT', label: 'TEXT', description: 'Text data' },
  { value: 'VARCHAR(255)', label: 'VARCHAR(255)', description: 'Variable length text (255 chars)' },
  { value: 'INT', label: 'INT', description: 'Integer (32-bit)' },
  { value: 'INT(11)', label: 'INT(11)', description: 'Integer with display width 11' },
  { value: 'BIGINT', label: 'BIGINT', description: 'Large integer (64-bit)' },
  { value: 'TINYINT(1)', label: 'TINYINT(1)', description: 'Boolean/TinyInt' },
  { value: 'DATETIME', label: 'DATETIME', description: 'Date and time values' },
  { value: 'DOUBLE', label: 'DOUBLE', description: 'Double precision decimal' },
  { value: 'FLOAT', label: 'FLOAT', description: 'Single precision decimal' },
  { value: 'DECIMAL(10,2)', label: 'DECIMAL(10,2)', description: 'Fixed precision decimal' },
  { value: 'BOOLEAN', label: 'BOOLEAN', description: 'Boolean values' },
  { value: 'DATE', label: 'DATE', description: 'Date only' },
  { value: 'TIME', label: 'TIME', description: 'Time only' },
  { value: 'TIMESTAMP', label: 'TIMESTAMP', description: 'Timestamp with timezone' }
]

// Computed
const canChangeType = computed(() => {
  const currentType = getCurrentColumnType(editingColumn.value)
  const selectedType = useCustomType.value ? customType.value : newType.value
  return editingColumn.value && selectedType && selectedType !== currentType
})

const canDropColumn = (columnName: string) => {
  // Prevent dropping system columns
  return columnName !== 'id' && columnName !== 'name' && columnName !== 'sr_created'
}

// Methods
async function loadColumns() {
  try {
    loading.value = true
    const response = await client.getTableStructure({ pageId: tableId })
    columns.value = response.fields?.map(field => ({
      name: field.name,
      type: field.type,
      required: !!field.required
    })) || []
  } catch (err) {
    error.value = `Error loading columns: ${err}`
  } finally {
    loading.value = false
  }
}

function getCurrentColumnType(columnName: string): string {
  const column = columns.value.find(col => col.name === columnName)
  return column?.type || ''
}

function startEdit(columnName: string) {
  editingColumn.value = columnName
  newType.value = getCurrentColumnType(columnName)
  customType.value = ''
  useCustomType.value = false
}

function startRename(columnName: string) {
  renamingColumn.value = columnName
  newColumnName.value = columnName
}

function cancelRename() {
  renamingColumn.value = null
  newColumnName.value = ''
}

async function renameColumn(columnName: string) {
  if (!newColumnName.value || newColumnName.value === columnName) {
    renamingColumn.value = null
    return
  }

  loading.value = true
  error.value = null

  try {
    const data = await client.changeColumnName({
      tableName: tableId,
      oldColumnName: columnName,
      newColumnName: newColumnName.value,
    })
    if (!data.success) throw new Error(data.message || 'Failed to rename column')

    // Update local list
    const col = columns.value.find(c => c.name === columnName)
    if (col) {
      col.name = newColumnName.value
    }
    renamingColumn.value = null
    newColumnName.value = ''
  } catch (err: any) {
    error.value = `Error renaming column: ${err?.message || err}`
  } finally {
    loading.value = false
  }
}

function cancelEdit() {
  editingColumn.value = null
  newType.value = ''
  customType.value = ''
  useCustomType.value = false
}

async function changeColumnType() {
  const selectedType = useCustomType.value ? customType.value : newType.value
  if (!editingColumn.value || !selectedType) {
    return
  }

  loading.value = true
  error.value = null

  try {
    const response = await client.changeColumnType({
      tableName: tableId,
      columnName: editingColumn.value,
      newType: selectedType
    })

    if (response.success) {
      // Update the local column type
      const column = columns.value.find(col => col.name === editingColumn.value)
      if (column) {
        column.type = selectedType
      }

      editingColumn.value = null
      newType.value = ''
      customType.value = ''
      useCustomType.value = false
    } else {
      error.value = response.message || 'Failed to change column type'
    }
  } catch (err) {
    error.value = `Error changing column type: ${err}`
  } finally {
    loading.value = false
  }
}

async function dropColumn(columnName: string) {
  dropping.value = true
  error.value = null

  try {
    const response = await client.dropColumn({
      tableName: tableId,
      columnName: columnName
    })

    if (response.success) {
      // Remove the column from the local list
      columns.value = columns.value.filter(col => col.name !== columnName)
      showDropConfirm.value = null
    } else {
      error.value = response.message || 'Failed to drop column'
    }
  } catch (err) {
    error.value = `Error dropping column: ${err}`
  } finally {
    dropping.value = false
  }
}

function goBack() {
  router.push({ name: 'table', params: { tableName: tableId } })
}

// Lifecycle
onMounted(async () => {
  await Promise.all([
    loadColumns(),
    loadForeignKeys(),
  ])
})
</script>

<template>
  <Section :title="`Column Types: ${tableId}`">
    <template #toolbar>
      <button @click="goBack" class="button neutral">
        <HugeiconsIcon :icon="ArrowLeft01Icon" />
        Back to Table
      </button>
      <router-link :to="`/table/${tableId}/foreign-keys`" class="button neutral">
        <HugeiconsIcon :icon="Edit03Icon" />
        Foreign Keys
      </router-link>
      <router-link :to="`/table/${tableId}/add-column`" class="button good">
        <HugeiconsIcon :icon="Edit03Icon" />
        Create Column
      </router-link>
    </template>

    <div v-if="error" class="error-message">
      {{ error }}
    </div>

    <div v-if="loading && columns.length === 0" class="loading">
      Loading columns...
    </div>

    <div v-else-if="columns.length === 0" class="empty-state">
      <div class="empty-state-icon">📊</div>
      <h3>No Columns</h3>
      <p>This table doesn't have any columns yet.</p>
    </div>

    <div v-else class="columns-list">
      <div class="column-item" v-for="column in sortedColumns" :key="column.name">
        <div class="column-info">
          <div class="column-name">
            <strong v-if="renamingColumn !== column.name">{{ column.name }}</strong>
            <span v-else class="rename-inline">
              <input v-model="newColumnName" type="text" class="rename-input" />
              <button class="button small good" @click="renameColumn(column.name)" :disabled="loading || !newColumnName.trim()">Save</button>
              <button class="button small neutral" @click="cancelRename" :disabled="loading">Cancel</button>
            </span>
            <span v-if="column.required" class="required-badge">Required</span>
            <span v-if="column.name && column.name.startsWith('sr_')" class="system-badge">System</span>
          </div>
          <div class="column-type">
            Current Type: <code>{{ column.type }}</code>
            <template v-if="foreignKeys.some(fk => fk.columnName === column.name)">
              <div class="fk-info" v-for="fk in foreignKeys.filter(fk => fk.columnName === column.name)" :key="fk.constraintName">
                FK → <router-link :to="`/table/${fk.referencedTable}`">{{ fk.referencedTable }}</router-link> ({{ fk.referencedColumn }})
                <span class="fk-action">on delete {{ fk.onDeleteAction }}, on update {{ fk.onUpdateAction }}</span>
              </div>
            </template>
          </div>
        </div>

        <div v-if="editingColumn === column.name" class="edit-form">
          <div class="type-selector">
            <label for="new-type">New Type:</label>
            <div class="type-options">
              <label class="radio-option">
                <input type="radio" v-model="useCustomType" :value="false" />
                <span>Select from common types</span>
              </label>
              <label class="radio-option">
                <input type="radio" v-model="useCustomType" :value="true" />
                <span>Enter custom type</span>
              </label>
            </div>

            <div v-if="!useCustomType">
              <select v-model="newType" id="new-type" class="type-select">
                <option value="">Select a type</option>
                <option v-for="type in columnTypes" :key="type.value" :value="type.value">
                  {{ type.label }}
                </option>
                <!-- Add current type if it's not in our predefined list -->
                <option v-if="!columnTypes.find(t => t.value === getCurrentColumnType(editingColumn))"
                        :value="getCurrentColumnType(editingColumn)"
                        disabled>
                  {{ getCurrentColumnType(editingColumn) }} (current)
                </option>
              </select>
              <div v-if="newType" class="type-description">
                {{ columnTypes.find(t => t.value === newType)?.description }}
              </div>
            </div>

            <div v-else>
              <input
                v-model="customType"
                type="text"
                placeholder="e.g., VARCHAR(500), DECIMAL(10,3), etc."
                class="type-input"
              />
              <div class="type-description">
                Enter the exact database type (e.g., VARCHAR(255), INT(11), DECIMAL(10,2))
              </div>
            </div>
          </div>
          <div class="edit-actions">
            <button @click="changeColumnType" :disabled="!canChangeType || loading" class="button good">
              <HugeiconsIcon :icon="CheckmarkSquare03Icon" />
              {{ loading ? 'Changing...' : 'Change Type' }}
            </button>
            <button @click="cancelEdit" :disabled="loading" class="button neutral">
              Cancel
            </button>
          </div>
        </div>

        <div v-else class="column-actions">
          <button v-if="!column.required && !(column.name && column.name.startsWith('sr_'))" @click="startRename(column.name)" class="button small neutral">
            <HugeiconsIcon :icon="Edit03Icon" />
            Rename
          </button>
          <router-link
            v-if="!column.required && !(column.name && column.name.startsWith('sr_'))"
            :to="`/table/${tableId}/foreign-keys?column=${encodeURIComponent(column.name)}`"
            class="button small neutral"
          >
            <HugeiconsIcon :icon="Edit03Icon" />
            Add FK
          </router-link>
          <button v-if="!(column.name && column.name.startsWith('sr_'))" @click="startEdit(column.name)" class="button small neutral">
            <HugeiconsIcon :icon="Edit03Icon" />
            Change Type
          </button>
          <button
            v-if="canDropColumn(column.name)"
            @click="showDropConfirm = column.name"
            class="button small bad"
          >
            <HugeiconsIcon :icon="Delete01Icon" />
            Drop
          </button>
        </div>
      </div>
    </div>

    <!-- Drop Column Confirmation Modal -->
    <div v-if="showDropConfirm" class="modal-overlay" @click="showDropConfirm = null">
      <div class="modal" @click.stop>
        <h3>Drop Column</h3>
        <p>Are you sure you want to drop the column <strong>{{ showDropConfirm }}</strong>?</p>
        <p class="warning">This action cannot be undone and will permanently delete all data in this column.</p>
        <div class="modal-actions">
          <button
            @click="dropColumn(showDropConfirm)"
            :disabled="dropping"
            class="button bad"
          >
            <HugeiconsIcon :icon="Delete01Icon" />
            {{ dropping ? 'Dropping...' : 'Drop Column' }}
          </button>
          <button @click="showDropConfirm = null" :disabled="dropping" class="button neutral">
            Cancel
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

.loading {
  text-align: center;
  padding: 2rem;
  color: #666;
}

.empty-state {
  text-align: center;
  padding: 3rem 1rem;
}

.empty-state-icon {
  font-size: 3rem;
  margin-bottom: 1rem;
}

.empty-state h3 {
  margin: 0 0 0.5rem 0;
  color: #333;
}

.empty-state p {
  margin: 0 0 2rem 0;
  color: #666;
}

.columns-list {
  margin-bottom: 2rem;
}

.columns-list h3 {
  margin: 0 0 1rem 0;
  color: #333;
}

.column-item {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  padding: 1rem;
  border: 1px solid #ddd;
  border-radius: 4px;
  margin-bottom: 0.5rem;
  background: #f9f9f9;
}

.column-info {
  flex: 1;
  margin-right: 1rem;
}

.column-name {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-bottom: 0.25rem;
}

.required-badge {
  background: #dc3545;
  color: white;
  padding: 0.125rem 0.5rem;
  border-radius: 12px;
  font-size: 0.75rem;
  font-weight: normal;
}

.system-badge {
  background: #6c757d;
  color: white;
  padding: 0.125rem 0.5rem;
  border-radius: 12px;
  font-size: 0.75rem;
  font-weight: normal;
}

.system-badge {
  background: #007bff;
  color: white;
  padding: 0.125rem 0.5rem;
  border-radius: 12px;
  font-size: 0.75rem;
  font-weight: normal;
}

.column-type {
  color: #666;
  font-size: 0.9rem;
}

.column-type code {
  background: #e9ecef;
  padding: 0.125rem 0.25rem;
  border-radius: 3px;
  font-family: monospace;
}

.fk-info {
  margin-top: 0.25rem;
  color: #555;
  font-size: 0.9rem;
}

.fk-info .fk-action {
  margin-left: .5rem;
  color: #777;
  font-style: italic;
}

.edit-form {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  min-width: 300px;
}

.type-selector {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.type-selector > label {
  font-weight: bold;
  color: #333;
}

.type-options {
  display: flex;
  gap: 1rem;
  margin-bottom: 0.5rem;
}

.radio-option {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-weight: normal;
  cursor: pointer;
}

.radio-option input[type="radio"] {
  margin: 0;
}

.type-select {
  padding: 0.5rem;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 1rem;
}

.type-input {
  padding: 0.5rem;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 1rem;
  width: 100%;
  font-family: monospace;
}

.type-description {
  font-size: 0.9rem;
  color: #666;
  font-style: italic;
  margin-top: 0.25rem;
}

.edit-actions {
  display: flex;
  gap: 0.5rem;
  justify-content: flex-end;
}

.column-actions {
  display: flex;
  align-items: center;
  gap: .5em;
}

/* Modal styles */
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
  max-width: 500px;
  width: 90%;
}

.modal h3 {
  margin: 0 0 1rem 0;
  color: #333;
}

.modal p {
  margin: 0 0 1rem 0;
  color: #666;
}

.modal .warning {
  color: #dc3545;
  font-weight: bold;
  background: #f8d7da;
  padding: 0.75rem;
  border-radius: 4px;
  border: 1px solid #f5c6cb;
}

.modal-actions {
  display: flex;
  gap: 1rem;
  justify-content: flex-end;
  margin-top: 1.5rem;
}
</style>
