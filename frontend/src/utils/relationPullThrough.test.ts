import { describe, expect, it } from 'vitest'
import type { Field } from '../gen/sickrock_pb'
import {
  buildPullThroughFieldDefs,
  isPullThroughColumn,
  isRelationSyntheticColumn,
  pullThroughStorageKey,
  relationCalcStorageKey,
} from './relationPullThrough'

describe('pullThroughStorageKey', () => {
  it('builds a stable synthetic column key', () => {
    expect(pullThroughStorageKey('currency', 'name')).toBe('__pt_currency_name')
  })
})

describe('isPullThroughColumn', () => {
  it('recognizes synthetic pull-through column keys', () => {
    expect(isPullThroughColumn('__pt_currency_name')).toBe(true)
    expect(isPullThroughColumn('amount')).toBe(false)
  })
})

describe('relationCalcStorageKey', () => {
  it('builds a slug from the display label', () => {
    expect(relationCalcStorageKey('Line total', 0)).toBe('__calc_line_total')
  })
})

describe('isRelationSyntheticColumn', () => {
  it('includes calculated columns', () => {
    expect(isRelationSyntheticColumn('__calc_line_total')).toBe(true)
  })
})

describe('buildPullThroughFieldDefs', () => {
  it('uses referenced column display label and clears affixes on synthetic fields', () => {
    const refField = {
      name: 'amount',
      type: 'decimal',
      required: true,
      defaultToCurrentTimestamp: false,
      displayLabel: 'Deal value',
      valuePrefix: '{currency} ',
      valueSuffix: ' total',
    } as Field
    const defs = buildPullThroughFieldDefs(
      [{ fkColumn: 'currency_id', field: 'amount' }],
      [{ name: 'currency_id', type: 'bigint', required: false, defaultToCurrentTimestamp: false, displayLabel: 'Currency', valuePrefix: '', valueSuffix: '' } as Field],
      {
        currency_id: {
          fields: [refField],
          foreignKeys: [],
          referencedItemsByColumn: {},
        },
      },
    )
    expect(defs[0].displayLabel).toBe('Deal value (Currency)')
    expect(defs[0].name).toBe('__pt_currency_id_amount')
    expect(defs[0].valuePrefix).toBe('{currency} ')
    expect(defs[0].valueSuffix).toBe(' total')
    expect(defs[0].type).toBe('decimal')
  })

  it('applies table_column_display overrides for synthetic keys', () => {
    const refField = {
      name: 'amount',
      type: 'decimal',
      required: true,
      defaultToCurrentTimestamp: false,
      displayLabel: 'Deal value',
      valuePrefix: '',
      valueSuffix: '',
    } as Field
    const defs = buildPullThroughFieldDefs(
      [{ fkColumn: 'sku', field: 'value' }],
      [],
      {
        sku: {
          fields: [refField],
          foreignKeys: [],
          referencedItemsByColumn: {},
        },
      },
      [{
        columnName: '__pt_sku_value',
        displayLabel: 'Unit price',
        valuePrefix: 'USD ',
        valueSuffix: '',
      }],
    )
    expect(defs[0].displayLabel).toBe('Unit price')
    expect(defs[0].valuePrefix).toBe('USD ')
  })
})
