import type { Field } from '../gen/sickrock_pb'

export const USER_REF_FIELD_TYPE = 'user_ref'

export function isUserRefFieldType(fieldType: string): boolean {
  return fieldType.toLowerCase() === USER_REF_FIELD_TYPE
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
