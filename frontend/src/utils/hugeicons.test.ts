import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join } from 'node:path'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
  HUGEICON_RETRY_MS,
  loadHugeicon,
  loadHugeiconCatalog,
  loadHugeiconResult,
  resetHugeiconCacheForTests,
  searchHugeicons,
  whenHugeiconAvailable,
  type HugeiconCatalog,
} from './hugeicons'

const SHIELD = [['path', { d: 'M1 1' }]]
const DATABASE = [['path', { d: 'M2 2' }]]

const catalog: HugeiconCatalog = {
  icons: ['DatabaseIcon', 'SecurityBlockIcon', 'ShieldIcon', 'UserShield01Icon'],
  aliases: { DatabaseFreeIcons: 'DatabaseIcon', Guard: 'ShieldIcon', ShieldBlockIcon: 'SecurityBlockIcon' },
}

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
}

/** Routes fetch by file name: icon JSON, catalog, 404 otherwise. */
function serveIcons(files: Record<string, unknown>) {
  return vi.fn(async (url: string) => {
    const name = decodeURIComponent(url.split('/').pop()!.replace(/\.json$/, ''))
    if (name === 'catalog') return jsonResponse(catalog)
    if (name in files) return jsonResponse(files[name])
    return new Response('not found', { status: 404 })
  })
}

describe('loadHugeicon', () => {
  beforeEach(() => resetHugeiconCacheForTests())
  afterEach(() => vi.unstubAllGlobals())

  it('loads an icon by its file name without fetching the catalog', async () => {
    const fetchMock = serveIcons({ ShieldIcon: SHIELD })
    vi.stubGlobal('fetch', fetchMock)

    expect(await loadHugeicon('ShieldIcon')).toEqual(SHIELD)
    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(fetchMock.mock.calls[0][0]).not.toContain('catalog')
  })

  it('loads export aliases from their own file, without the catalog', async () => {
    const fetchMock = serveIcons({ DatabaseFreeIcons: DATABASE })
    vi.stubGlobal('fetch', fetchMock)

    expect(await loadHugeicon('DatabaseFreeIcons')).toEqual(DATABASE)
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('returns null for unknown names and caches the miss', async () => {
    const fetchMock = serveIcons({})
    vi.stubGlobal('fetch', fetchMock)

    expect(await loadHugeiconResult('NoSuchIcon')).toEqual({ icon: null, transient: false })
    const calls = fetchMock.mock.calls.length
    expect(await loadHugeicon('NoSuchIcon')).toBeNull()
    expect(fetchMock.mock.calls.length).toBe(calls)
  })

  it('returns null without fetching for empty names', async () => {
    const fetchMock = serveIcons({})
    vi.stubGlobal('fetch', fetchMock)

    expect(await loadHugeicon('  ')).toBeNull()
    expect(await loadHugeicon(undefined)).toBeNull()
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('de-duplicates concurrent requests and caches hits', async () => {
    const fetchMock = serveIcons({ ShieldIcon: SHIELD })
    vi.stubGlobal('fetch', fetchMock)

    const [a, b] = await Promise.all([loadHugeicon('ShieldIcon'), loadHugeicon('ShieldIcon')])
    await loadHugeicon('ShieldIcon')
    expect(a).toEqual(SHIELD)
    expect(b).toEqual(SHIELD)
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('does not cache network failures, so a later attempt succeeds', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => { throw new TypeError('Failed to fetch') }))
    expect(await loadHugeiconResult('ShieldIcon')).toEqual({ icon: null, transient: true })

    vi.stubGlobal('fetch', serveIcons({ ShieldIcon: SHIELD }))
    expect(await loadHugeicon('ShieldIcon')).toEqual(SHIELD)
  })

  it('treats server errors as transient', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response('', { status: 503 })))
    expect(await loadHugeiconResult('ShieldIcon')).toEqual({ icon: null, transient: true })

    vi.stubGlobal('fetch', serveIcons({ ShieldIcon: SHIELD }))
    expect(await loadHugeicon('ShieldIcon')).toEqual(SHIELD)
  })

  it('treats a non-JSON 200 response (HTML fallback page) as missing', async () => {
    vi.stubGlobal('fetch', vi.fn(async () =>
      new Response('<!doctype html>', { status: 200, headers: { 'Content-Type': 'text/html' } })))
    expect(await loadHugeiconResult('NoSuchIcon')).toEqual({ icon: null, transient: false })
  })
})

describe('whenHugeiconAvailable', () => {
  beforeEach(() => {
    resetHugeiconCacheForTests()
    vi.useFakeTimers()
  })
  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('retries after a transient failure until the icon loads', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => { throw new TypeError('Failed to fetch') }))
    const apply = vi.fn()
    whenHugeiconAvailable('ShieldIcon', apply)
    await vi.advanceTimersByTimeAsync(0)
    expect(apply).not.toHaveBeenCalled()

    vi.stubGlobal('fetch', serveIcons({ ShieldIcon: SHIELD }))
    await vi.advanceTimersByTimeAsync(HUGEICON_RETRY_MS)
    expect(apply).toHaveBeenCalledWith(SHIELD)
  })

  it('stops retrying once cancelled', async () => {
    const fetchMock = vi.fn(async () => { throw new TypeError('Failed to fetch') })
    vi.stubGlobal('fetch', fetchMock)
    const cancel = whenHugeiconAvailable('ShieldIcon', vi.fn())
    await vi.advanceTimersByTimeAsync(0)
    cancel()
    await vi.advanceTimersByTimeAsync(HUGEICON_RETRY_MS * 3)
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('does not retry an icon that is known to be missing', async () => {
    const fetchMock = serveIcons({})
    vi.stubGlobal('fetch', fetchMock)
    whenHugeiconAvailable('NoSuchIcon', vi.fn())
    await vi.advanceTimersByTimeAsync(HUGEICON_RETRY_MS * 3)
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })
})

describe('loadHugeiconCatalog', () => {
  beforeEach(() => resetHugeiconCacheForTests())
  afterEach(() => vi.unstubAllGlobals())

  it('rejects when the server is unreachable and retries on the next call', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => { throw new TypeError('Failed to fetch') }))
    await expect(loadHugeiconCatalog()).rejects.toThrow()

    vi.stubGlobal('fetch', serveIcons({}))
    expect(await loadHugeiconCatalog()).toEqual(catalog)
  })
})

describe('searchHugeicons', () => {
  it('returns every icon for an empty query', () => {
    expect(searchHugeicons(catalog, '  ')).toEqual(catalog.icons)
  })

  it('ranks name prefixes, then alias prefixes, then substring matches', () => {
    expect(searchHugeicons(catalog, 'SHIELD')).toEqual(['ShieldIcon', 'SecurityBlockIcon', 'UserShield01Icon'])
  })

  it('matches aliases and returns the icon file name once', () => {
    expect(searchHugeicons(catalog, 'guard')).toEqual(['ShieldIcon'])
    expect(searchHugeicons(catalog, 'database')).toEqual(['DatabaseIcon'])
  })

  it('ignores spaces in the query', () => {
    expect(searchHugeicons(catalog, 'user shield')).toEqual(['UserShield01Icon'])
  })
})

describe('bundle guard', () => {
  function sourceFiles(dir: string): string[] {
    return readdirSync(dir).flatMap((entry) => {
      const path = join(dir, entry)
      if (statSync(path).isDirectory()) return entry === 'gen' ? [] : sourceFiles(path)
      return /\.(ts|vue)$/.test(entry) ? [path] : []
    })
  }

  it('never namespace-imports @hugeicons/core-free-icons (it would bundle every icon)', () => {
    const srcDir = join(__dirname, '..')
    const offenders = sourceFiles(srcDir).filter((file) =>
      /import\s+\*\s+as\s+\w+\s+from\s+['"]@hugeicons\/core-free-icons['"]/.test(readFileSync(file, 'utf8')))
    expect(offenders).toEqual([])
  })
})
