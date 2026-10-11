import type {
  ColumnDisplaySettingInput,
  Field,
  ForeignKey,
  Item,
  RelationPullThroughField,
} from '../gen/sickrock_pb'
import {
  columnDisplaySettingsByName,
  fieldDisplayLabel,
  fieldWithColumnDisplay,
  formatFieldDisplayValue,
} from './fieldColumnDisplay'

export type PullThroughSpec = { fkColumn: string; field: string }

export type CalculatedColumnSpec = { displayLabel: string; expr: string }

export type ReferencedPullThroughContext = {
  fields: Field[]
  foreignKeys: ForeignKey[]
  referencedItemsByColumn: Record<string, Item[]>
}

export function pullThroughStorageKey(fkColumn: string, field: string): string {
  return `__pt_${fkColumn}_${field}`
}

export function relationCalcStorageKey(displayLabel: string, index: number): string {
  const slug = displayLabel
    .trim()
    .toLowerCase()
    .replace(/\s+/g, '_')
    .replace(/[^a-z0-9_]/g, '')
  return slug ? `__calc_${slug}` : `__calc_${index}`
}

export function isPullThroughColumn(column: string): boolean {
  return column.startsWith('__pt_')
}

export function isCalculatedRelationColumn(column: string): boolean {
  return column.startsWith('__calc_')
}

export function isRelationSyntheticColumn(column: string): boolean {
  return isPullThroughColumn(column) || isCalculatedRelationColumn(column)
}

export function syntheticColumnNamesForSourceTable(
  tableConfiguration: string,
  foreignKeys: ForeignKey[],
): string[] {
  const tableTc = tableConfiguration.trim()
  const names: string[] = []
  const seen = new Set<string>()
  for (const fk of foreignKeys) {
    const source = (fk.tableTcName?.trim() || fk.tableName).trim()
    if (source !== tableTc) {
      continue
    }
    const cols = fk.pullThroughFields ?? []
    cols.forEach((col, index) => {
      let key = ''
      if (col.expr?.trim()) {
        key = relationCalcStorageKey(col.displayLabel, index)
      } else if (col.fkColumn && col.field) {
        key = pullThroughStorageKey(col.fkColumn, col.field)
      }
      if (key && !seen.has(key)) {
        seen.add(key)
        names.push(key)
      }
    })
  }
  return names
}

export function pullThroughColumnLabel(
  fkColumn: string,
  field: string,
  fkFields?: Field[],
  referencedFields?: Field[],
): string {
  const fkLabel = fieldDisplayLabel(
    fkFields?.find(f => f.name === fkColumn),
    fkColumn,
  )
  const fieldLabel = fieldDisplayLabel(
    referencedFields?.find(f => f.name === field),
    field,
  )
  return `${fieldLabel} (${fkLabel})`
}

export function pullThroughSpecsFromRelationColumns(
  columns: RelationPullThroughField[],
): PullThroughSpec[] {
  return columns
    .filter(col => !col.expr?.trim() && col.fkColumn && col.field)
    .map(col => ({ fkColumn: col.fkColumn, field: col.field }))
}

export function calculatedSpecsFromRelationColumns(
  columns: RelationPullThroughField[],
): CalculatedColumnSpec[] {
  return columns
    .filter(col => col.expr?.trim())
    .map(col => ({
      displayLabel: col.displayLabel?.trim() || 'Calculated',
      expr: col.expr.trim(),
    }))
}

export function buildPullThroughFieldDefs(
  specs: PullThroughSpec[],
  childFields: Field[],
  referencedContextByFk: Record<string, ReferencedPullThroughContext>,
  columnDisplaySettings?: ColumnDisplaySettingInput[],
): Field[] {
  const displayByName = columnDisplaySettingsByName(columnDisplaySettings)
  return specs.map(spec => {
    const ctx = referencedContextByFk[spec.fkColumn]
    const refField = ctx?.fields.find(f => f.name === spec.field)
    const name = pullThroughStorageKey(spec.fkColumn, spec.field)
    const defaultLabel = pullThroughColumnLabel(
      spec.fkColumn,
      spec.field,
      childFields,
      ctx?.fields ?? [],
    )
    const base = refField
      ? ({
        ...refField,
        name,
        required: false,
      } as Field)
      : ({
        name,
        type: 'string',
        required: false,
        defaultToCurrentTimestamp: false,
        displayLabel: defaultLabel,
        valuePrefix: '',
        valueSuffix: '',
      } as Field)
    return fieldWithColumnDisplay(base, displayByName, { defaultLabel })
  })
}

export function buildCalculatedFieldDefs(
  columns: RelationPullThroughField[],
  columnDisplaySettings?: ColumnDisplaySettingInput[],
): Field[] {
  const displayByName = columnDisplaySettingsByName(columnDisplaySettings)
  return columns.flatMap((col, index) => {
    if (!col.expr?.trim()) {
      return []
    }
    const defaultLabel = col.displayLabel?.trim() || 'Calculated'
    const name = relationCalcStorageKey(defaultLabel, index)
    const base = {
      name,
      type: 'string',
      required: false,
      defaultToCurrentTimestamp: false,
      displayLabel: defaultLabel,
      valuePrefix: '',
      valueSuffix: '',
    } as Field
    return [fieldWithColumnDisplay(base, displayByName, { defaultLabel })]
  })
}

export function buildRelationExtraFieldDefs(
  columns: RelationPullThroughField[],
  childFields: Field[],
  referencedContextByFk: Record<string, ReferencedPullThroughContext>,
  columnDisplaySettings?: ColumnDisplaySettingInput[],
): Field[] {
  const pullSpecs = pullThroughSpecsFromRelationColumns(columns)
  return [
    ...buildPullThroughFieldDefs(
      pullSpecs,
      childFields,
      referencedContextByFk,
      columnDisplaySettings,
    ),
    ...buildCalculatedFieldDefs(columns, columnDisplaySettings),
  ]
}

function readFieldValue(record: Record<string, unknown>, field: string): unknown {
  if (Object.prototype.hasOwnProperty.call(record, field)) {
    return record[field]
  }
  const af = record.additionalFields
  if (af && typeof af === 'object' && field in (af as Record<string, unknown>)) {
    return (af as Record<string, unknown>)[field]
  }
  return undefined
}

function writePullThroughValue(item: Item, key: string, value: unknown): Item {
  const af = { ...(item.additionalFields ?? {}) }
  af[key] = value == null ? '' : String(value)
  return { ...item, additionalFields: af } as Item
}

type GetItemFn = (pageId: string, id: string) => Promise<Item | null | undefined>

/** Copy configured fields from FK target rows onto related list items. */
export async function enrichItemsWithPullThrough(
  items: Item[],
  specs: PullThroughSpec[],
  childForeignKeys: ForeignKey[],
  getItem: GetItemFn,
  referencedContextByFk: Record<string, ReferencedPullThroughContext>,
  displayFieldsByStorageKey: Record<string, Field>,
): Promise<Item[]> {
  if (!items.length || !specs.length) {
    return items
  }
  const fkByColumn = new Map(
    childForeignKeys.map(fk => [fk.columnName.toLowerCase(), fk]),
  )
  const referencedCache = new Map<string, Record<string, unknown>>()

  async function referencedRow(fk: ForeignKey, id: string): Promise<Record<string, unknown> | undefined> {
    const tcName = fk.referencedTableTcName || fk.referencedTable
    const cacheKey = `${tcName}:${id}`
    if (referencedCache.has(cacheKey)) {
      return referencedCache.get(cacheKey)
    }
    try {
      const row = await getItem(tcName, id)
      if (!row) return undefined
      const record = {
        ...(row as unknown as Record<string, unknown>),
        ...(row.additionalFields ?? {}),
      }
      referencedCache.set(cacheKey, record)
      return record
    } catch {
      return undefined
    }
  }

  let out = items
  for (const spec of specs) {
    const fk = fkByColumn.get(spec.fkColumn.toLowerCase())
    if (!fk) continue
    const ctx = referencedContextByFk[spec.fkColumn]
    const refField = ctx?.fields.find(f => f.name === spec.field)
    const key = pullThroughStorageKey(spec.fkColumn, spec.field)
    const displayField = displayFieldsByStorageKey[key]
    const next: Item[] = []
    for (const item of out) {
      const record = item as unknown as Record<string, unknown>
      const rawId = readFieldValue(record, spec.fkColumn)
      if (rawId === null || rawId === undefined || rawId === '') {
        next.push(writePullThroughValue(item, key, ''))
        continue
      }
      const ref = await referencedRow(fk, String(rawId))
      if (!ref) {
        next.push(writePullThroughValue(item, key, ''))
        continue
      }
      const raw = readFieldValue(ref, spec.field)
      const fieldForDisplay = displayField ?? refField
      const display = ctx && fieldForDisplay
        ? formatFieldDisplayValue(
          spec.field,
          raw,
          fieldForDisplay,
          ref,
          ctx.foreignKeys,
          ctx.referencedItemsByColumn,
        )
        : raw == null ? '' : String(raw)
      next.push(writePullThroughValue(item, key, display))
    }
    out = next
  }
  return out
}
