<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { createApiClient } from '../stores/api'
import { SickRock } from '../gen/sickrock_pb'
import Section from 'picocrank/vue/components/Section.vue'
import Dialog from 'picocrank/vue/components/Dialog.vue'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { ArrowLeft01Icon, PlusSignIcon, ColumnDeleteIcon, Edit03Icon, Link01Icon } from '@hugeicons/core-free-icons'
import { tableListLinkLabel } from '../utils/rowInsertLabel'

const route = useRoute()
const router = useRouter()
const tableId = route.params.tableName as string

// Transport handled by authenticated client
const client = createApiClient()

// State
const foreignKeys = ref<Array<{
  constraintName: string
  tableName: string
  columnName: string
  referencedTable: string
  referencedColumn: string
  onDeleteAction: string
  onUpdateAction: string
}>>([])

const availableTables = ref<Array<{ name: string; title: string }>>([])
const availableColumns = ref<Array<{ name: string; type: string }>>([])
const referencedColumns = ref<Array<{ name: string; type: string }>>([])

const tableRowName = ref<string | null>(null)
const tableRowNamePlural = ref<string | null>(null)
const backToTableLabel = computed(() => tableListLinkLabel(tableRowNamePlural.value, tableRowName.value))

const loading = ref(false)
const error = ref<string | null>(null)
const showCreateForm = ref(false)
const showDeleteConfirm = ref<string | null>(null)

// Form state
const formData = ref({
  constraintName: '',
  columnName: '',
  referencedTable: '',
  referencedColumn: '',
  onDeleteAction: 'RESTRICT',
  onUpdateAction: 'RESTRICT'
})

const actionOptions = [
  { value: 'CASCADE', label: 'CASCADE' },
  { value: 'SET NULL', label: 'SET NULL' },
  { value: 'RESTRICT', label: 'RESTRICT' },
  { value: 'NO ACTION', label: 'NO ACTION' }
]

// Computed
const canCreateForeignKey = computed(() => {
  return formData.value.columnName &&
         formData.value.referencedTable &&
         formData.value.referencedColumn
})

// Methods
async function loadForeignKeys() {
  try {
    const response = await client.getTableStructure({ pageId: tableId })
    tableRowName.value = response.rowName?.trim() || null
    tableRowNamePlural.value = response.rowNamePlural?.trim() || null
    foreignKeys.value = (response.foreignKeys || []).map(fk => ({
      constraintName: fk.constraintName,
      tableName: fk.tableName,
      columnName: fk.columnName,
      referencedTable: fk.referencedTable,
      referencedColumn: fk.referencedColumn,
      onDeleteAction: fk.onDeleteAction,
      onUpdateAction: fk.onUpdateAction
    }))
  } catch (err) {
    error.value = `Error loading foreign keys: ${err}`
  }
}

async function loadAvailableTables() {
  try {
    const response = await client.getTableConfigurations({})
    availableTables.value = response.pages.map(page => ({
      name: page.id,
      title: page.title || page.id
    }))
  } catch (err) {
    error.value = `Error loading tables: ${err}`
  }
}

async function loadTableColumns(tableName: string) {
  try {
    const response = await client.getTableStructure({ pageId: tableName })
    const columns = response.fields?.map(field => ({
      name: field.name,
      type: field.type
    })) || []
    return columns
  } catch (err) {
    error.value = `Error loading columns for ${tableName}: ${err}`
    return []
  }
}

async function onReferencedTableChange() {
  if (formData.value.referencedTable) {
    referencedColumns.value = await loadTableColumns(formData.value.referencedTable)
    formData.value.referencedColumn = '' // Reset selected column
  } else {
    referencedColumns.value = []
  }
}

async function createForeignKey() {
  if (!canCreateForeignKey.value) {
    error.value = 'Please fill in all required fields'
    return
  }

  loading.value = true
  error.value = null

  try {
    const response = await client.createForeignKey({
      tableName: tableId,
      columnName: formData.value.columnName,
      referencedTable: formData.value.referencedTable,
      referencedColumn: formData.value.referencedColumn,
      onDeleteAction: formData.value.onDeleteAction,
      onUpdateAction: formData.value.onUpdateAction
    })

    if (response.success) {
      // Reset form
      formData.value = {
        constraintName: '',
        columnName: '',
        referencedTable: '',
        referencedColumn: '',
        onDeleteAction: 'RESTRICT',
        onUpdateAction: 'RESTRICT'
      }
      showCreateForm.value = false
      referencedColumns.value = []

      // Reload foreign keys
      await loadForeignKeys()
    } else {
      error.value = response.message || 'Failed to create foreign key'
    }
  } catch (err) {
    error.value = `Error creating foreign key: ${err}`
  } finally {
    loading.value = false
  }
}

async function deleteForeignKey(constraintName: string) {
  loading.value = true
  error.value = null

  try {
    const response = await client.deleteForeignKey({
      constraintName: constraintName
    })

    if (response.success) {
      showDeleteConfirm.value = null
      await loadForeignKeys()
    } else {
      error.value = response.message || 'Failed to delete foreign key'
    }
  } catch (err) {
    error.value = `Error deleting foreign key: ${err}`
  } finally {
    loading.value = false
  }
}


// Lifecycle
onMounted(async () => {
  await Promise.all([
    loadForeignKeys(),
    loadAvailableTables(),
    loadTableColumns(tableId).then(cols => {
      availableColumns.value = cols
    })
  ])
  // Preselect column from query string
  const preselect = route.query.column as string
  if (preselect) {
    formData.value.columnName = preselect
    showCreateForm.value = true
  }
})
</script>

<template>
  <Section :title="`Foreign Keys: ${tableId}`" :icon="Link01Icon">
    <template #toolbar>
      <router-link
        :to="`/table/${tableId}`"
        class="button inline-icon"
      >
        <HugeiconsIcon :icon="ArrowLeft01Icon" width="1em" height="1em" aria-hidden="true" />
        <span>{{ backToTableLabel }}</span>
      </router-link>
      <router-link :to="`/table/${tableId}/column-types`" class="button inline-icon neutral">
        <HugeiconsIcon :icon="Edit03Icon" width="1em" height="1em" aria-hidden="true" />
        <span>Structure</span>
      </router-link>
      <button @click="showCreateForm = true" class="button inline-icon good">
        <HugeiconsIcon :icon="PlusSignIcon" width="1em" height="1em" aria-hidden="true" />
        <span>Add Foreign Key</span>
      </button>
    </template>

    <NotificationBlock v-if="error" type="error" :message="error" />


    <!-- Foreign Keys List -->
    <div v-if="foreignKeys.length === 0 && !showCreateForm" class="empty-state">
      <div class="empty-state-icon">🔗</div>
      <h3>No Foreign Keys</h3>
      <p>This table doesn't have any foreign key constraints yet.</p>
      <button @click="showCreateForm = true" class="button good">
        <HugeiconsIcon :icon="PlusSignIcon" />
        Add First Foreign Key
      </button>
    </div>

    <div v-else-if="foreignKeys.length > 0" class="foreign-keys-list">
      <h3>Existing Foreign Keys</h3>
      <div class="foreign-key-item" v-for="fk in foreignKeys" :key="fk.constraintName">
        <div class="foreign-key-info">
          <div class="foreign-key-constraint">
            <strong>{{ fk.constraintName }}</strong>
          </div>
          <div class="foreign-key-relationship">
            <code>{{ fk.tableName }}.{{ fk.columnName }}</code>
            <span class="arrow">→</span>
            <code>{{ fk.referencedTable }}.{{ fk.referencedColumn }}</code>
          </div>
          <div class="foreign-key-actions">
            <span class="action">ON DELETE: {{ fk.onDeleteAction }}</span>
            <span class="action">ON UPDATE: {{ fk.onUpdateAction }}</span>
          </div>
        </div>
        <button
          @click="showDeleteConfirm = fk.constraintName"
          class="button small bad"
          :disabled="loading"
        >
          <HugeiconsIcon :icon="ColumnDeleteIcon" />
        </button>
      </div>
    </div>

    <!-- Create Foreign Key Form -->
    <div v-if="showCreateForm" class="create-form">
      <form @submit.prevent="createForeignKey">
        <label for="constraint-name">Constraint Name (optional)</label>
        <input
          id="constraint-name"
          v-model="formData.constraintName"
          type="text"
          placeholder="e.g. fk_orders_customer_id"
        />

        <label for="column-name">Column Name</label>
        <select v-model="formData.columnName" id="column-name" required>
          <option value="">Select a column</option>
          <option v-for="col in availableColumns" :key="col.name" :value="col.name">
            {{ col.name }} ({{ col.type }})
          </option>
        </select>

        <label for="referenced-table">Referenced Table</label>
        <select
          v-model="formData.referencedTable"
          @change="onReferencedTableChange"
          id="referenced-table"
          required
        >
          <option value="">Select a table</option>
          <option v-for="table in availableTables" :key="table.name" :value="table.name">
            {{ table.title }}
          </option>
        </select>

        <label for="referenced-column">Referenced Column</label>
        <select
          v-model="formData.referencedColumn"
          id="referenced-column"
          :disabled="!formData.referencedTable"
          required
        >
          <option value="">Select a column</option>
          <option v-for="col in referencedColumns" :key="col.name" :value="col.name">
            {{ col.name }} ({{ col.type }})
          </option>
        </select>

        <label for="on-delete">On Delete Action</label>
        <select v-model="formData.onDeleteAction" id="on-delete">
          <option v-for="action in actionOptions" :key="action.value" :value="action.value">
            {{ action.label }}
          </option>
        </select>

        <label for="on-update">On Update Action</label>
        <select v-model="formData.onUpdateAction" id="on-update">
          <option v-for="action in actionOptions" :key="action.value" :value="action.value">
            {{ action.label }}
          </option>
        </select>

        <div class="form-actions">
          <button type="button" @click="showCreateForm = false" class="button neutral">
            Cancel
          </button>
          <button
            type="submit"
            :disabled="loading || !canCreateForeignKey"
            class="button good"
          >
            {{ loading ? 'Creating...' : 'Create Foreign Key' }}
          </button>
        </div>
      </form>
    </div>

    <!-- Delete Confirmation -->
    <Dialog
      :open="showDeleteConfirm !== null"
      title="Delete Foreign Key"
      size="small"
      @update:open="open => { if (!open) showDeleteConfirm = null }"
    >
      <p>Are you sure you want to delete this foreign key constraint? This action cannot be undone.</p>
      <template #actions>
        <button type="button" :disabled="loading" class="button bad" @click="deleteForeignKey(showDeleteConfirm!)">
          {{ loading ? 'Deleting...' : 'Delete Foreign Key' }}
        </button>
      </template>
    </Dialog>
  </Section>
</template>

<style scoped>
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

.foreign-keys-list {
  margin-bottom: 2rem;
}

.foreign-keys-list h3 {
  margin: 0 0 1rem 0;
  color: #333;
}

.foreign-key-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem;
  border: 1px solid #ddd;
  border-radius: 4px;
  margin-bottom: 0.5rem;
  background: #f9f9f9;
}

.foreign-key-info {
  flex: 1;
}

.foreign-key-constraint {
  font-weight: bold;
  color: #333;
  margin-bottom: 0.25rem;
}

.foreign-key-relationship {
  font-family: monospace;
  color: #666;
  margin-bottom: 0.25rem;
}

.arrow {
  margin: 0 0.5rem;
  color: #999;
}

.foreign-key-actions {
  font-size: 0.9rem;
  color: #666;
}

.action {
  margin-right: 1rem;
}

.create-form h3 {
  margin: 0 0 1rem 0;
  color: #333;
}

label {
  display: block;
  margin: 0.75rem 0 0.25rem 0;
  font-weight: bold;
  color: #333;
}

select {
  width: 100%;
  padding: 0.5rem;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 1rem;
}

select:disabled {
  background: #f5f5f5;
  color: #999;
}

.form-actions {
  display: flex;
  gap: 1rem;
  justify-content: flex-end;
  margin-top: 1.5rem;
}

</style>
