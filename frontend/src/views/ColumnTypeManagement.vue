<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, computed, nextTick } from 'vue'
import { useRoute } from 'vue-router'

import { createApiClient } from '../stores/api'
import Section from 'picocrank/vue/components/Section.vue'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import Dialog from 'picocrank/vue/components/Dialog.vue'
import type { DialogApi } from '../utils/dialog'
import Table from 'picocrank/vue/components/Table.vue'
import ConditionalFormattingRules from '../components/ConditionalFormattingRules.vue'
import FieldGroupingEditor from '../components/FieldGroupingEditor.vue'
import ColumnDisplayEditor from '../components/ColumnDisplayEditor.vue'
import TablePullThroughSection from '../components/TablePullThroughSection.vue'
import TableRowLabelsEditor from '../components/TableRowLabelsEditor.vue'
import { useDesignMode } from '../composables/useDesignMode'
import TableShareSection from '../components/TableShareSection.vue'
import AddColumn from '../components/AddColumn.vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { tableListLinkLabel } from '../utils/rowInsertLabel'
import { syntheticColumnNamesForSourceTable } from '../utils/relationPullThrough'
import type { ForeignKey } from '../gen/sickrock_pb'
import {
  ArrowLeft01Icon,
  Edit03Icon,
  Delete01Icon,
  Settings01Icon,
  AddCircleIcon,
  Add01Icon,
  RefreshIcon,
  LayoutIcon,
  DatabaseSettingIcon,
  Link01Icon,
  TextIcon,
  GridIcon,
  Share08Icon,
  PaintBrushIcon,
} from '@hugeicons/core-free-icons'

const iconStrokeWidth = 2.5
const { designModeEnabled } = useDesignMode()

const route = useRoute()
const tableId = route.params.tableName as string

// Transport handled by authenticated client
const client = createApiClient()

// State
const columns = ref<Array<{ name: string; type: string; required: boolean }>>([])
const tableRowName = ref<string | null>(null)
const tableRowNamePlural = ref<string | null>(null)
const backToTableLabel = computed(() => tableListLinkLabel(tableRowNamePlural.value, tableRowName.value))
const loading = ref(false)
const error = ref<string | null>(null)
const editingColumn = ref<string | null>(null)
const newType = ref('')
const customType = ref('')
const useCustomType = ref(false)
const showDropConfirm = ref<string | null>(null)
const dropping = ref(false)
const createColumnDialog = ref<DialogApi | null>(null)
const createColumnName = ref<string | undefined>()
const createColumnType = ref<'string' | 'int64' | 'tinyint' | 'datetime' | 'user_ref' | 'image' | undefined>()

// Row id of this table's entry in table_configurations (for the edit link); null if not visible to the user.
const tableConfigurationRowId = ref<string | null>(null)

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

const structureForeignKeys = ref<ForeignKey[]>([])

const columnNamesForGrouping = computed(() => {
  const physical = sortedColumns.value.map(c => c.name)
  const synthetic = syntheticColumnNamesForSourceTable(tableId, structureForeignKeys.value)
  return [...physical, ...synthetic]
})

const tableHeaders = computed(() => [
  { key: 'name', label: 'Column', sortable: true },
  { key: 'type', label: 'Type', sortable: true },
  { key: 'tags', label: 'Tags', sortable: false, width: '10rem' },
  { key: 'actions', label: 'Actions', sortable: false, width: '22rem' },
])

function isSystemColumn(name: string | undefined) {
  return !!name?.startsWith('sr_')
}

function foreignKeysForColumn(columnName: string) {
  return foreignKeys.value.filter(fk => fk.columnName === columnName)
}

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

// Drop FK state
const showDropFkConfirm = ref<string | null>(null)
const droppingFk = ref(false)

async function loadForeignKeys() {
  try {
    const res = await client.getTableStructure({ pageId: tableId })
    structureForeignKeys.value = (res.foreignKeys ?? []).map(fk => ({ ...fk }))
    foreignKeys.value = (res.foreignKeys || []).map(fk => ({
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

// Standard fields definition
const standardFields = [
  {
    name: 'name',
    type: 'string' as const,
    description: 'Recommended for better table organization and display. Used as the primary identifier for rows.',
    recommended: true
  },
  {
    name: 'sr_created',
    type: 'datetime' as const,
    description: 'Automatically tracks when a row was created. Useful for auditing and sorting by creation time.',
    recommended: true
  },
  {
    name: 'sr_updated',
    type: 'datetime' as const,
    description: 'Automatically tracks when a row was last updated. Useful for auditing and tracking changes.',
    recommended: true
  },
  {
    name: 'uuid',
    type: 'string' as const,
    description: 'Universally unique identifier. Useful for distributed systems and external integrations.',
    recommended: false
  },
  {
    name: 'owner_id',
    type: 'user_ref' as const,
    description: 'References a SickRock user account id. Shown as a username chip in tables and insert forms.',
    recommended: false
  }
]

// Computed
const canChangeType = computed(() => {
  const currentType = getCurrentColumnType(editingColumn.value)
  const selectedType = useCustomType.value ? customType.value : newType.value
  return editingColumn.value && selectedType && selectedType !== currentType
})

const missingStandardFields = computed(() => {
  return standardFields.filter(field => {
    const exists = columns.value.some(col => col.name === field.name)
    return !exists
  })
})

const canDropColumn = (columnName: string) => {
  // Prevent dropping system columns and standard fields
  const isStandardField = standardFields.some(f => f.name === columnName)
  return columnName !== 'id' && !isStandardField
}

// Methods
async function loadColumns() {
  try {
    loading.value = true
    const response = await client.getTableStructure({ pageId: tableId })
    tableRowName.value = response.rowName?.trim() || null
    tableRowNamePlural.value = response.rowNamePlural?.trim() || null
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

async function dropForeignKey(constraintName: string) {
  droppingFk.value = true
  error.value = null
  try {
    const res = await client.deleteForeignKey({ constraintName })
    if (!res.success) {
      throw new Error(res.message || 'Failed to drop foreign key')
    }
    // Remove FK locally
    foreignKeys.value = foreignKeys.value.filter(fk => fk.constraintName !== constraintName)
    showDropFkConfirm.value = null
  } catch (err: any) {
    error.value = `Error dropping foreign key: ${err?.message || err}`
  } finally {
    droppingFk.value = false
  }
}


function openCreateColumn(name?: string, type?: 'string' | 'int64' | 'tinyint' | 'datetime' | 'user_ref' | 'image') {
  createColumnName.value = name
  createColumnType.value = type
  nextTick(() => createColumnDialog.value?.show())
}

function closeCreateColumn() {
  createColumnDialog.value?.close()
  createColumnName.value = undefined
  createColumnType.value = undefined
}

async function refreshAll() {
  await Promise.all([loadColumns(), loadForeignKeys()])
}

async function onColumnAdded() {
  closeCreateColumn()
  await loadColumns()
}

// Lifecycle
async function loadTableConfigurationRowId() {
  try {
    const res = await client.listItems({ tcName: 'table_configurations', where: { name: tableId } })
    tableConfigurationRowId.value = res.items?.[0]?.id || null
  } catch {
    tableConfigurationRowId.value = null
  }
}

// "On this page" links. Sections are wrapped in anchor divs; a link is shown only while its
// section is rendered (several decide that themselves, e.g. Share or pull-through columns).
// Icons match each section's own title icon.
const QUICK_JUMP_SECTIONS = [
  { id: 'table-configuration', label: 'Table configuration', icon: DatabaseSettingIcon },
  { id: 'pull-through', label: 'Pull-through columns', icon: Link01Icon },
  { id: 'row-labels', label: 'Row labels', icon: TextIcon },
  { id: 'column-display', label: 'Column labels & formatting', icon: Edit03Icon },
  { id: 'field-grouping', label: 'Field grouping', icon: GridIcon },
  { id: 'standard-fields', label: 'Standard fields', icon: AddCircleIcon },
  { id: 'share', label: 'Share', icon: Share08Icon },
  { id: 'conditional-formatting', label: 'Conditional formatting', icon: PaintBrushIcon },
]
const quickJumpLinks = ref<typeof QUICK_JUMP_SECTIONS>([])
let quickJumpObserver: MutationObserver | null = null

function refreshQuickJumpLinks() {
  quickJumpLinks.value = QUICK_JUMP_SECTIONS.filter(
    section => document.getElementById(section.id)?.firstElementChild,
  )
}

onMounted(() => {
  refreshQuickJumpLinks()
  const container = document.getElementById(QUICK_JUMP_SECTIONS[0].id)?.parentElement
  if (container) {
    quickJumpObserver = new MutationObserver(refreshQuickJumpLinks)
    quickJumpObserver.observe(container, { childList: true, subtree: true })
  }
})

onBeforeUnmount(() => quickJumpObserver?.disconnect())

onMounted(async () => {
  await Promise.all([
    loadColumns(),
    loadForeignKeys(),
    loadTableConfigurationRowId(),
  ])
})
</script>

<template>
  <Dialog ref="createColumnDialog" title="Create column" class="create-column-dialog" @close="closeCreateColumn">
    <p>Add a new column to this table. Choose a name and database type.</p>
    <AddColumn
      :key="`${createColumnName ?? ''}:${createColumnType ?? ''}`"
      :table-id="tableId"
      :initial-name="createColumnName"
      :initial-type="createColumnType"
      :navigate-on-success="false"
      @added="onColumnAdded"
    />
  </Dialog>

  <Section v-if="quickJumpLinks.length > 0" title="On this page" :icon="Link01Icon">
    <nav class="quick-jump" aria-label="Jump to sections on this page">
      <template v-for="(link, index) in quickJumpLinks" :key="link.id">
        <span v-if="index > 0" aria-hidden="true"> · </span>
        <a :href="`#${link.id}`" class="quick-jump-link">
          <HugeiconsIcon :icon="link.icon" width="1em" height="1em" aria-hidden="true" />
          <span>{{ link.label }}</span>
        </a>
      </template>
    </nav>
  </Section>

  <Section
    :padding="false"
    subtitle="View and manage column types, foreign keys, and formatting for this table."
  >
    <template #title>
      <span class="section-title-with-icon">
        <HugeiconsIcon :icon="Settings01Icon" width="22" height="22" aria-hidden="true" />
        Column types: {{ tableId }}
      </span>
    </template>

    <template #toolbar>
      <router-link :to="`/table/${tableId}`" class="button inline-icon neutral">
        <HugeiconsIcon :icon="ArrowLeft01Icon" width="1em" height="1em" aria-hidden="true" />
        <span>{{ backToTableLabel }}</span>
      </router-link>
      <router-link
        v-if="designModeEnabled"
        :to="`/table/${tableId}/foreign-keys`"
        class="button inline-icon neutral design-only"
      >
        <HugeiconsIcon :icon="Edit03Icon" width="1em" height="1em" aria-hidden="true" />
        <span>Foreign keys</span>
      </router-link>
      <router-link
        v-if="designModeEnabled"
        :to="`/table/${tableId}/item-view-layout`"
        class="button inline-icon neutral design-only"
      >
        <HugeiconsIcon :icon="LayoutIcon" width="1em" height="1em" aria-hidden="true" />
        <span>Item view layout</span>
      </router-link>
      <button
        type="button"
        class="inline-icon neutral"
        aria-label="Refresh"
        :disabled="loading"
        @click="refreshAll"
      >
        <HugeiconsIcon :icon="RefreshIcon" width="1em" height="1em" :strokeWidth="iconStrokeWidth" />
      </button>
      <button
        v-if="designModeEnabled"
        type="button"
        class="inline-icon design-only"
        aria-label="Create column"
        :disabled="loading"
        @click="openCreateColumn()"
      >
        <HugeiconsIcon :icon="Add01Icon" width="1em" height="1em" :strokeWidth="iconStrokeWidth" />
      </button>
    </template>

    <div v-if="error" class="list-banner-pad">
      <NotificationBlock type="error" :message="error" />
    </div>
    <div v-if="!designModeEnabled && !loading" class="list-banner-pad inline-notification note">
      Design mode is off. Use the <strong>Design</strong> button in the header to change columns, keys, and layout.
    </div>
    <div v-if="loading && columns.length === 0" class="list-banner-pad muted">Loading columns…</div>

    <template v-else>
      <p v-if="columns.length === 0" class="list-banner-pad inline-notification note">
        This table does not have any columns yet.
      </p>

      <Table
        v-else
        class="columns-table-wrap"
        :data="sortedColumns"
        :headers="tableHeaders"
        row-key="name"
        :filterable="false"
        :column-options="false"
      >
        <template #cell-name="{ row }">
          <template v-if="renamingColumn !== row.name">
            <strong>{{ row.name }}</strong>
          </template>
          <span v-else class="rename-inline">
            <input v-model="newColumnName" type="text" class="rename-input" />
            <button
              type="button"
              class="small good"
              :disabled="loading || !newColumnName.trim()"
              @click="renameColumn(row.name)"
            >
              Save
            </button>
            <button type="button" class="small neutral" :disabled="loading" @click="cancelRename">Cancel</button>
          </span>
        </template>

        <template #cell-tags="{ row }">
          <span class="column-tags">
            <span v-if="row.required" class="tag bad">Required</span>
            <span v-if="isSystemColumn(row.name)" class="tag note">System</span>
          </span>
        </template>

        <template #cell-type="{ row }">
          <div v-if="editingColumn === row.name" class="edit-form">
            <div class="type-selector">
              <label :for="`new-type-${row.name}`">New type</label>
              <div class="type-options">
                <label class="radio-option">
                  <input v-model="useCustomType" type="radio" :value="false" />
                  <span>Common types</span>
                </label>
                <label class="radio-option">
                  <input v-model="useCustomType" type="radio" :value="true" />
                  <span>Custom type</span>
                </label>
              </div>

              <div v-if="!useCustomType">
                <select :id="`new-type-${row.name}`" v-model="newType" class="type-select">
                  <option value="">Select a type</option>
                  <option v-for="type in columnTypes" :key="type.value" :value="type.value">
                    {{ type.label }}
                  </option>
                  <option
                    v-if="!columnTypes.find(t => t.value === getCurrentColumnType(editingColumn))"
                    :value="getCurrentColumnType(editingColumn)"
                    disabled
                  >
                    {{ getCurrentColumnType(editingColumn) }} (current)
                  </option>
                </select>
                <div v-if="newType" class="type-description muted">
                  {{ columnTypes.find(t => t.value === newType)?.description }}
                </div>
              </div>

              <div v-else>
                <input
                  v-model="customType"
                  type="text"
                  placeholder="e.g., VARCHAR(500), DECIMAL(10,3)"
                  class="type-input"
                />
                <div class="type-description muted">
                  Enter the exact database type (e.g., VARCHAR(255), INT(11)).
                </div>
              </div>
            </div>
            <div class="edit-actions">
              <button type="button" class="good small" :disabled="!canChangeType || loading" @click="changeColumnType">
                {{ loading ? 'Changing…' : 'Change type' }}
              </button>
              <button type="button" class="neutral small" :disabled="loading" @click="cancelEdit">Cancel</button>
            </div>
          </div>
          <template v-else>
            <code>{{ row.type }}</code>
            <div
              v-for="fk in foreignKeysForColumn(row.name)"
              :key="fk.constraintName"
              class="fk-info muted"
            >
              FK →
              <router-link :to="`/table/${fk.referencedTable}`">{{ fk.referencedTable }}</router-link>
              ({{ fk.referencedColumn }})
              <span class="fk-action">on delete {{ fk.onDeleteAction }}, on update {{ fk.onUpdateAction }}</span>
              <button
                v-if="designModeEnabled"
                type="button"
                class="xsmall bad"
                title="Drop foreign key"
                @click="showDropFkConfirm = fk.constraintName"
              >
                Drop FK
              </button>
            </div>
          </template>
        </template>

        <template #cell-actions="{ row }">
          <div v-if="editingColumn !== row.name && designModeEnabled" class="actions-cell">
            <button
              v-if="!row.required && !isSystemColumn(row.name)"
              type="button"
              class="small neutral design-only"
              @click="startRename(row.name)"
            >
              Rename
            </button>
            <router-link
              v-if="!row.required && !isSystemColumn(row.name)"
              :to="`/table/${tableId}/foreign-keys?column=${encodeURIComponent(row.name)}`"
              class="button small neutral design-only"
            >
              Add FK
            </router-link>
            <router-link
              v-if="!isSystemColumn(row.name)"
              :to="`/table/${tableId}/conditional-formatting?column=${encodeURIComponent(row.name)}`"
              class="button small neutral"
            >
              Formatting
            </router-link>
            <button
              v-if="!isSystemColumn(row.name)"
              type="button"
              class="small neutral design-only"
              @click="startEdit(row.name)"
            >
              Change type
            </button>
            <button
              v-if="canDropColumn(row.name)"
              type="button"
              class="small bad"
              @click="showDropConfirm = row.name"
            >
              Drop
            </button>
          </div>
          <div v-else-if="editingColumn !== row.name && !designModeEnabled" class="actions-cell">
            <router-link
              v-if="!isSystemColumn(row.name)"
              :to="`/table/${tableId}/conditional-formatting?column=${encodeURIComponent(row.name)}`"
              class="button small neutral"
            >
              Formatting
            </router-link>
          </div>
        </template>
      </Table>
    </template>
  </Section>

  <div id="table-configuration" class="quick-jump-anchor">
  <Section
    v-if="tableConfigurationRowId"
    title="Table configuration"
    :icon="DatabaseSettingIcon"
  >
    <div class="section-content padding">
      <p>Title, icon, row names, default sort and other settings for this table.</p>
      <router-link
        :to="`/table/table_configurations/${tableConfigurationRowId}/edit`"
        class="button inline-icon neutral"
      >
        <HugeiconsIcon :icon="Edit03Icon" width="1em" height="1em" aria-hidden="true" />
        <span>Edit table configuration</span>
      </router-link>
    </div>
  </Section>
  </div>

  <div id="pull-through" class="quick-jump-anchor">
    <TablePullThroughSection
      v-if="!loading && structureForeignKeys.length > 0"
      :table-id="tableId"
      :foreign-keys="structureForeignKeys"
      :client="client"
      :design-mode-enabled="designModeEnabled"
      @saved="loadForeignKeys"
    />
  </div>

  <div id="row-labels" class="quick-jump-anchor">
    <TableRowLabelsEditor
      v-if="!loading"
      :table-id="tableId"
      :client="client"
      :design-mode-enabled="designModeEnabled"
      @saved="loadColumns"
    />
  </div>

  <div id="column-display" class="quick-jump-anchor">
    <ColumnDisplayEditor
      v-if="!loading && columns.length > 0"
      :table-id="tableId"
      :column-names="columnNamesForGrouping"
      :client="client"
    />
  </div>

  <div id="field-grouping" class="quick-jump-anchor">
    <FieldGroupingEditor
      v-if="designModeEnabled && !loading && columns.length > 0"
      :table-id="tableId"
      :column-names="columnNamesForGrouping"
      :client="client"
    />
  </div>

  <!-- Drop Column Confirmation -->
  <Dialog
    :open="showDropConfirm !== null"
    title="Drop Column"
    size="small"
    @update:open="open => { if (!open) showDropConfirm = null }"
  >
    <p>Are you sure you want to drop the column <strong>{{ showDropConfirm }}</strong>?</p>
    <NotificationBlock
      type="warning"
      message="This action cannot be undone and will permanently delete all data in this column."
    />
    <template #actions>
      <button
        type="button"
        :disabled="dropping"
        class="button inline-icon bad"
        @click="dropColumn(showDropConfirm!)"
      >
        <HugeiconsIcon :icon="Delete01Icon" width="1em" height="1em" aria-hidden="true" />
        <span>{{ dropping ? 'Dropping…' : 'Drop column' }}</span>
      </button>
    </template>
  </Dialog>

  <!-- Standard Fields Section -->
  <div id="standard-fields" class="quick-jump-anchor">
  <Section
    v-if="designModeEnabled && !loading && missingStandardFields.length > 0 && columns.length > 0"
    title="Standard Fields"
    :icon="AddCircleIcon"
  >
    <p class="section-description">Standard fields are recommended columns that improve table organization and functionality.</p>
    <div class="standard-fields-list">
      <div v-for="field in missingStandardFields" :key="field.name" class="standard-field-item">
        <div class="field-info">
          <strong class="field-name">{{ field.name }}</strong>
          <p class="field-description">{{ field.description }}</p>
        </div>
        <button type="button" class="button design-only" @click="openCreateColumn(field.name, field.type)">
          <HugeiconsIcon :icon="Edit03Icon" />
          Create "{{ field.name }}" Column
        </button>
      </div>
    </div>
  </Section>
  </div>

  <div id="share" class="quick-jump-anchor">
    <TableShareSection :table-key="tableId" />
  </div>

  <!-- Conditional Formatting Rules Section -->
  <div id="conditional-formatting" class="quick-jump-anchor">
    <ConditionalFormattingRules />
  </div>

  <!-- Drop Foreign Key Confirmation -->
  <Dialog
    :open="showDropFkConfirm !== null"
    title="Drop Foreign Key"
    size="small"
    @update:open="open => { if (!open) showDropFkConfirm = null }"
  >
    <p>Are you sure you want to drop the foreign key <strong>{{ showDropFkConfirm }}</strong>?</p>
    <NotificationBlock
      type="warning"
      message="This action cannot be undone. It removes referential integrity for the linked column."
    />
    <template #actions>
      <button type="button" class="button bad" :disabled="droppingFk" @click="dropForeignKey(showDropFkConfirm!)">
        {{ droppingFk ? 'Dropping…' : 'Drop Foreign Key' }}
      </button>
    </template>
  </Dialog>
</template>

<style scoped>
.quick-jump {
  line-height: 1.5;
}

.quick-jump-link {
  display: inline-flex;
  align-items: center;
  gap: 0.3em;
}

.quick-jump-anchor {
  scroll-margin-top: 4rem;
}

.section-title-with-icon {
  display: inline-flex;
  align-items: center;
  gap: 0.45em;
  vertical-align: middle;
}

.list-banner-pad {
  padding-left: 1em;
  padding-right: 1em;
}

.columns-table-wrap {
  margin-top: 0.5rem;
  margin-bottom: 1.5rem;
}

.actions-cell {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 0.35rem;
}

.column-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem;
}

.rename-inline {
  display: inline-flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.35rem;
}

.rename-input {
  min-width: 10rem;
}

.section-description {
  margin: 0 0 1rem 0;
  color: #666;
  font-size: 0.9rem;
}

.standard-fields-list {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.standard-field-item {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  background: var(--background-color);
  color: var(--text-color);
  border: 1px solid var(--border-color);
  border-radius: 4px;
  padding: 1rem;
  gap: 1rem;
}

.field-info {
  flex: 1;
}

.field-name {
  display: block;
  color: var(--text-color);
  font-size: 1rem;
  margin-bottom: 0.5rem;
}

.field-description {
  margin: 0;
  color: #666;
  font-size: 0.9rem;
  line-height: 1.4;
}

.fk-info {
  margin-top: 0.35rem;
  font-size: 0.9rem;
}

.fk-info .fk-action {
  margin-left: 0.35rem;
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

</style>
