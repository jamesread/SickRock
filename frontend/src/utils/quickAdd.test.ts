import { describe, expect, it } from 'vitest'
import type { Item } from '../gen/sickrock_pb'
import {
  matchReferencedRow,
  nameColumnOf,
  parseQuickAddConfig,
  quickAddColumns,
  quickAddInputType,
  quickAddValue,
  hasQuickAddValue,
  referenceText,
  serializeQuickAddConfig,
} from './quickAdd'

const fields = [
  { name: 'id', type: 'int' },
  { name: 'sr_created', type: 'datetime' },
  { name: 'recipe', type: 'bigint' },
  { name: 'ingredient', type: 'bigint' },
  { name: 'quantity', type: 'text' },
  { name: 'unit', type: 'text' },
  { name: '__pt_ingredient_aisle', type: 'text' },
]

describe('quick add config', () => {
  it('round-trips and ignores junk', () => {
    const config = { enabled: true, columns: ['ingredient', 'quantity'], createMissing: true }
    expect(parseQuickAddConfig(serializeQuickAddConfig(config))).toEqual(config)
    expect(parseQuickAddConfig('nope').enabled).toBe(false)
    expect(parseQuickAddConfig('{"enabled":"yes","columns":[" a ","a",3]}')).toEqual({ enabled: false, columns: ['a'], createMissing: false })
  })
})

describe('quickAddColumns', () => {
  it('offers every writable column except the parent link by default', () => {
    expect(quickAddColumns(parseQuickAddConfig('{"enabled":true}'), fields, 'recipe').map(f => f.name))
      .toEqual(['ingredient', 'quantity', 'unit'])
  })

  it('keeps the configured order and drops unknown columns', () => {
    const config = parseQuickAddConfig('{"enabled":true,"columns":["quantity","gone","ingredient","recipe"]}')
    expect(quickAddColumns(config, fields, 'recipe').map(f => f.name)).toEqual(['quantity', 'ingredient'])
  })
})

describe('inputs and values', () => {
  it('picks an input type', () => {
    expect(quickAddInputType('BIGINT')).toBe('number')
    expect(quickAddInputType('decimal(10,2)')).toBe('number')
    expect(quickAddInputType('datetime')).toBe('datetime-local')
    expect(quickAddInputType('tinyint(1)')).toBe('checkbox')
    expect(quickAddInputType('text')).toBe('text')
  })

  it('formats values for the server', () => {
    expect(quickAddValue('datetime', '2026-11-22T18:00')).toBe('2026-11-22 18:00:00')
    expect(quickAddValue('text', ' 2 ')).toBe('2')
    expect(quickAddValue('tinyint(1)', true)).toBe('1')
  })

  it('accepts the numbers number inputs give', () => {
    expect(quickAddValue('decimal(10,2)', 200)).toBe('200')
    expect(quickAddValue('decimal(10,2)', 0.5)).toBe('0.5')
    expect(quickAddValue('decimal(10,2)', '')).toBe('')
  })

  it('knows which inputs hold something to add', () => {
    expect(hasQuickAddValue(' x ')).toBe(true)
    expect(hasQuickAddValue('  ')).toBe(false)
    expect(hasQuickAddValue(0)).toBe(true)
    expect(hasQuickAddValue(Number.NaN)).toBe(false)
    expect(hasQuickAddValue(true)).toBe(true)
    expect(hasQuickAddValue(false)).toBe(false)
    expect(hasQuickAddValue(undefined)).toBe(false)
  })
})

describe('referenced rows', () => {
  const rows = [
    { id: '1', additionalFields: { name: 'Onion' } },
    { id: '7', additionalFields: { name: 'Garlic' } },
  ] as unknown as Item[]

  it('matches by name ignoring case, else by id', () => {
    expect(matchReferencedRow(rows, ' onion ')?.id).toBe('1')
    expect(matchReferencedRow(rows, '#7')?.id).toBe('7')
    expect(matchReferencedRow(rows, 'Leek')).toBeUndefined()
    expect(matchReferencedRow(rows, '')).toBeUndefined()
  })

  it('types a picked row as its name, or #id when the name would find another row', () => {
    const twins = [
      { id: '1', additionalFields: { name: 'Onion' } },
      { id: '2', additionalFields: { name: 'onion' } },
      { id: '3', additionalFields: {} },
    ] as unknown as Item[]
    expect(referenceText(twins, twins[0])).toBe('Onion')
    expect(referenceText(twins, twins[1])).toBe('#2')
    expect(referenceText(twins, twins[2])).toBe('#3')
    for (const row of twins) expect(matchReferencedRow(twins, referenceText(twins, row))?.id).toBe(row.id)
  })

  it('finds where a new row name goes', () => {
    expect(nameColumnOf([{ name: 'title', type: 'text' }, { name: 'name', type: 'text' }])).toBe('name')
    expect(nameColumnOf([{ name: 'title', type: 'text' }])).toBe('title')
    expect(nameColumnOf([{ name: 'code', type: 'text' }])).toBe('')
  })
})
