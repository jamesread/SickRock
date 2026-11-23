<script setup lang="ts">
import RadioGroup from 'picocrank/vue/components/RadioGroup.vue'
import { ref, computed, watch, onMounted, nextTick } from 'vue'

import { createApiClient } from '../stores/api'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import FormLayout from 'picocrank/vue/components/FormLayout.vue'
import { SickRock } from '../gen/sickrock_pb'
import { useIamUsers } from '../composables/useIamUsers'
import { booleanFromStored, booleanToStored, isBarcodeFieldType, isBooleanFieldType, isIconFieldType, isImageFieldType, isTrueBooleanValue, isUserRefFieldType, YES_NO_OPTIONS } from '../utils/fieldTypes'
import IconPickerField from './IconPickerField.vue'
import ImageField from './ImageField.vue'
import BarcodeField from './BarcodeField.vue'
import { fieldDisplayLabel } from '../utils/fieldColumnDisplay'
import { toSqlDateTime } from '../utils/dateFormatting'
import Combobox from 'picocrank/vue/components/Combobox.vue'
import RowPickerDialog from './RowPickerDialog.vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { Search01Icon } from '@hugeicons/core-free-icons'
import { useRowPicker } from '../composables/useRowPicker'
import { useAuthStore } from '../stores/auth'
import { defaultExportViewOptions } from '../utils/exportView'
import {
  pickerLabel,
  recentPicks,
  recentPicksKey,
  rememberPick,
  type RowPickerOption,
} from '../utils/rowPicker'
import type { Field, Item, ItemFieldGroup } from '../gen/sickrock_pb'

type InsertFieldDef = { name: string; type: string; required: boolean; displayLabel?: string }

const props = defineProps<{
  tableId: string,
  fieldDefs: InsertFieldDef[],
  fieldGroups?: ItemFieldGroup[],
  selectedDate?: string | null,
  // Edit mode props
  editMode?: boolean,
  itemId?: string,
  existingItem?: Record<string, unknown> | null,
  initialValues?: Record<string, string> | null
  // Show Cancel when creating too (e.g. a form inside a dialog)
  cancellable?: boolean
}>()
const emit = defineEmits<{
  created: [item?: Item],
  updated: [],
  cancelled: []
}>()


const form = ref<Record<string, any>>({})
const loading = ref(false)
const error = ref<string | null>(null)

// Edit mode state
const isEditMode = computed(() => props.editMode || false)
const saving = ref(false)

function fieldLabel(field: InsertFieldDef): string {
  return fieldDisplayLabel(
    { displayLabel: field.displayLabel ?? '' } as Field,
    field.name,
  )
}

const orderedFields = computed(() => {
  const fields = props.fieldDefs.filter(field => field.name !== 'id')
  const groups = [...(props.fieldGroups ?? [])].sort((a, b) => (a.ordinal ?? 0) - (b.ordinal ?? 0))
  if (!groups.length) {
    return fields.map(field => ({ field, heading: '' }))
  }
  const byName = new Map(fields.map(field => [field.name, field]))
  const used = new Set<string>()
  const rows: Array<{ field: InsertFieldDef; heading: string }> = []
  for (const group of groups) {
    const title = group.title?.trim() ?? ''
    let first = true
    for (const columnName of group.columnNames ?? []) {
      const field = byName.get(columnName)
      if (!field || used.has(field.name)) continue
      used.add(field.name)
      rows.push({ field, heading: first && title ? title : '' })
      first = false
    }
  }
  for (const field of fields) {
    if (!used.has(field.name)) {
      rows.push({ field, heading: '' })
    }
  }
  return rows
})

const { users, loading: loadingUsers, loadUsers } = useIamUsers()
const auth = useAuthStore()

// Foreign key lookup state
const foreignKeys = ref<Array<{
  constraintName: string
  tableName: string
  columnName: string
  referencedTable: string
  referencedColumn: string
  onDeleteAction: string
  onUpdateAction: string
  tableTcName?: string
  referencedTableTcName?: string
}>>([])

const referencedTableData = ref<Record<string, any[]>>({})
const loadingForeignKeys = ref(false)
const initialApplied = ref(false)

/** An option of picocrank's Combobox. */
type ComboboxOption = { value: string; label: string; description?: string }

// The search box options of each relation field (the referenced rows), sorted by name.
const relationOptions = computed<Record<string, ComboboxOption[]>>(() => Object.fromEntries(
  Object.entries(referencedTableData.value).map(([fieldName, rows]) => [fieldName, rows
    .map(row => ({ value: String(row.id), label: pickerLabel(row), description: `#${row.id}` }))
    .sort((a, b) => a.label.localeCompare(b.label, undefined, { numeric: true }))]),
))
// The search box options of user fields.
const userComboboxOptions = computed<ComboboxOption[]>(() => userOptions.value
  .map(option => ({ value: option.value, label: option.label })))

// The Browse… dialog, shared by every relation and user field: pickerField is the field it is open for.
const rowPicker = ref<InstanceType<typeof RowPickerDialog> | null>(null)
const pickerField = ref('')
const pickerTable = computed(() => {
  const fk = foreignKeys.value.find(entry => entry.columnName === pickerField.value)
  return fk ? (fk.referencedTableTcName || fk.referencedTable) : ''
})
const picker = useRowPicker(pickerTable)
const pickingUser = computed(() => Boolean(pickerField.value) && isUserRefField(pickerField.value))
const userRecentKey = computed(() => recentPicksKey(auth.user?.username ?? '', 'users'))
const userRecent = ref<string[]>([])
const userOptions = computed<RowPickerOption[]>(() => users.value
  .filter(user => user.id != null)
  .map(user => ({ value: String(user.id), label: user.username ?? `#${user.id}`, imageId: '' }))
  .sort((a, b) => a.label.localeCompare(b.label)))
const pickerFieldDef = computed(() => props.fieldDefs.find(field => field.name === pickerField.value))
const pickerTitle = computed(() => (pickerFieldDef.value ? `Choose ${fieldLabel(pickerFieldDef.value)}` : 'Choose'))

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

// Transport handled by authenticated client
const client = createApiClient()

// Load foreign key information for the current table
async function loadForeignKeys() {
  try {
    loadingForeignKeys.value = true
    const response = await client.getTableStructure({ pageId: props.tableId })
    foreignKeys.value = (response.foreignKeys || []).map(fk => ({
      constraintName: fk.constraintName,
      tableName: fk.tableName,
      columnName: fk.columnName,
      referencedTable: fk.referencedTable,
      referencedColumn: fk.referencedColumn,
      onDeleteAction: fk.onDeleteAction,
      onUpdateAction: fk.onUpdateAction,
      tableTcName: fk.tableTcName,
      referencedTableTcName: fk.referencedTableTcName
    }))

    const fallbacks = FALLBACK_FK_BY_TABLE[props.tableId]
    if (fallbacks) {
      for (const [columnName, referencedTcName] of Object.entries(fallbacks)) {
        if (!foreignKeys.value.some(fk => fk.columnName === columnName)) {
          foreignKeys.value.push({
            constraintName: `fallback_${columnName}`,
            tableName: props.tableId,
            columnName,
            referencedTable: referencedTcName,
            referencedColumn: 'id',
            onDeleteAction: 'SET NULL',
            onUpdateAction: 'NO ACTION',
            referencedTableTcName: referencedTcName
          })
        }
      }
    }

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
      // Use referencedTableTcName if available, otherwise fall back to referencedTable
      const tcName = fk.referencedTableTcName || fk.referencedTable
      const response = await client.listItems({ tcName })
      data[fk.columnName] = response.items || []
    } catch (err) {
      console.error(`Error loading data for table ${fk.referencedTable}:`, err)
      data[fk.columnName] = []
    }
  }

  referencedTableData.value = data
}

/** Nullable FK columns on built-in tables (used when DB driver omits FK metadata, e.g. SQLite). */
const KNOWN_NULLABLE_FK_COLUMNS = new Set(['workflow_id', 'dashboard_id', 'table_configuration'])

/**
 * Columns of built-in tables that hold one of a table's views, by table then column, with the column
 * naming that table (e.g. a table configuration's default export view is a view of the table it
 * configures). They are picked from that table's views (utils/exportView.ts).
 */
const TABLE_VIEW_FIELDS: Record<string, Record<string, string>> = {
  table_configurations: { default_export_view_id: 'name' },
}

function tableViewFieldSource(fieldName: string): string {
  return TABLE_VIEW_FIELDS[props.tableId]?.[fieldName] ?? ''
}

// The views of the tables those columns refer to, by table name.
const viewsByTable = ref<Record<string, Array<{ id: number; viewName: string; isDefault: boolean }>>>({})

function tableViewOptions(fieldName: string) {
  const table = String(form.value[tableViewFieldSource(fieldName)] ?? '').trim()
  return defaultExportViewOptions(viewsByTable.value[table] ?? [])
}

watch(() => Object.values(TABLE_VIEW_FIELDS[props.tableId] ?? {})
  .map(source => String(form.value[source] ?? '').trim()), async (tables) => {
  for (const table of tables) {
    if (!table || viewsByTable.value[table]) continue
    try {
      const res = await client.getTableViews({ tableName: table })
      viewsByTable.value = {
        ...viewsByTable.value,
        [table]: (res.views ?? []).map(view => ({ id: view.id, viewName: view.viewName, isDefault: view.isDefault })),
      }
    } catch (err) {
      console.error(`Error loading views of ${table}:`, err)
    }
  }
}, { immediate: true })

const FALLBACK_FK_BY_TABLE: Record<string, Record<string, string>> = {
  table_navigation: {
    workflow_id: 'table_workflows',
    dashboard_id: 'table_dashboards',
    table_configuration: 'table_configurations'
  },
  table_app_navigation_links: {
    app_id: 'table_apps',
    navigation_item_id: 'table_navigation',
  },
  table_dashboard_components: {
    dashboard: 'table_dashboards',
    tc_id: 'table_configurations'
  }
}

// Check if a field is a foreign key
function isForeignKey(fieldName: string): boolean {
  return foreignKeys.value.some(fk => fk.columnName === fieldName)
    || KNOWN_NULLABLE_FK_COLUMNS.has(fieldName)
}

function isUserRefField(fieldName: string): boolean {
  const fieldDef = getFieldDef(fieldName)
  return fieldDef ? isUserRefFieldType(fieldDef.type) : false
}

function getFieldDef(fieldName: string) {
  return props.fieldDefs.find(f => f.name === fieldName)
}

/** A relation or user field's value for its search box: null for none (stored as '' or '0'). */
function choiceValue(fieldName: string): string | null {
  const value = form.value[fieldName]
  return value == null || value === '' || String(value) === '0' ? null : String(value)
}

/** Sets a relation or user field from its search box (null clears it). */
function setChoice(fieldName: string, value: string | number | null) {
  form.value[fieldName] = value == null ? '' : String(value)
}

function isIntegerLikeFieldType(fieldType: string): boolean {
  const t = fieldType.toLowerCase()
  return t.includes('int') || t === 'integer' || t === 'bigint'
}

/** Value sent to API to clear a nullable column (server maps "" and "0" to NULL for FK ints). */
function clearedFieldPayloadValue(field: { name: string; type: string }): string {
  if (isForeignKey(field.name) || isUserRefField(field.name) || isImageFieldType(field.type) || isIntegerLikeFieldType(field.type)) {
    return '0'
  }
  return ''
}

function hadStoredValue(fieldName: string): boolean {
  if (!props.existingItem?.additionalFields) return false
  const orig = props.existingItem.additionalFields[fieldName]
  if (orig == null || orig === '') return false
  return String(orig) !== '0'
}

function shouldSendClearedFieldOnEdit(fieldName: string, value: unknown): boolean {
  if (!isEditMode.value) return false
  if (value != null && value !== '') return false
  // Only send NULL/empty clears when the row previously had a value (avoids wiping FKs on unrelated edits)
  return hadStoredValue(fieldName)
}

/** Opens the Browse… dialog for a relation or user field. */
async function openPicker(fieldName: string, initialQuery = '') {
  pickerField.value = fieldName
  await nextTick()
  rowPicker.value?.open(initialQuery)
  if (isUserRefField(fieldName)) {
    userRecent.value = recentPicks(userRecentKey.value)
    void loadUsers()
  } else {
    await picker.load(referencedTableData.value[fieldName])
  }
}

function onPickerPick(option: RowPickerOption) {
  const fieldName = pickerField.value
  if (!fieldName) return
  if (isUserRefField(fieldName)) {
    setChoice(fieldName, option.value || null)
    if (option.value) userRecent.value = rememberPick(userRecentKey.value, option.value)
    return
  }
  setChoice(fieldName, option.value || null)
  if (option.value) picker.remember(option.value)
}

/** A row created from the dialog is added to the field's options and chosen. */
async function onPickerCreated(item: Item | undefined) {
  const fieldName = pickerField.value
  if (!fieldName) return
  try {
    referencedTableData.value = { ...referencedTableData.value, [fieldName]: await picker.reloadRows() }
  } catch (err) {
    console.error('Error reloading related rows:', err)
  }
  if (item) {
    setChoice(fieldName, String(item.id))
    picker.remember(String(item.id))
  }
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
    applyInitialValues()
  }
})

// Relation fields take their ids straight away: their search boxes show the names once the
// referenced rows have loaded.
function applyInitialValues() {
  if (isEditMode.value || !props.initialValues) return
  let appliedAny = false
  for (const [k, v] of Object.entries(props.initialValues)) {
    // Only set if not already filled by user or prior application
    const current = form.value[k]
    if (current == null || current === '') {
      form.value[k] = v
      appliedAny = true
    }
  }
  if (appliedAny) initialApplied.value = true
}

// Also react to initialValues arriving after mount (race with parent loading)
watch(() => props.initialValues, (val) => {
  if (val && !isEditMode.value && !initialApplied.value) {
    applyInitialValues()
  }
}, { immediate: true })

// Re-apply when field definitions are ready (some parents compute these async)
watch(() => props.fieldDefs, (defs) => {
  // User fields list the users; parents may pass the field definitions after mount.
  if (defs?.some(field => isUserRefFieldType(field.type))) {
    void loadUsers()
  }
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
    if (isBooleanFieldType(f.type)) {
      // An unticked box is false, so it is sent rather than left to the column default.
      payload[f.name] = isTrueBooleanValue(v) ? '1' : '0'
      continue
    }
    if (v == null || v === '') {
      if (shouldSendClearedFieldOnEdit(f.name, v)) {
        payload[f.name] = clearedFieldPayloadValue(f)
      }
      continue
    }

    if (f.type === 'datetime' || f.type === 'timestamp') {
      // For datetime fields, convert to MySQL-compatible format (YYYY-MM-DD HH:MM:SS); skip invalid dates
      const sqlDateTime = toSqlDateTime(v)
      if (sqlDateTime) {
        payload[f.name] = sqlDateTime
      }
    } else if (
      f.type === 'int64'
      || f.type === 'int'
      || f.type === 'bigint'
      || f.type === 'integer'
      || isUserRefFieldType(f.type)
      || isImageFieldType(f.type)
    ) {
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
      const created = await client.createItem(createRequest)
      form.value = {}
      emit('created', created.item)
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
    <FormLayout @submit.prevent="submit">
      <template v-for="{ field: f, heading } in orderedFields" :key="f.name">
        <template v-if="f.name !== 'id'">
            <h3 v-if="heading" class="field-group-heading">{{ heading }}</h3>
            <label :for="'field-' + f.name">{{ fieldLabel(f) }}<span v-if="f.required" aria-label="required" title="required">*</span></label>

            <!-- Icon picker -->
            <IconPickerField
              v-if="isIconFieldType(f.type)"
              :id="'field-' + f.name"
              v-model="form[f.name]"
              :clearable="!f.required"
            />

            <!-- Image: upload or camera capture -->
            <ImageField
              v-else-if="isImageFieldType(f.type)"
              :id="'field-' + f.name"
              v-model="form[f.name]"
              :table-id="tableId"
              :column-name="f.name"
              :clearable="!f.required"
            />

            <!-- Barcode: type, barcode scanner or camera -->
            <BarcodeField
              v-else-if="isBarcodeFieldType(f.type)"
              :id="'field-' + f.name"
              v-model="form[f.name]"
              :placeholder="fieldLabel(f)"
              @enter="submit"
            />

            <!-- Yes / no -->
            <RadioGroup
              v-else-if="isBooleanFieldType(f.type)"
              variant="boolean"
              :options="YES_NO_OPTIONS"
              :id="'field-' + f.name"
              :model-value="booleanFromStored(form[f.name])"
              :aria-label="fieldLabel(f)"
              @update:model-value="value => { form[f.name] = value === null ? '' : booleanToStored(value) }"
            />

            <!-- A view of the table another column names (e.g. a table's default export view). -->
            <select
              v-else-if="tableViewFieldSource(f.name)"
              :id="'field-' + f.name"
              :value="Number(form[f.name] || 0)"
              @change="form[f.name] = ($event.target as HTMLSelectElement).value"
            >
              <option v-for="option in tableViewOptions(f.name)" :key="option.value" :value="option.value">{{ option.label }}</option>
            </select>

            <!-- User and relation fields: a search box (picocrank Combobox), with Browse… for the full picker. -->
            <Combobox
              v-else-if="isUserRefField(f.name) || isForeignKey(f.name)"
              :id="'field-' + f.name"
              :model-value="choiceValue(f.name)"
              :options="isUserRefField(f.name) ? userComboboxOptions : (relationOptions[f.name] ?? [])"
              :placeholder="isUserRefField(f.name) ? `Search users for ${fieldLabel(f)}…` : `Search ${fieldLabel(f)}…`"
              :loading="isUserRefField(f.name) ? loadingUsers : loadingForeignKeys"
              :clearable="!f.required"
              @update:model-value="(value: string | number | null) => setChoice(f.name, value)"
              @more="(query: string) => openPicker(f.name, query)"
            >
              <template #actions="{ query }">
                <button
                  type="button"
                  class="neutral inline-icon"
                  :aria-label="`Browse ${fieldLabel(f)}`"
                  @click="openPicker(f.name, choiceValue(f.name) ? '' : query)"
                >
                  <HugeiconsIcon :icon="Search01Icon" width="1em" height="1em" aria-hidden="true" />
                  <span>Browse…</span>
                </button>
              </template>
            </Combobox>

            <!-- Regular text input -->
            <input
              v-else-if="f.type === 'string' || f.type === 'varchar' || f.type === 'text'"
              v-model="form[f.name]"
              :placeholder="fieldLabel(f)"
              :id="'field-' + f.name"
              type="text"
              @keyup.enter="submit"
            />
            <!-- Number input -->
            <input
              v-else-if="f.type === 'int64' || f.type === 'int' || f.type === 'bigint' || f.type === 'integer' || f.type === 'float'"
              v-model.number="form[f.name]"
              :placeholder="fieldLabel(f)"
              :id="'field-' + f.name"
              type="number"
              :step="f.type === 'float' ? '0.01' : '1'"
              @keyup.enter="submit"
            />
            <!-- DateTime input -->
            <input
              v-else-if="f.type === 'datetime' || f.type === 'timestamp'"
              v-model="form[f.name]"
              :placeholder="fieldLabel(f)"
              :id="'field-' + f.name"
              type="datetime-local"
              @keyup.enter="submit"
            />
            <!-- Fallback text input -->
            <input
              v-else
              v-model="form[f.name]"
              :placeholder="fieldLabel(f)"
              :id="'field-' + f.name"
              type="text"
              @keyup.enter="submit"
            />
        </template>
      </template>

      <template #actions>
        <button
          v-if="isEditMode || cancellable"
          type="button"
          class="neutral"
          @click="cancel"
          :disabled="loading || saving"
        >
          Cancel
        </button>
        <button
          type="submit"
          class="good"
          :disabled="loading || saving"
        >
          {{ isEditMode ? (saving ? 'Saving...' : 'Save Changes') : (loading ? 'Adding...' : 'Add') }}
        </button>
      </template>
    </FormLayout>
    <NotificationBlock v-if="error" type="error" :message="error" />
    <!-- Browse… for relation and user fields; outside the form, as it can hold a create form of its own. -->
    <RowPickerDialog
      ref="rowPicker"
      :title="pickerTitle"
      :options="pickingUser ? userOptions : picker.options.value"
      :layout="pickingUser ? 'list' : picker.layout.value"
      :selected-value="pickerField ? String(form[pickerField] ?? '') : ''"
      :allow-none="Boolean(pickerFieldDef && !pickerFieldDef.required)"
      :recent-values="pickingUser ? userRecent : picker.recent.value"
      :loading="pickingUser ? loadingUsers : picker.loading.value"
      :create-label="pickingUser ? '' : picker.createLabel.value"
      :create-table-id="pickingUser ? '' : pickerTable"
      :create-field-defs="picker.settings.value.fieldDefs"
      @pick="onPickerPick"
      @created="onPickerCreated"
    />
  </div>
</template>



<style scoped>
.field-group-heading {
  grid-column: 1 / -1;
  margin: 0.75rem 0 0;
  font-size: 1rem;
}
</style>
