import assert from 'node:assert'
import { Builder, By, until } from 'selenium-webdriver'
import chrome from 'selenium-webdriver/chrome.js'

const base = process.env.SICKROCK_BASE_URL || 'http://127.0.0.1:18080'

// The served manifest, its files and Chrome's own verdict on installing SickRock.
describe('SickRock PWA', function () {
  this.timeout(60000)
  let driver
  let manifest

  before(async function () {
    const response = await fetch(base + '/manifest.json')
    assert.strictEqual(response.status, 200)
    assert.strictEqual(response.headers.get('content-type'), 'application/manifest+json')
    manifest = await response.json()

    const options = new chrome.Options()
    options.addArguments('--headless=new', '--no-sandbox', '--disable-dev-shm-usage')
    driver = await new Builder().forBrowser('chrome').setChromeOptions(options).build()
  })

  after(async function () {
    if (driver) {
      await driver.quit()
    }
  })

  it('serves a manifest named after the instance, revalidated on every use', async function () {
    assert.strictEqual(manifest.name, 'SickRock')
    assert.strictEqual(manifest.short_name, 'SickRock')
    assert.strictEqual(manifest.start_url, '/')
    assert.strictEqual(manifest.display, 'standalone')
    const response = await fetch(base + '/manifest.json', { method: 'HEAD' })
    assert.strictEqual(response.headers.get('cache-control'), 'no-cache')
  })

  it('serves every icon and screenshot the manifest lists', async function () {
    const images = [...manifest.icons, ...(manifest.screenshots || [])]
    assert.ok(images.length > 0)
    for (const image of images) {
      const response = await fetch(base + image.src)
      assert.strictEqual(response.status, 200, `${image.src} status`)
      assert.strictEqual(response.headers.get('content-type'), image.type, `${image.src} type`)
      const bytes = new Uint8Array(await response.arrayBuffer())
      // PNG signature, then the IHDR width and height.
      assert.deepStrictEqual([...bytes.slice(1, 4)], [0x50, 0x4e, 0x47], `${image.src} is not a PNG`)
      const view = new DataView(bytes.buffer)
      assert.strictEqual(`${view.getUint32(16)}x${view.getUint32(20)}`, image.sizes, `${image.src} size`)
    }
  })

  it('serves the service worker uncached, and missing files as 404s', async function () {
    const sw = await fetch(base + '/sw.js')
    assert.strictEqual(sw.status, 200)
    assert.match(sw.headers.get('content-type'), /javascript/)
    assert.strictEqual(sw.headers.get('cache-control'), 'no-cache')
    assert.strictEqual(sw.headers.get('service-worker-allowed'), '/')

    for (const path of ['/manifest.template.json', '/missing.js', '/assets/index-Missing1.js', '/icons/missing.png']) {
      const response = await fetch(base + path)
      assert.strictEqual(response.status, 404, `${path} status`)
    }
    const page = await fetch(base + '/table/anything')
    assert.match(page.headers.get('content-type'), /text\/html/)
    assert.strictEqual(page.headers.get('cache-control'), 'no-cache')
  })

  it('is installable according to Chrome', async function () {
    await driver.get(base + '/login')
    await driver.wait(until.elementLocated(By.css('body')), 10000)

    const appManifest = await driver.sendAndGetDevToolsCommand('Page.getAppManifest', {})
    assert.strictEqual(new URL(appManifest.url).pathname, '/manifest.json')
    assert.deepStrictEqual(appManifest.errors, [], 'manifest parse errors')

    const installability = await driver.sendAndGetDevToolsCommand('Page.getInstallabilityErrors', {})
    assert.deepStrictEqual(installability.installabilityErrors, [], 'installability errors')
  })

  it('installs its service worker and controls the page', async function () {
    await driver.get(base + '/login')
    const state = await driver.executeAsyncScript(`
      const done = arguments[arguments.length - 1]
      navigator.serviceWorker.ready
        .then(async (registration) => {
          // A reload is the first load the active worker controls.
          if (!navigator.serviceWorker.controller) {
            await new Promise((resolve) => navigator.serviceWorker.addEventListener('controllerchange', resolve, { once: true }))
          }
          const cacheNames = await caches.keys()
          const shell = cacheNames.find((name) => name.startsWith('sickrock-shell-'))
          const cached = shell ? (await (await caches.open(shell)).keys()).map((r) => new URL(r.url).pathname) : []
          done({ scope: registration.scope, controlled: Boolean(navigator.serviceWorker.controller), cached })
        })
        .catch((err) => done({ error: String(err) }))
    `)
    assert.ok(!state.error, state.error)
    assert.strictEqual(new URL(state.scope).pathname, '/')
    assert.ok(state.controlled, 'page is not controlled by the service worker')
    for (const path of ['/', '/offline.html', '/manifest.json', '/icons/icon-192x192.png', '/icons/icon-512x512.png']) {
      assert.ok(state.cached.includes(path), `${path} not precached (cached: ${state.cached.join(', ')})`)
    }
  })
})
