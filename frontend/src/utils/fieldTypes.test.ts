import { describe, expect, it } from 'vitest'

import type { Field } from '../gen/sickrock_pb'
import {
  hasStoredUserRefValue,
  isLikelyIamUsersTable,
  isUserRefColumn,
  isUserRefFieldType,
} from './fieldTypes'

describe('fieldTypes', () => {
  it('detects user_ref semantic type case-insensitively', () => {
    expect(isUserRefFieldType('user_ref')).toBe(true)
    expect(isUserRefFieldType('USER_REF')).toBe(true)
  })

  it('detects user ref columns by semantic or users FK', () => {
    const fields = [{ name: 'account_owner', type: 'user_ref' }] as Field[]
    expect(isUserRefColumn('account_owner', fields, [])).toBe(true)

    const bigintFields = [{ name: 'account_owner', type: 'bigint' }] as Field[]
    const fks = [{ columnName: 'account_owner', referencedTable: 'users' }]
    expect(isUserRefColumn('account_owner', bigintFields, fks)).toBe(true)
  })

  it('treats 0 as empty user ref', () => {
    expect(hasStoredUserRefValue(0)).toBe(false)
    expect(hasStoredUserRefValue('42')).toBe(true)
  })

  it('recognizes likely IAM user tables', () => {
    expect(isLikelyIamUsersTable('users')).toBe(true)
    expect(isLikelyIamUsersTable('customers')).toBe(false)
  })
})
