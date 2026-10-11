import { describe, expect, it } from 'vitest'
import {
  affixSpacesToNbsp,
  applyFieldValueAffixes,
  fieldHasValueAffixes,
  formatFieldDisplayValue,
  interpolateFieldTemplate,
} from './fieldColumnDisplay'
import type { Field } from '../gen/sickrock_pb'

describe('fieldColumnDisplay', () => {
  it('interpolates column placeholders from the row record', () => {
    const row = { currency: 'USD', amount: 10 }
    expect(interpolateFieldTemplate('{currency} ', row)).toBe('USD ')
    expect(interpolateFieldTemplate(' per {currency}', row)).toBe(' per USD')
  })

  it('resolves placeholders from additionalFields with case-insensitive keys', () => {
    const item = {
      id: '1',
      additionalFields: { Currency: 'EUR', value: '1200' },
    }
    expect(interpolateFieldTemplate('{currency}', item)).toBe('EUR')
  })

  it('supports column named value without breaking affixes', () => {
    const field = {
      name: 'value',
      type: 'bigint',
      required: false,
      defaultToCurrentTimestamp: false,
      displayLabel: '',
      valuePrefix: '{currency} ',
      valueSuffix: '',
    } as Field
    const row = { currency: 'USD', value: '111234' }
    expect(applyFieldValueAffixes('111234', field, row)).toBe('USD\u00A0111234')
  })

  it('treats whitespace-only affixes as configured', () => {
    const field = {
      name: 'amount',
      type: 'int',
      required: false,
      defaultToCurrentTimestamp: false,
      displayLabel: '',
      valuePrefix: ' ',
      valueSuffix: '',
    } as Field
    expect(fieldHasValueAffixes(field)).toBe(true)
    expect(applyFieldValueAffixes('9', field, {})).toBe('\u00A09')
  })

  it('converts affix spaces to non-breaking spaces', () => {
    expect(affixSpacesToNbsp('USD ')).toBe('USD\u00A0')
  })

  it('applies prefix and suffix around the formatted core value', () => {
    const field = {
      name: 'amount',
      type: 'decimal',
      required: false,
      defaultToCurrentTimestamp: false,
      displayLabel: '',
      valuePrefix: '{currency} ',
      valueSuffix: '',
    } as Field
    const row = { currency: 'EUR', amount: '99.00' }
    expect(applyFieldValueAffixes('99.00', field, row)).toBe('EUR\u00A099.00')
  })

  it('formatFieldDisplayValue applies affixes using the source row record', () => {
    const field = {
      name: 'amount',
      type: 'decimal',
      required: false,
      defaultToCurrentTimestamp: false,
      displayLabel: '',
      valuePrefix: '{currency} ',
      valueSuffix: '',
    } as Field
    const row = { currency: 'GBP', amount: '50' }
    expect(
      formatFieldDisplayValue('amount', '50', field, row, [], {}),
    ).toBe('GBP\u00A050')
  })
})
