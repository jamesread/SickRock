import { describe, expect, it } from 'vitest'
import { VIEW_TYPE_OPTIONS, viewTypeLabel } from './viewTypes'

describe('viewTypeLabel', () => {
  it('names each view type', () => {
    expect(VIEW_TYPE_OPTIONS.map(option => viewTypeLabel(option.value))).toEqual(['Table', 'Calendar', 'Tick List', 'Weekly Plan', 'Cards'])
  })

  it('treats an empty type as a table and shows unknown types as stored', () => {
    expect(viewTypeLabel('')).toBe('Table')
    expect(viewTypeLabel(undefined)).toBe('Table')
    expect(viewTypeLabel(' kanban ')).toBe('kanban')
  })
})
