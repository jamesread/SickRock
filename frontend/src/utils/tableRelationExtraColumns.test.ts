import { describe, expect, it } from 'vitest'
import type { ForeignKey } from '../gen/sickrock_pb'
import { relationColumnsForSourceTable } from './tableRelationExtraColumns'

describe('relationColumnsForSourceTable', () => {
  it('merges pull-through config from outgoing foreign keys only', () => {
    const foreignKeys = [
      {
        tableName: 'customers',
        tableTcName: 'customers',
        columnName: 'country_id',
        pullThroughFields: [{ fkColumn: 'country_id', field: 'name', displayLabel: '', expr: '' }],
      },
      {
        tableName: 'orders',
        tableTcName: 'orders',
        columnName: 'customer_id',
        referencedTable: 'customers',
        pullThroughFields: [{ fkColumn: 'customer_id', field: 'email', displayLabel: '', expr: '' }],
      },
    ] as ForeignKey[]
    const cols = relationColumnsForSourceTable('customers', foreignKeys)
    expect(cols).toHaveLength(1)
    expect(cols[0].field).toBe('name')
  })
})
