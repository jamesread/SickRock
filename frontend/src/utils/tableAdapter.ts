import type { Field, GetTableStructureResponse, Item } from '../gen/sickrock_pb'

export type SortDirection = 'asc' | 'desc'
export type TableFilterType = 'text' | 'number' | 'boolean'
export type TableRow = Record<string, unknown>

export interface TableViewColumnLike {
  columnName: string
  isVisible: boolean
  columnOrder: number
  sortOrder: string
}

export interface TableViewLike {
  id?: number
  columns: readonly TableViewColumnLike[]
}

export interface InitialTableSort {
  sortBy: string | null
  sortDir: SortDirection
}

export interface MappedTableView extends InitialTableSort {
  orderedColumns: string[]
  visibleColumns: string[]
  columnVisibility: Record<string, boolean>
}

export type TableValueComparator = (
  left: unknown,
  right: unknown,
  leftRow?: TableRow,
  rightRow?: TableRow,
) => number

/**
 * PicoCrank's header shape plus the comparator used by the SickRock adapter.
 * PicoCrank currently ignores unknown header properties, so `comparator` can
 * be consumed by the table integration without changing its header input.
 */
export interface SickRockTableHeader {
  key: string
  label: string
  sortable: true
  filterable: true
  filterType: TableFilterType
  width?: string
  colPriority?: number
  fieldType: string
  comparator: TableValueComparator
}

const sourceItems = new WeakMap<TableRow, Item>()

const specialColumnAliases = {
  sr_created: 'srCreated',
  sr_updated: 'srUpdated',
  sr_created_relative: 'srCreatedRelative',
  sr_updated_relative: 'srUpdatedRelative',
} as const

function isDefined(value: unknown): boolean {
  return value !== null && value !== undefined
}

function firstDefined(...values: unknown[]): unknown {
  return values.find(isDefined)
}

function setAliases(row: TableRow, snakeName: string, camelName: string, value: unknown): void {
  if (!isDefined(value)) {
    return
  }
  row[snakeName] = value
  row[camelName] = value
}

/**
 * Flatten a protobuf Item for PicoCrank while preserving the original Item in
 * a private WeakMap. Additional fields take precedence over ordinary direct
 * properties, while protobuf-owned fields retain their canonical values.
 */
export function flattenItem(item: Item): TableRow {
  const source = item as Item & Record<string, unknown>
  const additionalFields = item.additionalFields ?? {}
  const row: TableRow = {}

  for (const [key, value] of Object.entries(source)) {
    if (key !== 'additionalFields' && !key.startsWith('$')) {
      row[key] = value
    }
  }

  for (const [key, value] of Object.entries(additionalFields)) {
    row[key] = value
  }

  const id = firstDefined(source.id, additionalFields.id)
  if (isDefined(id)) {
    row.id = id
  }

  setAliases(
    row,
    'sr_created',
    specialColumnAliases.sr_created,
    firstDefined(source.srCreated, additionalFields.sr_created, source.sr_created),
  )
  setAliases(
    row,
    'sr_updated',
    specialColumnAliases.sr_updated,
    firstDefined(source.srUpdated, additionalFields.sr_updated, source.sr_updated),
  )
  setAliases(
    row,
    'sr_created_relative',
    specialColumnAliases.sr_created_relative,
    firstDefined(
      source.srCreatedRelative,
      additionalFields.sr_created_relative,
      source.sr_created_relative,
    ),
  )
  setAliases(
    row,
    'sr_updated_relative',
    specialColumnAliases.sr_updated_relative,
    firstDefined(
      source.srUpdatedRelative,
      additionalFields.sr_updated_relative,
      source.sr_updated_relative,
    ),
  )

  sourceItems.set(row, item)
  return row
}

export function flattenItems(items: readonly Item[]): TableRow[] {
  return items.map(flattenItem)
}

export function getSourceItem(row: unknown): Item | undefined {
  if (typeof row !== 'object' || row === null) {
    return undefined
  }
  return sourceItems.get(row as TableRow)
}

/**
 * Return the string key expected by PicoCrank's rowKey API.
 */
export function getStableRowId(row: unknown): string {
  if (typeof row !== 'object' || row === null) {
    return ''
  }

  const tableRow = row as TableRow
  const source = getSourceItem(tableRow)
  const id = firstDefined(
    source?.id,
    source?.additionalFields?.id,
    tableRow.id,
  )
  return isDefined(id) ? String(id) : ''
}

function fieldsFrom(
  structureOrFields: Pick<GetTableStructureResponse, 'fields'> | readonly Field[],
): readonly Field[] {
  if ('fields' in structureOrFields) {
    return structureOrFields.fields
  }
  return structureOrFields
}

function displayFields(
  structureOrFields: Pick<GetTableStructureResponse, 'fields'> | readonly Field[],
): Field[] {
  const seen = new Set<string>()
  return fieldsFrom(structureOrFields).filter((field) => {
    if (!field.name || field.name.endsWith('Markdown') || seen.has(field.name)) {
      return false
    }
    seen.add(field.name)
    return true
  })
}

function isAllColumnsView(view: TableViewLike | null | undefined): boolean {
  return !view || view.id === -1 || view.columns.length === 0
}

/**
 * Resolve server view columns into the inputs accepted by PicoCrank Table.
 * Columns absent from a saved view remain available but start hidden. The
 * synthetic All Columns view keeps structure order, makes every column
 * visible, and resets sorting.
 */
export function mapTableView(
  structureOrFields: Pick<GetTableStructureResponse, 'fields'> | readonly Field[],
  view?: TableViewLike | null,
): MappedTableView {
  const fieldNames = displayFields(structureOrFields).map((field) => field.name)

  if (isAllColumnsView(view)) {
    return {
      orderedColumns: [...fieldNames],
      visibleColumns: [...fieldNames],
      columnVisibility: Object.fromEntries(fieldNames.map((name) => [name, true])),
      sortBy: null,
      sortDir: 'asc',
    }
  }

  const selectedView = view as TableViewLike
  const knownFields = new Set(fieldNames)
  const configuredColumns = selectedView.columns
    .map((column, index) => ({ column, index }))
    .filter(({ column }) => knownFields.has(column.columnName))
    .sort((left, right) => (
      left.column.columnOrder - right.column.columnOrder || left.index - right.index
    ))

  const configuredNames: string[] = []
  const configuredByName = new Map<string, TableViewColumnLike>()
  for (const { column } of configuredColumns) {
    if (!configuredByName.has(column.columnName)) {
      configuredNames.push(column.columnName)
      configuredByName.set(column.columnName, column)
    }
  }

  const orderedColumns = [
    ...configuredNames,
    ...fieldNames.filter((name) => !configuredByName.has(name)),
  ]
  const columnVisibility = Object.fromEntries(
    orderedColumns.map((name) => [name, configuredByName.get(name)?.isVisible === true]),
  )
  const visibleColumns = orderedColumns.filter((name) => columnVisibility[name])
  const sortColumn = configuredColumns.find(({ column }) => (
    column.sortOrder === 'asc' || column.sortOrder === 'desc'
  ))?.column

  return {
    orderedColumns,
    visibleColumns,
    columnVisibility,
    sortBy: sortColumn?.columnName ?? null,
    sortDir: sortColumn?.sortOrder === 'desc' ? 'desc' : 'asc',
  }
}

export function naturalCompare(left: unknown, right: unknown): number {
  const leftParts = String(left).match(/(\d+|\D+)/g) ?? []
  const rightParts = String(right).match(/(\d+|\D+)/g) ?? []
  const maxLength = Math.max(leftParts.length, rightParts.length)

  for (let index = 0; index < maxLength; index += 1) {
    const leftPart = leftParts[index] ?? ''
    const rightPart = rightParts[index] ?? ''
    const leftIsNumber = /^\d+$/.test(leftPart)
    const rightIsNumber = /^\d+$/.test(rightPart)

    if (leftIsNumber && rightIsNumber) {
      const leftNumber = BigInt(leftPart)
      const rightNumber = BigInt(rightPart)
      if (leftNumber !== rightNumber) {
        return leftNumber < rightNumber ? -1 : 1
      }
      continue
    }

    const comparison = leftPart.toLowerCase().localeCompare(rightPart.toLowerCase())
    if (comparison !== 0) {
      return comparison
    }
  }

  return 0
}

function normalizedFieldType(fieldType: string): string {
  return fieldType.trim().toLowerCase()
}

function isIntegerType(fieldType: string): boolean {
  const type = normalizedFieldType(fieldType)
  return /(^|[^a-z])(tinyint|smallint|mediumint|int|integer|bigint|int64|serial)/.test(type)
}

function isNumberType(fieldType: string): boolean {
  const type = normalizedFieldType(fieldType)
  return isIntegerType(type) || /(^|[^a-z])(decimal|numeric|float|double|real)/.test(type)
}

function isExactDecimalType(fieldType: string): boolean {
  const type = normalizedFieldType(fieldType)
  return type.startsWith('decimal') || type.startsWith('numeric')
}

function isBooleanType(fieldType: string): boolean {
  const type = normalizedFieldType(fieldType)
  return type === 'boolean' || type === 'bool' || type.startsWith('tinyint')
}

function isDatetimeType(fieldType: string): boolean {
  const type = normalizedFieldType(fieldType)
  return type === 'date'
    || type === 'datetime'
    || type.startsWith('datetime(')
    || type === 'timestamp'
    || type.startsWith('timestamp(')
}

function integerValue(value: unknown): bigint | undefined {
  if (typeof value === 'bigint') {
    return value
  }
  if (typeof value === 'number' && Number.isSafeInteger(value)) {
    return BigInt(value)
  }
  if (typeof value === 'string' && /^[+-]?\d+$/.test(value.trim())) {
    return BigInt(value.trim())
  }
  return undefined
}

function compareNumbers(left: unknown, right: unknown): number | undefined {
  const leftInteger = integerValue(left)
  const rightInteger = integerValue(right)
  if (leftInteger !== undefined && rightInteger !== undefined) {
    return leftInteger === rightInteger ? 0 : leftInteger < rightInteger ? -1 : 1
  }

  const leftNumber = Number(left)
  const rightNumber = Number(right)
  if (Number.isFinite(leftNumber) && Number.isFinite(rightNumber)) {
    return leftNumber - rightNumber
  }
  return undefined
}

function decimalParts(value: unknown): {
  sign: number
  integer: string
  fraction: string
} | undefined {
  const match = String(value).trim().match(/^([+-])?(\d*)(?:\.(\d*))?$/)
  if (!match || (!match[2] && !match[3])) {
    return undefined
  }
  const integer = (match[2] || '0').replace(/^0+(?=\d)/, '')
  const fraction = (match[3] || '').replace(/0+$/, '')
  const zero = integer === '0' && fraction === ''
  return {
    sign: zero ? 1 : match[1] === '-' ? -1 : 1,
    integer,
    fraction,
  }
}

function compareExactDecimals(left: unknown, right: unknown): number | undefined {
  const leftParts = decimalParts(left)
  const rightParts = decimalParts(right)
  if (!leftParts || !rightParts) {
    return undefined
  }
  if (leftParts.sign !== rightParts.sign) {
    return leftParts.sign - rightParts.sign
  }

  const sign = leftParts.sign
  if (leftParts.integer.length !== rightParts.integer.length) {
    return (leftParts.integer.length - rightParts.integer.length) * sign
  }
  const integerComparison = leftParts.integer.localeCompare(rightParts.integer)
  if (integerComparison !== 0) {
    return integerComparison * sign
  }

  const fractionLength = Math.max(leftParts.fraction.length, rightParts.fraction.length)
  const fractionComparison = leftParts.fraction
    .padEnd(fractionLength, '0')
    .localeCompare(rightParts.fraction.padEnd(fractionLength, '0'))
  return fractionComparison * sign
}

function datetimeValue(value: unknown): number | bigint | undefined {
  if (typeof value === 'bigint') {
    return value
  }
  if (typeof value === 'number' && Number.isFinite(value)) {
    return value
  }
  if (typeof value === 'string' && /^[+-]?\d+$/.test(value.trim())) {
    return BigInt(value.trim())
  }

  const parsed = Date.parse(String(value))
  return Number.isNaN(parsed) ? undefined : parsed
}

function compareDatetimes(left: unknown, right: unknown): number | undefined {
  const leftDate = datetimeValue(left)
  const rightDate = datetimeValue(right)
  if (leftDate === undefined || rightDate === undefined) {
    return undefined
  }
  return compareNumbers(leftDate, rightDate)
}

function compareValues(left: unknown, right: unknown, fieldType: string): number {
  if (isDatetimeType(fieldType)) {
    const comparison = compareDatetimes(left, right)
    if (comparison !== undefined) {
      return comparison
    }
  }

  if (isExactDecimalType(fieldType)) {
    const comparison = compareExactDecimals(left, right)
    if (comparison !== undefined) {
      return comparison
    }
  }

  if (
    isNumberType(fieldType)
    || typeof left === 'number'
    || typeof right === 'number'
    || typeof left === 'bigint'
    || typeof right === 'bigint'
  ) {
    const comparison = compareNumbers(left, right)
    if (comparison !== undefined) {
      return comparison
    }
  }

  return naturalCompare(left, right)
}

export function createTableComparator(fieldType: string): TableValueComparator {
  return (left, right) => {
    const leftIsNull = left === null || left === undefined
    const rightIsNull = right === null || right === undefined
    if (leftIsNull || rightIsNull) {
      if (leftIsNull && rightIsNull) {
        return 0
      }
      return leftIsNull ? 1 : -1
    }

    return compareValues(left, right, fieldType)
  }
}

function filterTypeFor(fieldType: string): TableFilterType {
  if (isBooleanType(fieldType)) {
    return 'boolean'
  }
  // SickRock's existing filter contract is string/exact-or-contains. Keeping
  // numeric database fields as text also avoids Number precision loss for
  // BIGINT and DECIMAL values.
  return 'text'
}

function widthFor(columnName: string, fieldType: string): string | undefined {
  if (columnName === 'id') {
    return '8rem'
  }
  if (isDatetimeType(fieldType)) {
    return '13rem'
  }
  if (isBooleanType(fieldType)) {
    return '7rem'
  }
  if (isNumberType(fieldType)) {
    return '10rem'
  }
  return undefined
}

function labelFor(columnName: string): string {
  if (columnName === 'id') {
    return 'ID'
  }
  if (columnName === 'sr_created') {
    return 'Created'
  }
  if (columnName === 'sr_updated') {
    return 'Updated'
  }
  return columnName
}

function priorityForIndex(index: number): number | undefined {
  if (index <= 0) {
    return undefined
  }
  return Math.max(1, 6 - index)
}

/**
 * Build ordered PicoCrank headers from the live structure and selected view.
 */
export function buildTableHeaders(
  structureOrFields: Pick<GetTableStructureResponse, 'fields'> | readonly Field[],
  view?: TableViewLike | null,
): SickRockTableHeader[] {
  const fields = displayFields(structureOrFields)
  const fieldByName = new Map(fields.map((field) => [field.name, field]))
  const mappedView = mapTableView(fields, view)

  return mappedView.orderedColumns.flatMap((columnName, index) => {
    const field = fieldByName.get(columnName)
    if (!field) {
      return []
    }

    const header: SickRockTableHeader = {
      key: columnName,
      label: labelFor(columnName),
      sortable: true,
      filterable: true,
      filterType: filterTypeFor(field.type),
      fieldType: field.type,
      comparator: createTableComparator(field.type),
    }
    const width = widthFor(columnName, field.type)
    const colPriority = priorityForIndex(index)
    if (width) {
      header.width = width
    }
    if (colPriority) {
      header.colPriority = colPriority
    }
    return [header]
  })
}

/**
 * Sort flattened rows with the same comparator exposed on a table header.
 */
export function sortTableRows(
  rows: readonly TableRow[],
  header: SickRockTableHeader,
  direction: SortDirection = 'asc',
): TableRow[] {
  return [...rows].sort((left, right) => {
    const leftValue = left[header.key]
    const rightValue = right[header.key]
    const leftIsNull = leftValue === null || leftValue === undefined
    const rightIsNull = rightValue === null || rightValue === undefined
    if (leftIsNull || rightIsNull) {
      if (leftIsNull && rightIsNull) {
        return 0
      }
      return leftIsNull ? 1 : -1
    }
    const comparison = header.comparator(leftValue, rightValue, left, right)
    return direction === 'desc' ? -comparison : comparison
  })
}
