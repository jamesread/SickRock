import { describe, expect, it } from 'vitest'
import type { Item } from '../gen/sickrock_pb'
import { exportCellValue, type ReferencedRowsByColumn } from './exportValues'

const row = (id: string, fields: Record<string, string>, srCreated = 0n) => ({ id, srCreated, additionalFields: fields }) as unknown as Item

const ingredients = new Map([
  ['1', row('1', { name: 'Onion' })],
  ['2', row('2', { aisle: 'Tins' })],
])
const referenced: ReferencedRowsByColumn = new Map([['ingredient', ingredients]])

describe('exportCellValue', () => {
  it('writes relation columns as the referenced row name', () => {
    expect(exportCellValue(row('9', { ingredient: '1' }), 'ingredient', referenced)).toBe('Onion')
  })

  it('falls back to the id when the referenced row has no name or is missing', () => {
    expect(exportCellValue(row('9', { ingredient: '2' }), 'ingredient', referenced)).toBe('2')
    expect(exportCellValue(row('9', { ingredient: '77' }), 'ingredient', referenced)).toBe('77')
    expect(exportCellValue(row('9', { ingredient: '' }), 'ingredient', referenced)).toBe('')
  })

  it('writes other columns as stored, the id, and the created time', () => {
    expect(exportCellValue(row('9', { quantity: '2' }), 'quantity', referenced)).toBe('2')
    expect(exportCellValue(row('9', {}), 'quantity')).toBe('')
    expect(exportCellValue(row('9', {}), 'id')).toBe('9')
    expect(exportCellValue(row('9', {}, 1795435200n), 'sr_created')).toBe('2026-11-23T12:00:00.000Z')
    expect(exportCellValue(row('9', {}), 'sr_created')).toBe('')
  })
})
