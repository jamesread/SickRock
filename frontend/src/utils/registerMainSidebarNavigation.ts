import type { NavigationApp, NavigationItem } from '../gen/sickrock_pb'
import { resolveNavigationIcon } from './navigationIcon'

export type MainSidebarPage = {
  id: string
  navigationItemId: number
  title: string
  slug: string
  view: string
  icon: string
  path: string
  workflowId: number
  dashboardId: number
}

export type SidebarNavigationApi = {
  clearNavigationLinks: () => void
  addNavigationLink: (link: Record<string, unknown>) => void
  addSection: (title: string, options?: { name?: string }) => void
  addSeparator: (id?: string) => void
  addRouterLink: (name: string) => void
}

export function navigationItemToPage(item: NavigationItem): MainSidebarPage | null {
  const title = item.title || String(item.id)
  const workflowId = item.workflowId || 0
  const dashboardId = item.dashboardId || 0
  let path = ''
  if (workflowId > 0 && !item.tableName && !(dashboardId > 0 && item.dashboardName)) {
    path = `/workflow/${workflowId}`
  } else if (dashboardId > 0 && item.dashboardName) {
    path = `/dashboard/${item.dashboardName}`
  } else if (item.tableName) {
    path = `/table/${item.tableName}`
  } else if (workflowId > 0) {
    path = `/workflow/${workflowId}`
  }
  if (!path) {
    return null
  }
  const slug = item.tableName || item.dashboardName || (workflowId > 0 ? `workflow-${workflowId}` : '')
  let defaultIcon = 'DatabaseIcon'
  if (dashboardId > 0) {
    defaultIcon = 'LayoutIcon'
  } else if (workflowId > 0) {
    defaultIcon = 'WorkflowIcon'
  }
  const icon = item.icon || defaultIcon
  const view = item.tableView || ''
  const navKey = `navigation-item-${item.id}`
  return {
    id: navKey,
    navigationItemId: item.id,
    title,
    slug,
    icon,
    view,
    path,
    workflowId,
    dashboardId,
  }
}

export function pagesFromNavigationItems(items: NavigationItem[]): MainSidebarPage[] {
  return [...items]
    .sort((a, b) => (a.ordinal ?? 0) - (b.ordinal ?? 0))
    .map(navigationItemToPage)
    .filter((pg): pg is MainSidebarPage => pg != null)
}

// Bumped on every sidebar rebuild so an icon that finishes loading late never
// re-adds a link from a previous registration.
let registrationGeneration = 0

function addPageLink(nav: SidebarNavigationApi, pg: MainSidebarPage) {
  const generation = registrationGeneration
  const link = {
    id: pg.id,
    name: pg.id,
    title: pg.title,
    path: pg.path,
  }
  const icon = resolveNavigationIcon(
    {
      icon: pg.icon,
      workflowId: pg.workflowId,
      dashboardId: pg.dashboardId,
      path: pg.path,
    },
    (loaded) => {
      if (generation === registrationGeneration) {
        nav.addNavigationLink({ ...link, icon: loaded })
      }
    },
  )
  nav.addNavigationLink({ ...link, icon })
}

export type RegisterMainSidebarOptions = {
  homeIcon: unknown
  canAccessControlPanel: boolean
  canSystemSettings: boolean
  databaseSettingIcon: unknown
  /** Icon for the System → Health link (shown with canSystemSettings). */
  healthIcon?: unknown
  ungroupedItems?: NavigationItem[]
  apps?: NavigationApp[]
}

/** Register Home, Ungrouped, app sections, then System (separator + section). */
export function registerMainSidebarNavigation(
  nav: SidebarNavigationApi,
  pages: MainSidebarPage[],
  options: RegisterMainSidebarOptions,
) {
  registrationGeneration++
  nav.clearNavigationLinks()

  nav.addNavigationLink({
    id: 'home',
    name: 'Home',
    title: 'Home',
    path: '/',
    icon: options.homeIcon,
  })

  // Fall back to every page only when the server sent no grouping at all; when every
  // link belongs to an app, ungroupedItems is legitimately empty.
  const hasAppItems = (options.apps ?? []).some(app => (app.items ?? []).length > 0)
  const ungroupedFromApi = options.ungroupedItems?.length || hasAppItems
    ? pagesFromNavigationItems(options.ungroupedItems ?? [])
    : pages

  if (ungroupedFromApi.length > 0) {
    nav.addSection('Ungrouped', { name: 'nav-section-ungrouped' })
    for (const pg of ungroupedFromApi) {
      addPageLink(nav, pg)
    }
  }

  const apps = [...(options.apps ?? [])].sort(
    (a, b) => (a.ordinal ?? 0) - (b.ordinal ?? 0) || (a.title || '').localeCompare(b.title || ''),
  )
  for (const app of apps) {
    const appPages = pagesFromNavigationItems(app.items ?? [])
    if (appPages.length === 0) {
      continue
    }
    nav.addSection(app.title || `App ${app.id}`, { name: `nav-app-${app.id}` })
    for (const pg of appPages) {
      addPageLink(nav, pg)
    }
  }

  if (options.canAccessControlPanel || options.canSystemSettings) {
    nav.addSeparator()
    nav.addSection('System', { name: 'nav-section-system' })
  }
  if (options.canAccessControlPanel) {
    nav.addRouterLink('control-panel')
  }
  if (options.canSystemSettings) {
    nav.addNavigationLink({
      id: 'table-configurations',
      name: 'Table Configurations',
      title: 'Table Configurations',
      path: '/table/table_configurations',
      icon: options.databaseSettingIcon,
    })
    updateSidebarHealthLink(nav, options.healthIcon, 0)
  }
}

/** Adds or updates the System → Health link; `count` is open, unacknowledged warnings and errors. */
export function updateSidebarHealthLink(nav: SidebarNavigationApi, icon: unknown, count: number) {
  nav.addNavigationLink({
    id: 'health',
    name: 'Health',
    title: 'Health',
    path: '/admin/health',
    icon,
    count,
  })
}
