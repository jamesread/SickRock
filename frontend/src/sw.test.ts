// Runs public/sw.js against a fake browser (caches, fetch, events) to check its caching and
// offline behaviour end to end.
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const ORIGIN = 'https://app.test'
const swSource = readFileSync(fileURLToPath(new URL('../public/sw.js', import.meta.url)), 'utf8')

type Listener = (event: FakeEvent) => void
type FakeRequest = { url: string; method: string; mode: string }
type FakeEvent = {
  request?: FakeRequest
  respondWith: (response: Promise<Response>) => void
  waitUntil: (promise: Promise<unknown>) => void
}

class FakeCache {
  entries = new Map<string, Response>()
  constructor(private readonly fetcher: (request: FakeRequest | string) => Promise<Response>) {}
  async match(key: string) {
    return this.entries.get(key)?.clone()
  }
  async put(key: string, response: Response) {
    this.entries.set(key, response)
  }
  async add(url: string) {
    const response = await this.fetcher(url)
    if (!response.ok) throw new TypeError(`bad response for ${url}`)
    this.entries.set(url, response)
  }
}

class FakeCacheStorage {
  stores = new Map<string, FakeCache>()
  constructor(private readonly fetcher: (request: FakeRequest | string) => Promise<Response>) {}
  async open(name: string) {
    if (!this.stores.has(name)) this.stores.set(name, new FakeCache(this.fetcher))
    return this.stores.get(name)!
  }
  async keys() {
    return [...this.stores.keys()]
  }
  async delete(name: string) {
    return this.stores.delete(name)
  }
  /** Every cached URL, as "cache-kind:path" (kind = shell, assets or hugeicons). */
  contents() {
    const out: string[] = []
    for (const [name, cache] of this.stores) {
      const kind = name.split('-')[1]
      for (const key of cache.entries.keys()) out.push(`${kind}:${key.replace(ORIGIN, '')}`)
    }
    return out.sort()
  }
}

/** A same-origin network response, as the browser would give the service worker. */
function serverResponse(body: string, contentType: string, status = 200): Response {
  const response = new Response(body, { status, headers: { 'Content-Type': contentType } })
  Object.defineProperty(response, 'type', { value: 'basic' })
  return response
}

const html = (body = '<!doctype html><title>SickRock</title>') => serverResponse(body, 'text/html; charset=utf-8')

let online: boolean
let network: Map<string, () => Response>
let fetchMock: ReturnType<typeof vi.fn>
let caches: FakeCacheStorage
let listeners: Record<string, Listener>
let self: Record<string, unknown>

function loadServiceWorker() {
  fetchMock = vi.fn(async (input: FakeRequest | string) => {
    const url = new URL(typeof input === 'string' ? input : input.url, ORIGIN)
    if (!online) throw new TypeError('Failed to fetch')
    const respond = network.get(url.pathname)
    return respond ? respond() : serverResponse('not found', 'text/plain', 404)
  })
  caches = new FakeCacheStorage(fetchMock)
  listeners = {}
  self = {
    registration: { scope: `${ORIGIN}/` },
    location: { origin: ORIGIN },
    addEventListener: (type: string, listener: Listener) => { listeners[type] = listener },
    skipWaiting: vi.fn(async () => undefined),
    clients: { claim: vi.fn(async () => undefined) },
  }
  new Function('self', 'caches', 'fetch', swSource)(self, caches, fetchMock)
}

async function lifecycle(type: 'install' | 'activate') {
  let pending: Promise<unknown> = Promise.resolve()
  listeners[type]({ respondWith: () => {}, waitUntil: (p) => { pending = p } })
  await pending
}

/** Dispatches a fetch event: the response, or undefined when the worker lets it through. */
async function request(path: string, init: Partial<FakeRequest> = {}): Promise<Response | undefined> {
  let responded: Promise<Response> | undefined
  const url = path.startsWith('http') || path.includes(':') ? path : ORIGIN + path
  listeners.fetch({
    request: { url, method: 'GET', mode: 'no-cors', ...init },
    respondWith: (p) => { responded = p },
    waitUntil: () => {},
  })
  return responded
}

const navigate = (path: string) => request(path, { mode: 'navigate' })
const settle = () => new Promise((resolve) => setTimeout(resolve, 0))

beforeEach(() => {
  online = true
  network = new Map<string, () => Response>([
    ['/', () => html('<!doctype html><title>shell</title>')],
    ['/offline.html', () => html('<!doctype html><title>offline</title>')],
    ['/manifest.json', () => serverResponse('{"name":"SickRock"}', 'application/manifest+json')],
    ['/icons/icon-192x192.png', () => serverResponse('png192', 'image/png')],
    ['/icons/icon-512x512.png', () => serverResponse('png512', 'image/png')],
  ])
  loadServiceWorker()
})

describe('install and activate', () => {
  it('precaches the shell, offline page, manifest and icons, then takes over', async () => {
    await lifecycle('install')
    expect(caches.contents()).toEqual([
      'shell:/', 'shell:/icons/icon-192x192.png', 'shell:/icons/icon-512x512.png',
      'shell:/manifest.json', 'shell:/offline.html',
    ])
    expect(self.skipWaiting).toHaveBeenCalled()
  })

  it('still installs when some files cannot be fetched', async () => {
    network.delete('/manifest.json')
    network.set('/icons/icon-512x512.png', () => { throw new Error('boom') })
    await lifecycle('install')
    expect(caches.contents()).toContain('shell:/offline.html')
    expect(caches.contents()).not.toContain('shell:/manifest.json')
    expect(self.skipWaiting).toHaveBeenCalled()
  })

  it('installs (with nothing cached) when offline', async () => {
    online = false
    await lifecycle('install')
    expect(caches.contents()).toEqual([])
    expect(self.skipWaiting).toHaveBeenCalled()
  })

  it("deletes this scope's old SickRock caches only, then claims pages", async () => {
    for (const name of ['sickrock-v7-root', 'sickrock-static-v7-root', 'sickrock-hugeicons-v7-root', 'sickrock-v7-other-scope', 'another-app-cache']) {
      await caches.open(name)
    }
    await lifecycle('install')
    await lifecycle('activate')
    const names = await caches.keys()
    expect(names).not.toContain('sickrock-v7-root')
    expect(names).not.toContain('sickrock-static-v7-root')
    expect(names).not.toContain('sickrock-hugeicons-v7-root')
    expect(names).toContain('sickrock-v7-other-scope')
    expect(names).toContain('another-app-cache')
    expect(names.some((n) => n.startsWith('sickrock-shell-'))).toBe(true)
    expect((self.clients as { claim: () => void }).claim).toHaveBeenCalled()
  })
})

describe('requests the worker leaves alone', () => {
  it.each([
    ['/api/sickrock.SickRock/ListItems', {}],
    ['/api/images/4', {}],
    ['/openapi', {}],
    ['/llms.txt', {}],
    ['/metrics', {}],
    ['/mcp', {}],
    ['/table/jobs', { method: 'POST', mode: 'navigate' }],
    ['https://cdn.example/lib.js', {}],
    ['chrome-extension://abc/x.js', {}],
    ['/assets/hugeicons/1.0.0/catalog.json', {}],
    ['/assets/unhashed.js', {}],
    ['/images/photo.png', {}],
  ])('%s', async (path, init) => {
    expect(await request(path, init)).toBeUndefined()
    expect(fetchMock).not.toHaveBeenCalled()
  })
})

describe('hashed build assets', () => {
  const asset = '/assets/index-AbCd1234.js'

  it('are cached on first use and then served offline', async () => {
    network.set(asset, () => serverResponse('console.log(1)', 'text/javascript'))
    expect(await (await request(asset))!.text()).toBe('console.log(1)')
    await settle()
    expect(caches.contents()).toContain(`assets:${asset}`)

    online = false
    const offline = await request(asset)
    expect(await offline!.text()).toBe('console.log(1)')
  })

  it('come from the cache without a network request once cached', async () => {
    network.set(asset, () => serverResponse('v1', 'text/javascript'))
    await request(asset)
    await settle()
    fetchMock.mockClear()
    expect(await (await request(asset))!.text()).toBe('v1')
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('are never cached as an HTML page or an error', async () => {
    network.set(asset, () => html())
    network.set('/assets/missing-Zz99Yy88.css', () => serverResponse('nope', 'text/plain', 404))
    await request(asset)
    await request('/assets/missing-Zz99Yy88.css')
    await settle()
    expect(caches.contents()).toEqual([])
  })

  it('fail as the network fails when they are not cached', async () => {
    online = false
    await expect(request(asset)).rejects.toThrow()
  })
})

describe('icon files', () => {
  const icon = '/assets/hugeicons/1.0.0/HomeIcon.json'

  it('are cached cache-first, but only as JSON', async () => {
    network.set(icon, () => serverResponse('[1]', 'application/json'))
    network.set('/assets/hugeicons/1.0.0/Broken.json', () => html())
    await request(icon)
    await request('/assets/hugeicons/1.0.0/Broken.json')
    await settle()
    expect(caches.contents()).toEqual([`hugeicons:${icon}`])

    online = false
    expect(await (await request(icon))!.text()).toBe('[1]')
  })
})

describe('page loads', () => {
  it('come from the network and refresh the cached shell', async () => {
    network.set('/table/jobs', () => html('<!doctype html><title>new shell</title>'))
    const response = await navigate('/table/jobs')
    expect(await response!.text()).toContain('new shell')
    await settle()

    online = false
    expect(await (await navigate('/table/other'))!.text()).toContain('new shell')
  })

  it('do not replace the shell with an error or a non-HTML response', async () => {
    await lifecycle('install')
    network.set('/table/broken', () => serverResponse('oops', 'text/html', 500))
    network.set('/table/json', () => serverResponse('{}', 'application/json'))
    await navigate('/table/broken')
    await navigate('/table/json')
    await settle()

    online = false
    expect(await (await navigate('/table/jobs'))!.text()).toContain('<title>shell</title>')
  })

  it.each(['/', '/table/jobs', '/table/jobs/12?view=3'])('offline, %s opens from the cached shell', async (path) => {
    await lifecycle('install')
    online = false
    const response = await navigate(path)
    expect(response!.status).toBe(200)
    expect(await response!.text()).toContain('<title>shell</title>')
  })

  it.each(['/admin/health', '/control-panel', '/user-preferences', '/login', '/tablex'])('offline, %s shows the offline page', async (path) => {
    await lifecycle('install')
    online = false
    expect(await (await navigate(path))!.text()).toContain('<title>offline</title>')
  })

  it('offline without a cached shell, shows the offline page', async () => {
    network.delete('/')
    await lifecycle('install')
    online = false
    expect(await (await navigate('/table/jobs'))!.text()).toContain('<title>offline</title>')
  })

  it('offline with nothing cached, answers 503 rather than failing', async () => {
    online = false
    const response = await navigate('/table/jobs')
    expect(response!.status).toBe(503)
    expect(response!.headers.get('Content-Type')).toContain('text/plain')
  })
})

describe('precached files', () => {
  it('come fresh from the network and fall back to the cache offline', async () => {
    await lifecycle('install')
    network.set('/manifest.json', () => serverResponse('{"name":"Renamed"}', 'application/manifest+json'))
    expect(await (await request('/manifest.json'))!.text()).toContain('Renamed')
    await settle()

    online = false
    expect(await (await request('/manifest.json'))!.text()).toContain('Renamed')
    expect(await (await request('/icons/icon-192x192.png'))!.text()).toBe('png192')
  })

  it('are not replaced by an HTML page', async () => {
    await lifecycle('install')
    network.set('/manifest.json', () => html())
    await request('/manifest.json')
    await settle()

    online = false
    expect(await (await request('/manifest.json'))!.text()).toContain('SickRock')
  })

  it('fail offline when they were never cached', async () => {
    online = false
    await expect(request('/manifest.json')).rejects.toThrow(/not cached/)
  })
})
