import { describe, expect, it } from 'vitest'
import { ALL_COLUMNS_VIEW_ID, defaultExportViewOptions, initialExportViewId } from './exportView'

const views = [
  { id: 3, viewName: 'Grid', isDefault: false },
  { id: 5, viewName: 'Main', isDefault: true },
  { id: 8, viewName: 'Printable', isDefault: false },
]

describe('initialExportViewId', () => {
  it('starts on the chosen default export view', () => {
    expect(initialExportViewId(views, 8)).toBe(8)
    expect(initialExportViewId(views, ALL_COLUMNS_VIEW_ID)).toBe(ALL_COLUMNS_VIEW_ID)
  })

  it("falls back to the table's default view, its first view, then All Columns", () => {
    expect(initialExportViewId(views, 0)).toBe(5)
    expect(initialExportViewId(views, undefined)).toBe(5)
    expect(initialExportViewId(views, 99), 'a deleted view').toBe(5)
    expect(initialExportViewId(views.map(view => ({ ...view, isDefault: false })), 0)).toBe(3)
    expect(initialExportViewId([], 0)).toBe(ALL_COLUMNS_VIEW_ID)
  })
})

describe('defaultExportViewOptions', () => {
  it('offers automatic, All Columns and each view', () => {
    expect(defaultExportViewOptions(views)).toEqual([
      { value: 0, label: "Table's default view (Main)" },
      { value: -1, label: 'All Columns' },
      { value: 3, label: 'Grid' },
      { value: 5, label: 'Main' },
      { value: 8, label: 'Printable' },
    ])
    expect(defaultExportViewOptions(views.map(view => ({ ...view, isDefault: false })))[0].label, 'the first view')
      .toBe("Table's default view (Grid)")
    expect(defaultExportViewOptions([])[0].label).toBe("Table's default view (All Columns)")
  })
})
