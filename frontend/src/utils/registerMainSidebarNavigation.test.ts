import { describe, expect, it, vi } from 'vitest'
import type { NavigationApp, NavigationItem } from '../gen/sickrock_pb'
import { registerMainSidebarNavigation, type MainSidebarPage } from './registerMainSidebarNavigation'

vi.mock('./navigationIcon', () => ({ resolveNavigationIcon: () => null }))

function navItem(id: number, tableName: string): NavigationItem {
  return { id, ordinal: id, tableName, title: tableName } as NavigationItem
}

function recordingNav() {
  const calls: string[] = []
  return {
    calls,
    api: {
      clearNavigationLinks: () => {},
      addNavigationLink: (link: Record<string, unknown>) => calls.push(`link:${link.title}`),
      addSection: (title: string) => calls.push(`section:${title}`),
      addSeparator: () => {},
      addRouterLink: () => {},
    },
  }
}

const options = {
  homeIcon: null,
  canAccessControlPanel: false,
  canSystemSettings: false,
  databaseSettingIcon: null,
}

const pages: MainSidebarPage[] = [
  { id: 'navigation-item-1', navigationItemId: 1, title: 'clients', slug: 'clients', view: '', icon: '', path: '/table/clients', workflowId: 0, dashboardId: 0 },
]

describe('registerMainSidebarNavigation', () => {
  it('omits Ungrouped when every link belongs to an app', () => {
    const nav = recordingNav()
    const apps = [{ id: 1, title: 'Customers', ordinal: 1, items: [navItem(1, 'clients')] }] as NavigationApp[]
    registerMainSidebarNavigation(nav.api, pages, { ...options, ungroupedItems: [], apps })
    expect(nav.calls).toEqual(['link:Home', 'section:Customers', 'link:clients'])
  })

  it('falls back to all pages when the server sent no grouping', () => {
    const nav = recordingNav()
    registerMainSidebarNavigation(nav.api, pages, { ...options, ungroupedItems: [], apps: [] })
    expect(nav.calls).toEqual(['link:Home', 'section:Ungrouped', 'link:clients'])
  })
})
