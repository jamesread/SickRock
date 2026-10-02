<script setup lang="ts">
import { computed, inject, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import Table from 'picocrank/vue/components/Table.vue'
import Section from 'picocrank/vue/components/Section.vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import {
  Add01Icon,
  AddCircleIcon,
  DatabaseIcon,
  Download01Icon,
  Settings01Icon,
} from '@hugeicons/core-free-icons'

import type { Field, GetTableStructureResponse, Item } from '../gen/sickrock_pb'
import type { TableView } from '../composables/useTableViewManager'
import { useKeyboardShortcuts, type KeyboardShortcut } from '../composables/useKeyboardShortcuts'
import { createApiClient } from '../stores/api'
import { formatUnixTimestamp } from '../utils/dateFormatting'
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
import InsertRow from './InsertRow.vue'
import RowActionsDropdown from './RowActionsDropdown.vue'
import ViewsButton from './ViewsButton.vue'

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
}>()

const emit = defineEmits<{
  'view-created': []
  'rows-updated': []
  'row-deleted': [id: string]
  'view-changed': [viewType: string]
}>()

const client = createApiClient()
const tableRef = ref<TableApi | null>(null)
const tableStructure = ref<GetTableStructureResponse | null>(props.tableStructure ?? null)
const tableTitle = ref('')
const items = ref<Item[]>([])
const loading = ref(true)
const error = ref<string | null>(null)

const fieldDefs = computed<Field[]>(() => {
  if (props.fields?.length) {
    return props.fields.map(field => ({
      name: field.name,
      type: field.type,
      required: false,
      defaultToCurrentTimestamp: false,
    })) as Field[]
  }
  return tableStructure.value?.fields ?? []
})

const sourceItems = computed<Item[]>(() => (props.items ?? items.value) as Item[])
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

const sectionTitle = computed(() => props.title || tableTitle.value || props.tableId)
const insertButtonText = computed(() => (
  props.createButtonText || tableStructure.value?.CreateButtonText || 'Insert row'
))

const showViewSwitcher = computed(() => props.showViewSwitcher !== false)
const showViewEdit = computed(() => props.showViewEdit !== false)
const showViewCreate = computed(() => props.showViewCreate !== false)
const showExport = computed(() => props.showExport !== false)
const showStructure = computed(() => props.showStructure !== false)
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

// Server table views remain the source of truth. PicoCrank changes the
// controlled state below for this component session only.
const tableViews = ref<TableView[]>([])
const selectedViewId = ref<number | null>(null)
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
const columnOrder = ref<string[]>([])
const sortBy = ref<string | null>(null)
const sortDir = ref<SortDirection>('asc')
const page = ref(1)
const pageSize = ref(10)
const selectedKeys = ref<string[]>([])
const activeCell = ref<ActiveCell>(null)

const tableHeaders = computed(() => [
  ...buildTableHeaders(fieldDefs.value, currentView.value),
  {
    key: ACTIONS_COLUMN,
    label: '',
    sortable: false,
    filterable: false,
    hideable: false,
    width: '5rem',
    class: 'actions',
  },
])

function applyServerView() {
  const mapped = mapTableView(fieldDefs.value, currentView.value)
  columnOrder.value = [...mapped.orderedColumns, ACTIONS_COLUMN]
  columnVisibility.value = {
    ...mapped.columnVisibility,
    [ACTIONS_COLUMN]: true,
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
  ],
  applyServerView,
  { immediate: true },
)

function onColumnVisibilityUpdate(value: Record<string, boolean>) {
  columnVisibility.value = { ...value, [ACTIONS_COLUMN]: true }
}

function onColumnOrderUpdate(value: string[]) {
  const dataColumns = value.filter(key => key !== ACTIONS_COLUMN)
  columnOrder.value = [...dataColumns, ACTIONS_COLUMN]
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
    tableViews.value = response.views.map(view => ({
      id: view.id,
      tableName: view.tableName,
      viewName: view.viewName,
      isDefault: view.isDefault,
      viewType: view.viewType || 'table',
      columns: view.columns.map(column => ({
        columnName: column.columnName,
        isVisible: column.isVisible,
        columnOrder: column.columnOrder,
        sortOrder: column.sortOrder,
      })),
    }))

    const fallback = tableViews.value.find(view => view.isDefault)
      ?? tableViews.value[0]
      ?? null
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
    emit('view-changed', view?.viewType || 'table')
  })
}

async function loadStructure() {
  if (props.tableStructure) {
    tableStructure.value = props.tableStructure
  } else {
    tableStructure.value = await client.getTableStructure({ pageId: props.tableId })
  }

  try {
    const configurations = await client.getTableConfigurations({})
    tableTitle.value = configurations.pages?.find(page => page.id === props.tableId)?.title ?? ''
  } catch (titleError) {
    console.warn('Failed to load table configuration for title:', titleError)
  }
}

watch(
  () => props.tableStructure,
  value => {
    if (value) tableStructure.value = value
  },
)

async function load() {
  if (props.items !== undefined) {
    loading.value = false
    return
  }

  loading.value = true
  error.value = null
  const fields = tableStructure.value?.fields ?? fieldDefs.value
  const canCacheOffline = hasUuidColumn(fields)
  const emptyWhere: Record<string, string> = {}

  try {
    if (!isOnline() && canCacheOffline) {
      const cachedItems = await loadTableData(props.tableId, emptyWhere)
      if (cachedItems !== null) {
        items.value = cachedItems as Item[]
        return
      }
      items.value = []
      error.value = 'No offline data available. Please connect to the internet to load table data.'
      return
    }

    // Keep the initial request unfiltered. PicoCrank owns filtering over the
    // complete local row set, including rows loaded from the offline cache.
    const response = await client.listItems({ tcName: props.tableId, where: emptyWhere })
    items.value = Array.isArray(response.items) ? response.items : []

    if (canCacheOffline && isOnline()) {
      await saveTableData(props.tableId, items.value, emptyWhere)
    }
  } catch (loadError) {
    if (canCacheOffline) {
      const cachedItems = await loadTableData(props.tableId, emptyWhere)
      if (cachedItems !== null) {
        items.value = cachedItems as Item[]
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
    }
  }
  return { content, styles }
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

// Bulk selection and deletion
const selectedItems = computed(() => {
  const keys = new Set(selectedKeys.value)
  return sourceItems.value.filter(item => keys.has(String(item.id)))
})
const hasSelectedItems = computed(() => selectedKeys.value.length > 0)
const showDeleteConfirm = ref(false)
const deleting = ref(false)

function confirmDeleteSelected() {
  if (selectedItems.value.length > 0) showDeleteConfirm.value = true
}

function cancelDeleteSelected() {
  showDeleteConfirm.value = false
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
const quickAddFieldDefs = computed(() => {
  if (quickAddSelectedViewId.value === null || quickAddSelectedViewId.value === -1) {
    return fieldDefs.value
  }
  const selectedView = tableViews.value.find(view => view.id === quickAddSelectedViewId.value)
  if (!selectedView) return fieldDefs.value
  const visibleColumns = selectedView.columns
    .filter(column => column.isVisible)
    .sort((left, right) => left.columnOrder - right.columnOrder)
    .map(column => column.columnName)
  return fieldDefs.value.filter(field => visibleColumns.includes(field.name))
})

function openQuickAddDialog() {
  quickAddSelectedViewId.value = selectedViewId.value
  showQuickAddDialog.value = true
  setTimeout(() => {
    const dialog = document.querySelector('.modal-overlay')
    if (dialog instanceof HTMLElement) dialog.focus()
  }, 100)
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
  <Section
    :title="sectionTitle"
    :icon="DatabaseIcon"
    :padding="false"
    class="table-component-wrapper"
  >
    <template v-if="showToolbar" #toolbar>
      <ViewsButton
        v-if="showViewSwitcher || showViewEdit || showViewCreate"
        :table-id="props.tableId"
        :show-view-create="showViewCreate"
        :show-view-edit="showViewEdit"
        :external-views="tableViews"
        :external-selected-view-id="selectedViewId"
        :external-current-view="currentView"
        @view-selected="selectView"
      />
      <router-link
        v-if="showExport"
        :to="`/table/${props.tableId}/export`"
        class="button inline-icon neutral ss-large"
      >
        <HugeiconsIcon :icon="Download01Icon" width="1em" height="1em" aria-hidden="true" />
        <span>Export</span>
      </router-link>
      <router-link
        v-if="showStructure"
        :to="`/table/${props.tableId}/column-types`"
        class="button inline-icon neutral ss-large"
      >
        <HugeiconsIcon :icon="Settings01Icon" width="1em" height="1em" aria-hidden="true" />
        <span>Structure</span>
      </router-link>
      <div v-if="showInsert" class="insert-button-group">
        <router-link
          :to="`/table/${props.tableId}/insert-row`"
          class="button inline-icon neutral insert-button"
          accesskey="n"
          title="Insert row"
        >
          <HugeiconsIcon :icon="AddCircleIcon" width="1em" height="1em" aria-hidden="true" />
          <span>{{ insertButtonText }}</span>
        </router-link>
        <button
          class="button inline-icon good quick-add-button"
          title="Quick Add"
          aria-label="Quick Add"
          @click="openQuickAddDialog"
        >
          <HugeiconsIcon :icon="Add01Icon" width="1em" height="1em" aria-hidden="true" />
          <span class="quick-add-text" />
        </button>
      </div>
    </template>

    <div v-if="error" class="error">{{ error }}</div>
    <div v-else-if="loading" class="loading-state">Loading…</div>
    <div v-else class="section-content">
      <div class="table-host">
        <Table
          ref="tableRef"
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
          :column-options="true"
          :cell-style="cellStyle"
          selectable
          selection-click-mode="extended"
          @update:filters="onFiltersUpdate"
          @update:column-visibility="onColumnVisibilityUpdate"
          @update:column-order="onColumnOrderUpdate"
          @update:sort-by="sortBy = $event"
          @update:sort-dir="sortDir = $event"
          @update:page="page = $event"
          @update:page-size="pageSize = $event"
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
                  @keyup.enter="saveEditRow(row)"
                  @keyup.escape="cancelEdit"
                  @blur="saveEditRow(row)"
                />
                <input
                  v-else
                  :ref="setEditInput"
                  v-model="editingValue"
                  type="text"
                  :disabled="saving"
                  class="edit-input"
                  @keyup.enter="saveEditRow(row)"
                  @keyup.escape="cancelEdit"
                  @blur="saveEditRow(row)"
                />
              </div>
            </template>

            <div
              v-else
              class="cell-content"
              :class="{ editable: canEditColumn(header.key) }"
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
              <span v-else>
                {{ applyConditionalFormatting(header.key, cellValue(row, header.key)).content }}
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

    <div v-if="showDeleteConfirm" class="modal-overlay" @click="cancelDeleteSelected">
      <div class="modal-content" @click.stop>
        <h3>Confirm Delete</h3>
        <p>
          Are you sure you want to delete {{ selectedKeys.length }} selected row(s)?
          This action cannot be undone.
        </p>
        <div class="modal-actions">
          <button
            class="button neutral"
            :disabled="deleting"
            @click="cancelDeleteSelected"
          >
            Cancel
          </button>
          <button
            class="button bad"
            :disabled="deleting"
            @click="deleteSelectedItems"
          >
            {{ deleting ? 'Deleting...' : `Delete ${selectedKeys.length} Row(s)` }}
          </button>
        </div>
      </div>
    </div>

    <div
      v-if="showQuickAddDialog"
      class="modal-overlay"
      tabindex="0"
      @click="closeQuickAddDialog"
      @keydown.escape="closeQuickAddDialog"
    >
      <div class="modal-content quick-add-modal" @click.stop>
        <div class="modal-header">
          <div class="modal-header-left">
            <h3>{{ insertButtonText }}</h3>
            <div v-if="viewOptions.length > 1" class="quick-add-view-selector">
              <label for="quick-add-view">View:</label>
              <select
                id="quick-add-view"
                v-model="quickAddSelectedViewId"
                class="view-dropdown"
              >
                <option v-for="view in viewOptions" :key="view.id" :value="view.id">
                  {{ view.viewName }}
                </option>
              </select>
            </div>
          </div>
          <button class="button neutral" title="Close" @click="closeQuickAddDialog">✕</button>
        </div>
        <div class="modal-body">
          <InsertRow
            :table-id="props.tableId"
            :field-defs="quickAddFieldDefs"
            @created="onQuickAddCreated"
            @cancelled="closeQuickAddDialog"
          />
        </div>
      </div>
    </div>
  </Section>
</template>

<style scoped>
.error {
  color: #b00020;
}

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
  display: flex;
  justify-content: center;
  gap: 1rem;
  margin-top: 1.5rem;
  flex-wrap: wrap;
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

.modal-overlay {
  position: fixed;
  z-index: 1000;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  box-sizing: border-box;
  padding: 1rem;
  background-color: rgb(0 0 0 / 50%);
}

.modal-content {
  width: 90%;
  max-width: 400px;
  padding: 1rem;
  border-radius: 8px;
  background: white;
  box-shadow: 0 4px 20px rgb(0 0 0 / 30%);
}

.modal-content p {
  margin: 0 0 1.5rem;
  color: #666;
  line-height: 1.5;
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 1rem;
}

.quick-add-modal {
  width: 90%;
  max-width: 600px;
  max-height: 80vh;
  overflow-y: auto;
}

.modal-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  padding: 0.75rem;
  border-bottom: 1px solid #ddd;
}

.modal-header-left {
  display: flex;
  align-items: center;
  gap: 1rem;
  flex: 1;
}

.modal-header h3 {
  margin: 0;
  font-size: 1.25rem;
}

.modal-header button {
  min-width: auto;
  padding: 0.5rem;
  font-size: 1.2rem;
  line-height: 1;
}

.modal-body {
  padding: 0.75rem;
}

.quick-add-view-selector {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 0.75rem;
  border: 1px solid #e9ecef;
  border-radius: 4px;
  background: #f8f9fa;
}

.quick-add-view-selector label {
  margin: 0;
  color: #333;
  font-size: 0.9rem;
  font-weight: 600;
}

.view-dropdown {
  min-width: 120px;
  padding: 0.4rem 0.6rem;
  border: 1px solid #ddd;
  border-radius: 4px;
  background: white;
  cursor: pointer;
  font-size: 0.9rem;
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

  .ss-large,
  .quick-add-text {
    display: none;
  }

  .insert-button-group {
    flex-direction: row;
  }

  .insert-button {
    padding-right: 0.5rem;
    border-right: 0;
    border-top-right-radius: 0;
    border-bottom-right-radius: 0;
  }

  .quick-add-button {
    padding-left: 0.5rem;
    border-left: 0;
    border-top-left-radius: 0;
    border-bottom-left-radius: 0;
  }

  .selection-controls {
    justify-content: flex-start;
    width: 100%;
    margin-top: 0.5rem;
    margin-left: 0;
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

  .empty-state-actions {
    flex-direction: column;
    align-items: center;
  }

  .modal-overlay {
    align-items: flex-start;
    padding: 2rem 0.5rem 0.5rem;
  }

  .modal-content {
    width: 100%;
    max-width: none;
    padding: 0.5rem;
    border-radius: 4px;
  }

  .quick-add-modal {
    width: 95%;
    max-width: 95%;
    max-height: 90vh;
    margin: 0.5rem;
  }

  .modal-header,
  .modal-header-left {
    flex-direction: column;
    align-items: stretch;
    gap: 0.5rem;
  }

  .quick-add-view-selector {
    flex-direction: column;
    align-items: stretch;
  }

  .quick-add-view-selector .view-dropdown {
    width: 100%;
  }
}
</style>
