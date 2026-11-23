import { describe, expect, it } from 'vitest'

import type { Item } from '../gen/sickrock_pb'
import {
  MAX_LIST_DETAIL_COLUMNS,
  MAX_RECENT_PICKS,
  pickerColumns,
  pickerImageId,
  pickerLabel,
  pickerOptions,
  recentOptions,
  recentPicks,
  recentPicksKey,
  rememberPick,
} from './rowPicker'

function row(id: string, fields: Record<string, string>): Item {
  return { id, srCreated: 0n, srUpdated: 0n, additionalFields: fields } as unknown as Item
}

/** A Storage stand-in backed by a Map. */
function memoryStorage() {
  const values = new Map<string, string>()
  return {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => { values.set(key, value) },
  }
}

const ingredientFields = [
  { name: 'id', type: 'bigint' },
  { name: 'name', type: 'varchar' },
  { name: 'aisle', type: 'varchar' },
  { name: 'calories', type: 'int' },
  { name: 'supplier', type: 'varchar' },
  { name: 'notes', type: 'text' },
  { name: 'sr_created', type: 'datetime' },
]

describe('pickerLabel', () => {
  it('uses the name, then the title, then the id', () => {
    expect(pickerLabel(row('1', { name: ' Beef mince ', title: 'Mince' }))).toBe('Beef mince')
    expect(pickerLabel(row('2', { title: ' Weekly shop ' }))).toBe('Weekly shop')
    expect(pickerLabel(row('3', { aisle: 'Meat' }))).toBe('#3')
  })
})

describe('pickerImageId', () => {
  it('reads the image column, ignoring empty and zero values', () => {
    expect(pickerImageId(row('1', { picture: '12' }), 'picture')).toBe('12')
    expect(pickerImageId(row('1', { picture: '0' }), 'picture')).toBe('')
    expect(pickerImageId(row('1', { picture: '12' }), '')).toBe('')
    expect(pickerImageId(undefined, 'picture')).toBe('')
  })
})

describe('pickerColumns', () => {
  it('shows tiles with the Cards view image and details', () => {
    const views = [{
      id: 4,
      viewType: 'cards',
      isDefault: false,
      layoutJson: '{"cards":{"imageColumn":"picture","detailColumns":["aisle","missing","sr_created"]}}',
    }]
    const fields = [...ingredientFields, { name: 'picture', type: 'image' }]
    expect(pickerColumns(fields, views)).toEqual({
      layout: 'tiles',
      imageColumn: 'picture',
      detailColumns: ['aisle', 'sr_created'],
    })
  })

  it('falls back to the first image column when the Cards view names none', () => {
    const views = [{ id: 4, viewType: 'cards', isDefault: false, layoutJson: '{"cards":{"detailColumns":["aisle"]}}' }]
    const fields = [...ingredientFields, { name: 'picture', type: 'image' }]
    expect(pickerColumns(fields, views).imageColumn).toBe('picture')
  })

  it('shows tiles without details for a table with an image column but no Cards view', () => {
    const fields = [...ingredientFields, { name: 'picture', type: 'image' }]
    expect(pickerColumns(fields, [])).toEqual({ layout: 'tiles', imageColumn: 'picture', detailColumns: [] })
  })

  it('lists the visible columns of the default table view, in order, without the label columns', () => {
    const views = [
      { id: 1, viewType: 'table', isDefault: false, columns: [{ columnName: 'notes', isVisible: true, columnOrder: 0 }] },
      {
        id: 2,
        viewType: 'table',
        isDefault: true,
        columns: [
          { columnName: 'calories', isVisible: true, columnOrder: 2 },
          { columnName: 'name', isVisible: true, columnOrder: 0 },
          { columnName: 'aisle', isVisible: true, columnOrder: 1 },
          { columnName: 'supplier', isVisible: false, columnOrder: 3 },
        ],
      },
    ]
    expect(pickerColumns(ingredientFields, views)).toEqual({ layout: 'list', imageColumn: '', detailColumns: ['aisle', 'calories'] })
  })

  it("prefers the caller's own default view", () => {
    const views = [
      { id: 1, viewType: 'table', isDefault: false, columns: [{ columnName: 'notes', isVisible: true, columnOrder: 0 }] },
      { id: 2, viewType: 'table', isDefault: true, columns: [{ columnName: 'aisle', isVisible: true, columnOrder: 0 }] },
    ]
    expect(pickerColumns(ingredientFields, views, 1).detailColumns).toEqual(['notes'])
  })

  it('falls back to the first plain columns when no table view says which', () => {
    expect(pickerColumns(ingredientFields, []).detailColumns)
      .toEqual(['aisle', 'calories', 'supplier'].slice(0, MAX_LIST_DETAIL_COLUMNS))
  })
})

describe('pickerOptions', () => {
  it('builds one option per row with its details, sorted by label', () => {
    const rows = [
      row('2', { name: 'Milk', aisle: 'Dairy' }),
      row('10', { name: 'Item 10', aisle: '' }),
      row('9', { name: 'Item 9', aisle: 'Tins' }),
    ]
    const options = pickerOptions(rows, { layout: 'list', imageColumn: '', detailColumns: ['aisle'] }, {}, { aisle: 'Aisle' })
    expect(options.map(option => option.label)).toEqual(['Item 9', 'Item 10', 'Milk'])
    expect(options[2]).toEqual({ value: '2', label: 'Milk', imageId: '', details: [{ label: 'Aisle', value: 'Dairy' }] })
    // An empty detail is left out rather than shown blank.
    expect(options[1].details).toEqual([])
  })
})

describe('recentOptions', () => {
  it('returns the options for the recent values, in recent order', () => {
    const options = [{ value: '1' }, { value: '2' }, { value: '3' }]
    expect(recentOptions(options, ['3', 'gone', '1'])).toEqual([{ value: '3' }, { value: '1' }])
  })
})

describe('recent picks', () => {
  it('keys picks by user and source', () => {
    expect(recentPicksKey('alice', 'ingredients')).toBe('sickrock.recentPicks.alice.ingredients')
    expect(recentPicksKey('', 'users')).toBe('sickrock.recentPicks.anonymous.users')
  })

  it('puts the newest first, once, and keeps at most the limit', () => {
    const storage = memoryStorage()
    rememberPick('k', '1', storage)
    rememberPick('k', '2', storage)
    expect(rememberPick('k', '1', storage)).toEqual(['1', '2'])
    for (let i = 0; i < MAX_RECENT_PICKS + 3; i++) rememberPick('k', `x${i}`, storage)
    const picks = recentPicks('k', storage)
    expect(picks).toHaveLength(MAX_RECENT_PICKS)
    expect(picks[0]).toBe(`x${MAX_RECENT_PICKS + 2}`)
  })

  it('ignores an empty value', () => {
    const storage = memoryStorage()
    expect(rememberPick('k', '', storage)).toEqual([])
  })

  it('treats missing, corrupt or unavailable storage as no picks', () => {
    expect(recentPicks('k', undefined)).toEqual([])
    expect(recentPicks('k', { getItem: () => 'not json' })).toEqual([])
    expect(recentPicks('k', { getItem: () => '{"a":1}' })).toEqual([])
    expect(recentPicks('k', { getItem: () => { throw new Error('blocked') } })).toEqual([])
    const full = { getItem: () => null, setItem: () => { throw new Error('quota') } }
    expect(rememberPick('k', '1', full)).toEqual(['1'])
  })
})
