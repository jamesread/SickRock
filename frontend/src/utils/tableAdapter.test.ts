import { describe, expect, it } from 'vitest'
import type { Field, Item } from '../gen/sickrock_pb'
import {
  buildTableHeaders,
  createTableComparator,
  flattenItem,
  flattenItems,
  getSourceItem,
  getStableRowId,
  mapTableView,
  naturalCompare,
  sortTableRows,
  type TableViewLike,
} from './tableAdapter'

function item(values: Record<string, unknown>): Item {
  return values as unknown as Item
}

function field(name: string, type = 'string'): Field {
  return { name, type, required: false, defaultToCurrentTimestamp: false } as Field
}

describe('protobuf Item flattening', () => {
  it('flattens protobuf and dynamic fields while retaining canonical aliases', () => {
    const source = item({
      id: 'row-1',
      srCreated: 1_700_000_000n,
      srUpdated: 1_700_000_100n,
      srCreatedRelative: 30,
      srUpdatedRelative: 5,
      directOnly: 'direct value',
      name: 'direct name',
      additionalFields: {
        name: 'dynamic name',
        count: '42',
        sr_created: 'should not replace protobuf data',
      },
    })

    const row = flattenItem(source)

    expect(row).toMatchObject({
      id: 'row-1',
      sr_created: 1_700_000_000n,
      srCreated: 1_700_000_000n,
      sr_updated: 1_700_000_100n,
      srUpdated: 1_700_000_100n,
      sr_created_relative: 30,
      srCreatedRelative: 30,
      sr_updated_relative: 5,
      srUpdatedRelative: 5,
      name: 'dynamic name',
      count: '42',
      directOnly: 'direct value',
    })
    expect(row).not.toHaveProperty('additionalFields')
    expect(Object.keys(row)).not.toContain('$typeName')
    expect(getSourceItem(row)).toBe(source)
    expect(getStableRowId(row)).toBe('row-1')
    row.id = 'changed-copy'
    expect(getStableRowId(row)).toBe('row-1')
  })

  it('supports direct-property fallback and snake-case transport data', () => {
    const source = item({
      id: 17,
      sr_created: 100n,
      sr_updated: 200n,
      sr_created_relative: 12,
      sr_updated_relative: 4,
      title: 'Direct title',
      additionalFields: {},
    })

    const row = flattenItem(source)

    expect(row).toMatchObject({
      id: 17,
      sr_created: 100n,
      srCreated: 100n,
      sr_updated: 200n,
      srUpdated: 200n,
      sr_created_relative: 12,
      srCreatedRelative: 12,
      sr_updated_relative: 4,
      srUpdatedRelative: 4,
      title: 'Direct title',
    })
    expect(getStableRowId(row)).toBe('17')
  })

  it('flattens lists and does not expose source references as row properties', () => {
    const sources = [
      item({ id: 'a', additionalFields: { value: 'one' } }),
      item({ id: 'b', additionalFields: { value: 'two' } }),
    ]

    const rows = flattenItems(sources)

    expect(rows.map((row) => row.value)).toEqual(['one', 'two'])
    expect(rows.map(getStableRowId)).toEqual(['a', 'b'])
    expect(Reflect.ownKeys(rows[0])).toEqual(['id', 'value'])
    expect(getSourceItem({ ...rows[0] })).toBeUndefined()
    expect(getStableRowId(null)).toBe('')
  })
})

describe('table view mapping', () => {
  const fields = [
    field('id', 'string'),
    field('name', 'varchar(255)'),
    field('score', 'int64'),
    field('sr_created', 'datetime'),
    field('descriptionMarkdown', 'text'),
  ]

  it('maps All Columns to structure order, full visibility, and no initial sort', () => {
    const mapped = mapTableView(fields, null)
    const syntheticAllColumns: TableViewLike = { id: -1, columns: [] }

    expect(mapped).toEqual({
      orderedColumns: ['id', 'name', 'score', 'sr_created'],
      visibleColumns: ['id', 'name', 'score', 'sr_created'],
      columnVisibility: {
        id: true,
        name: true,
        score: true,
        sr_created: true,
      },
      sortBy: null,
      sortDir: 'asc',
    })
    expect(mapTableView(fields, syntheticAllColumns)).toEqual(mapped)
    expect(mapTableView(fields, { id: 3, columns: [] })).toEqual(mapped)
  })

  it('uses server order and visibility, then appends unconfigured fields hidden', () => {
    const view: TableViewLike = {
      id: 7,
      columns: [
        { columnName: 'score', isVisible: true, columnOrder: 2, sortOrder: 'desc' },
        { columnName: 'stale_column', isVisible: true, columnOrder: 0, sortOrder: 'asc' },
        { columnName: 'name', isVisible: false, columnOrder: 1, sortOrder: '' },
      ],
    }

    expect(mapTableView(fields, view)).toEqual({
      orderedColumns: ['name', 'score', 'id', 'sr_created'],
      visibleColumns: ['score'],
      columnVisibility: {
        name: false,
        score: true,
        id: false,
        sr_created: false,
      },
      sortBy: 'score',
      sortDir: 'desc',
    })
  })

  it('keeps equal column orders stable and ignores duplicate view entries', () => {
    const view: TableViewLike = {
      columns: [
        { columnName: 'score', isVisible: true, columnOrder: 1, sortOrder: '' },
        { columnName: 'name', isVisible: true, columnOrder: 1, sortOrder: 'asc' },
        { columnName: 'score', isVisible: false, columnOrder: 2, sortOrder: 'desc' },
      ],
    }

    const mapped = mapTableView(fields, view)

    expect(mapped.orderedColumns).toEqual(['score', 'name', 'id', 'sr_created'])
    expect(mapped.visibleColumns).toEqual(['score', 'name'])
    expect(mapped.sortBy).toBe('name')
    expect(mapped.sortDir).toBe('asc')
  })
})

describe('PicoCrank header construction', () => {
  const fields = [
    field('id', 'string'),
    field('sr_created', 'datetime'),
    field('enabled', 'tinyint(1)'),
    field('amount', 'decimal(10,2)'),
    field('filename', 'varchar(255)'),
    field('notesMarkdown', 'text'),
  ]

  it('builds ordered labels and type-aware PicoCrank metadata', () => {
    const view: TableViewLike = {
      columns: [
        { columnName: 'filename', isVisible: true, columnOrder: 0, sortOrder: '' },
        { columnName: 'enabled', isVisible: true, columnOrder: 1, sortOrder: '' },
        { columnName: 'amount', isVisible: true, columnOrder: 2, sortOrder: '' },
        { columnName: 'sr_created', isVisible: true, columnOrder: 3, sortOrder: '' },
        { columnName: 'id', isVisible: true, columnOrder: 4, sortOrder: '' },
      ],
    }

    const headers = buildTableHeaders(fields, view)

    expect(headers.map(({ key, label, filterType, width, colPriority }) => ({
      key,
      label,
      filterType,
      width,
      colPriority,
    }))).toEqual([
      {
        key: 'filename',
        label: 'filename',
        filterType: 'text',
        width: undefined,
        colPriority: undefined,
      },
      {
        key: 'enabled',
        label: 'enabled',
        filterType: 'boolean',
        width: '7rem',
        colPriority: 5,
      },
      {
        key: 'amount',
        label: 'amount',
        filterType: 'text',
        width: '10rem',
        colPriority: 4,
      },
      {
        key: 'sr_created',
        label: 'Created',
        filterType: 'text',
        width: '13rem',
        colPriority: 3,
      },
      {
        key: 'id',
        label: 'ID',
        filterType: 'text',
        width: '8rem',
        colPriority: 2,
      },
    ])
    expect(headers.every((header) => header.sortable && header.filterable)).toBe(true)
    expect(headers.every((header) => typeof header.comparator === 'function')).toBe(true)
    expect(headers.map((header) => header.key)).not.toContain('notesMarkdown')
  })
})

describe('type-aware sorting', () => {
  it('natural-sorts alphanumeric values case-insensitively', () => {
    const comparator = createTableComparator('varchar(255)')

    expect(naturalCompare('file2', 'file10')).toBeLessThan(0)
    expect(comparator('File2', 'file10')).toBeLessThan(0)
    expect(comparator('item20a', 'item3a')).toBeGreaterThan(0)
  })

  it('sorts signed numbers, decimals, and integers larger than Number.MAX_SAFE_INTEGER', () => {
    const integerComparator = createTableComparator('bigint')
    const decimalComparator = createTableComparator('decimal(20,4)')

    expect(integerComparator('-10', '-2')).toBeLessThan(0)
    expect(integerComparator('9007199254740993', '9007199254740992')).toBeGreaterThan(0)
    expect(integerComparator(2n, 10n)).toBeLessThan(0)
    expect(decimalComparator('-2.50', '-10.75')).toBeGreaterThan(0)
    expect(
      decimalComparator(
        '9007199254740993.00000000000000000001',
        '9007199254740993.00000000000000000000',
      ),
    ).toBeGreaterThan(0)
  })

  it('sorts Unix and textual datetimes chronologically', () => {
    const comparator = createTableComparator('datetime')

    expect(comparator(1_700_000_000n, 1_700_000_001n)).toBeLessThan(0)
    expect(comparator('2026-02-01 08:00:00', '2025-12-31 23:00:00')).toBeGreaterThan(0)
    expect(comparator('2026-02-01T08:00:00Z', '2026-02-01T09:00:00Z')).toBeLessThan(0)
  })

  it('keeps nullish values last in both directions', () => {
    const comparator = createTableComparator('int64')

    expect(comparator(null, 1)).toBeGreaterThan(0)
    expect(comparator(1, undefined)).toBeLessThan(0)
    expect(comparator(null, undefined)).toBe(0)
  })

  it('sorts table rows through the comparator attached to each header', () => {
    const header = buildTableHeaders([field('name', 'string')])[0]
    const rows = [
      { id: '3', name: null },
      { id: '1', name: 'row10' },
      { id: '2', name: 'row2' },
    ]

    expect(sortTableRows(rows, header).map((row) => row.id)).toEqual(['2', '1', '3'])
    expect(sortTableRows(rows, header, 'desc').map((row) => row.id)).toEqual(['1', '2', '3'])
  })
})
