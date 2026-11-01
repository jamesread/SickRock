// SickRock service worker: makes the app installable and lets table pages open offline.
//
// - Hashed build assets (/assets/*) are cache-first: their names change with their content.
// - Page loads are network-first. Each successful one refreshes the cached app shell (every
//   page is the same index.html). Offline, home and table pages get the shell, which loads
//   table data from IndexedDB; other pages get offline.html.
// - Nothing is cached unless it is a 200 from this site of the expected kind: an HTML page is
//   never stored as a script, stylesheet, image or manifest.
// - API calls, /openapi and /llms.txt always go to the network.
//
// Bump CACHE_VERSION whenever the caching rules change; old caches are deleted on activate.
// frontend/src/sw.test.ts runs this file against a fake browser.

const CACHE_VERSION = 'v8';
const SCOPE_PATH = (() => {
  try {
    const scopeUrl = new URL(self.registration.scope);
    return (scopeUrl.pathname || '/')
      .toLowerCase()
      .replace(/[^a-z0-9/_-]/g, '-')
      .replace(/\/+/g, '/')
      .replace(/^\//, '')
      .replace(/\/$/, '') || 'root';
  } catch {
    return 'root';
  }
})();

const SHELL_CACHE_NAME = `sickrock-shell-${CACHE_VERSION}-${SCOPE_PATH}`;
const ASSET_CACHE_NAME = `sickrock-assets-${CACHE_VERSION}-${SCOPE_PATH}`;
// Per-icon JSON files (/assets/hugeicons/<version>/<Name>.json): versioned paths, cache-first.
const HUGEICONS_CACHE_NAME = `sickrock-hugeicons-${CACHE_VERSION}-${SCOPE_PATH}`;
const CURRENT_CACHES = [SHELL_CACHE_NAME, ASSET_CACHE_NAME, HUGEICONS_CACHE_NAME];

const SHELL_KEY = '/';
const OFFLINE_PAGE = '/offline.html';

// Fetched on install so the app can start offline (the shell is refreshed by every page load).
const PRECACHE = [SHELL_KEY, OFFLINE_PAGE, '/manifest.json', '/icons/icon-192x192.png', '/icons/icon-512x512.png'];

// Pages that open offline from the cached shell. Others (admin, settings) show offline.html.
const OFFLINE_PAGES = ['/', '/table/'];

// Always from the network; the service worker does not touch them.
const BYPASS_PATHS = ['/llms.txt', '/openapi.json', '/openapi', '/metrics'];
const BYPASS_PREFIXES = ['/api/', '/mcp'];

function pathOf(url) {
  return new URL(url).pathname;
}

function shouldBypass(urlPath) {
  return BYPASS_PATHS.includes(urlPath) || BYPASS_PREFIXES.some((prefix) => urlPath.startsWith(prefix));
}

function opensOffline(urlPath) {
  return OFFLINE_PAGES.some((page) => (page === '/' ? urlPath === '/' : urlPath.startsWith(page)));
}

// Icon files only; the picker's catalog.json always comes from the network.
function isHugeiconFile(urlPath) {
  return urlPath.startsWith('/assets/hugeicons/') && urlPath.endsWith('.json') && !urlPath.endsWith('/catalog.json');
}

// Vite build output: name-<8 character hash>.<ext>. Never changes once published.
function isHashedAsset(urlPath) {
  return urlPath.startsWith('/assets/') && /-[A-Za-z0-9_-]{8}\.[a-z0-9]+$/.test(urlPath);
}

function contentType(response) {
  return (response && response.headers && response.headers.get('Content-Type')) || '';
}

function isHTML(response) {
  return contentType(response).includes('text/html');
}

// A response worth caching: a full 200 from this site.
function isCacheable(response) {
  return Boolean(response) && response.status === 200 && response.type === 'basic';
}

function putInCache(cacheName, key, response) {
  return caches.open(cacheName)
    .then((cache) => cache.put(key, response))
    .catch((err) => console.warn(`Service worker could not cache ${key}:`, err));
}

function matchInCache(cacheName, key) {
  return caches.open(cacheName)
    .then((cache) => cache.match(key))
    .catch(() => undefined);
}

// Cache-first for files whose content never changes at a given URL. `accept` decides whether a
// network response may be stored. Network failures pass through to the page.
function cacheFirst(cacheName, request, accept) {
  return matchInCache(cacheName, request.url).then((cached) => {
    if (cached) {
      return cached;
    }
    return fetch(request).then((response) => {
      if (isCacheable(response) && accept(response)) {
        putInCache(cacheName, request.url, response.clone());
      }
      return response;
    });
  });
}

function offlineFallback(urlPath) {
  const shell = opensOffline(urlPath) ? matchInCache(SHELL_CACHE_NAME, SHELL_KEY) : Promise.resolve(undefined);
  return shell
    .then((cached) => cached || matchInCache(SHELL_CACHE_NAME, OFFLINE_PAGE))
    .then((cached) => cached || new Response('SickRock is offline.', {
      status: 503,
      headers: { 'Content-Type': 'text/plain; charset=utf-8' },
    }));
}

// Network-first for page loads. A good page refreshes the cached shell.
function handleNavigation(request) {
  const urlPath = pathOf(request.url);
  return fetch(request)
    .then((response) => {
      if (isCacheable(response) && isHTML(response)) {
        putInCache(SHELL_CACHE_NAME, SHELL_KEY, response.clone());
      }
      return response;
    })
    .catch(() => offlineFallback(urlPath));
}

// Network-first for the precached files (icons, manifest, offline page): fresh when online,
// cached when not.
function handlePrecached(request) {
  return fetch(request)
    .then((response) => {
      const urlPath = pathOf(request.url);
      const expectHTML = urlPath === SHELL_KEY || urlPath === OFFLINE_PAGE;
      if (isCacheable(response) && isHTML(response) === expectHTML) {
        putInCache(SHELL_CACHE_NAME, urlPath, response.clone());
      }
      return response;
    })
    .catch(() => matchInCache(SHELL_CACHE_NAME, pathOf(request.url)).then((cached) => {
      if (cached) {
        return cached;
      }
      throw new TypeError(`Offline and ${request.url} is not cached`);
    }));
}

self.addEventListener('install', (event) => {
  // One missing file must not stop installation; each is cached on its own.
  event.waitUntil(
    caches.open(SHELL_CACHE_NAME)
      .then((cache) => Promise.allSettled(PRECACHE.map((url) => cache.add(url).catch((err) => {
        console.warn(`Service worker could not precache ${url}:`, err);
      }))))
      .catch((err) => console.error('Service worker install error:', err))
      .then(() => self.skipWaiting()),
  );
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys()
      .then((names) => Promise.allSettled(names
        .filter((name) => name.startsWith('sickrock-') && name.endsWith(`-${SCOPE_PATH}`) && !CURRENT_CACHES.includes(name))
        .map((name) => caches.delete(name))))
      .catch((err) => console.warn('Service worker could not delete old caches:', err))
      .then(() => self.clients.claim()),
  );
});

self.addEventListener('fetch', (event) => {
  const request = event.request;
  if (request.method !== 'GET' || !request.url.startsWith('http')) {
    return;
  }
  const url = new URL(request.url);
  if (url.origin !== self.location.origin || shouldBypass(url.pathname)) {
    return;
  }

  if (isHugeiconFile(url.pathname)) {
    event.respondWith(cacheFirst(HUGEICONS_CACHE_NAME, request, (response) => contentType(response).includes('application/json')));
    return;
  }
  if (isHashedAsset(url.pathname)) {
    event.respondWith(cacheFirst(ASSET_CACHE_NAME, request, (response) => !isHTML(response)));
    return;
  }
  if (request.mode === 'navigate') {
    event.respondWith(handleNavigation(request));
    return;
  }
  if (PRECACHE.includes(url.pathname)) {
    event.respondWith(handlePrecached(request));
  }
  // Anything else goes straight to the network.
});
