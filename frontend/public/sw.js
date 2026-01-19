const CACHE_NAME = 'sickrock-v6';
const STATIC_CACHE_NAME = 'sickrock-static-v6';

// Routes that should be cached for offline use (table views only)
const CACHEABLE_ROUTES = [
  '/',
  '/table/'
];

// Routes that should NOT be cached (admin/control panel)
const EXCLUDED_ROUTES = [
  '/admin/',
  '/control-panel',
  '/user-preferences',
  '/dashboards',
  '/workflow/',
  '/device-code-claimer'
];

// Check if a URL should be cached
function shouldCache(url) {
  const urlPath = new URL(url).pathname;

  // Don't cache excluded routes
  if (EXCLUDED_ROUTES.some(route => urlPath.startsWith(route))) {
    return false;
  }

  // Don't cache API requests (we use IndexedDB for table data)
  if (urlPath.startsWith('/api/')) {
    return false;
  }

  // Cache table routes and home
  if (urlPath === '/' || urlPath.startsWith('/table/')) {
    return true;
  }

  // Don't cache other routes
  return false;
}

// Install event - cache essential resources
self.addEventListener('install', (event) => {
  console.log('Service Worker installing...');

  // Use a more resilient approach - don't block installation on errors
  const installPromise = caches.open(STATIC_CACHE_NAME)
    .then((cache) => {
      console.log('Opened cache');
      // Cache resources individually to prevent one failure from blocking installation
      const resourcesToCache = [
        '/',
        '/manifest.json',
        '/offline.html',
        '/icons/icon-192x192.png',
        '/icons/icon-512x512.png'
      ];

      // Cache each resource individually, logging failures but not blocking installation
      return Promise.allSettled(
        resourcesToCache.map(url =>
          cache.add(url).catch(err => {
            console.warn(`Failed to cache ${url}:`, err);
            return null; // Don't throw, just log the error
          })
        )
      );
    })
    .then(() => {
      console.log('Service Worker installed');
      return self.skipWaiting();
    })
    .catch((error) => {
      console.error('Service Worker installation error:', error);
      // Still skip waiting to allow activation even if caching fails
      // This ensures the service worker doesn't block the app
      return self.skipWaiting();
    });

  // Use waitUntil but ensure it doesn't block if there's an error
  event.waitUntil(
    installPromise.catch((error) => {
      console.error('Critical service worker installation error:', error);
      // Even if everything fails, skip waiting so the app can continue
      return self.skipWaiting();
    })
  );
});

// Activate event - clean up old caches
self.addEventListener('activate', (event) => {
  console.log('Service Worker activating...');

  const activatePromise = caches.keys()
    .then((cacheNames) => {
      return Promise.allSettled(
        cacheNames.map((cacheName) => {
          if (cacheName !== CACHE_NAME && cacheName !== STATIC_CACHE_NAME) {
            console.log('Deleting old cache:', cacheName);
            return caches.delete(cacheName).catch(err => {
              console.warn(`Failed to delete cache ${cacheName}:`, err);
              return null; // Don't throw, just log
            });
          }
          return Promise.resolve();
        })
      );
    })
    .then(() => {
      console.log('Service Worker activated');
      return self.clients.claim();
    })
    .catch((error) => {
      console.error('Service Worker activation error:', error);
      // Still try to claim clients even if cache cleanup fails
      return self.clients.claim().catch(err => {
        console.warn('Failed to claim clients:', err);
        return null;
      });
    });

  event.waitUntil(
    activatePromise.catch((error) => {
      console.error('Critical service worker activation error:', error);
      // Don't block activation even on errors
      return Promise.resolve();
    })
  );
});

// Fetch event - serve from cache when offline
self.addEventListener('fetch', (event) => {
  // Skip non-GET requests
  if (event.request.method !== 'GET') {
    return;
  }

  // Skip chrome-extension and other non-http requests
  if (!event.request.url.startsWith('http')) {
    return;
  }

  const url = new URL(event.request.url);

  // Don't cache API requests - table data is handled via IndexedDB
  if (url.pathname.startsWith('/api/')) {
    return;
  }

  // Handle navigation requests (page loads)
  if (event.request.mode === 'navigate') {
    // Wrap in try-catch to gracefully handle any errors
    try {
      event.respondWith(
        fetch(event.request)
          .then((response) => {
            // Cache successful responses
            if (response && response.status === 200 && response.type === 'basic') {
              const responseToCache = response.clone();
              caches.open(CACHE_NAME)
                .then((cache) => {
                  cache.put(event.request, responseToCache).catch(err => {
                    console.warn('Failed to cache navigation response:', err);
                  });
                })
                .catch(err => {
                  console.warn('Failed to open cache for navigation:', err);
                });
            }
            return response;
          })
          .catch(() => {
            // When offline, try to serve cached version of the page
            return caches.match(event.request)
              .then((cachedResponse) => {
                if (cachedResponse) {
                  return cachedResponse;
                }
                // If no cached version, try to serve the app shell (index.html) from both caches
                return Promise.all([
                  caches.open(CACHE_NAME).then(cache => cache.match('/')).catch(() => null),
                  caches.open(STATIC_CACHE_NAME).then(cache => cache.match('/')).catch(() => null)
                ]).then(([cached1, cached2]) => {
                  if (cached1) return cached1;
                  if (cached2) return cached2;
                  // Last resort: serve offline.html
                  return caches.match('/offline.html').catch(() => null);
                }).catch(() => {
                  // If all caching fails, fetch the request normally (don't cache)
                  console.warn('Service worker cache failed, fetching normally');
                  return fetch(event.request);
                });
              })
              .catch(() => {
                // If cache matching fails completely, fetch the request normally
                console.warn('Service worker cache match failed, fetching normally');
                return fetch(event.request);
              });
          })
          .catch(() => {
            // Final fallback: fetch the request normally without caching
            console.warn('Service worker fetch handler failed, fetching normally');
            return fetch(event.request);
          })
      );
    } catch (error) {
      // If respondWith itself fails, log and let the browser handle it
      console.error('Service worker respondWith failed:', error);
      // Don't call respondWith, let browser handle the request
    }
    return;
  }

  // Only cache specific routes (table views, not admin/control panel)
  if (!shouldCache(event.request.url)) {
    return;
  }

  // Wrap in try-catch to gracefully handle any errors
  try {
    event.respondWith(
      caches.match(event.request)
        .then((response) => {
          // Return cached version if available
          if (response) {
            return response;
          }

          // Otherwise fetch from network
          return fetch(event.request)
            .then((response) => {
              // Don't cache if not a valid response
              if (!response || response.status !== 200 || response.type !== 'basic') {
                return response;
              }

              // Clone the response
              const responseToCache = response.clone();

              // Cache the response (only for cacheable routes)
              if (shouldCache(event.request.url)) {
                caches.open(CACHE_NAME)
                  .then((cache) => {
                    cache.put(event.request, responseToCache).catch(err => {
                      console.warn('Failed to cache response:', err);
                    });
                  })
                  .catch(err => {
                    console.warn('Failed to open cache:', err);
                  });
              }

              return response;
            })
            .catch(() => {
              // For other requests, try cached version if available
              return caches.match(event.request).catch(() => {
                // If cache match fails, fetch normally
                console.warn('Service worker cache match failed, fetching normally');
                return fetch(event.request);
              });
            });
        })
        .catch(() => {
          // If cache match fails, fetch the request normally
          console.warn('Service worker cache match failed, fetching normally');
          return fetch(event.request);
        })
    );
  } catch (error) {
    // If respondWith itself fails, log and let the browser handle it
    console.error('Service worker respondWith failed:', error);
    // Don't call respondWith, let browser handle the request
  }
});
