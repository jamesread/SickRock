import type { Field, ForeignKey, Item } from '../gen/sickrock_pb'
import { getItemNameFieldValue } from './itemViewSectionTitle'

const FIELD_PLACEHOLDER = /\{([a-zA-Z_][a-zA-Z0-9_]*)\}/g

/** Optional resolver for placeholder columns (e.g. FK id → referenced row label). */
export type ColumnPlaceholderResolver = (
  columnName: string,
  rawValue: unknown,
) => string

/** Flatten top-level item fields and additionalFields into one lookup map. */
export function mergeItemFieldRecord(
  source: Record<string, unknown> | null | undefined,
): Record<string, unknown> {
  if (!source) return {}
  const out: Record<string, unknown> = {}
  for (const [key, value] of Object.entries(source)) {
    if (key === 'additionalFields' || key.startsWith('$')) continue
    out[key] = value
  }
  const additional = source.additionalFields
  if (additional && typeof additional === 'object' && !Array.isArray(additional)) {
    for (const [key, value] of Object.entries(additional as Record<string, unknown>)) {
      out[key] = value
    }
  }
  return out
}

function lookupColumnValue(
  merged: Record<string, unknown>,
  columnName: string,
): unknown {
  if (!columnName) return undefined
  if (Object.prototype.hasOwnProperty.call(merged, columnName)) {
    return merged[columnName]
  }
  const lower = columnName.toLowerCase()
  for (const [key, value] of Object.entries(merged)) {
    if (key.toLowerCase() === lower) {
      return value
    }
  }
  return undefined
}

/** Resolve a column value from a flat row or item record for template interpolation. */
export function resolveFieldPlaceholderValue(
  record: Record<string, unknown> | null | undefined,
  columnName: string,
): unknown {
  if (!record || !columnName) return undefined
  const merged = mergeItemFieldRecord(record)
  return lookupColumnValue(merged, columnName)
}

export function interpolateFieldTemplate(
  template: string,
  record: Record<string, unknown> | null | undefined,
  resolveColumn?: ColumnPlaceholderResolver,
): string {
  if (!template) return ''
  return template.replace(FIELD_PLACEHOLDER, (_match, columnName: string) => {
    const raw = resolveFieldPlaceholderValue(record, columnName)
    if (resolveColumn) {
      return resolveColumn(columnName, raw)
    }
    if (raw === null || raw === undefined) return ''
    return String(raw)
  })
}

/** Display label for a FK target row (uses `name` when present). */
export function foreignKeyReferencedDisplayLabel(
  item: Record<string, unknown> | null | undefined,
  fallbackId: unknown,
): string {
  const name = getItemNameFieldValue(item)
  if (name) return name
  if (fallbackId != null && String(fallbackId) !== '') return String(fallbackId)
  return ''
}

/** Preserve spacing in affixes: regular spaces render as non-breaking spaces (HTML &nbsp;). */
export function affixSpacesToNbsp(text: string): string {
  return text.replace(/ /g, '\u00A0')
}

function hasAffixText(value: string | undefined | null): boolean {
  return value != null && value.length > 0
}

export function fieldDisplayLabel(field: Field | undefined, columnName: string): string {
  const custom = field?.displayLabel?.trim()
  if (custom) return custom
  if (columnName === 'id') return 'ID'
  if (columnName === 'srCreated' || columnName === 'sr_created') return 'Created'
  if (columnName === 'srUpdated' || columnName === 'sr_updated') return 'Updated'
  return columnName
}

export function applyFieldValueAffixes(
  formattedCore: string,
  field: Field | undefined,
  record: Record<string, unknown> | null | undefined,
  resolveColumn?: ColumnPlaceholderResolver,
): string {
  if (!fieldHasValueAffixes(field)) {
    return formattedCore
  }
  const merged = mergeItemFieldRecord(record)
  const prefix = affixSpacesToNbsp(
    interpolateFieldTemplate(field!.valuePrefix ?? '', merged, resolveColumn),
  )
  const suffix = affixSpacesToNbsp(
    interpolateFieldTemplate(field!.valueSuffix ?? '', merged, resolveColumn),
  )
  return `${prefix}${formattedCore}${suffix}`
}

export function fieldHasValueAffixes(field: Field | undefined): boolean {
  return hasAffixText(field?.valuePrefix) || hasAffixText(field?.valueSuffix)
}

export type ColumnDisplaySettingLike = {
  columnName?: string
  displayLabel?: string
  valuePrefix?: string
  valueSuffix?: string
}

export function columnDisplaySettingsByName(
  settings: ColumnDisplaySettingLike[] | undefined,
): Map<string, ColumnDisplaySettingLike> {
  const out = new Map<string, ColumnDisplaySettingLike>()
  for (const setting of settings ?? []) {
    const name = setting.columnName?.trim()
    if (name) {
      out.set(name, setting)
    }
  }
  return out
}

/** Apply table_column_display overrides onto a field (physical or synthetic column). */
export function fieldWithColumnDisplay(
  field: Field,
  settingsByName: Map<string, ColumnDisplaySettingLike>,
  options?: { defaultLabel?: string },
): Field {
  const setting = settingsByName.get(field.name)
  if (!setting) {
    if (options?.defaultLabel) {
      return { ...field, displayLabel: options.defaultLabel }
    }
    return field
  }
  const labelFromSetting = setting.displayLabel?.trim()
  return {
    ...field,
    displayLabel: labelFromSetting
      ? labelFromSetting
      : (options?.defaultLabel ?? field.displayLabel ?? ''),
    valuePrefix: setting.valuePrefix ?? field.valuePrefix ?? '',
    valueSuffix: setting.valueSuffix ?? field.valueSuffix ?? '',
  }
}

export function isTinyintFieldType(type: string | undefined): boolean {
  const normalized = (type ?? '').toLowerCase()
  return normalized.startsWith('tinyint') || normalized === 'bool' || normalized === 'boolean'
}

export function isDatetimeFieldType(type: string | undefined): boolean {
  const normalized = (type ?? '').toLowerCase()
  return normalized === 'datetime' || normalized.startsWith('datetime(')
}

export function createForeignKeyPlaceholderResolver(
  foreignKeys: ForeignKey[],
  itemsByFkColumn: Record<string, Item[]>,
): ColumnPlaceholderResolver {
  return (columnName: string, rawValue: unknown) => {
    if (rawValue === null || rawValue === undefined || rawValue === '') return ''
    const fk = foreignKeys.find(entry => entry.columnName === columnName)
    if (fk) {
      const ref = (itemsByFkColumn[columnName] ?? []).find(
        item => String(item.id) === String(rawValue),
      )
      return foreignKeyReferencedDisplayLabel(
        ref as unknown as Record<string, unknown> | undefined,
        rawValue,
      )
    }
    return String(rawValue)
  }
}

/** Core cell text before value affixes (matches table view formatting). */
export function formatFieldCoreDisplayValue(
  columnName: string,
  rawValue: unknown,
  field: Field | undefined,
  foreignKeys: ForeignKey[],
  itemsByFkColumn: Record<string, Item[]>,
): string {
  if (rawValue === null || rawValue === undefined || rawValue === '') return ''
  const type = field?.type
  if (isTinyintFieldType(type)) {
    const truthy = rawValue === true || Number(rawValue) === 1
    return truthy ? '✓' : '✗'
  }
  if (isDatetimeFieldType(type)) {
    const date = new Date(rawValue as string | number)
    return Number.isNaN(date.getTime()) ? String(rawValue) : date.toLocaleString()
  }
  if (foreignKeys.some(entry => entry.columnName === columnName)) {
    return createForeignKeyPlaceholderResolver(foreignKeys, itemsByFkColumn)(
      columnName,
      rawValue,
    )
  }
  return String(rawValue)
}

export function formatFieldDisplayValue(
  columnName: string,
  rawValue: unknown,
  field: Field | undefined,
  record: Record<string, unknown> | null | undefined,
  foreignKeys: ForeignKey[],
  itemsByFkColumn: Record<string, Item[]>,
): string {
  const resolver = createForeignKeyPlaceholderResolver(foreignKeys, itemsByFkColumn)
  const core = formatFieldCoreDisplayValue(
    columnName,
    rawValue,
    field,
    foreignKeys,
    itemsByFkColumn,
  )
  return applyFieldValueAffixes(core, field, record, resolver)
}
