import { describe, expect, it } from 'vitest'
import {
  relatedTableEmptyHeading,
  rowEditActionLabel,
  rowInsertActionLabel,
  tableEmptyHeading,
  tableListLinkLabel,
  tableViewSectionTitle,
  ucFirst,
} from './rowInsertLabel'

describe('ucFirst', () => {
  it('capitalizes the first letter', () => {
    expect(ucFirst('product')).toBe('Product')
  })
})

describe('rowInsertActionLabel', () => {
  it('capitalizes row_name in the label', () => {
    expect(rowInsertActionLabel('product')).toBe('Create Product')
  })
})

describe('rowEditActionLabel', () => {
  it('capitalizes row_name in the label', () => {
    expect(rowEditActionLabel('product')).toBe('Edit Product')
  })
})

describe('tableViewSectionTitle', () => {
  it('prefers capitalized row_name_plural', () => {
    expect(tableViewSectionTitle('opportunities', 'opportunity', 'CRM', 'opportunities')).toBe(
      'Opportunities',
    )
  })

  it('capitalizes table title when row names are unset', () => {
    expect(tableViewSectionTitle(null, null, 'products', 'products')).toBe('Products')
  })
})

describe('tableListLinkLabel', () => {
  it('uses plural row name', () => {
    expect(tableListLinkLabel('Opportunities', 'Opportunity')).toBe('Opportunities table')
  })

  it('capitalizes lowercase plural fallback', () => {
    expect(tableListLinkLabel(null, 'product')).toBe('Products table')
  })
})

describe('tableEmptyHeading', () => {
  it('uses row_name_plural when set', () => {
    expect(tableEmptyHeading('Opportunities', 'Opportunity')).toBe('No Opportunities in this table')
  })

  it('falls back from row_name with capitalization', () => {
    expect(tableEmptyHeading(null, 'customer')).toBe('No Customers in this table')
  })
})

describe('relatedTableEmptyHeading', () => {
  it('includes parent label', () => {
    expect(relatedTableEmptyHeading('Opportunities', 'Opportunity', 'Acme Corp')).toBe(
      'No Opportunities for Acme Corp',
    )
  })
})
