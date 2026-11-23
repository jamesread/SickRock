import { describe, expect, it } from 'vitest'
import type { Item } from '../gen/sickrock_pb'
import {
  compareValues,
  defaultTickListConfig,
  groupTickListRows,
  parseTickListConfig,
  pathValue,
  relationColumnsUsed,
  tileDetails,
  tickListProgress,
  tileImageId,
  tileLabel,
  withTickListConfig,
  type ReferencedRows,
  type TickListConfig,
} from './tickList'

const item = (id: string, fields: Record<string, string>) => ({ id, additionalFields: fields }) as unknown as Item

const ingredients = new Map([
  ['1', item('1', { name: 'Onion', aisle: 'Aisle 2', picture: '41' })],
  ['2', item('2', { name: 'Beans', aisle: 'Aisle 10', picture: '0' })],
  ['3', item('3', { name: 'Apples', aisle: 'Aisle 2' })],
])
const refs: ReferencedRows = new Map([['ingredient', ingredients]])
const rows = [
  item('10', { ingredient: '2', quantity: '2', unit: 'tin' }),
  item('11', { ingredient: '1', quantity: '550', unit: 'g' }),
  item('12', { ingredient: '3', quantity: '6', unit: '' }),
  item('13', { ingredient: '', quantity: '1', unit: '', name: 'Candles' }),
]
const shopping: TickListConfig = {
  labelColumn: 'ingredient',
  detailColumns: ['quantity', 'unit'],
  sortColumns: ['ingredient'],
  groupColumn: 'ingredient.aisle',
  imageColumn: 'ingredient.picture',
  showProgress: true,
}

describe('tick list config', () => {
  it('round-trips through layout_json, keeping other keys', () => {
    const json = withTickListConfig('{"filters":{"a":1}}', shopping)
    expect(JSON.parse(json).filters).toEqual({ a: 1 })
    expect(parseTickListConfig(json)).toEqual(shopping)
  })

  it('defaults when missing or broken', () => {
    expect(parseTickListConfig('')).toEqual(defaultTickListConfig())
    expect(parseTickListConfig('{"tickList":[]}')).toEqual(defaultTickListConfig())
    expect(parseTickListConfig('not json')).toEqual(defaultTickListConfig())
  })

  it('drops blank and repeated paths and caps lists', () => {
    const config = parseTickListConfig('{"tickList":{"detailColumns":["a","a"," ","b","c","d","e"],"sortColumns":["x","y","z","w"]}}')
    expect(config.detailColumns).toEqual(['a', 'b', 'c', 'd'])
    expect(config.sortColumns).toEqual(['x', 'y', 'z'])
  })

  it('names the relation columns it needs', () => {
    expect(relationColumnsUsed(shopping, ['ingredient', 'shop'])).toEqual(['ingredient'])
    const imageOnly = { ...defaultTickListConfig(), imageColumn: 'shop.logo' }
    expect(relationColumnsUsed(imageOnly, ['ingredient', 'shop'])).toEqual(['shop'])
  })

  it('reads a missing image column as none', () => {
    expect(parseTickListConfig('{"tickList":{"labelColumn":"name"}}').imageColumn).toBe('')
  })
})

describe('path values', () => {
  it('shows a relation by the referenced row name, or one of its columns', () => {
    expect(pathValue(rows[1], 'ingredient', refs)).toBe('Onion')
    expect(pathValue(rows[1], 'ingredient.aisle', refs)).toBe('Aisle 2')
    expect(pathValue(rows[1], 'unit', refs)).toBe('g')
  })

  it('falls back to the raw value, or empty, when the row is not found', () => {
    expect(pathValue(item('1', { ingredient: '99' }), 'ingredient', refs)).toBe('99')
    expect(pathValue(item('1', { ingredient: '99' }), 'ingredient.aisle', refs)).toBe('')
  })

  it('labels tiles and joins details', () => {
    expect(tileLabel(rows[1], shopping, refs)).toBe('Onion')
    expect(tileDetails(rows[1], shopping, refs)).toBe('550 g')
    expect(tileDetails(rows[2], shopping, refs)).toBe('6')
    expect(tileLabel(rows[3], shopping, refs)).toBe('Candles')
  })
})

describe('tile images', () => {
  it("shows the image column's image, from the row or a referenced row", () => {
    expect(tileImageId(rows[1], shopping, refs)).toBe('41')
    const own = { ...defaultTickListConfig(), imageColumn: 'photo' }
    expect(tileImageId(item('20', { photo: '7' }), own, refs)).toBe('7')
  })

  it('shows none without a setting, a value or a referenced row', () => {
    expect(tileImageId(rows[1], defaultTickListConfig(), refs)).toBe('')
    expect(tileImageId(rows[0], shopping, refs), 'picture 0 is no image').toBe('')
    expect(tileImageId(rows[2], shopping, refs), 'Apples has no picture').toBe('')
    expect(tileImageId(rows[3], shopping, refs), 'no ingredient').toBe('')
  })
})

describe('progress', () => {
  it('counts the ticked rows', () => {
    const ticked = new Set(['1', '3', '4'])
    expect(tickListProgress(['1', '2', '3', '4'], id => ticked.has(id))).toEqual({ ticked: 3, total: 4, percent: 75, label: '3 of 4 ticked' })
    expect(tickListProgress([], () => true)).toEqual({ ticked: 0, total: 0, percent: 0, label: '0 of 0 ticked' })
  })

  it('is off unless turned on', () => {
    expect(parseTickListConfig('{"tickList":{}}').showProgress).toBe(false)
    expect(parseTickListConfig('{"tickList":{"showProgress":"yes"}}').showProgress).toBe(false)
  })
})

describe('sorting and grouping', () => {
  it('compares naturally, numbers by value, empty last', () => {
    expect(['Aisle 10', '', 'Aisle 2'].sort(compareValues)).toEqual(['Aisle 2', 'Aisle 10', ''])
    expect(['10', '9'].sort(compareValues)).toEqual(['9', '10'])
  })

  it('groups by aisle in order, sorted within, with the rest last', () => {
    const groups = groupTickListRows(rows, shopping, refs)
    expect(groups.map(g => [g.label, g.rows.map(r => tileLabel(r, shopping, refs))])).toEqual([
      ['Aisle 2', ['Apples', 'Onion']],
      ['Aisle 10', ['Beans']],
      ['Other', ['Candles']],
    ])
  })

  it('keeps the original order without settings', () => {
    expect(groupTickListRows(rows, defaultTickListConfig(), refs)).toEqual([{ label: '', rows }])
  })
})
