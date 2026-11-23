import { describe, expect, it } from 'vitest'
import { allRowsRollupRequest, rollupResultText, rollupSourceIdsFrom, weekRollupRequest } from './rollups'

describe('rollup requests', () => {
  it('covers a week with an exclusive end', () => {
    expect(weekRollupRequest(3n, 'datetime', new Date(2026, 10, 23))).toEqual({
      rollupId: 3n, dateColumn: 'datetime', from: '2026-11-23', to: '2026-11-30', where: {}, sourceIds: [], limitToSourceIds: false,
    })
  })

  it('crosses month and year ends', () => {
    const req = weekRollupRequest(1n, 'day', new Date(2026, 11, 28))
    expect([req.from, req.to]).toEqual(['2026-12-28', '2027-01-04'])
  })

  it('runs over every row without a range', () => {
    expect(allRowsRollupRequest(2n)).toMatchObject({ rollupId: 2n, dateColumn: '', from: '', limitToSourceIds: false })
  })

  it('limits to listed rows, even when none are listed', () => {
    expect(allRowsRollupRequest(2n, ['4', '9'])).toMatchObject({ sourceIds: ['4', '9'], limitToSourceIds: true })
    expect(allRowsRollupRequest(2n, [])).toMatchObject({ sourceIds: [], limitToSourceIds: true })
  })
})

describe('rollupSourceIdsFrom', () => {
  const rows = [{ id: 1 }, { id: 3 }]

  it('is undefined without active filters', () => {
    expect(rollupSourceIdsFrom(rows, { filtersActive: false })).toBeUndefined()
  })

  it('lists the ids of the rows the filters leave', () => {
    expect(rollupSourceIdsFrom(rows, { filtersActive: true })).toEqual(['1', '3'])
    expect(rollupSourceIdsFrom([], { filtersActive: true })).toEqual([])
  })
})

describe('rollupResultText', () => {
  it('counts rows', () => {
    expect(rollupResultText({ rowsWritten: 12n, sourceRows: 5n })).toBe('Wrote 12 rows from 5 source rows.')
    expect(rollupResultText({ rowsWritten: 1n, sourceRows: 1n })).toBe('Wrote 1 row from 1 source row.')
  })
})
