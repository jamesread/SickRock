// The home page layout each user can customise (Edit home, in design mode): the order and visibility
// of the home sections and the bookmarks shown as quick access buttons. Stored on the server as JSON
// (GetHomeLayout/SetHomeLayout); unknown sections are dropped and new ones appear in their default
// place, so layouts saved by older or newer versions still load.

export type HomeSectionId =
  | 'quick-access'
  | 'welcome'
  | 'shared-calendars'
  | 'workflows'
  | 'recently-viewed'
  | 'bookmarks'
  | 'pages'

export type HomeSectionDef = {
  id: HomeSectionId
  title: string
  description: string
  /** Shown until the user hides it. Extra sections start hidden. */
  defaultVisible: boolean
}

/** Every home section, in the default order. */
export const HOME_SECTIONS: HomeSectionDef[] = [
  { id: 'quick-access', title: 'Quick access', description: 'Buttons for the bookmarks you choose.', defaultVisible: true },
  { id: 'welcome', title: 'Welcome', description: 'The SickRock version.', defaultVisible: true },
  { id: 'shared-calendars', title: 'Shared calendars', description: 'Read-only calendar exports you can open.', defaultVisible: true },
  { id: 'workflows', title: 'Workflows', description: 'Start a workflow.', defaultVisible: true },
  { id: 'recently-viewed', title: 'Recently viewed', description: 'Rows you opened lately, by table.', defaultVisible: true },
  { id: 'bookmarks', title: 'Bookmarks', description: 'All your bookmarks.', defaultVisible: false },
  { id: 'pages', title: 'Pages', description: 'Every page in the sidebar.', defaultVisible: false },
]

export type HomeSection = { id: HomeSectionId; visible: boolean }

export type HomeLayout = {
  sections: HomeSection[]
  /** Navigation item ids of the bookmarks shown as quick access buttons, in order. */
  quickAccess: number[]
}

export function homeSectionDef(id: HomeSectionId): HomeSectionDef {
  return HOME_SECTIONS.find(s => s.id === id) ?? HOME_SECTIONS[0]
}

export function defaultHomeLayout(): HomeLayout {
  return {
    sections: HOME_SECTIONS.map(s => ({ id: s.id, visible: s.defaultVisible })),
    quickAccess: [],
  }
}

function isSectionId(value: unknown): value is HomeSectionId {
  return HOME_SECTIONS.some(s => s.id === value)
}

/**
 * Reads a stored layout: known sections in their stored order (first occurrence), then any sections
 * it does not mention in their default place and visibility; quick access ids deduplicated.
 */
export function parseHomeLayout(raw: string | null | undefined): HomeLayout {
  if (!raw?.trim()) return defaultHomeLayout()
  let parsed: unknown
  try {
    parsed = JSON.parse(raw)
  } catch {
    return defaultHomeLayout()
  }
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return defaultHomeLayout()
  const obj = parsed as { sections?: unknown; quickAccess?: unknown }

  const sections: HomeSection[] = []
  for (const entry of Array.isArray(obj.sections) ? obj.sections : []) {
    const id = (entry as { id?: unknown })?.id
    if (!isSectionId(id) || sections.some(s => s.id === id)) continue
    const visible = (entry as { visible?: unknown }).visible
    sections.push({ id, visible: typeof visible === 'boolean' ? visible : homeSectionDef(id).defaultVisible })
  }
  for (const [index, def] of HOME_SECTIONS.entries()) {
    if (sections.some(s => s.id === def.id)) continue
    // Insert after the section that precedes it by default, so a new section lands in its usual place.
    const before = HOME_SECTIONS.slice(0, index).map(d => d.id)
    const at = sections.reduce((pos, s, i) => (before.includes(s.id) ? i + 1 : pos), 0)
    sections.splice(at, 0, { id: def.id, visible: def.defaultVisible })
  }

  const quickAccess: number[] = []
  for (const id of Array.isArray(obj.quickAccess) ? obj.quickAccess : []) {
    if (Number.isInteger(id) && (id as number) > 0 && !quickAccess.includes(id as number)) quickAccess.push(id as number)
  }
  return { sections, quickAccess }
}

/** The stored form of a layout; the default layout is stored as empty. */
export function serializeHomeLayout(layout: HomeLayout): string {
  return isDefaultHomeLayout(layout) ? '' : JSON.stringify(layout)
}

export function isDefaultHomeLayout(layout: HomeLayout): boolean {
  return JSON.stringify(layout) === JSON.stringify(defaultHomeLayout())
}

/** The visible sections, in order. */
export function visibleHomeSections(layout: HomeLayout): HomeSectionId[] {
  return layout.sections.filter(s => s.visible).map(s => s.id)
}

/** The parts of a navigation item (a sidebar link or a bookmark's target) that decide where it goes. */
export type NavigationTarget = {
  tableName?: string
  dashboardId?: number
  dashboardName?: string
  workflowId?: number
}

/** Where a navigation item links to: its dashboard, else its workflow hub, else its table. */
export function navigationItemPath(item: NavigationTarget): string {
  if ((item.dashboardId ?? 0) > 0 && item.dashboardName) return `/dashboard/${item.dashboardName}`
  if ((item.workflowId ?? 0) > 0 && !item.tableName) return `/workflow/${item.workflowId}`
  return item.tableName ? `/table/${item.tableName}` : ''
}

export type HomeLink = { id: number; title: string; path: string; icon: string }

/** The quick access buttons: chosen bookmarks that still exist, in the chosen order. */
export function quickAccessLinks(layout: HomeLayout, bookmarks: HomeLink[]): HomeLink[] {
  return layout.quickAccess
    .map(id => bookmarks.find(b => b.id === id))
    .filter((b): b is HomeLink => !!b)
}
