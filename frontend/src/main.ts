import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import './style.css'
import 'femtocrank/style.css'
import 'femtocrank/dark.css'
import 'picocrank/styles.css'
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
    const apiClient = createApiClient()
    const initResponse = await apiClient.init(create(InitRequestSchema, {}))

    // Store init response for reuse in App.vue
    authStore.setInitResponse(initResponse)

    if (!initResponse.currentUsername || initResponse.currentUsername === '') {
      authStore.user = null
    } else {
      authStore.setUserFromInit(initResponse)
      const token = localStorage.getItem('session-token')
      if (token) {
        await authStore.validateToken()
      }
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
    app.mount('#app')
  })
  .catch((error) => {
    console.error('Unhandled error during initialization:', error)
    // Still mount the app even if initialization fails completely
    app.mount('#app')
  })

// Register service worker for PWA support
// If registration fails, unregister any existing service workers to prevent blank pages
if ('serviceWorker' in navigator) {
  window.addEventListener('load', () => {
    // updateViaCache 'none': always check /sw.js itself for updates, bypassing the HTTP cache.
    // A new worker that fails to install becomes "redundant" and the current one keeps
    // working, so that needs no handling here.
    navigator.serviceWorker.register('/sw.js', { updateViaCache: 'none' })
      .catch((registrationError) => {
        console.error('SW registration failed: ', registrationError);
        // Unregister any existing service workers to prevent blank pages
        navigator.serviceWorker.getRegistrations().then((registrations) => {
          registrations.forEach((registration) => {
            console.warn('Unregistering existing service worker due to registration failure');
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
