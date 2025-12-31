const CACHE_NAME = 'sickrock-v3';
const STATIC_CACHE_NAME = 'sickrock-static-v3';

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
  event.waitUntil(
    caches.open(STATIC_CACHE_NAME)
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
        return self.skipWaiting();
      })
  );
});

// Activate event - clean up old caches
self.addEventListener('activate', (event) => {
  console.log('Service Worker activating...');
  event.waitUntil(
    caches.keys().then((cacheNames) => {
      return Promise.all(
        cacheNames.map((cacheName) => {
          if (cacheName !== CACHE_NAME && cacheName !== STATIC_CACHE_NAME) {
            console.log('Deleting old cache:', cacheName);
            return caches.delete(cacheName);
          }
        })
      );
    }).then(() => {
      console.log('Service Worker activated');
      return self.clients.claim();
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

  // Only cache specific routes (table views, not admin/control panel)
  if (!shouldCache(event.request.url)) {
    return;
  }

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
                  cache.put(event.request, responseToCache);
                });
            }

            return response;
          })
          .catch(() => {
            // Return offline page for navigation requests when offline
            if (event.request.mode === 'navigate') {
              return caches.match('/offline.html') || caches.match('/');
            }
            // For other requests, return cached version if available
            return caches.match(event.request);
          });
      })
  );
});
