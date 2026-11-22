import { describe, expect, it } from 'vitest'

import type { Field } from '../gen/sickrock_pb'
import {
  booleanFromStored,
  booleanToStored,
  isBooleanFieldType,
  isTrueBooleanValue,
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

describe('boolean fields', () => {
  it.each(['tinyint', 'TINYINT(1)', 'tinyint(4)', 'BOOLEAN', 'bool', ' Boolean '])('%s is a boolean column', (type) => {
    expect(isBooleanFieldType(type)).toBe(true)
  })

  it.each(['int', 'INTEGER', 'bigint', 'varchar', 'text', '', undefined])('%s is not a boolean column', (type) => {
    expect(isBooleanFieldType(type)).toBe(false)
  })

  it.each([['1', true], [1, true], [true, true], ['true', true], ['TRUE', true], ['yes', true], ['2', true],
    ['0', false], [0, false], [false, false], ['', false], [null, false], [undefined, false], ['false', false], ['no', false], ['null', false]])(
    '%j is %s', (raw, want) => {
      expect(isTrueBooleanValue(raw)).toBe(want)
    })
})

describe('boolean columns', () => {
  it('recognise boolean types', () => {
    expect(isBooleanFieldType('tinyint')).toBe(true)
    expect(isBooleanFieldType('TINYINT(1)')).toBe(true)
    expect(isBooleanFieldType('boolean')).toBe(true)
    expect(isBooleanFieldType('int64')).toBe(false)
    expect(isBooleanFieldType(undefined)).toBe(false)
  })

  it('read stored values, keeping unset distinct', () => {
    expect(booleanFromStored('1')).toBe(true)
    expect(booleanFromStored(1)).toBe(true)
    expect(booleanFromStored('0')).toBe(false)
    expect(booleanFromStored('true')).toBe(true)
    expect(booleanFromStored('false')).toBe(false)
    expect(booleanFromStored('')).toBeNull()
    expect(booleanFromStored(null)).toBeNull()
    expect(booleanFromStored('yes')).toBe(true)
    expect(booleanFromStored('no')).toBe(false)
  })

  it('write 1 or 0', () => {
    expect(booleanToStored(true)).toBe('1')
    expect(booleanToStored(false)).toBe('0')
  })
})
