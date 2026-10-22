<script setup lang="ts">
import { ref, shallowRef, onMounted, onUnmounted, computed, inject, provide, nextTick, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import type { Field, ForeignKey, GetTableStructureResponse, Item } from '../gen/sickrock_pb'
import { HugeiconsIcon } from '@hugeicons/vue'
import {
  ArrowLeft01Icon,
  Cancel01Icon,
  DatabaseIcon,
  Delete01Icon,
  ViewIcon,
  Link01Icon,
  LayoutIcon,
  GridIcon,
} from '@hugeicons/core-free-icons'
import FieldGroupingEditor from '../components/FieldGroupingEditor.vue'
import { syntheticColumnNamesForSourceTable } from '../utils/relationPullThrough'
import Section from 'picocrank/vue/components/Section.vue'
import LoadingArea from 'picocrank/vue/components/LoadingArea.vue'
import Tabs from 'picocrank/vue/components/Tabs.vue'
import RelatedRelationPanel, { type RelatedTableEntry } from './RelatedRelationPanel.vue'
import DangerZone from 'picocrank/vue/components/DangerZone.vue'
import FormField from 'picocrank/vue/components/FormField.vue'
import type { createApiClient } from '../stores/api'
import { formatUnixTimestamp } from '../utils/dateFormatting'
import { useTableAccess } from '../composables/useTableAccess'
import { useIamUsers } from '../composables/useIamUsers'
import {
  findFieldDef,
  hasStoredUserRefValue,
  isUserRefColumn as columnIsUserRef,
  isUserRefFieldType,
} from '../utils/fieldTypes'
import { appTitle, formatDocumentTitle } from '../appTitle'
import { rowEditActionLabel, rowNamePluralLabel, tableListLinkLabel } from '../utils/rowInsertLabel'
import { getItemNameFieldValue, itemViewSectionTitle } from '../utils/itemViewSectionTitle'
import { clearRowViewPageLabel, rowViewPageLabel } from '../utils/rowViewPageTitle'
import {
  relatedRowsStorageKey,
  relationSourceTableTcName,
  shouldShowRelatedRelationPanel,
} from '../utils/rowViewRelatedRelations'
import type { RelationPullThroughField } from '../gen/sickrock_pb'
import {
  buildRelationExtraFieldDefs,
  enrichItemsWithPullThrough,
  pullThroughSpecsFromRelationColumns,
  type CalculatedColumnSpec,
  type ReferencedPullThroughContext,
} from '../utils/relationPullThrough'
import {
  applyFieldValueAffixes,
  fieldDisplayLabel,
  fieldHasValueAffixes,
  foreignKeyReferencedDisplayLabel,
  mergeItemFieldRecord,
} from '../utils/fieldColumnDisplay'
import { useDesignMode } from '../composables/useDesignMode'

const { designModeEnabled } = useDesignMode()
import { Accordion, AccordionItem } from '../components/accordion'
import RowViewFieldEntries from './RowViewFieldEntries.vue'
import AbsorbedRelationFields from './AbsorbedRelationFields.vue'
import { whenHugeiconAvailable, type IconData } from '../utils/hugeicons'
import { rowViewFieldUiKey } from './rowViewFieldUi'
import { orderBySchema } from '../utils/schemaFieldOrder'

type FieldEntry = {
  key: string
  displayValue: unknown
  valueClass: string
}

const NON_EDITABLE_FIELDS = new Set([
  'id',
  'srCreated',
  'srUpdated',
  'sr_created',
  'sr_updated',
  'srCreatedRelative',
  'srUpdatedRelative',
])

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
const { access: tableAccess } = useTableAccess(tableName)
const { users, loadUsers, displayUserLabel } = useIamUsers()

const tableStructure = ref<GetTableStructureResponse | null>(null)
const fieldDefs = computed<Field[]>(() => tableStructure.value?.fields ?? [])
const primaryKeyColumn = computed(() => tableStructure.value?.primaryKeyColumn ?? 'id')

const editingField = ref<string | null>(null)
const editingValue = ref('')
const savingField = ref(false)
const fieldError = ref<string | null>(null)
const editInput = ref<HTMLInputElement | HTMLSelectElement | null>(null)

function setEditInput(element: unknown) {
  editInput.value = element instanceof HTMLInputElement || element instanceof HTMLSelectElement
    ? element
    : null
}

const item = ref<Record<string, unknown> | null>(null)
const loading = ref(false)
const error = ref<string | null>(null)
const showDeleteConfirm = ref(false)
const deleting = ref(false)

// Foreign key and related rows state
const foreignKeys = ref<Array<{
  relationId: number
  constraintName: string
  tableName: string
  columnName: string
  referencedTable: string
  referencedColumn: string
  onDeleteAction: string
  onUpdateAction: string
  displayName: string
  tableTcName?: string
  referencedTableTcName?: string
  pullThroughFields: RelationPullThroughField[]
  absorbIntoItem: boolean
  absorbType: string
}>>([])
const relatedChildStructures = ref<Record<string, GetTableStructureResponse>>({})
const relatedPullThroughExtraFields = ref<Record<string, Field[]>>({})
const fkReferencedItems = ref<Record<string, Item[]>>({})
const relationDialog = ref<HTMLDialogElement | null>(null)
const managingConstraintName = ref('')
const relationDisplayNameDraft = ref('')
const relationAbsorbDraft = ref(false)
const relationAbsorbTypeDraft = ref('first_row')
const relationPullThroughDraft = ref<Array<{ fkColumn: string; field: string }>>([])
const relationCalculatedDraft = ref<CalculatedColumnSpec[]>([])
const relationChildForeignKeys = ref<ForeignKey[]>([])
const relationChildFields = ref<Field[]>([])
const relationReferencedFieldsByFk = ref<Record<string, Field[]>>({})
const relationDialogLoading = ref(false)
const savingRelation = ref(false)
const relationSaveError = ref<string | null>(null)
const relatedRows = ref<Record<string, any[]>>({})
const relatedTableRowNames = ref<Record<string, string>>({})
// Loaded icons of related table configurations (by tc name); missing until loaded or when unset.
const relatedTableIcons = shallowRef<Record<string, IconData>>({})
let relatedTableIconNames: Promise<Map<string, string>> | null = null

/** Loads each related table configuration's icon on demand (falls back to the default icon). */
async function loadRelatedTableIcons(tcNames: string[]) {
  relatedTableIconNames ??= client!.getTableConfigurations({})
    .then(res => new Map((res.pages ?? []).map(page => [page.id, page.icon])))
    .catch(() => {
      relatedTableIconNames = null
      return new Map<string, string>()
    })
  const iconNames = await relatedTableIconNames
  for (const tcName of tcNames) {
    if (relatedTableIcons.value[tcName]) continue
    whenHugeiconAvailable(iconNames.get(tcName), (icon) => {
      relatedTableIcons.value = { ...relatedTableIcons.value, [tcName]: icon }
    })
  }
}
const loadingRelated = ref(false)

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
  if (isUserRefColumn(key)) return {}
  const fk = foreignKeys.value.find(entry => entry.columnName === key)
  if (!fk) return {}
  const raw = resolveRawFieldValue(key)
  if (raw === null || raw === undefined || raw === '') return {}
  const id = String(raw)
  const tableRoute = fk.referencedTableTcName || fk.referencedTable
  const items = fkReferencedItems.value[key] ?? []
  const match = items.find(entry => String(entry.id) === id)
  const label = match
    ? foreignKeyReferencedDisplayLabel(match as unknown as Record<string, unknown>, id)
    : id
  return { to: `/table/${tableRoute}/${id}`, label }
}

function isForeignKey(columnName: string): boolean {
  return foreignKeys.value.some(fk => fk.columnName === columnName)
}

function fieldType(column: string): string {
  return findFieldDef(fieldDefs.value, column)?.type ?? 'unknown'
}

function isUserRefColumn(column: string): boolean {
  return columnIsUserRef(column, fieldDefs.value, foreignKeys.value)
}

function isTinyintColumn(column: string): boolean {
  const type = fieldType(column).toLowerCase()
  return type.startsWith('tinyint') || type === 'bool' || type === 'boolean'
}

function isDatetimeColumn(column: string): boolean {
  const type = fieldType(column).toLowerCase()
  return type === 'datetime' || type.startsWith('datetime(')
}

function isIntegerLikeFieldType(fieldTypeName: string): boolean {
  const t = fieldTypeName.toLowerCase()
  return t.includes('int') || t === 'integer' || t === 'bigint'
}

function getFieldDef(fieldName: string): Field | undefined {
  return findFieldDef(fieldDefs.value, fieldName)
}

function withValueAffixes(key: string, displayValue: unknown, valueClass: string): unknown {
  if (
    valueClass === 'markdown-content'
    || valueClass === 'json'
    || valueClass === 'boolean-true'
    || valueClass === 'boolean-false'
  ) {
    return displayValue
  }
  if (isUserRefColumn(key) || isForeignKey(key)) {
    return displayValue
  }
  const field = getFieldDef(key)
  if (!fieldHasValueAffixes(field)) {
    return displayValue
  }
  const record = mergeItemFieldRecord(item.value as Record<string, unknown> | null)
  return applyFieldValueAffixes(String(displayValue), field, record, resolveAffixPlaceholder)
}

function resolveAffixPlaceholder(columnName: string, rawValue: unknown): string {
  if (rawValue === null || rawValue === undefined || rawValue === '') return ''
  const fk = foreignKeyByColumn.value[columnName]
  if (fk) {
    const items = fkReferencedItems.value[columnName] ?? []
    const match = items.find(entry => String(entry.id) === String(rawValue))
    if (match) {
      return foreignKeyReferencedDisplayLabel(
        match as unknown as Record<string, unknown>,
        rawValue,
      )
    }
  }
  return String(rawValue)
}

function getBooleanValue(column: string): boolean {
  const value = resolveRawFieldValue(column)
  return value === true || Number(value) === 1
}

function canEditField(key: string, valueClass: string): boolean {
  if (!tableAccess.value.canEdit) return false
  if (valueClass === 'markdown-content') return false
  if (NON_EDITABLE_FIELDS.has(key)) return false
  if (key === primaryKeyColumn.value) return false
  if (isForeignKey(key)) return false
  return true
}

function showFullEditIcon(key: string, valueClass: string): boolean {
  if (!tableAccess.value.canEdit) return false
  if (canEditField(key, valueClass)) return false
  if (NON_EDITABLE_FIELDS.has(key)) return false
  if (key === primaryKeyColumn.value) return false
  return true
}

function showReadOnlyIcon(key: string, valueClass: string): boolean {
  return !canEditField(key, valueClass) && !showFullEditIcon(key, valueClass)
}

function fullEditTo(): string {
  return `/table/${tableName}/${rowId}/edit`
}

function fullEditLabel(): string {
  return editButtonText.value
}

function mysqlDatetime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hours = String(date.getHours()).padStart(2, '0')
  const minutes = String(date.getMinutes()).padStart(2, '0')
  const seconds = String(date.getSeconds()).padStart(2, '0')
  return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`
}

function buildAdditionalFieldsPayload(): Record<string, string> {
  const additionalFields: Record<string, string> = {}
  for (const [key, value] of Object.entries(item.value?.additionalFields ?? {})) {
    additionalFields[key] = String(value)
  }
  return additionalFields
}

function startEdit(key: string, valueClass: string) {
  if (!canEditField(key, valueClass)) return
  const value = resolveRawFieldValue(key)
  editingField.value = key
  fieldError.value = null

  if (isTinyintColumn(key)) {
    editingValue.value = getBooleanValue(key) ? '1' : '0'
    return
  }
  if (isDatetimeColumn(key) && value != null && value !== '') {
    const date = new Date(value as string | number)
    editingValue.value = Number.isNaN(date.getTime()) ? '' : date.toISOString().slice(0, 16)
  } else {
    editingValue.value = value == null || value === '' ? '' : String(value)
  }

  void nextTick(() => {
    editInput.value?.focus()
    if (editInput.value instanceof HTMLInputElement) {
      editInput.value.select()
    }
  })
}

function cancelEdit() {
  editingField.value = null
  editingValue.value = ''
}

async function saveField() {
  if (!editingField.value || savingField.value || !item.value) return
  const column = editingField.value
  savingField.value = true
  fieldError.value = null
  try {
    const additionalFields = buildAdditionalFieldsPayload()
    let newValue = editingValue.value
    if (isDatetimeColumn(column)) {
      newValue = mysqlDatetime(editingValue.value)
    } else if (
      editingValue.value === ''
      && (isUserRefColumn(column) || isIntegerLikeFieldType(fieldType(column)))
      && !getFieldDef(column)?.required
    ) {
      newValue = '0'
    }
    additionalFields[column] = newValue

    const res = await client.editItem({
      id: rowId,
      additionalFields,
      pageId: tableName,
    })
    item.value = res.item as Record<string, unknown> ?? null
    cancelEdit()
    if (item.value) {
      await loadRelatedRows()
    }
  } catch (e) {
    fieldError.value = String(e)
  } finally {
    savingField.value = false
  }
}

watch(
  fieldDefs,
  defs => {
    if (defs.some(field => isUserRefFieldType(field.type))) {
      void loadUsers()
    }
  },
  { immediate: true },
)

// Load table structure and foreign keys
async function loadTableStructure() {
  try {
    const response = await client.getTableStructure({ pageId: tableName })
    tableStructure.value = response
    foreignKeys.value = (response.foreignKeys || []).map(fk => ({
      relationId: fk.relationId ?? 0,
      constraintName: fk.constraintName,
      tableName: fk.tableName,
      columnName: fk.columnName,
      referencedTable: fk.referencedTable,
      referencedColumn: fk.referencedColumn,
      onDeleteAction: fk.onDeleteAction,
      onUpdateAction: fk.onUpdateAction,
      displayName: fk.displayName ?? '',
      tableTcName: fk.tableTcName,
      referencedTableTcName: fk.referencedTableTcName,
      pullThroughFields: (fk.pullThroughFields ?? []).map(pt => ({ ...pt })),
      absorbIntoItem: fk.absorbIntoItem,
      absorbType: fk.absorbType,
    }))
    if (response.fields?.some(field => isUserRefColumn(field.name))) {
      void loadUsers()
    }
    await loadFkReferencedItems()
  } catch (err) {
    console.error('Error loading table structure:', err)
  }
}

async function ensureChildTableStructure(childTcName: string): Promise<GetTableStructureResponse> {
  const cached = relatedChildStructures.value[childTcName]
  if (cached) {
    return cached
  }
  const structure = await client.getTableStructure({ pageId: childTcName })
  relatedChildStructures.value = {
    ...relatedChildStructures.value,
    [childTcName]: structure,
  }
  return structure
}

async function loadFkReferencedItems() {
  const data: Record<string, Item[]> = {}
  for (const fk of foreignKeys.value) {
    try {
      const tcName = fk.referencedTableTcName || fk.referencedTable
      const res = await client.listItems({ tcName })
      data[fk.columnName] = res.items ?? []
    } catch (err) {
      console.error(`Error loading FK target ${fk.referencedTable}:`, err)
      data[fk.columnName] = []
    }
  }
  fkReferencedItems.value = data
}

// Load related rows based on foreign key relationships (bidirectional)
async function loadRelatedRows() {
  if (!item.value || foreignKeys.value.length === 0) {
    return
  }

  loadingRelated.value = true
  try {
    const relatedData: Record<string, any[]> = {}

    for (const fk of foreignKeys.value) {
      if (!shouldShowRelatedRelationPanel(tableName, fk)) {
        continue
      }

      const childTcName = relationSourceTableTcName(fk)
      const storageKey = relatedRowsStorageKey(fk)
      const referencedValue = resolveRawFieldValue(fk.referencedColumn)
      const pkValue = resolveRawFieldValue(primaryKeyColumn.value)
      const parentKey =
        referencedValue !== null && referencedValue !== undefined && referencedValue !== ''
          ? referencedValue
          : pkValue

      try {
        let matchingItems: any[] = []
        const relationColumns = fk.pullThroughFields ?? []
        const pullSpecs = pullThroughSpecsFromRelationColumns(relationColumns)
        const hasCalculatedColumns = relationColumns.some(col => col.expr?.trim())
        if (parentKey !== null && parentKey !== undefined && parentKey !== '') {
          const listParams = {
            tcName: childTcName,
            where: { [fk.columnName]: String(parentKey) },
            relationConstraintName: hasCalculatedColumns ? fk.constraintName : '',
          }
          try {
            const res = await client.listItems(listParams)
            matchingItems = res.items || []
          } catch (listErr) {
            console.error(
              `ListItems failed with relation calculated columns (${childTcName}):`,
              listErr,
            )
            const fallback = await client.listItems({
              tcName: childTcName,
              where: listParams.where,
            })
            matchingItems = fallback.items || []
          }
        }
        if (relationColumns.length > 0) {
          const childStructure = await ensureChildTableStructure(childTcName)
          const referencedContextByFk: Record<string, ReferencedPullThroughContext> = {}
          for (const spec of pullSpecs) {
            if (referencedContextByFk[spec.fkColumn]) continue
            const childFk = (childStructure.foreignKeys ?? []).find(
              entry => entry.columnName === spec.fkColumn,
            )
            if (!childFk) continue
            const refTc = childFk.referencedTableTcName || childFk.referencedTable
            const refStructure = await ensureChildTableStructure(refTc)
            const refFks = refStructure.foreignKeys ?? []
            const referencedItemsByColumn: Record<string, Item[]> = {}
            for (const refFk of refFks) {
              const targetTc = refFk.referencedTableTcName || refFk.referencedTable
              try {
                const targetRes = await client.listItems({ tcName: targetTc })
                referencedItemsByColumn[refFk.columnName] = targetRes.items ?? []
              } catch (listErr) {
                console.error(`Error loading FK targets for pull-through (${targetTc}):`, listErr)
                referencedItemsByColumn[refFk.columnName] = []
              }
            }
            referencedContextByFk[spec.fkColumn] = {
              fields: refStructure.fields ?? [],
              foreignKeys: refFks,
              referencedItemsByColumn,
            }
          }
          const extraFields = buildRelationExtraFieldDefs(
            relationColumns,
            childStructure.fields ?? [],
            referencedContextByFk,
            childStructure.columnDisplaySettings,
          )
          const displayFieldsByStorageKey = Object.fromEntries(
            extraFields.map(field => [field.name, field]),
          )
          matchingItems = await enrichItemsWithPullThrough(
            matchingItems as Item[],
            pullSpecs,
            childStructure.foreignKeys ?? [],
            async (pageId, id) => {
              const res = await client.getItem({ pageId, id })
              return res.item ?? null
            },
            referencedContextByFk,
            displayFieldsByStorageKey,
          )
          relatedPullThroughExtraFields.value = {
            ...relatedPullThroughExtraFields.value,
            [storageKey]: extraFields,
          }
        } else {
          const { [storageKey]: _removed, ...rest } = relatedPullThroughExtraFields.value
          relatedPullThroughExtraFields.value = rest
        }
        relatedData[storageKey] = matchingItems
      } catch (err) {
        console.error(`Error loading related rows for ${childTcName}:`, err)
        relatedData[storageKey] = []
      }
    }

    relatedRows.value = relatedData

    const relatedTableNames = [
      ...new Set(
        foreignKeys.value
          .filter(fk => shouldShowRelatedRelationPanel(tableName, fk))
          .map(fk => relationSourceTableTcName(fk)),
      ),
    ]
    loadRelatedTableIcons(relatedTableNames)
    await Promise.all(
      relatedTableNames.map(async (tcName) => {
        if (relatedTableRowNames.value[tcName] !== undefined) {
          return
        }
        try {
          const structure = await client.getTableStructure({ pageId: tcName })
          relatedTableRowNames.value[tcName] = structure.rowName ?? ''
        } catch {
          relatedTableRowNames.value[tcName] = ''
        }
      }),
    )
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
      await loadTableStructure()
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

  const entries: FieldEntry[] = []

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

    displayValue = withValueAffixes(key, displayValue, valueClass)
    entries.push({ key, displayValue, valueClass })
  }

  // Handle additionalFields by flattening them
  if (item.value.additionalFields && typeof item.value.additionalFields === 'object') {
    const additionalFields = item.value.additionalFields
    const schemaOrder = (tableStructure.value?.fields ?? []).map(f => f.name)
    for (const key of orderBySchema(Object.keys(additionalFields), schemaOrder)) {
      const value = additionalFields[key]
      // Remove internal/system fields
      if (key === '$typeName') continue
      // Skip markdown fields - they're handled by their base field
      if (key.endsWith('Markdown')) continue

      let displayValue
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

      displayValue = withValueAffixes(key, displayValue, valueClass)
      entries.push({ key, displayValue, valueClass })
    }
  }

  return entries
})

type FieldEntrySection = { key: string; title: string; entries: FieldEntry[] }

/** Item rows expose timestamps as srCreated/srUpdated; field groups store sr_created/sr_updated. */
function fieldGroupEntryKeys(column: string): string[] {
  switch (column) {
    case 'sr_created':
    case 'srCreated':
      return ['srCreated', 'sr_created']
    case 'sr_updated':
    case 'srUpdated':
      return ['srUpdated', 'sr_updated']
    default:
      return [column]
  }
}

function entryForFieldGroupColumn(
  byKey: Map<string, FieldEntry>,
  column: string,
): FieldEntry | undefined {
  for (const key of fieldGroupEntryKeys(column)) {
    const entry = byKey.get(key)
    if (entry) return entry
  }
  return undefined
}

const fieldEntrySections = computed((): FieldEntrySection[] => {
  const all = entries.value
  if (!all.length) return []
  const rawGroups = tableStructure.value?.itemFieldGroups?.groups ?? []
  if (!rawGroups.length) {
    return [{ key: 'all', title: '', entries: all }]
  }
  const sorted = [...rawGroups].sort((a, b) => (a.ordinal ?? 0) - (b.ordinal ?? 0))
  const byKey = new Map(all.map(e => [e.key, e]))
  const used = new Set<string>()
  const sections: FieldEntrySection[] = []
  for (const g of sorted) {
    const groupEntries: FieldEntry[] = []
    for (const col of g.columnNames ?? []) {
      const entry = entryForFieldGroupColumn(byKey, col)
      if (entry && !used.has(entry.key)) {
        groupEntries.push(entry)
        used.add(entry.key)
      }
    }
    if (groupEntries.length > 0) {
      sections.push({
        key: `group-${g.ordinal ?? sections.length}`,
        title: (g.title ?? '').trim(),
        entries: groupEntries,
      })
    }
  }
  const rest = all.filter(e => !used.has(e.key))
  if (rest.length > 0) {
    sections.push({ key: 'ungrouped', title: '', entries: rest })
  }
  return sections.length > 0 ? sections : [{ key: 'all', title: '', entries: all }]
})

const accordionFieldSections = computed(() =>
  fieldEntrySections.value.filter(section => section.title),
)

const ungroupedFieldSection = computed(() =>
  fieldEntrySections.value.find(section => !section.title),
)

const showFieldGroupAccordion = computed(
  () => accordionFieldSections.value.length > 0,
)

const fieldGroupAccordionDefaultOpen = computed(() =>
  accordionFieldSections.value.map(section => section.key),
)

const columnNamesForFieldGrouping = computed(() => {
  const physical = tableStructure.value?.fields?.map(field => field.name) ?? []
  const synthetic = syntheticColumnNamesForSourceTable(
    tableName,
    tableStructure.value?.foreignKeys ?? [],
  )
  const seen = new Set(physical)
  return [...physical, ...synthetic.filter(name => !seen.has(name))]
})

const fieldGroupingDialog = ref<HTMLDialogElement | null>(null)

function openFieldGroupingDialog() {
  fieldGroupingDialog.value?.showModal()
}

function closeFieldGroupingDialog() {
  if (fieldGroupingDialog.value?.open) {
    fieldGroupingDialog.value.close()
  }
}

async function onFieldGroupsSaved() {
  await loadTableStructure()
}

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
    constraintName: string
    displayName: string
    pullThroughExtraFields?: Field[]
    absorbIntoItem: boolean
    absorbType: string
  }> = []

  for (const [key, rows] of Object.entries(relatedRows.value)) {
    const dotIndex = key.indexOf('.')
    if (dotIndex <= 0) continue
    const relatedTableTc = key.slice(0, dotIndex)
    const columnName = key.slice(dotIndex + 1)

    const fk = foreignKeys.value.find(
      entry => relatedRowsStorageKey(entry) === key,
    )
    if (!fk || !shouldShowRelatedRelationPanel(tableName, fk)) {
      continue
    }

    const direction: 'referencing' = 'referencing'

    const relationName = fk?.constraintName || columnName
    const displayName = fk?.displayName?.trim() ?? ''
    const title = displayName || `${relatedTableTc} (${columnName})`

    const filterColumn = fk.columnName
    const parentKey = resolveRawFieldValue(fk.referencedColumn) ?? resolveRawFieldValue(primaryKeyColumn.value)
    const filterValue = parentKey == null ? '' : String(parentKey)

    tables.push({
      tableName: relatedTableTc,
      columnName,
      rows,
      title,
      direction,
      filterColumn,
      filterValue,
      relationName,
      constraintName: fk?.constraintName ?? '',
      displayName,
      pullThroughExtraFields: relatedPullThroughExtraFields.value[key],
      absorbIntoItem: !!fk.absorbIntoItem,
      absorbType: fk.absorbType || '',
    })
  }

  return tables
})

// Relations absorbed into the item ("first row") are shown in the main section, not as panels.
const panelRelations = computed(() => relatedTables.value.filter(entry => !entry.absorbIntoItem))
const absorbedRelations = computed(() => relatedTables.value.filter(entry => entry.absorbIntoItem))

const absorbedSections = computed(() => absorbedRelations.value.map(entry => {
  const structure = relatedChildStructures.value[entry.tableName]
  return {
    entry,
    key: entry.constraintName || `${entry.tableName}.${entry.columnName}`,
    title: entry.title,
    tableName: entry.tableName,
    row: entry.rows[0] ?? null,
    fields: structure?.fields ?? [],
    hiddenColumns: [entry.filterColumn],
    icon: relatedTableIcons.value[entry.tableName],
    viewAllTo: {
      path: `/table/${entry.tableName}`,
      query: entry.filterValue ? { where: JSON.stringify({ [entry.filterColumn]: entry.filterValue }) } : {},
    },
    viewAllLabel: `View all related ${rowNamePluralLabel(structure?.rowNamePlural, structure?.rowName)}`,
  }
}))

watch(absorbedRelations, (entries) => {
  for (const entry of entries) {
    ensureChildTableStructure(entry.tableName).catch(err => {
      console.warn(`Failed to load structure for absorbed relation ${entry.tableName}:`, err)
    })
  }
})

function visibleTabs<T extends { constraintName: string }>(tabs: T[]): T[] {
  const absorbed = new Set(absorbedRelations.value.map(entry => entry.constraintName))
  return tabs.filter(tab => !absorbed.has(tab.constraintName))
}

type ItemLayoutBlock =
  | { kind: 'quick_jump'; sectionTitle: string }
  | { kind: 'relation'; constraintName: string }
  | { kind: 'relation_tabs'; sectionTitle: string; tabs: Array<{ constraintName: string; label: string }> }

type ItemLayoutJumpLink = { anchorId: string; label: string; icon: IconData }

function itemLayoutAnchorId(blockIndex: number): string {
  return `item-layout-${blockIndex}`
}

function itemLayoutUnplacedAnchor(key: string): string {
  const safe = key.replace(/[^a-zA-Z0-9_-]+/g, '-')
  return `item-layout-unplaced-${safe}`
}

const itemLayoutBlocks = computed((): ItemLayoutBlock[] => {
  const raw = tableStructure.value?.itemPageLayout?.blocks ?? []
  const sorted = [...raw].sort((a, b) => (a.ordinal ?? 0) - (b.ordinal ?? 0))
  const out: ItemLayoutBlock[] = []
  for (const b of sorted) {
    if (!b.blockType) continue
    try {
      const cfg = JSON.parse(b.configJson || '{}') as Record<string, unknown>
      if (b.blockType === 'quick_jump') {
        out.push({
          kind: 'quick_jump',
          sectionTitle: String(cfg.section_title ?? '').trim(),
        })
      } else if (b.blockType === 'relation') {
        const constraintName = String(cfg.constraint_name ?? '').trim()
        if (constraintName) {
          out.push({ kind: 'relation', constraintName })
        }
      } else if (b.blockType === 'relation_tabs') {
        const tabsRaw = Array.isArray(cfg.tabs) ? cfg.tabs : []
        const tabs = tabsRaw
          .map((t: Record<string, unknown>) => ({
            constraintName: String(t.constraint_name ?? '').trim(),
            label: String(t.label ?? '').trim(),
          }))
          .filter(t => t.constraintName)
        if (tabs.length > 0) {
          out.push({
            kind: 'relation_tabs',
            sectionTitle: String(cfg.section_title ?? '').trim(),
            tabs,
          })
        }
      }
    } catch {
      // skip invalid block config
    }
  }
  return out
})

const placedConstraintNames = computed(() => {
  const names = new Set<string>()
  for (const block of itemLayoutBlocks.value) {
    if (block.kind === 'relation') {
      names.add(block.constraintName)
    } else if (block.kind === 'relation_tabs') {
      for (const tab of block.tabs) {
        names.add(tab.constraintName)
      }
    }
  }
  return names
})

const relatedByConstraint = computed(() => {
  const map = new Map<string, RelatedTableEntry>()
  for (const entry of panelRelations.value) {
    if (entry.constraintName) {
      map.set(entry.constraintName, entry)
    }
  }
  return map
})

const unplacedRelations = computed(() =>
  panelRelations.value.filter(
    entry => !entry.constraintName || !placedConstraintNames.value.has(entry.constraintName),
  ),
)

function relatedEntry(constraintName: string): RelatedTableEntry | undefined {
  return relatedByConstraint.value.get(constraintName)
}

function tabLabelFor(constraintName: string, configuredLabel: string): string {
  if (configuredLabel) return configuredLabel
  const entry = relatedEntry(constraintName)
  if (entry?.displayName) return entry.displayName
  if (entry) return `${entry.tableName} (${entry.columnName})`
  return constraintName
}

function layoutBlockJumpLabel(block: ItemLayoutBlock): string | null {
  if (block.kind === 'quick_jump') return null
  if (block.kind === 'relation') {
    const entry = relatedEntry(block.constraintName)
    if (!entry) return null
    return entry.displayName?.trim() || entry.title
  }
  return block.sectionTitle?.trim() || 'Related'
}

function quickJumpLinksAfter(blockIndex: number): ItemLayoutJumpLink[] {
  const links: ItemLayoutJumpLink[] = []
  const blocks = itemLayoutBlocks.value
  for (let i = blockIndex + 1; i < blocks.length; i++) {
    const block = blocks[i]
    if (block.kind === 'relation' && !relatedEntry(block.constraintName)) {
      continue
    }
    const label = layoutBlockJumpLabel(block)
    if (!label) continue
    // Same icon as the section: the related table's for a relation, DatabaseIcon for tab groups.
    const entry = block.kind === 'relation' ? relatedEntry(block.constraintName) : undefined
    links.push({
      anchorId: itemLayoutAnchorId(i),
      label,
      icon: (entry && relatedTableIcons.value[entry.tableName]) || DatabaseIcon,
    })
  }
  for (const table of unplacedRelations.value) {
    const key = table.constraintName || `${table.tableName}.${table.columnName}`
    links.push({
      anchorId: itemLayoutUnplacedAnchor(key),
      label: table.displayName?.trim() || table.title,
      icon: relatedTableIcons.value[table.tableName] || DatabaseIcon,
    })
  }
  return links
}

function scrollToLayoutAnchor(anchorId: string) {
  const el = document.getElementById(anchorId)
  el?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

function onRelatedRowDeleted(table: RelatedTableEntry, id: string) {
  const key = `${table.tableName}.${table.columnName}`
  if (relatedRows.value[key]) {
    relatedRows.value[key] = relatedRows.value[key].filter((r: { id?: unknown }) => String(r.id) !== String(id))
  }
}

const managingRelationFk = computed(() =>
  foreignKeys.value.find(fk => fk.constraintName === managingConstraintName.value),
)

function relationEndpointLabel(table: string, tcName: string | undefined, column: string): string {
  const tableLabel = (tcName?.trim() || table).trim()
  return `${tableLabel}.${column}`
}

async function loadRelationDialogContext() {
  const fk = managingRelationFk.value
  if (!fk) return
  relationDialogLoading.value = true
  relationSaveError.value = null
  try {
    const childTc = relationSourceTableTcName(fk)
    const childStructure = await ensureChildTableStructure(childTc)
    relationChildFields.value = childStructure.fields ?? []
    relationChildForeignKeys.value = childStructure.foreignKeys ?? []
    relationReferencedFieldsByFk.value = {}
    for (const row of relationPullThroughDraft.value) {
      if (!row.fkColumn || relationReferencedFieldsByFk.value[row.fkColumn]) continue
      const childFk = relationChildForeignKeys.value.find(entry => entry.columnName === row.fkColumn)
      if (!childFk) continue
      const refTc = childFk.referencedTableTcName || childFk.referencedTable
      const refStructure = await ensureChildTableStructure(refTc)
      relationReferencedFieldsByFk.value[row.fkColumn] = refStructure.fields ?? []
    }
  } catch (e) {
    relationSaveError.value = e instanceof Error ? e.message : String(e)
  } finally {
    relationDialogLoading.value = false
  }
}

async function onRelationPullThroughFkChange(index: number) {
  const row = relationPullThroughDraft.value[index]
  if (!row?.fkColumn) return
  row.field = ''
  const childFk = relationChildForeignKeys.value.find(entry => entry.columnName === row.fkColumn)
  if (!childFk) return
  const refTc = childFk.referencedTableTcName || childFk.referencedTable
  const refStructure = await ensureChildTableStructure(refTc)
  relationReferencedFieldsByFk.value = {
    ...relationReferencedFieldsByFk.value,
    [row.fkColumn]: refStructure.fields ?? [],
  }
}

function addRelationPullThroughRow() {
  relationPullThroughDraft.value.push({ fkColumn: '', field: '' })
}

function removeRelationPullThroughRow(index: number) {
  relationPullThroughDraft.value.splice(index, 1)
}

function addRelationCalculatedRow() {
  relationCalculatedDraft.value.push({ displayLabel: '', expr: '' })
}

function removeRelationCalculatedRow(index: number) {
  relationCalculatedDraft.value.splice(index, 1)
}

function openManageRelation(constraintName: string, displayName?: string) {
  if (!constraintName) return
  managingConstraintName.value = constraintName
  relationDisplayNameDraft.value = displayName ?? ''
  const fk = foreignKeys.value.find(entry => entry.constraintName === constraintName)
  relationAbsorbDraft.value = !!fk?.absorbIntoItem
  relationAbsorbTypeDraft.value = fk?.absorbType || 'first_row'
  const columns = fk?.pullThroughFields ?? []
  relationPullThroughDraft.value = columns
    .filter(pt => !pt.expr?.trim())
    .map(pt => ({ fkColumn: pt.fkColumn, field: pt.field }))
  relationCalculatedDraft.value = columns
    .filter(pt => pt.expr?.trim())
    .map(pt => ({
      displayLabel: pt.displayLabel?.trim() ?? '',
      expr: pt.expr.trim(),
    }))
  relationSaveError.value = null
  nextTick(async () => {
    relationDialog.value?.showModal()
    await loadRelationDialogContext()
  })
}

function closeManageRelation() {
  relationDialog.value?.close()
  managingConstraintName.value = ''
  relationDisplayNameDraft.value = ''
  relationAbsorbDraft.value = false
  relationAbsorbTypeDraft.value = 'first_row'
  relationPullThroughDraft.value = []
  relationCalculatedDraft.value = []
  relationChildForeignKeys.value = []
  relationChildFields.value = []
  relationReferencedFieldsByFk.value = {}
  relationSaveError.value = null
}

async function saveRelationDisplayName() {
  if (!managingConstraintName.value || savingRelation.value) return
  savingRelation.value = true
  relationSaveError.value = null
  try {
    const pullThroughFields: RelationPullThroughField[] = [
      ...relationPullThroughDraft.value
        .filter(row => row.fkColumn.trim() && row.field.trim())
        .map(row => ({
          fkColumn: row.fkColumn.trim(),
          field: row.field.trim(),
          displayLabel: '',
          expr: '',
        })),
      ...relationCalculatedDraft.value
        .filter(row => row.displayLabel.trim() && row.expr.trim())
        .map(row => ({
          fkColumn: '',
          field: '',
          displayLabel: row.displayLabel.trim(),
          expr: row.expr.trim(),
        })),
    ]
    const res = await client.updateTableRelation({
      constraintName: managingConstraintName.value,
      displayName: relationDisplayNameDraft.value.trim(),
      pullThroughFields,
      absorbIntoItem: relationAbsorbDraft.value,
      absorbType: relationAbsorbDraft.value ? relationAbsorbTypeDraft.value : '',
    })
    if (!res.success) {
      throw new Error(res.message || 'Failed to update relation')
    }
    closeManageRelation()
    await loadTableStructure()
    await loadRelatedRows()
  } catch (e) {
    relationSaveError.value = e instanceof Error ? e.message : String(e)
  } finally {
    savingRelation.value = false
  }
}

const sectionTitle = computed(() =>
  itemViewSectionTitle({
    rowName: tableStructure.value?.rowName,
    tableTitle: tableStructure.value?.title,
    tableName,
    item: item.value,
  }),
)

watch(
  [sectionTitle, appTitle],
  ([heading]) => {
    rowViewPageLabel.value = heading
    document.title = formatDocumentTitle(heading, appTitle.value)
  },
  { immediate: true },
)

onUnmounted(() => {
  clearRowViewPageLabel()
})

const parentRowLabel = computed(() => {
  const name = getItemNameFieldValue(item.value as Record<string, unknown> | null)
  return name || rowId
})

const editButtonText = computed(() =>
  rowEditActionLabel(tableStructure.value?.rowName),
)

const backToTableLabel = computed(() =>
  tableListLinkLabel(tableStructure.value?.rowNamePlural, tableStructure.value?.rowName),
)

function displayFieldLabel(key: string): string {
  return fieldDisplayLabel(getFieldDef(key), key)
}

provide(rowViewFieldUiKey, {
  editingField,
  editingValue,
  savingField,
  users,
  setEditInput,
  displayFieldLabel,
  resolveRawFieldValue,
  getFkDisplay,
  isUserRefColumn,
  isTinyintColumn,
  isDatetimeColumn,
  getFieldDef,
  getBooleanValue,
  canEditField,
  showFullEditIcon,
  showReadOnlyIcon,
  fullEditTo,
  fullEditLabel,
  startEdit,
  cancelEdit,
  saveField,
  hasStoredUserRefValue,
  displayUserLabel,
})

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
    <Section :title="sectionTitle" :icon="ViewIcon" :padding="false">
      <template #toolbar>
        <router-link
          :to="`/table/${tableName}`"
          class="button inline-icon neutral"
        >
          <HugeiconsIcon :icon="ArrowLeft01Icon" width="1em" height="1em" aria-hidden="true" />
          <span>{{ backToTableLabel }}</span>
        </router-link>
        <router-link
          v-if="tableAccess.canEdit"
          :to="`/table/${tableName}/${rowId}/edit`"
          class="button inline-icon good"
        >
          <span>{{ editButtonText }}</span>
        </router-link>
        <router-link
          v-if="designModeEnabled && tableAccess.canEdit && foreignKeys.length > 0"
          :to="`/table/${tableName}/item-view-layout`"
          class="button inline-icon neutral design-only"
        >
          <HugeiconsIcon :icon="LayoutIcon" width="1em" height="1em" aria-hidden="true" />
          <span>Item layout</span>
        </router-link>
        <button
          v-if="designModeEnabled && tableAccess.canEdit"
          type="button"
          class="button inline-icon neutral design-only"
          @click="openFieldGroupingDialog"
        >
          <HugeiconsIcon :icon="GridIcon" width="1em" height="1em" aria-hidden="true" />
          <span>Field groups</span>
        </button>
      </template>

      <div v-if="error">{{ error }}</div>
      <LoadingArea v-else-if="loading" message="Loading…" min-height="8rem" />
      <div v-else-if="fieldError" class="field-save-error">{{ fieldError }}</div>
      <div
        v-if="!loading && !error && entries.length > 0"
        class="row-main-layout"
        :class="{ 'has-absorbed': absorbedSections.length > 0 }"
      >
       <div class="row-main-fields">
        <Accordion
          v-if="showFieldGroupAccordion"
          :default-open-ids="fieldGroupAccordionDefaultOpen"
        >
          <AccordionItem
            v-for="section in accordionFieldSections"
            :key="section.key"
            :item-id="section.key"
            :title="section.title"
          >
            <RowViewFieldEntries :entries="section.entries" />
          </AccordionItem>
        </Accordion>
        <RowViewFieldEntries
          v-if="showFieldGroupAccordion && ungroupedFieldSection"
          :entries="ungroupedFieldSection.entries"
        />
        <RowViewFieldEntries v-else-if="!showFieldGroupAccordion" :entries="entries" />
       </div>
       <!-- Absorbed relations: a right-hand column that wraps below the fields when narrow -->
       <div v-if="absorbedSections.length > 0" class="row-main-absorbed">
        <AbsorbedRelationFields
          v-for="section in absorbedSections"
          :key="section.key"
          :title="section.title"
          :table-name="section.tableName"
          :row="section.row"
          :fields="section.fields"
          :hidden-columns="section.hiddenColumns"
          :icon="section.icon"
          :can-manage="designModeEnabled && tableAccess.canEdit && !!section.entry.constraintName"
          :view-all-to="section.viewAllTo"
          :view-all-label="section.viewAllLabel"
          @manage="openManageRelation(section.entry.constraintName, section.entry.displayName)"
        />
       </div>
      </div>
      <div v-else-if="!loading && !error" class="no-data">
        <p>No data available for this row.</p>
      </div>
    </Section>

    <!-- Related tables: custom layout blocks, then unplaced defaults -->
    <template v-if="foreignKeys.length > 0">
      <template v-for="(block, blockIndex) in itemLayoutBlocks" :key="`layout-block-${blockIndex}`">
        <Section
          v-if="block.kind === 'quick_jump' && quickJumpLinksAfter(blockIndex).length > 0"
          :title="block.sectionTitle || 'On this page'"
          :icon="Link01Icon"
        >
          <nav class="item-layout-quick-jump" aria-label="Jump to sections on this page">
            <template v-for="(link, linkIndex) in quickJumpLinksAfter(blockIndex)" :key="link.anchorId">
              <span v-if="linkIndex > 0" class="item-layout-quick-jump-sep" aria-hidden="true"> · </span>
              <a href="#" class="item-layout-quick-jump-link" @click.prevent="scrollToLayoutAnchor(link.anchorId)">
                <HugeiconsIcon :icon="link.icon" width="1em" height="1em" aria-hidden="true" />
                <span>{{ link.label }}</span>
              </a>
            </template>
          </nav>
        </Section>
        <div
          v-else-if="block.kind === 'relation' && relatedEntry(block.constraintName)"
          :id="itemLayoutAnchorId(blockIndex)"
          class="item-layout-anchor"
        >
          <RelatedRelationPanel
            :table="relatedEntry(block.constraintName)!"
            :parent-table-name="tableName"
            :parent-row-id="rowId"
            :parent-row-label="parentRowLabel"
            :loading-related="loadingRelated"
            :related-row-name="relatedTableRowNames[relatedEntry(block.constraintName)!.tableName] ?? ''"
            :icon="relatedTableIcons[relatedEntry(block.constraintName)!.tableName]"
            :can-edit-relation="tableAccess.canEdit"
            @rows-updated="loadRelatedRows"
            @row-deleted="(id) => onRelatedRowDeleted(relatedEntry(block.constraintName)!, id)"
            @manage-relation="openManageRelation"
          />
        </div>
        <div
          v-else-if="block.kind === 'relation_tabs' && visibleTabs(block.tabs).length > 0"
          :id="itemLayoutAnchorId(blockIndex)"
          class="item-layout-anchor"
        >
          <Section
            :title="block.sectionTitle || 'Related'"
            :icon="DatabaseIcon"
            :padding="false"
          >
            <Tabs
              :tabs="visibleTabs(block.tabs).map(tab => ({
                id: tab.constraintName,
                label: tabLabelFor(tab.constraintName, tab.label),
                icon: relatedTableIcons[relatedEntry(tab.constraintName)?.tableName ?? ''],
              }))"
              :padding="false"
            >
              <template
                v-for="tab in visibleTabs(block.tabs)"
                :key="tab.constraintName"
                #[`tab-${tab.constraintName}`]
              >
                <RelatedRelationPanel
                  v-if="relatedEntry(tab.constraintName)"
                  :table="relatedEntry(tab.constraintName)!"
                  :parent-table-name="tableName"
                  :parent-row-id="rowId"
                  :parent-row-label="parentRowLabel"
                  :loading-related="loadingRelated"
                  :related-row-name="relatedTableRowNames[relatedEntry(tab.constraintName)!.tableName] ?? ''"
                  :can-edit-relation="tableAccess.canEdit"
                  embed-in-tabs
                  @rows-updated="loadRelatedRows"
                  @row-deleted="(id) => onRelatedRowDeleted(relatedEntry(tab.constraintName)!, id)"
                  @manage-relation="openManageRelation"
                />
                <p v-else class="layout-missing-relation">This relation is no longer available.</p>
              </template>
            </Tabs>
          </Section>
        </div>
      </template>
      <div
        v-for="table in unplacedRelations"
        :id="itemLayoutUnplacedAnchor(table.constraintName || `${table.tableName}.${table.columnName}`)"
        :key="`${table.tableName}.${table.columnName}`"
        class="item-layout-anchor"
      >
        <RelatedRelationPanel
          :table="table"
          :parent-table-name="tableName"
          :parent-row-id="rowId"
          :parent-row-label="parentRowLabel"
          :loading-related="loadingRelated"
          :related-row-name="relatedTableRowNames[table.tableName] ?? ''"
          :icon="relatedTableIcons[table.tableName]"
          :can-edit-relation="tableAccess.canEdit"
          @rows-updated="loadRelatedRows"
          @row-deleted="(id) => onRelatedRowDeleted(table, id)"
          @manage-relation="openManageRelation"
        />
      </div>
    </template>

    <DangerZone
      v-if="tableAccess.canDelete && !loading && !error"
      title="Danger zone"
      subtitle="Destructive actions for this row"
      description="Permanently remove this record from the table."
    >
      <div role="toolbar" class="danger-zone-actions">
        <button
          type="button"
          class="bad inline-icon"
          :disabled="deleting"
          @click="confirmDelete"
        >
          <HugeiconsIcon :icon="Delete01Icon" width="1em" height="1em" aria-hidden="true" />
          <span>{{ deleting ? 'Deleting…' : 'Delete row' }}</span>
        </button>
      </div>
    </DangerZone>

    <dialog ref="fieldGroupingDialog" class="dialog field-grouping-dialog" @close="closeFieldGroupingDialog">
      <div class="field-grouping-dialog-titlebar">
        <h2>Field grouping</h2>
        <button
          type="button"
          class="button inline-icon neutral dialog-close"
          aria-label="Close"
          @click="closeFieldGroupingDialog"
        >
          <HugeiconsIcon :icon="Cancel01Icon" width="1.25em" height="1.25em" aria-hidden="true" />
        </button>
      </div>
      <p class="muted field-grouping-dialog-intro">
        Organize fields on every item row page into titled accordion sections. Unassigned fields stay at the bottom.
      </p>
      <FieldGroupingEditor
        embedded
        :table-id="tableName"
        :column-names="columnNamesForFieldGrouping"
        :client="client"
        @saved="onFieldGroupsSaved"
      />
    </dialog>

    <dialog ref="relationDialog" class="dialog relation-dialog" @close="closeManageRelation">
      <h2>Manage relation</h2>
      <dl v-if="managingRelationFk" class="relation-endpoints">
        <dt>Relation ID</dt>
        <dd>
          <code v-if="managingRelationFk.relationId">{{ managingRelationFk.relationId }}</code>
          <span v-else class="muted">—</span>
        </dd>
        <dt>Constraint name</dt>
        <dd><code>{{ managingRelationFk.constraintName }}</code></dd>
        <dt>Source</dt>
        <dd>
          <code>{{
            relationEndpointLabel(
              managingRelationFk.tableName,
              managingRelationFk.tableTcName,
              managingRelationFk.columnName,
            )
          }}</code>
        </dd>
        <dt>Destination</dt>
        <dd>
          <code>{{
            relationEndpointLabel(
              managingRelationFk.referencedTable,
              managingRelationFk.referencedTableTcName,
              managingRelationFk.referencedColumn,
            )
          }}</code>
        </dd>
      </dl>
      <p class="muted">Give this related list a friendly name shown in the section title.</p>
      <FormField label="Friendly name" for="relation-display-name" :disabled="savingRelation">
        <input
          id="relation-display-name"
          v-model="relationDisplayNameDraft"
          type="text"
          placeholder="e.g. Orders, Tasks, Line items"
          autocomplete="off"
          :disabled="savingRelation"
        />
      </FormField>
      <h3 class="relation-dialog-subheading">Absorb into item</h3>
      <p class="muted">
        Show the related row inside this item's main section, as if its fields were part of the item, instead of as a separate section.
      </p>
      <FormField label="Absorb into item" for="relation-absorb" :disabled="savingRelation">
        <input
          id="relation-absorb"
          v-model="relationAbsorbDraft"
          type="checkbox"
          :disabled="savingRelation"
        />
      </FormField>
      <FormField label="Absorb type" for="relation-absorb-type" :disabled="savingRelation || !relationAbsorbDraft">
        <select
          id="relation-absorb-type"
          v-model="relationAbsorbTypeDraft"
          :disabled="savingRelation || !relationAbsorbDraft"
        >
          <option value="first_row">First row</option>
        </select>
      </FormField>
      <h3 class="relation-dialog-subheading">Pull-through columns</h3>
      <p class="muted">
        Show fields from rows linked by foreign keys on the related table (for example, currency name on each opportunity).
      </p>
      <p v-if="relationDialogLoading" class="muted">Loading relation fields…</p>
      <template v-else>
        <div
          v-for="(row, index) in relationPullThroughDraft"
          :key="`pt-${index}`"
          class="relation-pull-through-row"
        >
          <FormField label="Foreign key column" :for="`pt-fk-${index}`" :disabled="savingRelation">
            <select
              :id="`pt-fk-${index}`"
              v-model="row.fkColumn"
              :disabled="savingRelation"
              @change="onRelationPullThroughFkChange(index)"
            >
              <option value="">— Select —</option>
              <option
                v-for="childFk in relationChildForeignKeys"
                :key="childFk.constraintName || childFk.columnName"
                :value="childFk.columnName"
              >
                {{ childFk.columnName }} → {{ childFk.referencedTableTcName || childFk.referencedTable }}
              </option>
            </select>
          </FormField>
          <FormField label="Field on linked row" :for="`pt-field-${index}`" :disabled="savingRelation">
            <select
              :id="`pt-field-${index}`"
              v-model="row.field"
              :disabled="savingRelation || !row.fkColumn"
            >
              <option value="">— Select —</option>
              <option
                v-for="field in (relationReferencedFieldsByFk[row.fkColumn] ?? [])"
                :key="field.name"
                :value="field.name"
              >
                {{ field.displayLabel || field.name }}
              </option>
            </select>
          </FormField>
          <button
            type="button"
            class="neutral inline-icon"
            :disabled="savingRelation"
            @click="removeRelationPullThroughRow(index)"
          >
            Remove
          </button>
        </div>
        <button
          type="button"
          class="neutral inline-icon"
          :disabled="savingRelation || relationChildForeignKeys.length === 0"
          @click="addRelationPullThroughRow"
        >
          Add pull-through column
        </button>
      </template>
      <h3 class="relation-dialog-subheading">Calculated columns</h3>
      <p class="muted">
        Expressions use <code>{table.column}</code> placeholders (table configuration or physical table name).
        Values are computed on the server when related rows load, e.g.
        <code>{skus.value} * {quote_lines.quantity}</code>.
      </p>
      <div
        v-for="(row, index) in relationCalculatedDraft"
        :key="`calc-${index}`"
        class="relation-pull-through-row"
      >
        <FormField label="Column label" :for="`calc-label-${index}`" :disabled="savingRelation">
          <input
            :id="`calc-label-${index}`"
            v-model="row.displayLabel"
            type="text"
            placeholder="e.g. Line total"
            autocomplete="off"
            :disabled="savingRelation"
          />
        </FormField>
        <FormField label="Expression (expr)" :for="`calc-expr-${index}`" :disabled="savingRelation">
          <input
            :id="`calc-expr-${index}`"
            v-model="row.expr"
            type="text"
            placeholder="{skus.value} * {quote_lines.quantity}"
            autocomplete="off"
            :disabled="savingRelation"
          />
        </FormField>
        <button
          type="button"
          class="neutral inline-icon"
          :disabled="savingRelation"
          @click="removeRelationCalculatedRow(index)"
        >
          Remove
        </button>
      </div>
      <button
        type="button"
        class="neutral inline-icon"
        :disabled="savingRelation"
        @click="addRelationCalculatedRow"
      >
        Add calculated column
      </button>
      <p v-if="relationSaveError" class="inline-notification error">{{ relationSaveError }}</p>
      <div class="dialog-actions">
        <button type="button" class="neutral" :disabled="savingRelation" @click="closeManageRelation">Cancel</button>
        <button type="button" class="good" :disabled="savingRelation" @click="saveRelationDisplayName">
          {{ savingRelation ? 'Saving…' : 'Save' }}
        </button>
      </div>
    </dialog>

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
.row-main-layout.has-absorbed {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  column-gap: 1.5rem;
}

.row-main-layout.has-absorbed .row-main-fields {
  flex: 2 1 32rem;
  min-width: 0;
}

.row-main-absorbed {
  flex: 1 1 22rem;
  min-width: 0;
}

/* Line the first subsection up with the first field row (femtocrank dt/dd padding). */
.row-main-absorbed > :first-child {
  margin-top: 0.6em;
}

.item-layout-quick-jump {
  line-height: 1.5;
}

.item-layout-quick-jump-link {
  display: inline-flex;
  align-items: center;
  gap: 0.3em;
}

.item-layout-anchor {
  scroll-margin-top: 1rem;
}

.danger-zone-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
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

.field-save-error {
  color: #dc3545;
  margin-bottom: 1rem;
}

.field-display {
  min-height: 1.5em;
  padding: 0.25rem;
  border-radius: 3px;
}

.field-display.editable {
  cursor: pointer;
}

.field-display.editable:hover {
  background-color: var(--hover-background-color, #f8f9fa);
}

.field-grouping-dialog {
  max-width: min(56rem, 96vw);
  max-height: 90vh;
  overflow: auto;
}

.field-grouping-dialog-titlebar {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 0.35rem;
}

.field-grouping-dialog-titlebar h2 {
  margin: 0;
}

.field-grouping-dialog-intro {
  margin-top: 0;
}

.field-grouping-dialog .dialog-close {
  flex-shrink: 0;
}

.relation-dialog-subheading {
  margin: 1rem 0 0.25rem;
  font-size: 1.05em;
}

.relation-pull-through-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  align-items: flex-end;
  margin-bottom: 0.75rem;
}

.inline-edit {
  padding: 0;
}

.edit-input {
  width: 100%;
  max-width: 28rem;
  padding: 0.25rem;
  border: 2px solid #007bff;
  border-radius: 3px;
  background: white;
  font-size: inherit;
  outline: none;
}

.edit-checkbox {
  width: 1.25rem;
  height: 1.25rem;
}

</style>
