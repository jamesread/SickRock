import { describe, expect, it } from 'vitest'
import { create } from '@bufbuild/protobuf'
import { FieldSchema } from '../gen/sickrock_pb'
import { bulkEntryInputType, buildBulkEntryValues, orderBulkEntryColumns } from './bulkEntry'
import { createScanDebouncer } from '../composables/useBarcodeCamera'

const fieldDefs = [
  create(FieldSchema, { name: 'barcode', type: 'text' }),
  create(FieldSchema, { name: 'title', type: 'text' }),
  create(FieldSchema, { name: 'quantity', type: 'int64' }),
  create(FieldSchema, { name: 'expirydate', type: 'datetime' }),
]
const config = { scanColumn: 'barcode', matchColumns: ['expirydate'], fields: ['title'] }

describe('bulkEntryInputType', () => {
  it('maps column types to inputs', () => {
    expect(bulkEntryInputType('datetime')).toBe('date')
    expect(bulkEntryInputType('int64')).toBe('number')
    expect(bulkEntryInputType('text')).toBe('text')
  })
})

describe('buildBulkEntryValues', () => {
  it('normalises dates and keeps blank match values so they match NULL', () => {
    expect(buildBulkEntryValues(config, ' 5012345 ', { expirydate: '2027-03-01', title: ' Beans ' }, fieldDefs)).toEqual({
      match: { barcode: '5012345', expirydate: '2027-03-01 00:00:00' },
      fields: { title: 'Beans' },
    })
    expect(buildBulkEntryValues(config, '1', {}, fieldDefs)).toEqual({
      match: { barcode: '1', expirydate: '' },
      fields: {},
    })
  })
})

describe('createScanDebouncer', () => {
  it('ignores the same code while it stays in view', () => {
    const shouldCount = createScanDebouncer(1500)
    expect(shouldCount('A', 0)).toBe(true)
    expect(shouldCount('A', 1000)).toBe(false)
    expect(shouldCount('A', 2000)).toBe(false)
    expect(shouldCount('A', 4000)).toBe(true)
    expect(shouldCount('B', 4100)).toBe(true)
    expect(shouldCount('A', 4200)).toBe(true)
  })
})

describe('orderBulkEntryColumns', () => {
  const full = { ...config, quantityColumn: 'quantity' }
  const col = (columnName: string, columnOrder: number, isVisible = true) => ({ columnName, columnOrder, isVisible })

  it('keeps config order with no view', () => {
    expect(orderBulkEntryColumns(full, fieldDefs, [], -1)).toEqual(['barcode', 'quantity', 'expirydate', 'title'])
  })

  it('follows the view order, hides optional fields but never required ones', () => {
    const views = [{ id: 3, columns: [col('title', 1, false), col('expirydate', 2), col('barcode', 3, false), col('quantity', 4)] }]
    expect(orderBulkEntryColumns(full, fieldDefs, views, 3)).toEqual(['expirydate', 'barcode', 'quantity'])
  })

  it('puts columns the view does not list after the listed ones', () => {
    const views = [{ id: 4, columns: [col('title', 1), col('barcode', 2)] }]
    expect(orderBulkEntryColumns(full, fieldDefs, views, 4)).toEqual(['title', 'barcode', 'quantity', 'expirydate'])
  })
})
