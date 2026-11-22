import type { Field } from '../gen/sickrock_pb'

export const USER_REF_FIELD_TYPE = 'user_ref'

export function isUserRefFieldType(fieldType: string): boolean {
  return fieldType.toLowerCase() === USER_REF_FIELD_TYPE
}

/** Column semantic for a Hugeicons icon name (edited with IconPickerField). */
export const ICON_FIELD_TYPE = 'icon'

export function isIconFieldType(fieldType: string | undefined): boolean {
  return (fieldType ?? '').toLowerCase() === ICON_FIELD_TYPE
}

/** Column semantic for an images.id (edited with ImageField: file upload or camera capture). */
export const IMAGE_FIELD_TYPE = 'image'

export function isImageFieldType(fieldType: string | undefined): boolean {
  return (fieldType ?? '').toLowerCase() === IMAGE_FIELD_TYPE
}

/**
 * Options for a yes/no choice: picocrank's RadioGroup with variant="boolean", used instead of a
 * checkbox for boolean settings and boolean columns.
 */
export const YES_NO_OPTIONS = [
  { value: true, label: 'Yes' },
  { value: false, label: 'No' },
]

/** Whether a column type stores a yes/no value (edited with a boolean RadioGroup). */
export function isBooleanFieldType(fieldType: string | undefined): boolean {
  const type = (fieldType ?? '').toLowerCase()
  return type.startsWith('tinyint') || type === 'bool' || type === 'boolean'
}

/** A stored boolean column value as a choice: true, false, or null when unset. */
export function booleanFromStored(raw: unknown): boolean | null {
  if (raw === null || raw === undefined || raw === '') return null
  if (raw === true || raw === false) return raw
  const text = String(raw).trim().toLowerCase()
  if (text === 'true') return true
  if (text === 'false') return false
  const n = Number(text)
  return Number.isNaN(n) ? null : n !== 0
}

/** The value written for a boolean column. */
export function booleanToStored(value: boolean): '1' | '0' {
  return value ? '1' : '0'
}

/** Column semantic for a barcode on a text column (edited with BarcodeField: typing, scanner or camera). */
export const BARCODE_FIELD_TYPE = 'barcode'

export function isBarcodeFieldType(fieldType: string | undefined): boolean {
  return (fieldType ?? '').toLowerCase() === BARCODE_FIELD_TYPE
}

/** Whether a stored image column value refers to an image (NULL, empty and 0 mean none). */
export function hasImageValue(raw: unknown): boolean {
  if (raw == null) return false
  const value = String(raw).trim()
  return value !== '' && value !== '0' && value !== 'null'
}

export function findFieldDef(fields: Field[], columnName: string): Field | undefined {
  const lower = columnName.toLowerCase()
  return fields.find(field => field.name.toLowerCase() === lower)
}

/** FK target tables that store SickRock IAM user ids (when semantics row is missing). */
export function isLikelyIamUsersTable(tableName: string): boolean {
  const t = tableName.toLowerCase()
  return t === 'users'
    || t === 'sr_users'
    || t === 'iam_users'
    || t === 'user_accounts'
    || t.endsWith('_users')
}

export type UserRefForeignKey = {
  columnName: string
  referencedTable: string
}

export function isUserRefColumn(
  columnName: string,
  fields: Field[],
  foreignKeys: UserRefForeignKey[] = [],
): boolean {
  const def = findFieldDef(fields, columnName)
  if (def && isUserRefFieldType(def.type)) {
    return true
  }
  const lower = columnName.toLowerCase()
  const fk = foreignKeys.find(
    candidate => candidate.columnName.toLowerCase() === lower,
  )
  if (fk && isLikelyIamUsersTable(fk.referencedTable)) {
    return true
  }
  return false
}

export function hasStoredUserRefValue(raw: unknown): boolean {
  if (raw == null || raw === '') return false
  if (raw === 0 || raw === '0') return false
  return true
}
