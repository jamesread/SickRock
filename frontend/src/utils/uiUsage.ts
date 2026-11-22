// Opt-in UI usage tracking ("Track and log UI usage"): which routes a user visits and which sidebar
// and header controls they click, buffered on the client and sent to the server, which keeps the
// newest lines in the user's own JSONL file for offline analysis of navigation loops and hot spots.

export type UiUsageKind = 'route' | 'sidebar' | 'header'

export type UiUsageEvent = {
  kind: UiUsageKind
  fromPath: string
  toPath: string
  label: string
  timestampMs: number
  sessionId: string
  appVersion: string
}

/** picocrank's Sidebar renders <aside id="picocrank-sidebar">; its Header renders <header>. */
export const SIDEBAR_SELECTOR = '#picocrank-sidebar'
export const HEADER_SELECTOR = 'header'

/** Elements whose clicks count: links, buttons and menu items. */
const CLICKABLE_SELECTOR = 'a[href], button, [role="button"], [role="menuitem"], [role="link"], summary'

/** Most events kept on the client while they wait to be sent (the oldest are dropped first). */
export const MAX_PENDING_EVENTS = 500

const MAX_LABEL_LENGTH = 120

/**
 * The sidebar or header control a click landed on, or null for clicks elsewhere (page content is
 * covered by route events) or on nothing clickable.
 */
export function describeClick(target: EventTarget | null, origin: string): Pick<UiUsageEvent, 'kind' | 'toPath' | 'label'> | null {
  if (!isElementLike(target)) return null
  const control = target.closest(CLICKABLE_SELECTOR)
  if (!control) return null
  let kind: UiUsageKind
  if (control.closest(SIDEBAR_SELECTOR)) {
    kind = 'sidebar'
  } else if (control.closest(HEADER_SELECTOR)) {
    kind = 'header'
  } else {
    return null
  }
  return { kind, toPath: linkPath(control, origin), label: controlLabel(control) }
}

/** The parts of Element used here (duck-typed, so the logic can be tested without a DOM). */
type ElementLike = Pick<Element, 'closest' | 'getAttribute' | 'textContent'>

function isElementLike(target: unknown): target is ElementLike {
  return !!target && typeof (target as Element).closest === 'function'
}

/** A link's target as a path (with query and hash) when it is on this site, otherwise in full. */
export function linkPath(control: ElementLike, origin: string): string {
  const href = control.getAttribute('href')
  if (!href) return ''
  try {
    const url = new URL(href, origin)
    return url.origin === origin ? url.pathname + url.search + url.hash : url.href
  } catch {
    return href
  }
}

/** What a control says: its aria-label, else its title, else its text. */
export function controlLabel(control: ElementLike): string {
  const text = control.getAttribute('aria-label')
    || control.getAttribute('title')
    || control.textContent
    || ''
  const label = text.replace(/\s+/g, ' ').trim()
  return label.length > MAX_LABEL_LENGTH ? `${label.slice(0, MAX_LABEL_LENGTH - 1)}…` : label
}

/** Adds events to the pending list, dropping the oldest beyond max. */
export function appendCapped(pending: UiUsageEvent[], events: UiUsageEvent[], max = MAX_PENDING_EVENTS): UiUsageEvent[] {
  const all = [...pending, ...events]
  return all.length > max ? all.slice(all.length - max) : all
}

/** Reads stored pending events, ignoring anything malformed. */
export function parsePending(raw: string | null): UiUsageEvent[] {
  if (!raw) return []
  try {
    const parsed: unknown = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    return parsed.filter((e): e is UiUsageEvent =>
      !!e && typeof e === 'object'
      && ['route', 'sidebar', 'header'].includes((e as UiUsageEvent).kind)
      && typeof (e as UiUsageEvent).timestampMs === 'number')
  } catch {
    return []
  }
}

/** Where a user's pending events are kept between page loads (per user, so accounts never mix). */
export function pendingStorageKey(username: string): string {
  return `sickrock.uiUsage.pending.${username}`
}

/** A short random id for this page load, so one tab's path can be followed through the log. */
export function newSessionId(): string {
  const bytes = new Uint8Array(6)
  globalThis.crypto?.getRandomValues?.(bytes)
  return Array.from(bytes, b => b.toString(16).padStart(2, '0')).join('') || Math.random().toString(16).slice(2, 14)
}
