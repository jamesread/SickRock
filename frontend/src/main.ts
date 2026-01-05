import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import './style.css'
import 'femtocrank/style.css'
import 'femtocrank/dark.css'
import router from './router'
import { useAuthStore } from './stores/auth'
import { createApiClient } from './stores/api'
import { create } from '@bufbuild/protobuf'
import { InitRequestSchema } from './gen/sickrock_pb'

const app = createApp(App)
const pinia = createPinia()

app.use(pinia)
app.use(router)

// Initialize auth store
const authStore = useAuthStore()

// Call init API first - this is the first API call the app makes
async function initializeApp() {
  try {
    console.log('Initializing app...')
    const apiClient = createApiClient()
    console.log('Calling init API...')
    const initResponse = await apiClient.init(create(InitRequestSchema, {}))
    console.log('Init API response received:', initResponse)

    // Store init response for reuse in App.vue
    authStore.setInitResponse(initResponse)

    // If currentUsername is empty, user is not authenticated
    if (!initResponse.currentUsername || initResponse.currentUsername === '') {
      // Clear any stale auth state
      authStore.user = null
      // App will show login form via router guard
      // No further API calls will be made
    } else {
      // User is authenticated, validate token from localStorage
      await authStore.validateToken()
    }
  } catch (error) {
    console.error('Failed to initialize app:', error)
    // On error, clear auth state and show login
    authStore.user = null
    // Still set a null initResponse so App.vue knows init was attempted
    authStore.setInitResponse(null as any)
  }
}

// Start initialization and wait for it to complete before mounting
initializeApp()
  .then(() => {
    // Mount app after initialization completes
    console.log('Mounting app...')
    app.mount('#app')
  })
  .catch((error) => {
    console.error('Unhandled error during initialization:', error)
    // Still mount the app even if initialization fails completely
    console.log('Mounting app despite initialization error...')
    app.mount('#app')
  })

// Register service worker for PWA support
// If registration fails, unregister any existing service workers to prevent blank pages
if ('serviceWorker' in navigator) {
  window.addEventListener('load', () => {
    navigator.serviceWorker.register('/sw.js')
      .then((registration) => {
        console.log('SW registered: ', registration);

        // Monitor for service worker errors and updates
        registration.addEventListener('updatefound', () => {
          const newWorker = registration.installing;
          if (newWorker) {
            newWorker.addEventListener('statechange', () => {
              if (newWorker.state === 'redundant') {
                console.warn('Service worker became redundant, unregistering to prevent issues');
                registration.unregister().catch(err => {
                  console.warn('Failed to unregister redundant service worker:', err);
                });
              }
            });
          }
        });
      })
      .catch((registrationError) => {
        console.error('SW registration failed: ', registrationError);
        // Unregister any existing service workers to prevent blank pages
        navigator.serviceWorker.getRegistrations().then((registrations) => {
          registrations.forEach((registration) => {
            console.log('Unregistering existing service worker due to registration failure');
            registration.unregister().catch((err) => {
              console.warn('Failed to unregister service worker:', err);
            });
          });
        }).catch((err) => {
          console.warn('Failed to get service worker registrations:', err);
        });
      });
  });
}
