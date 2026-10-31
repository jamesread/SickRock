<script setup lang="ts">
import type { IconData } from '../utils/hugeicons'
import { computed, inject, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import Table from 'picocrank/vue/components/Table.vue'
import Section from 'picocrank/vue/components/Section.vue'
import Dialog from 'picocrank/vue/components/Dialog.vue'
import NotificationBlock from 'picocrank/vue/components/NotificationBlock.vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import {
  Add01Icon,
  DatabaseIcon,
} from '@hugeicons/core-free-icons'

import type { Field, GetTableStructureResponse, Item } from '../gen/sickrock_pb'
import type { TableView } from '../composables/useTableViewManager'
import { useKeyboardShortcuts, type KeyboardShortcut } from '../composables/useKeyboardShortcuts'
import { createApiClient } from '../stores/api'
import { formatUnixTimestamp } from '../utils/dateFormatting'
import {
  relatedTableEmptyHeading,
  rowInsertActionLabel,
  tableEmptyHeading,
  tableViewSectionTitle,
} from '../utils/rowInsertLabel'
import { hasUuidColumn, isOnline, loadTableData, saveTableData } from '../utils/indexedDB'
import {
  buildTableHeaders,
  flattenItems,
  getSourceItem,
  getStableRowId,
  mapTableView,
  type SortDirection,
  type TableRow,
} from '../utils/tableAdapter'
import {
  applyViewToInsertFieldDefs,
  fieldDefsForInsertForm,
} from '../utils/insertRowViewFields'
import { useIamUsers } from '../composables/useIamUsers'
import { hasImageValue, isImageFieldType, isUserRefColumn as columnIsUserRef, isUserRefFieldType } from '../utils/fieldTypes'
import InsertRow from './InsertRow.vue'
import RowActionsDropdown from './RowActionsDropdown.vue'
import TableComponentToolbar from './TableComponentToolbar.vue'
import UserRefChip from './UserRefChip.vue'
import ImageThumb from './ImageThumb.vue'
import { useDesignMode } from '../composables/useDesignMode'
import {
  applyFieldValueAffixes,
  foreignKeyReferencedDisplayLabel,
  mergeItemFieldRecord,
} from '../utils/fieldColumnDisplay'
import { findFieldDef } from '../utils/fieldTypes'
import { isPullThroughColumn, isRelationSyntheticColumn } from '../utils/relationPullThrough'
import { enrichTableItemsWithRelationExtras } from '../utils/tableRelationExtraColumns'
import { urlConditionalFormatLink } from '../utils/conditionalFormattingUrl'
import { initialTableView, parseViewLayout, tableViewFromProto } from '../utils/serverTableViews'
import { useServerTableViewSettings } from '../composables/useServerTableViewSettings'
import { useTableAccess } from '../composables/useTableAccess'

const { designModeEnabled } = useDesignMode()

const ACTIONS_COLUMN = '__sickrock_actions'

type ActiveCell = {
  rowKey: string | number
  columnKey: string | number
} | null

type CellContext = {
  row: TableRow
  value: unknown
  header: { key: string }
  rowIndex: number
  sourceIndex: number
  rowKey?: string | number
  columnKey?: string | number
  columnIndex?: number
}

type TableApi = {
  clearFilters: () => void
  activateCellAt: (
    rowIndex: number,
    columnIndex: number,
    options?: { focus?: boolean; wrapColumns?: boolean },
  ) => CellContext | null
  moveActiveCell: (
    rowDelta: number,
    columnDelta: number,
    options?: { focus?: boolean; wrapColumns?: boolean },
  ) => CellContext | null
  getActiveCellContext: () => CellContext | null
  focusActiveCell: (options?: FocusOptions) => boolean | void
}

const props = defineProps<{
  tableId: string
  tableStructure?: GetTableStructureResponse | null
  fields?: Array<{ name: string; type: string }>
  createButtonText?: string
  items?: any[]
  showToolbar?: boolean
  showViewSwitcher?: boolean
  showViewEdit?: boolean
  showViewCreate?: boolean
  showExport?: boolean
  showStructure?: boolean
  showInsert?: boolean
  showPagination?: boolean
  title?: string
  wrapInSection?: boolean
  toolbarTeleport?: string
  /** When set, empty state shows "No {plural} for {label}" (related item panels). */
  relatedEmptyParentLabel?: string
  insertRouteOverride?: string | { path: string; query?: Record<string, string> }
  /** Synthetic columns (e.g. relation pull-through); forced visible when embedded. */
  extraFields?: Field[]
  /** Icon for the section title (defaults to DatabaseIcon). */
  sectionIcon?: IconData
  /** Server-side exact-match filters (column → value), e.g. "view all related rows" links. */
  where?: Record<string, string>
}>()

const wrapInSection = computed(() => props.wrapInSection !== false)
const useOwnSection = computed(() => wrapInSection.value && !props.toolbarTeleport)

const emit = defineEmits<{
  'view-created': []
  'rows-updated': []
  'row-deleted': [id: string]
  'view-changed': [viewType: string, viewId?: number]
}>()

const client = createApiClient()
const { loadUsers, displayUserLabel } = useIamUsers()
const tableRef = ref<TableApi | null>(null)
const tableStructure = ref<GetTableStructureResponse | null>(props.tableStructure ?? null)
const tableTitle = ref('')
const loadedItems = ref<Item[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
const relationExtraFields = ref<Field[]>([])

const fieldDefs = computed<Field[]>(() => {
  let base: Field[]
  if (props.fields?.length) {
    base = props.fields.map(field => ({
      name: field.name,
      type: field.type,
      required: false,
      defaultToCurrentTimestamp: false,
    })) as Field[]
  } else {
    base = tableStructure.value?.fields ?? []
  }
  const extra = props.extraFields?.length
    ? props.extraFields
    : relationExtraFields.value
  if (!extra.length) {
    return base
  }
  const names = new Set(base.map(field => field.name))
  return [...base, ...extra.filter(field => !names.has(field.name))]
})

const sourceItems = computed<Item[]>(() => (props.items ?? loadedItems.value) as Item[])
const flattenedRows = computed<TableRow[]>(() => {
  const rows = flattenItems(sourceItems.value)
  const primaryKeyColumn = tableStructure.value?.primaryKeyColumn
  const booleanFields = fieldDefs.value
    .filter(field => field.type.toLowerCase().startsWith('tinyint') || ['bool', 'boolean'].includes(field.type.toLowerCase()))
    .map(field => field.name)

  for (const row of rows) {
    if (primaryKeyColumn && row[primaryKeyColumn] === undefined) {
      row[primaryKeyColumn] = row.id
    }
    for (const fieldName of booleanFields) {
      const value = row[fieldName]
      if (value !== null && value !== undefined) {
        row[fieldName] = value === true || value === 1 || value === '1'
      }
    }
  }
  return rows
})

const sectionTitle = computed(() => {
  if (props.title) {
    return props.title
  }
  return tableViewSectionTitle(
    tableStructure.value?.rowNamePlural,
    tableStructure.value?.rowName,
    tableTitle.value || tableStructure.value?.title,
    props.tableId,
  )
})
const insertButtonText = computed(() => {
  if (props.createButtonText) {
    return props.createButtonText
  }
  return rowInsertActionLabel(tableStructure.value?.rowName)
})

const insertRoute = computed(() => {
  const delegate = tableStructure.value?.createDelegate?.trim()
  if (delegate) {
    return delegate
  }
  return `/table/${props.tableId}/insert-row`
})

// A related-panel override keeps its query (parent row context) but still goes to the
// table's create delegate when it has one.
const effectiveInsertRoute = computed(() => {
  const override = props.insertRouteOverride
  if (override == null) {
    return insertRoute.value
  }
  const delegate = tableStructure.value?.createDelegate?.trim()
  if (delegate && typeof override === 'object') {
    return { ...override, path: delegate }
  }
  return override
})

const isRelatedEmptyState = computed(
  () => props.relatedEmptyParentLabel != null && props.relatedEmptyParentLabel !== '',
)

const emptyStateHeading = computed(() => {
  const rowNamePlural = tableStructure.value?.rowNamePlural
  const rowName = tableStructure.value?.rowName
  if (isRelatedEmptyState.value) {
    return relatedTableEmptyHeading(rowNamePlural, rowName, props.relatedEmptyParentLabel ?? '')
  }
  return tableEmptyHeading(rowNamePlural, rowName)
})

const showQuickAdd = computed(() => showInsert.value && !tableStructure.value?.createDelegate?.trim())

const showViewSwitcher = computed(() => props.showViewSwitcher !== false)
const showViewEdit = computed(() => props.showViewEdit !== false && designModeEnabled.value)
const showViewCreate = computed(() => props.showViewCreate !== false && designModeEnabled.value)
const showExport = computed(() => props.showExport !== false)
const showStructure = computed(() => props.showStructure !== false && designModeEnabled.value)
const showInsert = computed(() => props.showInsert !== false)
const showPagination = computed(() => props.showPagination !== false)
const showToolbar = computed(() => {
  if (props.showToolbar === false) return false
  return showViewSwitcher.value
    || showViewEdit.value
    || showViewCreate.value
    || showExport.value
    || showStructure.value
    || showInsert.value
})

const useInlineToolbar = computed(
  () => showToolbar.value && !props.toolbarTeleport && !wrapInSection.value,
)

const tableToolbarBindings = computed(() => ({
  tableId: props.tableId,
  showViewSwitcher: showViewSwitcher.value,
  showViewEdit: showViewEdit.value,
  showViewCreate: showViewCreate.value,
  showExport: showExport.value,
  showStructure: showStructure.value,
  showInsert: showInsert.value,
  showQuickAdd: showQuickAdd.value,
  showBulkEntry: showInsert.value && tableStructure.value?.bulkEntry != null,
  insertRoute: insertRoute.value,
  insertButtonText: insertButtonText.value,
  tableViews: tableViews.value,
  selectedViewId: selectedViewId.value,
  currentView: currentView.value,
  multiSelectEnabled: multiSelectEnabled.value,
}))

// Server table views remain the source of truth. PicoCrank changes the
// controlled state below for this component session only; views saved from
// PicoCrank's view menu go to the server (see useServerTableViewSettings).
const tableViews = ref<TableView[]>([])
const selectedViewId = ref<number | null>(null)
const userDefaultViewId = ref(0)
const currentView = computed(() => (
  tableViews.value.find(view => view.id === selectedViewId.value) ?? null
))
const viewOptions = computed<TableView[]>(() => {
  const options = [...tableViews.value]
  if (options.length === 0 || !options.some(view => view.isDefault)) {
    options.unshift({
      id: -1,
      tableName: props.tableId,
      viewName: 'All Columns',
      isDefault: true,
      viewType: 'table',
      columns: [],
    })
  }
  return options
})

const tableFilters = ref<Record<string, unknown>>({})
const columnVisibility = ref<Record<string, boolean>>({})
const sessionColumnOrder = ref<string[] | null>(null)
const sortBy = ref<string | null>(null)
const sortDir = ref<SortDirection>('asc')
const page = ref(1)
const pageSize = ref(10)
const groupBy = ref<string | null>(null)
const columnPriorities = ref<Record<string, number | null>>({})
const selectedKeys = ref<string[]>([])
const multiSelectEnabled = ref(false)
const activeCell = ref<ActiveCell>(null)

function toggleMultiSelect() {
  multiSelectEnabled.value = !multiSelectEnabled.value
  if (!multiSelectEnabled.value) {
    selectedKeys.value = []
  }
}

const tableHeaders = computed(() => [
  ...buildTableHeaders(fieldDefs.value, currentView.value),
  {
    key: ACTIONS_COLUMN,
    // PicoCrank shows the key when the label is empty, so give the column a real heading.
    label: 'Actions',
    sortable: false,
    filterable: false,
    hideable: false,
    width: '5rem',
    class: 'actions',
  },
])

function serverColumnOrder(): string[] {
  const mapped = mapTableView(fieldDefs.value, currentView.value)
  const extraNames = (props.extraFields ?? []).map(field => field.name)
  const ordered = [...mapped.orderedColumns]
  for (const name of extraNames) {
    if (!ordered.includes(name)) {
      ordered.push(name)
    }
  }
  return [...ordered, ACTIONS_COLUMN]
}

const columnOrder = computed(() => sessionColumnOrder.value ?? serverColumnOrder())

function applyServerView() {
  const mapped = mapTableView(fieldDefs.value, currentView.value)
  const extraNames = (props.extraFields ?? []).map(field => field.name)
  sessionColumnOrder.value = null
  columnVisibility.value = {
    ...mapped.columnVisibility,
    [ACTIONS_COLUMN]: true,
  }
  for (const name of extraNames) {
    columnVisibility.value[name] = true
  }
  sortBy.value = mapped.sortBy
  sortDir.value = mapped.sortDir
  activeCell.value = null
}

watch(
  [
    () => fieldDefs.value.map(field => `${field.name}:${field.type}`).join('|'),
    () => selectedViewId.value,
    () => JSON.stringify(currentView.value?.columns ?? []),
    () => JSON.stringify((props.extraFields ?? []).map(field => field.name)),
    () => JSON.stringify(relationExtraFields.value.map(field => field.name)),
  ],
  applyServerView,
  { immediate: true },
)

// Filters, group-by, page size and column priorities saved with a view (layout_json).
// Applied only when the selected view changes, so structure reloads keep the user's filters.
function applyViewLayout() {
  const layout = parseViewLayout(currentView.value?.layoutJson)
  tableFilters.value = layout.filters ?? {}
  groupBy.value = layout.groupBy ?? null
  columnPriorities.value = layout.columnPriorities ?? {}
  if (layout.pageSize) {
    pageSize.value = layout.pageSize
  }
  page.value = 1
}

watch(
  [() => selectedViewId.value, () => currentView.value?.layoutJson ?? ''],
  applyViewLayout,
)

const { access: tableAccess } = useTableAccess(props.tableId)
const serverViewSettings = useServerTableViewSettings({
  client,
  tableId: () => props.tableId,
  views: tableViews,
  userDefaultViewId,
  fieldNames: () => mapTableView(fieldDefs.value, null).orderedColumns,
  canEdit: () => tableAccess.value.canEdit,
  reload: loadTableViews,
  select: selectView,
})

function onColumnVisibilityUpdate(value: Record<string, boolean>) {
  columnVisibility.value = { ...value, [ACTIONS_COLUMN]: true }
}

function onColumnOrderUpdate(value: string[]) {
  const dataColumns = value.filter(key => key !== ACTIONS_COLUMN)
  sessionColumnOrder.value = [...dataColumns, ACTIONS_COLUMN]
}

function onFiltersUpdate(value: Record<string, unknown>) {
  tableFilters.value = value
}

function onSelectedKeysUpdate(value: unknown[]) {
  selectedKeys.value = value.map(String)
}

function onActiveCellUpdate(value: ActiveCell) {
  activeCell.value = value
}

async function loadTableViews() {
  try {
    const response = await client.getTableViews({ tableName: props.tableId })
    tableViews.value = response.views.map(tableViewFromProto)
    userDefaultViewId.value = response.userDefaultViewId

    const fallback = initialTableView(tableViews.value, userDefaultViewId.value)
    const hasValidSelection = selectedViewId.value === -1
      || (
        selectedViewId.value !== null
        && tableViews.value.some(view => view.id === selectedViewId.value)
      )

    if (!hasValidSelection) {
      selectedViewId.value = fallback?.id ?? -1
    }
  } catch (viewError) {
    console.error('Failed to load table views:', viewError)
    selectedViewId.value = -1
  }
}

function selectView(viewId: number) {
  selectedViewId.value = viewId
  nextTick(() => {
    const view = tableViews.value.find(candidate => candidate.id === viewId)
    emit('view-changed', view?.viewType || 'table', viewId)
  })
}

async function loadStructure() {
  if (props.tableStructure) {
    tableStructure.value = props.tableStructure
    if (props.tableStructure.title) {
      tableTitle.value = props.tableStructure.title
    }
  } else {
    const structure = await client.getTableStructure({ pageId: props.tableId })
    tableStructure.value = structure
    if (structure.title) {
      tableTitle.value = structure.title
    }
  }
}

watch(
  () => props.tableStructure,
  value => {
    if (!value) return
    tableStructure.value = value
    if (value.title) tableTitle.value = value.title
  },
)

async function applySourceRelationExtras(loaded: Item[]): Promise<Item[]> {
  if (props.extraFields?.length || props.items !== undefined) {
    return loaded
  }
  const structure = tableStructure.value
  if (!structure) {
    relationExtraFields.value = []
    return loaded
  }
  try {
    const enriched = await enrichTableItemsWithRelationExtras(
      client,
      props.tableId,
      loaded,
      structure,
    )
    relationExtraFields.value = enriched.extraFields
    return enriched.items
  } catch (enrichError) {
    console.error('Failed to apply relation pull-through columns:', enrichError)
    relationExtraFields.value = []
    return loaded
  }
}

async function load() {
  if (props.items !== undefined) {
    loading.value = false
    return
  }

  loading.value = true
  error.value = null
  const fields = tableStructure.value?.fields ?? fieldDefs.value
  const canCacheOffline = hasUuidColumn(fields)
  const emptyWhere: Record<string, string> = { ...(props.where ?? {}) }

  try {
    if (!isOnline() && canCacheOffline) {
      const cachedItems = await loadTableData(props.tableId, emptyWhere)
      if (cachedItems !== null) {
        loadedItems.value = await applySourceRelationExtras(cachedItems as Item[])
        return
      }
      loadedItems.value = []
      error.value = 'No offline data available. Please connect to the internet to load table data.'
      return
    }

    // Only the optional `where` prop filters on the server. PicoCrank owns any other
    // filtering over the complete local row set, including rows from the offline cache.
    const response = await client.listItems({ tcName: props.tableId, where: emptyWhere })
    let loaded = Array.isArray(response.items) ? response.items : []
    loaded = await applySourceRelationExtras(loaded)
    loadedItems.value = loaded

    if (canCacheOffline && isOnline()) {
      await saveTableData(props.tableId, loadedItems.value, emptyWhere)
    }
  } catch (loadError) {
    if (canCacheOffline) {
      const cachedItems = await loadTableData(props.tableId, emptyWhere)
      if (cachedItems !== null) {
        loadedItems.value = await applySourceRelationExtras(cachedItems as Item[])
        error.value = null
        return
      }
    }
    error.value = String(loadError)
  } finally {
    loading.value = false
  }
}

watch(
  () => props.items,
  (value, previous) => {
    if (value === undefined && previous !== undefined) void load()
  },
)

watch(
  () => JSON.stringify(props.where ?? {}),
  (value, previous) => {
    if (value !== previous && props.items === undefined) void load()
  },
)

// Foreign-key lookup state
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
const referencedTableData = ref<Record<string, Item[]>>({})

async function loadForeignKeys() {
  try {
    const structure = tableStructure.value
      ?? await client.getTableStructure({ pageId: props.tableId })
    foreignKeys.value = (structure.foreignKeys ?? []).map(foreignKey => ({
      constraintName: foreignKey.constraintName,
      tableName: foreignKey.tableName,
      columnName: foreignKey.columnName,
      referencedTable: foreignKey.referencedTable,
      referencedColumn: foreignKey.referencedColumn,
      onDeleteAction: foreignKey.onDeleteAction,
      onUpdateAction: foreignKey.onUpdateAction,
      tableTcName: foreignKey.tableTcName,
      referencedTableTcName: foreignKey.referencedTableTcName,
    }))

    const data: Record<string, Item[]> = {}
    for (const foreignKey of foreignKeys.value) {
      try {
        const tcName = foreignKey.referencedTableTcName || foreignKey.referencedTable
        const response = await client.listItems({ tcName })
        data[foreignKey.columnName] = response.items ?? []
      } catch (foreignKeyError) {
        console.error(`Error loading data for table ${foreignKey.referencedTable}:`, foreignKeyError)
        data[foreignKey.columnName] = []
      }
    }
    referencedTableData.value = data
  } catch (foreignKeyError) {
    console.error('Error loading foreign keys:', foreignKeyError)
  }
}

function isForeignKey(columnName: string): boolean {
  return foreignKeys.value.some(foreignKey => foreignKey.columnName === columnName)
}

function getForeignKeyInfo(columnName: string) {
  return foreignKeys.value.find(foreignKey => foreignKey.columnName === columnName)
}

function getReferencedItem(columnName: string, foreignKeyValue: unknown): Item | undefined {
  return (referencedTableData.value[columnName] ?? [])
    .find(item => String(item.id) === String(foreignKeyValue))
}

function getReferencedItemName(item: Item | undefined): string {
  if (!item) return 'Unknown'
  return (item as Item & { name?: string }).name
    || item.additionalFields?.name
    || `ID: ${item.id}`
}

// Conditional formatting state
const conditionalFormattingRules = ref<any[]>([])

async function loadConditionalFormattingRules() {
  try {
    const response = await client.getConditionalFormattingRules({ tableName: props.tableId })
    conditionalFormattingRules.value = response.rules.map(rule => ({
      id: rule.id,
      tableName: rule.tableName,
      columnName: rule.columnName,
      conditionType: rule.conditionType,
      conditionValue: rule.conditionValue,
      formatType: rule.formatType,
      formatValue: rule.formatValue,
      priority: rule.priority,
      isActive: rule.isActive,
    }))
  } catch (formattingError) {
    console.error('Error loading conditional formatting rules:', formattingError)
    conditionalFormattingRules.value = []
  }
}

function applyConditionalFormatting(
  columnName: string,
  cellValue: unknown,
): { content: string; styles: Record<string, string> } {
  const styles: Record<string, string> = {}
  const content = cellValue == null ? '' : String(cellValue)
  const rules = conditionalFormattingRules.value
    .filter(rule => rule.columnName === columnName && rule.isActive && rule.formatType !== 'markdown')
    .sort((left, right) => right.priority - left.priority)

  for (const rule of rules) {
    let applies = false
    switch (rule.conditionType) {
      case 'always':
        applies = true
        break
      case 'equals':
        applies = content === rule.conditionValue
        break
      case 'contains':
        applies = content.toLowerCase().includes(rule.conditionValue.toLowerCase())
        break
      case 'greater_than':
        applies = Number(content) > Number(rule.conditionValue)
        break
      case 'less_than':
        applies = Number(content) < Number(rule.conditionValue)
        break
    }

    if (!applies) continue
    switch (rule.formatType) {
      case 'color':
        styles['background-color'] = rule.formatValue
        break
      case 'text_color':
        styles.color = rule.formatValue
        break
      case 'bold':
        if (rule.formatValue === 'true') styles['font-weight'] = 'bold'
        break
      case 'italic':
        if (rule.formatValue === 'true') styles['font-style'] = 'italic'
        break
      case 'url':
        break
    }
  }
  return { content, styles }
}

function resolveAffixPlaceholder(columnName: string, rawValue: unknown): string {
  if (rawValue === null || rawValue === undefined || rawValue === '') return ''
  if (isForeignKey(columnName)) {
    const ref = getReferencedItem(columnName, rawValue)
    return foreignKeyReferencedDisplayLabel(
      ref as unknown as Record<string, unknown> | undefined,
      rawValue,
    )
  }
  return String(rawValue)
}

function cellUrlLink(columnKey: string, row: TableRow) {
  const item = sourceItemForRow(row)
  const raw = getItemValue(item, columnKey)
  return urlConditionalFormatLink(columnKey, raw, conditionalFormattingRules.value)
}

function formattedCellContent(columnKey: string, row: TableRow): string {
  const value = cellValue(row, columnKey)
  const formatted = applyConditionalFormatting(columnKey, value).content
  if (isPullThroughColumn(columnKey)) {
    return formatted
  }
  const field = findFieldDef(fieldDefs.value, columnKey)
  const source = sourceItemForRow(row)
  const record = source
    ? mergeItemFieldRecord(source as unknown as Record<string, unknown>)
    : mergeItemFieldRecord(row as Record<string, unknown>)
  return applyFieldValueAffixes(formatted, field, record, resolveAffixPlaceholder)
}

function cellStyle(context: CellContext) {
  if (context.header.key === ACTIONS_COLUMN) return {}
  const value = getItemValue(sourceItemForRow(context.row), context.header.key)
  return applyConditionalFormatting(context.header.key, value).styles
}

function sourceItemForRow(row: TableRow): Item {
  return getSourceItem(row) ?? row as unknown as Item
}

function getItemValue(item: Item | Record<string, any>, column: string): any {
  const source = item as Record<string, any>
  if (column === 'id') return source.id
  if (column === tableStructure.value?.primaryKeyColumn) return source.id
  if (column === 'sr_created') return source.srCreated ?? source.sr_created
  if (column === 'sr_updated') return source.srUpdated ?? source.sr_updated
  if (column === 'sr_created_relative') {
    return source.srCreatedRelative ?? source.sr_created_relative
  }
  if (column === 'sr_updated_relative') {
    return source.srUpdatedRelative ?? source.sr_updated_relative
  }
  if (source.additionalFields?.[column] !== undefined) {
    return source.additionalFields[column]
  }
  return source[column]
}

function cellValue(row: TableRow, column: string): any {
  return getItemValue(sourceItemForRow(row), column)
}

function fieldType(column: string): string {
  return fieldDefs.value.find(field => field.name === column)?.type ?? 'unknown'
}

function isTinyintColumn(column: string): boolean {
  const type = fieldType(column).toLowerCase()
  return type.startsWith('tinyint') || type === 'bool' || type === 'boolean'
}

function isDatetimeColumn(column: string): boolean {
  const type = fieldType(column).toLowerCase()
  return type === 'datetime' || type.startsWith('datetime(')
}

function isUserRefColumn(column: string): boolean {
  return columnIsUserRef(column, fieldDefs.value, foreignKeys.value)
}

function isImageColumn(column: string): boolean {
  return isImageFieldType(fieldType(column))
}

function getBooleanValue(item: Item, column: string): boolean {
  const value = getItemValue(item, column)
  return value === true || Number(value) === 1
}

function hasMarkdownField(column: string, item: Item): boolean {
  return Boolean(item.additionalFields?.[`${column}Markdown`])
}

function getMarkdownContent(column: string, item: Item): string {
  return item.additionalFields?.[`${column}Markdown`] ?? ''
}

function formatRelativeTime(seconds: number): string {
  if (seconds < 60) return `${seconds}s ago`
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`
  return `${Math.floor(seconds / 86400)}d ago`
}

function relativeTime(item: Item, column: 'sr_created' | 'sr_updated'): number | null {
  const value = column === 'sr_created' ? item.srCreatedRelative : item.srUpdatedRelative
  return value === null || value === undefined ? null : Number(value)
}

// Inline editing
const editingCell = ref<{ rowId: string; column: string } | null>(null)
const editingValue = ref('')
const saving = ref(false)
const editInput = ref<HTMLInputElement | null>(null)

function setEditInput(element: unknown) {
  editInput.value = element instanceof HTMLInputElement ? element : null
}

function canEditColumn(column: string): boolean {
  return !['id', 'name', 'sr_created', 'sr_updated', ACTIONS_COLUMN].includes(column)
    && column !== tableStructure.value?.primaryKeyColumn
    && !isForeignKey(column)
    && !isRelationSyntheticColumn(column)
}

function startEditRow(row: TableRow, column: string) {
  if (!canEditColumn(column)) return
  const item = sourceItemForRow(row)
  const value = getItemValue(item, column)
  editingCell.value = { rowId: String(item.id), column }

  if (isDatetimeColumn(column) && value != null) {
    const date = new Date(value)
    editingValue.value = Number.isNaN(date.getTime()) ? '' : date.toISOString().slice(0, 16)
  } else {
    editingValue.value = value == null ? '' : String(value)
  }

  void nextTick(() => {
    editInput.value?.focus()
    editInput.value?.select()
  })
}

function cancelEdit() {
  editingCell.value = null
  editingValue.value = ''
}

function isEditingRow(row: TableRow, column: string): boolean {
  const item = sourceItemForRow(row)
  return editingCell.value?.rowId === String(item.id)
    && editingCell.value.column === column
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

async function saveEditRow(row: TableRow) {
  if (!editingCell.value || saving.value) return
  saving.value = true
  try {
    const item = sourceItemForRow(row)
    const { rowId, column } = editingCell.value
    const currentItem = sourceItems.value.find(candidate => String(candidate.id) === rowId) ?? item
    const additionalFields: Record<string, string> = {}
    for (const [key, value] of Object.entries(currentItem.additionalFields ?? {})) {
      if (isRelationSyntheticColumn(key)) continue
      additionalFields[key] = String(value)
    }
    additionalFields[column] = isDatetimeColumn(column)
      ? mysqlDatetime(editingValue.value)
      : editingValue.value

    await client.editItem({
      id: rowId,
      additionalFields,
      pageId: props.tableId,
    })
    cancelEdit()
    if (props.items === undefined) await load()
    emit('rows-updated')
  } catch (saveError) {
    console.error('Failed to save edit:', saveError)
  } finally {
    saving.value = false
  }
}

// Enter/Escape in the editor: save or cancel, then hand focus back to the
// cell so keyboard navigation (and Enter to edit again) carries on.
async function finishEditFromKeyboard(row: TableRow, save: boolean) {
  if (save) await saveEditRow(row)
  else cancelEdit()
  await nextTick()
  tableRef.value?.focusActiveCell()
}

// Bulk selection and deletion
const selectedItems = computed(() => {
  const keys = new Set(selectedKeys.value)
  return sourceItems.value.filter(item => keys.has(String(item.id)))
})
const hasSelectedItems = computed(() => multiSelectEnabled.value && selectedKeys.value.length > 0)
const showDeleteConfirm = ref(false)
const deleting = ref(false)

function confirmDeleteSelected() {
  if (selectedItems.value.length > 0) showDeleteConfirm.value = true
}

async function deleteSelectedItems() {
  if (selectedItems.value.length === 0) return
  deleting.value = true
  error.value = null
  try {
    for (const item of selectedItems.value) {
      const id = String(item.id)
      await client.deleteItem({ pageId: props.tableId, id })
      emit('row-deleted', id)
    }
    selectedKeys.value = []
    showDeleteConfirm.value = false
    if (props.items === undefined) await load()
    emit('rows-updated')
  } catch (deleteError) {
    error.value = String(deleteError)
  } finally {
    deleting.value = false
  }
}

async function onRowActionDeleted(row: TableRow) {
  const id = String(cellValue(row, 'id'))
  emit('row-deleted', id)
  if (props.items === undefined) await load()
  emit('rows-updated')
}

// Quick add
const showQuickAddDialog = ref(false)
const quickAddSelectedViewId = ref<number | null>(null)
const quickAddFieldDefs = computed(() => applyViewToInsertFieldDefs(
  fieldDefsForInsertForm(fieldDefs.value.map(field => ({
    name: field.name,
    type: field.type,
    required: field.required,
    displayLabel: field.displayLabel,
  }))),
  tableViews.value,
  quickAddSelectedViewId.value,
))

function openQuickAddDialog() {
  quickAddSelectedViewId.value = selectedViewId.value
    ?? initialTableView(tableViews.value, userDefaultViewId.value)?.id
    ?? -1
  showQuickAddDialog.value = true
}

function closeQuickAddDialog() {
  showQuickAddDialog.value = false
}

async function onQuickAddCreated() {
  if (props.items === undefined) await load()
  closeQuickAddDialog()
  emit('rows-updated')
}

// Keyboard navigation is delegated to PicoCrank's active-cell model.
function moveCell(rowDelta: number, columnDelta: number, wrapColumns = false) {
  tableRef.value?.moveActiveCell(rowDelta, columnDelta, {
    focus: true,
    wrapColumns,
  })
}

function editCurrentCell() {
  const context = tableRef.value?.getActiveCellContext()
  if (context && context.header.key !== ACTIONS_COLUMN) {
    startEditRow(context.row, context.header.key)
  }
}

const tableShortcuts = ref<KeyboardShortcut[]>([
  {
    key: 'ArrowUp',
    handler: event => {
      if (!editingCell.value) {
        event.preventDefault()
        moveCell(-1, 0)
      }
    },
    description: 'Navigate to previous row',
  },
  {
    key: 'ArrowDown',
    handler: event => {
      if (!editingCell.value) {
        event.preventDefault()
        moveCell(1, 0)
      }
    },
    description: 'Navigate to next row',
  },
  {
    key: 'ArrowLeft',
    handler: event => {
      if (!editingCell.value) {
        event.preventDefault()
        moveCell(0, -1)
      }
    },
    description: 'Navigate to previous column',
  },
  {
    key: 'ArrowRight',
    handler: event => {
      if (!editingCell.value) {
        event.preventDefault()
        moveCell(0, 1)
      }
    },
    description: 'Navigate to next column',
  },
  {
    key: 'Tab',
    shift: true,
    handler: event => {
      if (!editingCell.value) {
        event.preventDefault()
        moveCell(0, -1, true)
      }
    },
    description: 'Navigate to previous cell',
  },
  {
    key: 'Tab',
    handler: event => {
      if (!editingCell.value) {
        event.preventDefault()
        moveCell(0, 1, true)
      }
    },
    description: 'Navigate to next cell',
  },
  {
    key: 'Enter',
    handler: event => {
      if (!editingCell.value && tableRef.value?.getActiveCellContext()) {
        event.preventDefault()
        editCurrentCell()
      }
    },
    description: 'Edit current cell',
  },
  {
    key: 'Delete',
    handler: event => {
      if (!editingCell.value && hasSelectedItems.value) {
        event.preventDefault()
        confirmDeleteSelected()
      }
    },
    description: 'Delete selected rows',
  },
  {
    key: 'Backspace',
    handler: event => {
      if (!editingCell.value && hasSelectedItems.value) {
        event.preventDefault()
        confirmDeleteSelected()
      }
    },
    description: 'Delete selected rows',
  },
])

useKeyboardShortcuts(tableShortcuts)

const tableFilterFocusRequest = inject<{ value: (() => void) | null } | null>(
  'tableFilterFocusRequest',
  null,
)
if (tableFilterFocusRequest) {
  tableFilterFocusRequest.value = () => {
    const filterInput = document.querySelector(
      'input[type="search"], input[placeholder*="filter" i], input[placeholder*="value" i]',
    )
    if (filterInput instanceof HTMLInputElement) filterInput.focus()
  }
}

function handleOpenQuickAdd() {
  openQuickAddDialog()
}

function handleSaveCurrentEdit() {
  if (!editingCell.value) return
  const row = flattenedRows.value.find(candidate => (
    getStableRowId(candidate) === editingCell.value?.rowId
  ))
  if (row) void saveEditRow(row)
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

onMounted(async () => {
  window.addEventListener('open-quick-add', handleOpenQuickAdd)
  window.addEventListener('save-current-edit', handleSaveCurrentEdit)
  try {
    await Promise.all([
      loadStructure(),
      loadTableViews(),
      loadConditionalFormattingRules(),
    ])
    await Promise.all([
      loadForeignKeys(),
      props.items === undefined ? load() : Promise.resolve(),
    ])
  } catch (initializationError) {
    error.value = String(initializationError)
  } finally {
    loading.value = false
  }
})

onUnmounted(() => {
  window.removeEventListener('open-quick-add', handleOpenQuickAdd)
  window.removeEventListener('save-current-edit', handleSaveCurrentEdit)
  if (tableFilterFocusRequest) tableFilterFocusRequest.value = null
})
</script>

<template>
  <Teleport
    v-if="showToolbar && toolbarTeleport"
    :to="toolbarTeleport"
    defer
  >
    <TableComponentToolbar
      v-bind="tableToolbarBindings"
      @view-selected="selectView"
      @open-quick-add="openQuickAddDialog"
      @toggle-multi-select="toggleMultiSelect"
    >
      <template #toolbar-prefix>
        <slot name="toolbar-prefix" />
      </template>
      <template #toolbar-suffix>
        <slot name="toolbar-suffix" />
      </template>
    </TableComponentToolbar>
  </Teleport>

  <Section
    v-if="useOwnSection"
    :title="sectionTitle"
    :icon="sectionIcon ?? DatabaseIcon"
    :padding="false"
  >
    <template v-if="showToolbar" #toolbar>
      <TableComponentToolbar
        v-bind="tableToolbarBindings"
        @view-selected="selectView"
        @open-quick-add="openQuickAddDialog"
        @toggle-multi-select="toggleMultiSelect"
      >
        <template #toolbar-prefix>
          <slot name="toolbar-prefix" />
        </template>
        <template #toolbar-suffix>
          <slot name="toolbar-suffix" />
        </template>
      </TableComponentToolbar>
    </template>

    <div class="table-component-wrapper">
    <NotificationBlock v-if="error" type="error" :message="error" />
    <div v-else-if="loading" class="loading-state">Loading…</div>
    <div v-else class="section-content">
      <div class="table-host">
        <Table
          ref="tableRef"
          :table-id="props.tableId"
          :data="flattenedRows"
          :headers="tableHeaders"
          :row-key="getStableRowId"
          :filters="tableFilters"
          :column-visibility="columnVisibility"
          :column-order="columnOrder"
          :sort-by="sortBy"
          :sort-dir="sortDir"
          :page="page"
          :page-size="pageSize"
          :selected-keys="selectedKeys"
          :active-cell="activeCell"
          :show-pagination="showPagination"
          :reset-page-on-sort="false"
          :load-saved-layout="false"
          view-control-mode="callbackControlled"
          :view-settings="serverViewSettings"
          :group-by="groupBy"
          :column-priorities="columnPriorities"
          :column-options="true"
          :cell-style="cellStyle"
          :selectable="multiSelectEnabled"
          :selection-click-mode="multiSelectEnabled ? 'extended' : 'row'"
          @update:filters="onFiltersUpdate"
          @update:column-visibility="onColumnVisibilityUpdate"
          @update:column-order="onColumnOrderUpdate"
          @update:sort-by="sortBy = $event"
          @update:sort-dir="sortDir = $event"
          @update:page="page = $event"
          @update:page-size="pageSize = $event"
          @update:group-by="groupBy = $event"
          @update:column-priorities="columnPriorities = $event"
          @update:selected-keys="onSelectedKeysUpdate"
          @update:active-cell="onActiveCellUpdate"
        >
          <template #cell="{ row, header }">
            <template v-if="header.key === ACTIONS_COLUMN">
              <RowActionsDropdown
                :table-id="props.tableId"
                :row-id="String(cellValue(row, 'id'))"
                @deleted="onRowActionDeleted(row)"
              />
            </template>

            <template v-else-if="isEditingRow(row, header.key)">
              <div class="inline-edit" data-row-click-ignore>
                <input
                  v-if="isTinyintColumn(header.key)"
                  type="checkbox"
                  :checked="getBooleanValue(sourceItemForRow(row), header.key)"
                  :disabled="saving"
                  class="edit-checkbox"
                  @change="event => {
                    editingValue = (event.target as HTMLInputElement).checked ? '1' : '0'
                    saveEditRow(row)
                  }"
                />
                <input
                  v-else-if="isDatetimeColumn(header.key)"
                  :ref="setEditInput"
                  v-model="editingValue"
                  type="datetime-local"
                  :disabled="saving"
                  class="edit-input"
                  @keydown.enter.prevent="finishEditFromKeyboard(row, true)"
                  @keydown.escape.prevent="finishEditFromKeyboard(row, false)"
                  @blur="saveEditRow(row)"
                />
                <input
                  v-else
                  :ref="setEditInput"
                  v-model="editingValue"
                  type="text"
                  :disabled="saving"
                  class="edit-input"
                  @keydown.enter.prevent="finishEditFromKeyboard(row, true)"
                  @keydown.escape.prevent="finishEditFromKeyboard(row, false)"
                  @blur="saveEditRow(row)"
                />
              </div>
            </template>

            <div
              v-else
              class="cell-content"
              :class="{ editable: canEditColumn(header.key) }"
              :data-row-click-ignore="canEditColumn(header.key) ? '' : undefined"
              @click="startEditRow(row, header.key)"
            >
              <span
                v-if="header.key === 'sr_created' && cellValue(row, header.key) != null"
                class="date"
              >
                {{ formatUnixTimestamp(cellValue(row, header.key)) }}
                <span
                  v-if="relativeTime(sourceItemForRow(row), 'sr_created') != null"
                  class="relative-time"
                >
                  ({{ formatRelativeTime(relativeTime(sourceItemForRow(row), 'sr_created')!) }})
                </span>
              </span>
              <span
                v-else-if="header.key === 'sr_updated' && cellValue(row, header.key) != null"
                class="date"
              >
                {{ formatUnixTimestamp(cellValue(row, header.key)) }}
                <span
                  v-if="relativeTime(sourceItemForRow(row), 'sr_updated') != null"
                  class="relative-time"
                >
                  ({{ formatRelativeTime(relativeTime(sourceItemForRow(row), 'sr_updated')!) }})
                </span>
              </span>
              <router-link
                v-else-if="
                  header.key === 'id'
                  || header.key === 'name'
                  || header.key === tableStructure?.primaryKeyColumn
                "
                :to="`/table/${props.tableId}/${cellValue(row, 'id')}`"
              >
                {{ cellValue(row, header.key) }}
              </router-link>
              <ImageThumb
                v-else-if="isImageColumn(header.key) && hasImageValue(cellValue(row, header.key))"
                :image-id="String(cellValue(row, header.key))"
                size="thumb"
              />
              <UserRefChip
                v-else-if="isUserRefColumn(header.key) && cellValue(row, header.key) != null"
                :to="`/admin/iam/users/${cellValue(row, header.key)}`"
                :label="displayUserLabel(cellValue(row, header.key))"
              />
              <span
                v-else-if="isForeignKey(header.key) && cellValue(row, header.key) != null"
              >
                <router-link
                  v-if="getReferencedItem(header.key, cellValue(row, header.key))"
                  :to="`/table/${
                    getForeignKeyInfo(header.key)?.referencedTableTcName
                    || getForeignKeyInfo(header.key)?.referencedTable
                  }/${cellValue(row, header.key)}`"
                >
                  {{
                    getReferencedItemName(
                      getReferencedItem(header.key, cellValue(row, header.key)),
                    )
                  }}
                </router-link>
                <template v-else>{{ cellValue(row, header.key) }}</template>
              </span>
              <span v-else-if="cellValue(row, header.key) == null" class="subtle">NULL</span>
              <span v-else-if="isTinyintColumn(header.key)" class="boolean-display">
                <span
                  v-if="getBooleanValue(sourceItemForRow(row), header.key)"
                  class="boolean-true"
                >✓</span>
                <span v-else class="boolean-false">✗</span>
              </span>
              <span v-else-if="isDatetimeColumn(header.key)">
                {{ new Date(cellValue(row, header.key)).toLocaleString() }}
              </span>
              <div
                v-else-if="hasMarkdownField(header.key, sourceItemForRow(row))"
                class="markdown-content"
                v-html="getMarkdownContent(header.key, sourceItemForRow(row))"
              />
              <a
                v-else-if="cellUrlLink(header.key, row)"
                :href="cellUrlLink(header.key, row)!.href"
                target="_blank"
                rel="noopener noreferrer"
                @click.stop
              >
                {{ cellUrlLink(header.key, row)!.label }}
              </a>
              <span v-else>
                {{ formattedCellContent(header.key, row) }}
              </span>
            </div>
          </template>

          <template #filtered-empty="{ clearFilters }">
            <div class="filtered-empty-message">
              <div>No items match this filter.</div>
              <button class="button" @click="clearFilters">Clear filters</button>
            </div>
          </template>

          <template #empty>
            <div class="empty-state">
              <div class="empty-state-content">
                <div v-if="!isRelatedEmptyState" class="empty-state-icon">📋</div>
                <h3>{{ emptyStateHeading }}</h3>
                <router-link class="button inline-icon good" :to="effectiveInsertRoute">
                  <span>{{ insertButtonText }}</span>
                </router-link>
                <div v-if="!isRelatedEmptyState" class="action-row center empty-state-actions">
                  <router-link class="button" :to="`/table/${props.tableId}/add-column`">
                    Add Column
                  </router-link>
                </div>
              </div>
            </div>
          </template>
        </Table>
      </div>

      <div v-if="hasSelectedItems" class="selection-controls padding">
        <button
          class="button bad"
          :disabled="deleting"
          @click="confirmDeleteSelected"
        >
          🗑️ Delete Selected ({{ selectedKeys.length }})
        </button>
      </div>
    </div>

    <Dialog v-model:open="showDeleteConfirm" title="Confirm Delete" size="small">
      <p>
        Are you sure you want to delete {{ selectedKeys.length }} selected row(s)?
        This action cannot be undone.
      </p>
      <template #actions>
        <button
          type="button"
          class="button bad"
          :disabled="deleting"
          @click="deleteSelectedItems"
        >
          {{ deleting ? 'Deleting...' : `Delete ${selectedKeys.length} Row(s)` }}
        </button>
      </template>
    </Dialog>

    <Dialog
      :open="showQuickAddDialog"
      :title="insertButtonText"
      size="large"
      class="quick-add-dialog"
      @update:open="open => { if (!open) closeQuickAddDialog() }"
    >
      <div class="view-selector">
        <label for="quick-add-view-select">View:</label>
        <select
          id="quick-add-view-select"
          v-model="quickAddSelectedViewId"
          class="view-dropdown"
        >
          <option v-for="view in viewOptions" :key="view.id" :value="view.id">
            {{ view.viewName }}
          </option>
        </select>
      </div>
      <InsertRow
        :table-id="props.tableId"
        :field-defs="quickAddFieldDefs"
        @created="onQuickAddCreated"
        @cancelled="closeQuickAddDialog"
      />
    </Dialog>
    </div>
  </Section>

  <div
    v-else
    class="table-component-wrapper"
    :class="{ 'table-component-embedded': useInlineToolbar }"
  >
    <div
      v-if="useInlineToolbar"
      class="section-header flex-row embedded-table-toolbar"
    >
      <div class="fg1" />
      <div role="toolbar">
        <TableComponentToolbar
          v-bind="tableToolbarBindings"
          @view-selected="selectView"
          @open-quick-add="openQuickAddDialog"
          @toggle-multi-select="toggleMultiSelect"
        >
          <template #toolbar-prefix>
            <slot name="toolbar-prefix" />
          </template>
          <template #toolbar-suffix>
            <slot name="toolbar-suffix" />
          </template>
        </TableComponentToolbar>
      </div>
    </div>

    <NotificationBlock v-if="error" type="error" :message="error" />
    <div v-else-if="loading" class="loading-state">Loading…</div>
    <div v-else class="section-content">
      <div class="table-host">
        <Table
          ref="tableRef"
          :table-id="props.tableId"
          :data="flattenedRows"
          :headers="tableHeaders"
          :row-key="getStableRowId"
          :filters="tableFilters"
          :column-visibility="columnVisibility"
          :column-order="columnOrder"
          :sort-by="sortBy"
          :sort-dir="sortDir"
          :page="page"
          :page-size="pageSize"
          :selected-keys="selectedKeys"
          :active-cell="activeCell"
          :show-pagination="showPagination"
          :reset-page-on-sort="false"
          :load-saved-layout="false"
          view-control-mode="callbackControlled"
          :view-settings="serverViewSettings"
          :group-by="groupBy"
          :column-priorities="columnPriorities"
          :column-options="true"
          :cell-style="cellStyle"
          :selectable="multiSelectEnabled"
          :selection-click-mode="multiSelectEnabled ? 'extended' : 'row'"
          @update:filters="onFiltersUpdate"
          @update:column-visibility="onColumnVisibilityUpdate"
          @update:column-order="onColumnOrderUpdate"
          @update:sort-by="sortBy = $event"
          @update:sort-dir="sortDir = $event"
          @update:page="page = $event"
          @update:page-size="pageSize = $event"
          @update:group-by="groupBy = $event"
          @update:column-priorities="columnPriorities = $event"
          @update:selected-keys="onSelectedKeysUpdate"
          @update:active-cell="onActiveCellUpdate"
        >
          <template #cell="{ row, header }">
            <template v-if="header.key === ACTIONS_COLUMN">
              <RowActionsDropdown
                :table-id="props.tableId"
                :row-id="String(cellValue(row, 'id'))"
                @deleted="onRowActionDeleted(row)"
              />
            </template>

            <template v-else-if="isEditingRow(row, header.key)">
              <div class="inline-edit" data-row-click-ignore>
                <input
                  v-if="isTinyintColumn(header.key)"
                  type="checkbox"
                  :checked="getBooleanValue(sourceItemForRow(row), header.key)"
                  :disabled="saving"
                  class="edit-checkbox"
                  @change="event => {
                    editingValue = (event.target as HTMLInputElement).checked ? '1' : '0'
                    saveEditRow(row)
                  }"
                />
                <input
                  v-else-if="isDatetimeColumn(header.key)"
                  :ref="setEditInput"
                  v-model="editingValue"
                  type="datetime-local"
                  :disabled="saving"
                  class="edit-input"
                  @keydown.enter.prevent="finishEditFromKeyboard(row, true)"
                  @keydown.escape.prevent="finishEditFromKeyboard(row, false)"
                  @blur="saveEditRow(row)"
                />
                <input
                  v-else
                  :ref="setEditInput"
                  v-model="editingValue"
                  type="text"
                  :disabled="saving"
                  class="edit-input"
                  @keydown.enter.prevent="finishEditFromKeyboard(row, true)"
                  @keydown.escape.prevent="finishEditFromKeyboard(row, false)"
                  @blur="saveEditRow(row)"
                />
              </div>
            </template>

            <div
              v-else
              class="cell-content"
              :class="{ editable: canEditColumn(header.key) }"
              :data-row-click-ignore="canEditColumn(header.key) ? '' : undefined"
              @click="startEditRow(row, header.key)"
            >
              <span
                v-if="header.key === 'sr_created' && cellValue(row, header.key) != null"
                class="date"
              >
                {{ formatUnixTimestamp(cellValue(row, header.key)) }}
                <span
                  v-if="relativeTime(sourceItemForRow(row), 'sr_created') != null"
                  class="relative-time"
                >
                  ({{ formatRelativeTime(relativeTime(sourceItemForRow(row), 'sr_created')!) }})
                </span>
              </span>
              <span
                v-else-if="header.key === 'sr_updated' && cellValue(row, header.key) != null"
                class="date"
              >
                {{ formatUnixTimestamp(cellValue(row, header.key)) }}
                <span
                  v-if="relativeTime(sourceItemForRow(row), 'sr_updated') != null"
                  class="relative-time"
                >
                  ({{ formatRelativeTime(relativeTime(sourceItemForRow(row), 'sr_updated')!) }})
                </span>
              </span>
              <router-link
                v-else-if="
                  header.key === 'id'
                  || header.key === 'name'
                  || header.key === tableStructure?.primaryKeyColumn
                "
                :to="`/table/${props.tableId}/${cellValue(row, 'id')}`"
              >
                {{ cellValue(row, header.key) }}
              </router-link>
              <ImageThumb
                v-else-if="isImageColumn(header.key) && hasImageValue(cellValue(row, header.key))"
                :image-id="String(cellValue(row, header.key))"
                size="thumb"
              />
              <UserRefChip
                v-else-if="isUserRefColumn(header.key) && cellValue(row, header.key) != null"
                :to="`/admin/iam/users/${cellValue(row, header.key)}`"
                :label="displayUserLabel(cellValue(row, header.key))"
              />
              <span
                v-else-if="isForeignKey(header.key) && cellValue(row, header.key) != null"
              >
                <router-link
                  v-if="getReferencedItem(header.key, cellValue(row, header.key))"
                  :to="`/table/${
                    getForeignKeyInfo(header.key)?.referencedTableTcName
                    || getForeignKeyInfo(header.key)?.referencedTable
                  }/${cellValue(row, header.key)}`"
                >
                  {{
                    getReferencedItemName(
                      getReferencedItem(header.key, cellValue(row, header.key)),
                    )
                  }}
                </router-link>
                <template v-else>{{ cellValue(row, header.key) }}</template>
              </span>
              <span v-else-if="cellValue(row, header.key) == null" class="subtle">NULL</span>
              <span v-else-if="isTinyintColumn(header.key)" class="boolean-display">
                <span
                  v-if="getBooleanValue(sourceItemForRow(row), header.key)"
                  class="boolean-true"
                >✓</span>
                <span v-else class="boolean-false">✗</span>
              </span>
              <span v-else-if="isDatetimeColumn(header.key)">
                {{ new Date(cellValue(row, header.key)).toLocaleString() }}
              </span>
              <div
                v-else-if="hasMarkdownField(header.key, sourceItemForRow(row))"
                class="markdown-content"
                v-html="getMarkdownContent(header.key, sourceItemForRow(row))"
              />
              <a
                v-else-if="cellUrlLink(header.key, row)"
                :href="cellUrlLink(header.key, row)!.href"
                target="_blank"
                rel="noopener noreferrer"
                @click.stop
              >
                {{ cellUrlLink(header.key, row)!.label }}
              </a>
              <span v-else>
                {{ formattedCellContent(header.key, row) }}
              </span>
            </div>
          </template>

          <template #filtered-empty="{ clearFilters }">
            <div class="filtered-empty-message">
              <div>No items match this filter.</div>
              <button class="button" @click="clearFilters">Clear filters</button>
            </div>
          </template>

          <template #empty>
            <div class="empty-state">
              <div class="empty-state-content">
                <div v-if="!isRelatedEmptyState" class="empty-state-icon">📋</div>
                <h3>{{ emptyStateHeading }}</h3>
                <router-link class="button inline-icon good" :to="effectiveInsertRoute">
                  <span>{{ insertButtonText }}</span>
                </router-link>
                <div v-if="!isRelatedEmptyState" class="action-row center empty-state-actions">
                  <router-link class="button" :to="`/table/${props.tableId}/add-column`">
                    Add Column
                  </router-link>
                </div>
              </div>
            </div>
          </template>
        </Table>
      </div>

      <div v-if="hasSelectedItems" class="selection-controls padding">
        <button
          class="button bad"
          :disabled="deleting"
          @click="confirmDeleteSelected"
        >
          🗑️ Delete Selected ({{ selectedKeys.length }})
        </button>
      </div>
    </div>

    <Dialog v-model:open="showDeleteConfirm" title="Confirm Delete" size="small">
      <p>
        Are you sure you want to delete {{ selectedKeys.length }} selected row(s)?
        This action cannot be undone.
      </p>
      <template #actions>
        <button
          type="button"
          class="button bad"
          :disabled="deleting"
          @click="deleteSelectedItems"
        >
          {{ deleting ? 'Deleting...' : `Delete ${selectedKeys.length} Row(s)` }}
        </button>
      </template>
    </Dialog>

    <Dialog
      :open="showQuickAddDialog"
      :title="insertButtonText"
      size="large"
      class="quick-add-dialog"
      @update:open="open => { if (!open) closeQuickAddDialog() }"
    >
      <div class="view-selector">
        <label for="quick-add-view-select">View:</label>
        <select
          id="quick-add-view-select"
          v-model="quickAddSelectedViewId"
          class="view-dropdown"
        >
          <option v-for="view in viewOptions" :key="view.id" :value="view.id">
            {{ view.viewName }}
          </option>
        </select>
      </div>
      <InsertRow
        :table-id="props.tableId"
        :field-defs="quickAddFieldDefs"
        @created="onQuickAddCreated"
        @cancelled="closeQuickAddDialog"
      />
    </Dialog>
  </div>
</template>

<style scoped>
.loading-state {
  padding: 1rem;
}

.table-component-wrapper {
  display: flex !important;
  flex-direction: column !important;
  overflow: hidden !important;
}

.table-component-wrapper :deep([class*="section-body"]),
.table-component-wrapper :deep([class*="section-content"]) {
  display: flex !important;
  flex: 1 !important;
  flex-direction: column !important;
  min-height: 0 !important;
  overflow: hidden !important;
}

.section-content,
.table-host {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
  max-width: calc(100dvw - 3rem);
  overflow: auto;
}

.cell-content {
  min-height: 1.5em;
  padding: 0.25rem;
  border-radius: 3px;
}

.cell-content.editable {
  cursor: pointer;
}

.cell-content.editable:hover {
  background-color: var(--hover-background-color, #f8f9fa);
}

.inline-edit {
  padding: 0;
}

.edit-input {
  width: 100%;
  padding: 0.25rem;
  border: 2px solid #007bff;
  border-radius: 3px;
  background: white;
  font-size: inherit;
  outline: none;
}

.edit-input:focus {
  box-shadow: 0 0 0 2px rgb(0 123 255 / 25%);
}

.edit-input:disabled,
.edit-checkbox:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.edit-checkbox {
  cursor: pointer;
  transform: scale(1.2);
}

.boolean-display {
  display: flex;
  align-items: center;
  min-height: 1.5em;
}

.boolean-true,
.boolean-false {
  font-size: 1.2em;
  font-weight: bold;
}

.boolean-true {
  color: #28a745;
}

.boolean-false {
  color: #dc3545;
}

.relative-time {
  margin-left: 0.5em;
  color: #888;
  font-size: 0.8em;
  font-weight: normal;
}

.selection-controls {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-left: auto;
  flex-wrap: wrap;
}

.filtered-empty-message {
  text-align: center;
}

.filtered-empty-message .button {
  margin-top: 0.5rem;
}

.empty-state {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 400px;
  padding: 2rem;
}

.empty-state-content {
  max-width: 500px;
  text-align: center;
}

.empty-state-icon {
  margin-bottom: 1rem;
  font-size: 4rem;
  opacity: 0.6;
}

.empty-state-content h3 {
  margin: 0 0 0.5rem;
  font-size: 1.5rem;
  font-weight: 600;
}

.empty-state-content p {
  margin: 0 0 2rem;
  color: #666;
  line-height: 1.5;
}

.empty-state-actions {
  margin-top: 1.5rem;
}

.insert-button-group {
  display: flex;
  align-items: center;
  min-width: auto;
  flex: 0 0 auto;
}

.insert-button-group .button {
  margin: 0;
}

/* Not a picocrank Section toolbar (no overflow menu): wrap the controls. */
.embedded-table-toolbar > [role="toolbar"] {
  flex-wrap: wrap;
  justify-content: flex-end;
}

.view-selector {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-bottom: 1rem;
}

.view-selector label {
  font-weight: 600;
}

.view-dropdown {
  min-width: 150px;
}

.view-dropdown:focus {
  outline: none;
  border-color: #007bff;
  box-shadow: 0 0 0 2px rgba(0, 123, 255, 0.25);
}

.markdown-content {
  line-height: 1.4;
}

.markdown-content :deep(h1),
.markdown-content :deep(h2),
.markdown-content :deep(h3),
.markdown-content :deep(h4),
.markdown-content :deep(h5),
.markdown-content :deep(h6) {
  margin: 0.5em 0 0.25em;
  font-weight: bold;
}

.markdown-content :deep(p) {
  margin: 0.25em 0;
}

.markdown-content :deep(ul),
.markdown-content :deep(ol) {
  margin: 0.25em 0;
  padding-left: 1.5em;
}

.markdown-content :deep(code) {
  padding: 0.1em 0.3em;
  border-radius: 3px;
  background: #f5f5f5;
  font-family: monospace;
  font-size: 0.9em;
}

.markdown-content :deep(a) {
  color: #007bff;
  text-decoration: none;
}

.markdown-content :deep(a:hover) {
  text-decoration: underline;
}

@media (max-width: 768px) {
  .section-content,
  .table-host {
    max-width: 100%;
  }

  .selection-controls {
    justify-content: flex-start;
    width: 100%;
    margin-top: 0.5rem;
    margin-left: 0;
  }

  .empty-state {
    min-height: 300px;
    padding: 1rem;
  }

  .view-selector .view-dropdown {
    width: 100%;
  }
}
</style>
