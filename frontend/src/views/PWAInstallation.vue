<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { usePWAInstall } from '../composables/usePWAInstall'
import { HugeiconsIcon } from '@hugeicons/vue'
import * as Hugeicons from '@hugeicons/core-free-icons'
import { Download01Icon, CheckmarkSquare03Icon, QuestionIcon } from '@hugeicons/core-free-icons'
import Section from 'picocrank/vue/components/Section.vue'

// PWA Install
const { isInstallable, isInstalled, promptInstall } = usePWAInstall()
const installingPWA = ref(false)

// Service Worker Status
interface ServiceWorkerStatus {
  registered: boolean
  state: string | null
  version: string | null
  scope: string | null
  error: string | null
}

const swStatus = ref<ServiceWorkerStatus>({
  registered: false,
  state: null,
  version: null,
  scope: null,
  error: null
})

async function checkServiceWorkerStatus() {
  if (!('serviceWorker' in navigator)) {
    swStatus.value = {
      registered: false,
      state: null,
      version: null,
      scope: null,
      error: 'Service workers are not supported in this browser.'
    }
    return
  }

  try {
    const registration = await navigator.serviceWorker.getRegistration()

    if (!registration) {
      swStatus.value = {
        registered: false,
        state: null,
        version: null,
        scope: null,
        error: 'Service worker is not registered.'
      }
      return
    }

    // Get the active service worker
    const activeWorker = registration.active
    const installingWorker = registration.installing
    const waitingWorker = registration.waiting

    let worker = activeWorker || waitingWorker || installingWorker
    let state = worker?.state || 'unknown'

    // Extract version from cache name
    let version: string | null = null
    try {
      const cacheNames = await caches.keys()
      // Look for cache names matching our pattern (sickrock-v2, sickrock-static-v2)
      // Prefer the main cache over static cache
      const mainCache = cacheNames.find(name => /^sickrock-v\d+$/.test(name))
      const staticCache = cacheNames.find(name => /^sickrock-static-v\d+$/.test(name))
      const cacheName = mainCache || staticCache

      if (cacheName) {
        // Extract version (e.g., "v2" from "sickrock-v2" or "sickrock-static-v2")
        const match = cacheName.match(/sickrock-(?:static-)?(v?\d+)/)
        if (match) {
          version = match[1]
        } else {
          version = 'unknown'
        }
      }
    } catch (e) {
      console.warn('Failed to get cache version:', e)
    }

    swStatus.value = {
      registered: true,
      state: state,
      version: version,
      scope: registration.scope,
      error: null
    }
  } catch (error: any) {
    swStatus.value = {
      registered: false,
      state: null,
      version: null,
      scope: null,
      error: error?.message || 'Failed to check service worker status.'
    }
  }
}

async function handlePWAInstall() {
  installingPWA.value = true
  try {
    await promptInstall()
  } catch (error) {
    console.error('Failed to install PWA:', error)
  } finally {
    installingPWA.value = false
  }
}

interface PWADiagnostic {
  name: string
  status: 'pass' | 'fail' | 'warning'
  message: string
  details?: string
}

const diagnostics = ref<PWADiagnostic[]>([])
const showDiagnostics = ref(false)

async function runPWADiagnostics(): Promise<PWADiagnostic[]> {
  const results: PWADiagnostic[] = []

  // 1. Check secure context (window.isSecureContext)
  const isSecureContext = window.isSecureContext === true
  const isSecureProtocol = location.protocol === 'https:' ||
                           location.hostname === 'localhost' ||
                           location.hostname === '127.0.0.1' ||
                           location.hostname.includes('localhost')
  results.push({
    name: 'Secure Context (window.isSecureContext)',
    status: isSecureContext ? 'pass' : 'fail',
    message: isSecureContext
      ? 'Window is in a secure context'
      : 'Window is not in a secure context',
    details: `window.isSecureContext: ${window.isSecureContext}, Protocol: ${location.protocol}, Hostname: ${location.hostname}`
  })

  // 2. Check HTTPS/secure protocol
  results.push({
    name: 'Secure Protocol (HTTPS)',
    status: isSecureProtocol ? 'pass' : 'fail',
    message: isSecureProtocol
      ? 'App is served over HTTPS or localhost'
      : 'App must be served over HTTPS (or localhost)',
    details: `Protocol: ${location.protocol}, Hostname: ${location.hostname}`
  })

  // 3. Check service worker support
  const hasServiceWorkerSupport = 'serviceWorker' in navigator
  results.push({
    name: 'Service Worker Support',
    status: hasServiceWorkerSupport ? 'pass' : 'fail',
    message: hasServiceWorkerSupport
      ? 'Browser supports service workers'
      : 'Browser does not support service workers',
    details: hasServiceWorkerSupport ? undefined : 'Required for PWA installation'
  })

  // 4. Check service worker registration
  let swRegistered = false
  let swState = 'unknown'
  try {
    const registration = await navigator.serviceWorker.getRegistration()
    swRegistered = !!registration
    if (registration) {
      const worker = registration.active || registration.waiting || registration.installing
      swState = worker?.state || 'unknown'
    }
  } catch (e) {
    // Already handled
  }
  results.push({
    name: 'Service Worker Registered',
    status: swRegistered ? 'pass' : 'fail',
    message: swRegistered
      ? `Service worker is registered (state: ${swState})`
      : 'Service worker is not registered',
    details: swRegistered ? `Current state: ${swState}` : 'Service worker must be registered for PWA installation'
  })

  // 5. Check manifest link
  const manifestLink = document.querySelector('link[rel="manifest"]')
  const manifestHref = manifestLink?.getAttribute('href')
  results.push({
    name: 'Manifest Link',
    status: manifestLink ? 'pass' : 'fail',
    message: manifestLink
      ? 'Web app manifest is linked'
      : 'Web app manifest is missing or not linked',
    details: manifestHref ? `Manifest URL: ${manifestHref}` : undefined
  })

  // 6. Check manifest file
  let manifestValid = false
  let manifestData: any = null
  let manifestError = ''
  if (manifestHref) {
    try {
      const response = await fetch(manifestHref)
      if (response.ok) {
        manifestData = await response.json()
        manifestValid = true
      } else {
        manifestError = `HTTP ${response.status}: ${response.statusText}`
      }
    } catch (e: any) {
      manifestError = e.message || 'Failed to fetch manifest'
    }
  }
  results.push({
    name: 'Manifest File',
    status: manifestValid ? 'pass' : 'fail',
    message: manifestValid
      ? 'Manifest file is accessible and valid'
      : `Manifest file error: ${manifestError || 'Not found'}`,
    details: manifestData ? `Name: ${manifestData.name || 'N/A'}, Short name: ${manifestData.short_name || 'N/A'}` : undefined
  })

  // 7. Check manifest properties
  if (manifestData) {
    // Check required fields
    const hasName = !!manifestData.name || !!manifestData.short_name
    results.push({
      name: 'Manifest: Name',
      status: hasName ? 'pass' : 'fail',
      message: hasName
        ? `App name: ${manifestData.name || manifestData.short_name}`
        : 'Manifest missing required "name" or "short_name"',
    })

    // Check icons
    const hasIcons = Array.isArray(manifestData.icons) && manifestData.icons.length > 0
    const iconSizes = manifestData.icons?.map((i: any) => i.sizes).filter(Boolean) || []
    results.push({
      name: 'Manifest: Icons',
      status: hasIcons ? 'pass' : 'fail',
      message: hasIcons
        ? `Icons configured (${manifestData.icons.length} icons)`
        : 'Manifest missing icons',
      details: iconSizes.length > 0 ? `Available sizes: ${iconSizes.join(', ')}` : undefined
    })

    // Check start_url
    const hasStartUrl = !!manifestData.start_url
    results.push({
      name: 'Manifest: Start URL',
      status: hasStartUrl ? 'pass' : 'warning',
      message: hasStartUrl
        ? `Start URL: ${manifestData.start_url}`
        : 'Manifest missing "start_url" (optional but recommended)',
    })

    // Check display mode
    const displayMode = manifestData.display || 'browser'
    results.push({
      name: 'Manifest: Display Mode',
      status: displayMode !== 'browser' ? 'pass' : 'warning',
      message: `Display mode: ${displayMode}`,
      details: displayMode === 'standalone' || displayMode === 'fullscreen'
        ? 'Recommended for PWA'
        : 'Consider using "standalone" or "fullscreen"'
    })
  }

  // 8. Check browser support
  const userAgent = navigator.userAgent
  const isChrome = /Chrome/.test(userAgent) && !/Edg/.test(userAgent)
  const isEdge = /Edg/.test(userAgent)
  const isFirefox = /Firefox/.test(userAgent)
  const isSafari = /Safari/.test(userAgent) && !/Chrome/.test(userAgent)
  const isIOS = /iPad|iPhone|iPod/.test(userAgent)
  const isAndroid = /Android/.test(userAgent)

  const browserSupport = isChrome || isEdge || isFirefox || (isSafari && isIOS) || (isChrome && isAndroid)
  results.push({
    name: 'Browser Support',
    status: browserSupport ? 'pass' : 'warning',
    message: browserSupport
      ? `Supported browser detected (${isChrome ? 'Chrome' : isEdge ? 'Edge' : isFirefox ? 'Firefox' : isSafari ? 'Safari' : 'Other'})`
      : 'Browser may not fully support PWA installation',
    details: isIOS && !isSafari
      ? 'iOS requires Safari browser for PWA installation'
      : undefined
  })

  // 9. Check display mode (if installed)
  const isStandalone = window.matchMedia('(display-mode: standalone)').matches
  const isFullscreen = window.matchMedia('(display-mode: fullscreen)').matches
  if (isStandalone || isFullscreen) {
    results.push({
      name: 'App Installation Status',
      status: 'pass',
      message: 'App is already installed',
      details: `Running in ${isStandalone ? 'standalone' : 'fullscreen'} mode`
    })
  }

  // 10. Check if beforeinstallprompt event is available
  results.push({
    name: 'Install Prompt Available',
    status: isInstallable.value ? 'pass' : 'warning',
    message: isInstallable.value
      ? 'Browser install prompt is available'
      : 'Browser install prompt not yet available',
    details: !isInstallable.value
      ? 'The beforeinstallprompt event may not have fired yet, or installation criteria may not be met'
      : undefined
  })

  return results
}

async function loadDiagnostics() {
  diagnostics.value = await runPWADiagnostics()
}

function getPWAInstallStatus() {
  if (isInstalled.value) {
    return {
      canInstall: false,
      reason: 'already-installed',
      message: 'SickRock is already installed as an app on this device.'
    }
  }

  if (isInstallable.value) {
    return {
      canInstall: true,
      reason: null,
      message: null
    }
  }

  // Check various reasons why installation might not be possible
  const reasons: string[] = []

  // Check HTTPS
  if (location.protocol !== 'https:' && !location.hostname.includes('localhost') && !location.hostname.includes('127.0.0.1')) {
    reasons.push('The app must be served over HTTPS (or localhost) to be installable.')
  }

  // Check service worker support
  if (!('serviceWorker' in navigator)) {
    reasons.push('Your browser does not support service workers, which are required for PWA installation.')
  }

  // Check manifest
  const manifestLink = document.querySelector('link[rel="manifest"]')
  if (!manifestLink) {
    reasons.push('Web app manifest is missing or not linked.')
  }

  // Check browser support
  const isIOS = /iPad|iPhone|iPod/.test(navigator.userAgent)
  const isSafari = /^((?!chrome|android).)*safari/i.test(navigator.userAgent)
  if (isIOS && !isSafari) {
    reasons.push('iOS requires Safari browser for PWA installation.')
  }

  // Generic fallback
  if (reasons.length === 0) {
    reasons.push('PWA installation is not available. This may be due to browser limitations or the app not meeting installation criteria.')
  }

  return {
    canInstall: false,
    reason: 'not-available',
    message: reasons.join(' '),
    reasons
  }
}

function formatSWState(state: string | null): string {
  if (!state) return 'Unknown'
  const stateMap: Record<string, string> = {
    'installing': 'Installing',
    'installed': 'Installed',
    'activating': 'Activating',
    'activated': 'Active',
    'redundant': 'Redundant'
  }
  return stateMap[state] || state.charAt(0).toUpperCase() + state.slice(1)
}

const pwaStatus = computed(() => getPWAInstallStatus())

onMounted(async () => {
  await Promise.all([
    checkServiceWorkerStatus(),
    loadDiagnostics()
  ])

  // Refresh service worker status periodically
  setInterval(() => {
    checkServiceWorkerStatus()
  }, 5000) // Check every 5 seconds
})
</script>

<template>
  <Section title="PWA & Service Worker">
    <div class="pwa-section">
      <!-- Service Worker Status -->
      <div class="sw-status">
        <h4>Service Worker Status</h4>
        <div v-if="swStatus.registered" class="sw-status-info registered">
          <div class="sw-status-item">
            <strong>Status:</strong>
            <span class="sw-state" :class="swStatus.state">{{ formatSWState(swStatus.state) }}</span>
          </div>
          <div v-if="swStatus.version" class="sw-status-item">
            <strong>Version:</strong> {{ swStatus.version }}
          </div>
          <div v-if="swStatus.scope" class="sw-status-item">
            <strong>Scope:</strong> <code>{{ swStatus.scope }}</code>
          </div>
        </div>
        <div v-else class="sw-status-info not-registered">
          <div class="sw-status-item">
            <strong>Status:</strong> Not Registered
          </div>
          <div v-if="swStatus.error" class="sw-status-item sw-error">
            <strong>Error:</strong> {{ swStatus.error }}
          </div>
        </div>
      </div>

      <!-- PWA Installation Status -->
      <div class="pwa-install-section">
        <h4>App Installation</h4>

        <div v-if="isInstalled" class="pwa-status installed">
          <div class="pwa-status-icon">
            <HugeiconsIcon :icon="Hugeicons.CheckmarkSquare03Icon" />
          </div>
          <div class="pwa-status-content">
            <h5>App Installed</h5>
            <p>SickRock is installed as an app on this device. You can use it offline and access it from your home screen.</p>
          </div>
        </div>

        <div v-else-if="pwaStatus.canInstall" class="pwa-status installable">
          <div class="pwa-status-icon">
            <HugeiconsIcon :icon="Hugeicons.Download01Icon" />
          </div>
          <div class="pwa-status-content">
            <h5>Install SickRock</h5>
            <p>Install SickRock as an app for a better experience, offline access, and quick launch from your home screen.</p>
            <button
              @click="handlePWAInstall"
              :disabled="installingPWA"
              class="pwa-install-button"
            >
              <HugeiconsIcon :icon="Hugeicons.Download01Icon" />
              {{ installingPWA ? 'Installing...' : 'Install App' }}
            </button>
          </div>
        </div>

          <div v-else class="pwa-status not-available">
            <div class="pwa-status-icon">
              <HugeiconsIcon :icon="Hugeicons.QuestionIcon" />
            </div>
            <div class="pwa-status-content">
              <h5>Installation Not Available</h5>
              <p>PWA installation is not currently available. Possible reasons:</p>
              <ul class="pwa-reasons-list">
                <li v-for="(reason, index) in pwaStatus.reasons" :key="index">
                  {{ reason }}
                </li>
              </ul>
              <div class="pwa-help">
                <p><strong>To enable installation:</strong></p>
                <ul>
                  <li>Make sure you're using a modern browser (Chrome, Edge, Safari, Firefox)</li>
                  <li>Ensure the app is served over HTTPS (or localhost for development)</li>
                  <li>Check that your browser supports PWA installation</li>
                  <li>On iOS, use Safari browser</li>
                </ul>
              </div>
            </div>
          </div>
        </div>

      <!-- Detailed Diagnostics -->
      <div class="diagnostics-section">
        <div class="diagnostics-header">
          <h4>Detailed Diagnostics</h4>
          <button
            @click="showDiagnostics = !showDiagnostics; if (showDiagnostics && diagnostics.length === 0) loadDiagnostics()"
            class="diagnostics-toggle"
          >
            {{ showDiagnostics ? 'Hide' : 'Show' }} Diagnostics
          </button>
        </div>

        <div v-if="showDiagnostics" class="diagnostics-content">
          <div v-if="diagnostics.length === 0" class="diagnostics-loading">
            Loading diagnostics...
          </div>
          <div v-else class="diagnostics-list">
            <div
              v-for="(diagnostic, index) in diagnostics"
              :key="index"
              class="diagnostic-item"
              :class="diagnostic.status"
            >
              <div class="diagnostic-header">
                <span class="diagnostic-status-icon">
                  <span v-if="diagnostic.status === 'pass'">✓</span>
                  <span v-else-if="diagnostic.status === 'fail'">✗</span>
                  <span v-else>⚠</span>
                </span>
                <strong class="diagnostic-name">{{ diagnostic.name }}</strong>
              </div>
              <div class="diagnostic-message">{{ diagnostic.message }}</div>
              <div v-if="diagnostic.details" class="diagnostic-details">
                {{ diagnostic.details }}
              </div>
            </div>
          </div>
          <button
            @click="loadDiagnostics()"
            class="refresh-diagnostics-button"
          >
            Refresh Diagnostics
          </button>
        </div>
      </div>
    </div>
  </Section>
</template>

<style scoped>
.pwa-section {
  display: flex;
  flex-direction: column;
  gap: 2rem;
}

/* Service Worker Status */
.sw-status {
  padding: 1rem;
  background: white;
  border: 1px solid #e9ecef;
  border-radius: 6px;
}

.sw-status h4 {
  margin: 0 0 1rem 0;
  color: #333;
  font-size: 1rem;
}

.sw-status-info {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.sw-status-info.registered {
  border-left: 3px solid #28a745;
  padding-left: 0.75rem;
}

.sw-status-info.not-registered {
  border-left: 3px solid #dc3545;
  padding-left: 0.75rem;
}

.sw-status-item {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  color: #666;
  font-size: 0.9rem;
}

.sw-status-item strong {
  color: #333;
  min-width: 80px;
}

.sw-status-item code {
  background: #f8f9fa;
  padding: 0.25rem 0.5rem;
  border-radius: 3px;
  font-size: 0.85rem;
  word-break: break-all;
}

.sw-state {
  padding: 0.25rem 0.5rem;
  border-radius: 3px;
  font-size: 0.85rem;
  font-weight: 500;
}

.sw-state.installing {
  background: #fff3cd;
  color: #856404;
}

.sw-state.installed {
  background: #d1ecf1;
  color: #0c5460;
}

.sw-state.activating {
  background: #fff3cd;
  color: #856404;
}

.sw-state.activated {
  background: #d4edda;
  color: #155724;
}

.sw-state.redundant {
  background: #f8d7da;
  color: #721c24;
}

.sw-error {
  color: #dc3545;
}

/* PWA Installation Section */
.pwa-install-section {
  padding: 1rem;
  background: #f8f9fa;
  border: 1px solid #e9ecef;
  border-radius: 6px;
}

.pwa-install-section h4 {
  margin: 0 0 1.5rem 0;
  color: #333;
}

.pwa-status {
  display: flex;
  gap: 1rem;
  padding: 1rem;
  background: white;
  border: 1px solid #e9ecef;
  border-radius: 6px;
}

.pwa-status.installed {
  border-color: #28a745;
  background: #d4edda;
}

.pwa-status.installable {
  border-color: #007bff;
  background: #e7f3ff;
}

.pwa-status.not-available {
  border-color: #ffc107;
  background: #fff3cd;
}

.pwa-status-icon {
  flex-shrink: 0;
  width: 48px;
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
}

.pwa-status.installed .pwa-status-icon {
  background: #28a745;
  color: white;
}

.pwa-status.installable .pwa-status-icon {
  background: #007bff;
  color: white;
}

.pwa-status.not-available .pwa-status-icon {
  background: #ffc107;
  color: #212529;
}

.pwa-status-icon svg {
  width: 24px;
  height: 24px;
}

.pwa-status-content {
  flex: 1;
  min-width: 0;
}

.pwa-status-content h5 {
  margin: 0 0 0.5rem 0;
  color: #333;
  font-size: 1.1rem;
}

.pwa-status-content p {
  margin: 0 0 1rem 0;
  color: #666;
  line-height: 1.5;
}

.pwa-install-button {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.75rem 1.5rem;
  background: #007bff;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-weight: 500;
  transition: background-color 0.2s ease;
}

.pwa-install-button:hover:not(:disabled) {
  background: #0056b3;
}

.pwa-install-button:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.pwa-install-button svg {
  width: 18px;
  height: 18px;
}

.pwa-reasons-list {
  margin: 0.75rem 0;
  padding-left: 1.5rem;
  color: #666;
}

.pwa-reasons-list li {
  margin-bottom: 0.5rem;
  line-height: 1.5;
}

.pwa-help {
  margin-top: 1rem;
  padding: 1rem;
  background: rgba(255, 255, 255, 0.7);
  border-radius: 4px;
}

.pwa-help p {
  margin: 0 0 0.5rem 0;
  font-weight: 500;
  color: #333;
}

.pwa-help ul {
  margin: 0.5rem 0 0 0;
  padding-left: 1.5rem;
  color: #666;
}

.pwa-help ul li {
  margin-bottom: 0.25rem;
  line-height: 1.5;
}

/* Diagnostics Section */
.diagnostics-section {
  margin-top: 2rem;
  padding: 1rem;
  background: #f8f9fa;
  border: 1px solid #e9ecef;
  border-radius: 6px;
}

.diagnostics-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1rem;
}

.diagnostics-header h4 {
  margin: 0;
  color: #333;
  font-size: 1rem;
}

.diagnostics-toggle {
  padding: 0.5rem 1rem;
  background: #007bff;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.9rem;
  transition: background-color 0.2s ease;
}

.diagnostics-toggle:hover {
  background: #0056b3;
}

.diagnostics-content {
  background: white;
  border: 1px solid #e9ecef;
  border-radius: 6px;
  padding: 1rem;
}

.diagnostics-loading {
  text-align: center;
  padding: 2rem;
  color: #666;
  font-style: italic;
}

.diagnostics-list {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  margin-bottom: 1rem;
}

.diagnostic-item {
  padding: 0.75rem;
  border-radius: 4px;
  border-left: 4px solid;
}

.diagnostic-item.pass {
  background: #d4edda;
  border-left-color: #28a745;
}

.diagnostic-item.fail {
  background: #f8d7da;
  border-left-color: #dc3545;
}

.diagnostic-item.warning {
  background: #fff3cd;
  border-left-color: #ffc107;
}

.diagnostic-header {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-bottom: 0.5rem;
}

.diagnostic-status-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  font-weight: bold;
  font-size: 0.9rem;
}

.diagnostic-item.pass .diagnostic-status-icon {
  background: #28a745;
  color: white;
}

.diagnostic-item.fail .diagnostic-status-icon {
  background: #dc3545;
  color: white;
}

.diagnostic-item.warning .diagnostic-status-icon {
  background: #ffc107;
  color: #212529;
}

.diagnostic-name {
  color: #333;
  font-size: 0.95rem;
}

.diagnostic-message {
  color: #666;
  font-size: 0.9rem;
  margin-bottom: 0.25rem;
}

.diagnostic-details {
  color: #888;
  font-size: 0.85rem;
  font-style: italic;
  margin-top: 0.25rem;
}

.refresh-diagnostics-button {
  width: 100%;
  padding: 0.75rem;
  background: #6c757d;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.9rem;
  transition: background-color 0.2s ease;
}

.refresh-diagnostics-button:hover {
  background: #545b62;
}
</style>
