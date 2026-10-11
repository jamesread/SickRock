import { describe, expect, it } from 'vitest'
import { relatedRowsStorageKey, shouldShowRelatedRelationPanel } from './rowViewRelatedRelations'

const opportunitiesToCustomers = {
  tableTcName: 'opportunities',
  tableName: 'opportunities',
  columnName: 'customer',
  referencedTableTcName: 'customers',
  referencedTable: 'customers',
}

describe('shouldShowRelatedRelationPanel', () => {
  it('shows child rows on the referenced (parent) table', () => {
    expect(shouldShowRelatedRelationPanel('customers', opportunitiesToCustomers)).toBe(true)
  })

  it('hides parent-as-related-list on the source (child) table', () => {
    expect(shouldShowRelatedRelationPanel('opportunities', opportunitiesToCustomers)).toBe(false)
  })
})

describe('relatedRowsStorageKey', () => {
  it('keys related data by source table and column', () => {
    expect(relatedRowsStorageKey(opportunitiesToCustomers)).toBe('opportunities.customer')
  })
})
