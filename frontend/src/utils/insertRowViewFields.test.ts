import { describe, expect, it } from 'vitest'
import { applyViewToInsertFieldDefs, fieldDefsForInsertForm } from './insertRowViewFields'

describe('insertRowViewFields', () => {
  const defs = fieldDefsForInsertForm([
    { name: 'sr_created', type: 'datetime' },
    { name: 'a', type: 'string' },
    { name: 'b', type: 'string' },
    { name: 'c', type: 'string' },
  ])

  it('drops sr_created and sr_updated from insert form fields', () => {
    expect(defs.map(d => d.name)).toEqual(['a', 'b', 'c'])
  })

  it('orders and filters by selected view', () => {
    const views = [{
      id: 2,
      columns: [
        { columnName: 'c', isVisible: true, columnOrder: 0 },
        { columnName: 'a', isVisible: false, columnOrder: 1 },
        { columnName: 'b', isVisible: true, columnOrder: 2 },
      ],
    }]
    const result = applyViewToInsertFieldDefs(defs, views, 2)
    expect(result.map(d => d.name)).toEqual(['c', 'b'])
  })

  it('returns all fields when view is all columns', () => {
    const views = [{ id: 2, columns: [{ columnName: 'b', isVisible: true, columnOrder: 0 }] }]
    expect(applyViewToInsertFieldDefs(defs, views, -1).map(d => d.name)).toEqual(['a', 'b', 'c'])
  })
})
