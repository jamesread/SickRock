// On-demand loading of Hugeicons by stored name (e.g. table_configurations.icon).
//
// Never `import * as ... from '@hugeicons/core-free-icons'`: that bundles the whole
// catalog into every page. Icons referenced by name are fetched one at a time from
// the per-name JSON files written by vite-plugin-hugeicons.js (one per export name,
// aliases included; served from the SickRock server and cached by the service
// worker); callers show a static default icon until the real one arrives.
import type { DatabaseIcon } from '@hugeicons/core-free-icons'

export type IconData = typeof DatabaseIcon

export type HugeiconCatalog = {
  /** Icon file names, which are also their primary export names. */
  icons: string[]
  /** Other export names (e.g. "DatabaseFreeIcons") mapped to their icon file name. */
  aliases: Record<string, string>
}

export type HugeiconResult = {
  icon: IconData | null
  /** True when the icon could not be fetched (offline, server error) and should be retried. */
  transient: boolean
}

type FetchOutcome<T> =
  | { kind: 'ok'; data: T }
  | { kind: 'missing' }
  | { kind: 'transient' }

const BASE = typeof __HUGEICONS_BASE__ === 'string' ? __HUGEICONS_BASE__ : '/assets/hugeicons'

const resolved = new Map<string, IconData | null>()
const inflight = new Map<string, Promise<HugeiconResult>>()
let catalogPromise: Promise<HugeiconCatalog> | null = null

async function fetchJson<T>(url: string): Promise<FetchOutcome<T>> {
  let response: Response
  try {
    response = await fetch(url)
  } catch {
    return { kind: 'transient' }
  }
  if (response.status >= 500) return { kind: 'transient' }
  if (!response.ok) return { kind: 'missing' }
  try {
    return { kind: 'ok', data: (await response.json()) as T }
  } catch {
    // Not JSON (e.g. an HTML fallback page): treat as not found.
    return { kind: 'missing' }
  }
}

function fetchIcon(name: string): Promise<FetchOutcome<IconData>> {
  return fetchJson<IconData>(`${BASE}/${encodeURIComponent(name)}.json`)
}

/** Loads the icon name catalog, used by the icon picker. Failures are not cached. */
export function loadHugeiconCatalog(): Promise<HugeiconCatalog> {
  catalogPromise ??= fetchJson<HugeiconCatalog>(`${BASE}/catalog.json`).then((outcome) => {
    if (outcome.kind !== 'ok') {
      catalogPromise = null
      throw new Error('Could not load the icon catalog from the server.')
    }
    return outcome.data
  })
  return catalogPromise
}

async function resolveIcon(name: string): Promise<HugeiconResult> {
  const outcome = await fetchIcon(name)
  if (outcome.kind === 'ok') return { icon: outcome.data, transient: false }
  return { icon: null, transient: outcome.kind === 'transient' }
}

/** Loads an icon by name. Found and permanently-missing results are cached; transient failures are not. */
export function loadHugeiconResult(rawName: string | null | undefined): Promise<HugeiconResult> {
  const name = rawName?.trim() ?? ''
  if (!name) return Promise.resolve({ icon: null, transient: false })
  if (resolved.has(name)) return Promise.resolve({ icon: resolved.get(name) ?? null, transient: false })

  let pending = inflight.get(name)
  if (!pending) {
    pending = resolveIcon(name).then((result) => {
      if (!result.transient) resolved.set(name, result.icon)
      return result
    }).finally(() => inflight.delete(name))
    inflight.set(name, pending)
  }
  return pending
}

export async function loadHugeicon(name: string | null | undefined): Promise<IconData | null> {
  return (await loadHugeiconResult(name)).icon
}

/** How long to wait before retrying an icon that could not be fetched. */
export const HUGEICON_RETRY_MS = 30_000

/**
 * Calls `apply` with the icon once it loads. After a transient failure (offline, or the
 * server unreachable) it retries when the browser comes back online or after
 * HUGEICON_RETRY_MS, whichever is first, until the icon loads or is known to be missing.
 * Returns a function that cancels any pending retry.
 */
export function whenHugeiconAvailable(
  name: string | null | undefined,
  apply: (icon: IconData) => void,
): () => void {
  let cancelled = false
  let retryTimer: ReturnType<typeof setTimeout> | undefined
  const hasWindow = typeof window !== 'undefined'

  const stopWaiting = () => {
    clearTimeout(retryTimer)
    if (hasWindow) window.removeEventListener('online', attempt)
  }

  function attempt() {
    stopWaiting()
    loadHugeiconResult(name).then((result) => {
      if (cancelled) return
      if (result.icon) {
        apply(result.icon)
      } else if (result.transient) {
        retryTimer = setTimeout(attempt, HUGEICON_RETRY_MS)
        if (hasWindow) window.addEventListener('online', attempt)
      }
    })
  }

  attempt()
  return () => {
    cancelled = true
    stopWaiting()
  }
}

/** Display form of an icon name for searching: lower case, without the trailing "Icon". */
function searchKey(name: string): string {
  return name.replace(/Icons?$/, '').toLowerCase()
}

/**
 * Returns icon file names matching `query`, best matches first: icon names starting
 * with the query, then aliases starting with it, then names and aliases containing
 * it. An empty query returns every icon.
 */
export function searchHugeicons(catalog: HugeiconCatalog, query: string): string[] {
  const q = query.trim().toLowerCase().replace(/[\s_-]+/g, '')
  if (!q) return catalog.icons

  const best = new Map<string, { rank: number; key: string }>()
  const consider = (key: string, file: string, isAlias: boolean) => {
    const index = key.indexOf(q)
    if (index < 0) return
    const rank = (index === 0 ? 0 : 2) + (isAlias ? 1 : 0)
    const current = best.get(file)
    if (!current || rank < current.rank || (rank === current.rank && key < current.key)) {
      best.set(file, { rank, key })
    }
  }

  for (const file of catalog.icons) consider(searchKey(file), file, false)
  for (const [alias, file] of Object.entries(catalog.aliases)) consider(searchKey(alias), file, true)

  return [...best.entries()]
    .sort(([, a], [, b]) => a.rank - b.rank || a.key.localeCompare(b.key))
    .map(([file]) => file)
}

/** Test helper: forget cached icons and catalog. */
export function resetHugeiconCacheForTests() {
  resolved.clear()
  inflight.clear()
  catalogPromise = null
}
