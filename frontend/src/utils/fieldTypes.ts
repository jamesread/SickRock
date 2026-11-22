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

/** A boolean column: MySQL tinyint (TINYINT(1)), or a SQLite column declared BOOLEAN/BOOL/TINYINT. */
export function isBooleanFieldType(fieldType: string | undefined): boolean {
  const type = (fieldType ?? '').trim().toLowerCase()
  return type.startsWith('tinyint') || type === 'bool' || type === 'boolean'
}

/** Whether a stored boolean column value is true ("1", "true", "yes"; NULL, "" and "0" are false). */
export function isTrueBooleanValue(raw: unknown): boolean {
  if (typeof raw === 'boolean') return raw
  if (typeof raw === 'number') return raw !== 0
  const value = String(raw ?? '').trim().toLowerCase()
  return value !== '' && value !== '0' && value !== 'false' && value !== 'no' && value !== 'null'
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
