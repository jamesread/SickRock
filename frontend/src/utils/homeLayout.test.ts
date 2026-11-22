import { describe, expect, it } from 'vitest'

import {
  HOME_SECTIONS,
  defaultHomeLayout,
  isDefaultHomeLayout,
  navigationItemPath,
  parseHomeLayout,
  quickAccessLinks,
  serializeHomeLayout,
  visibleHomeSections,
} from './homeLayout'

describe('parseHomeLayout', () => {
  it('falls back to the default layout', () => {
    for (const raw of [undefined, '', 'not json', '[1]', 'null']) {
      expect(parseHomeLayout(raw)).toEqual(defaultHomeLayout())
    }
  })

  it('keeps the stored order and visibility', () => {
    const layout = parseHomeLayout(JSON.stringify({
      sections: [{ id: 'recently-viewed', visible: true }, { id: 'welcome', visible: false }],
      quickAccess: [4, 2, 4, -1, 'x'],
    }))
    const stored = layout.sections.filter(s => s.id === 'recently-viewed' || s.id === 'welcome')
    expect(stored).toEqual([{ id: 'recently-viewed', visible: true }, { id: 'welcome', visible: false }])
    expect(layout.quickAccess).toEqual([4, 2])
  })

  it('drops unknown and repeated sections and adds missing ones in their default place', () => {
    const layout = parseHomeLayout(JSON.stringify({
      sections: [{ id: 'welcome', visible: true }, { id: 'weather', visible: true }, { id: 'welcome', visible: false }, { id: 'workflows' }],
    }))
    const ids = layout.sections.map(s => s.id)
    expect(ids).toHaveLength(HOME_SECTIONS.length)
    expect(new Set(ids).size).toBe(ids.length)
    expect(ids).not.toContain('weather')
    expect(layout.sections.find(s => s.id === 'welcome')?.visible).toBe(true)
    expect(layout.sections.find(s => s.id === 'workflows')?.visible).toBe(true)
    // shared-calendars defaults to after welcome, so it is put back there.
    expect(ids.indexOf('shared-calendars')).toBe(ids.indexOf('welcome') + 1)
    expect(layout.sections.find(s => s.id === 'pages')?.visible).toBe(false)
  })
})

describe('serializeHomeLayout', () => {
  it('stores the default layout as empty and round-trips others', () => {
    expect(serializeHomeLayout(defaultHomeLayout())).toBe('')
    expect(isDefaultHomeLayout(defaultHomeLayout())).toBe(true)
    const custom = defaultHomeLayout()
    custom.sections.reverse()
    custom.quickAccess = [7]
    expect(parseHomeLayout(serializeHomeLayout(custom))).toEqual(custom)
  })
})

describe('visibleHomeSections', () => {
  it('lists the shown sections in order', () => {
    expect(visibleHomeSections(defaultHomeLayout())).toEqual(['quick-access', 'welcome', 'shared-calendars', 'workflows', 'recently-viewed'])
  })
})

describe('navigationItemPath', () => {
  it('links to a dashboard, a workflow hub or a table', () => {
    expect(navigationItemPath({ dashboardId: 3, dashboardName: 'home', tableName: 'x' })).toBe('/dashboard/home')
    expect(navigationItemPath({ workflowId: 5 })).toBe('/workflow/5')
    expect(navigationItemPath({ workflowId: 5, tableName: 'pantry' })).toBe('/table/pantry')
    expect(navigationItemPath({ tableName: 'pantry' })).toBe('/table/pantry')
    expect(navigationItemPath({})).toBe('')
  })
})

describe('quickAccessLinks', () => {
  it('keeps the chosen order and skips bookmarks that are gone', () => {
    const bookmarks = [
      { id: 1, title: 'A', path: '/a', icon: '' },
      { id: 2, title: 'B', path: '/b', icon: '' },
    ]
    const layout = { ...defaultHomeLayout(), quickAccess: [2, 9, 1] }
    expect(quickAccessLinks(layout, bookmarks).map(b => b.title)).toEqual(['B', 'A'])
  })
})
