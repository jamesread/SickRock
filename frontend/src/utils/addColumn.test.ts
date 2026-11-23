import { describe, expect, it } from 'vitest'

import {
  COLUMN_TYPE_OPTIONS,
  STANDARD_FIELDS,
  columnNameTaken,
  columnTypeOption,
  isNewColumnType,
  isStandardFieldName,
  missingStandardFields,
} from './addColumn'

describe('standard fields', () => {
  it('include barcode, set up for bulk entry', () => {
    const barcode = STANDARD_FIELDS.find(f => f.name === 'barcode')
    expect(barcode?.type).toBe('barcode')
    expect(barcode?.bulkEntryScan).toBe(true)
  })

  it('lists only those the table does not have, ignoring case', () => {
    const names = missingStandardFields(['id', 'Name', 'sr_created']).map(f => f.name)
    expect(names).not.toContain('name')
    expect(names).not.toContain('sr_created')
    expect(names).toContain('sr_updated')
    expect(names).toContain('barcode')
    expect(missingStandardFields(STANDARD_FIELDS.map(f => f.name))).toEqual([])
  })

  it('are recognised by name', () => {
    expect(isStandardFieldName('owner_id')).toBe(true)
    expect(isStandardFieldName('price')).toBe(false)
  })

  it('use types a custom column can have', () => {
    for (const f of STANDARD_FIELDS) expect(isNewColumnType(f.type)).toBe(true)
  })
})

describe('column types', () => {
  it('offer the enhanced types', () => {
    const enhanced = COLUMN_TYPE_OPTIONS.filter(o => o.enhanced).map(o => o.value)
    expect(enhanced).toEqual(['user_ref', 'image', 'icon', 'barcode'])
  })

  it('validate query values and fall back to text', () => {
    expect(isNewColumnType('barcode')).toBe(true)
    expect(isNewColumnType('VARCHAR(255)')).toBe(false)
    expect(isNewColumnType(undefined)).toBe(false)
    expect(columnTypeOption('barcode').label).toBe('Barcode')
  })
})

describe('columnNameTaken', () => {
  it('compares trimmed names case-insensitively', () => {
    expect(columnNameTaken(' Barcode ', ['id', 'barcode'])).toBe(true)
    expect(columnNameTaken('ean', ['id', 'barcode'])).toBe(false)
    expect(columnNameTaken('  ', ['id'])).toBe(false)
  })
})
