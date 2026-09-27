<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { usePWAInstall } from '../composables/usePWAInstall'
import { HugeiconsIcon } from '@hugeicons/vue'
import {
  Download01Icon,
  CheckmarkSquare03Icon,
  QuestionIcon,
  RefreshIcon,
} from '@hugeicons/core-free-icons'
import Section from 'picocrank/vue/components/Section.vue'
import StatusCard from 'picocrank/vue/components/StatusCard.vue'

const iconStrokeWidth = 2.5

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

function toggleDiagnostics() {
  showDiagnostics.value = !showDiagnostics.value
  if (showDiagnostics.value && diagnostics.value.length === 0) {
    void loadDiagnostics()
  }
}

function diagnosticKarma(status: PWADiagnostic['status']): string {
  switch (status) {
    case 'pass':
      return 'good'
    case 'fail':
      return 'bad'
    default:
      return 'warning'
  }
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

function swStateTagClass(state: string | null): string {
  switch (state) {
    case 'activated':
    case 'installed':
      return 'fg-good'
    case 'redundant':
      return 'bad'
    default:
      return 'note'
  }
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
  <Section
    title="PWA & Service Worker"
    subtitle="Install SickRock as an app and check offline service worker status."
    :icon="Download01Icon"
  >
    <div class="pwa-section">
      <h3 class="subsection-title">Service worker status</h3>
      <StatusCard :karma="swStatus.registered ? 'good' : 'bad'">
        <template v-if="swStatus.registered">
          <dl class="meta-list">
            <dt>Status</dt>
            <dd>
              <span class="tag" :class="swStateTagClass(swStatus.state)">
                {{ formatSWState(swStatus.state) }}
              </span>
            </dd>
            <template v-if="swStatus.version">
              <dt>Version</dt>
              <dd>{{ swStatus.version }}</dd>
            </template>
            <template v-if="swStatus.scope">
              <dt>Scope</dt>
              <dd><code class="mono">{{ swStatus.scope }}</code></dd>
            </template>
          </dl>
        </template>
        <template v-else>
          <p class="status-lead">Service worker is not registered.</p>
          <p v-if="swStatus.error" class="inline-notification error">{{ swStatus.error }}</p>
        </template>
      </StatusCard>

      <h3 class="subsection-title">App installation</h3>
      <StatusCard v-if="isInstalled" karma="good">
        <div class="status-block">
          <HugeiconsIcon
            :icon="CheckmarkSquare03Icon"
            width="1.5em"
            height="1.5em"
            :strokeWidth="iconStrokeWidth"
            class="status-block-icon"
          />
          <div>
            <h4 class="status-heading">App installed</h4>
            <p class="subtle">
              SickRock is installed on this device. You can use it offline and open it from your home
              screen.
            </p>
          </div>
        </div>
      </StatusCard>

      <StatusCard v-else-if="pwaStatus.canInstall" karma="note">
        <div class="status-block">
          <HugeiconsIcon
            :icon="Download01Icon"
            width="1.5em"
            height="1.5em"
            :strokeWidth="iconStrokeWidth"
            class="status-block-icon"
          />
          <div>
            <h4 class="status-heading">Install SickRock</h4>
            <p class="subtle">
              Install as an app for offline access and quick launch from your home screen.
            </p>
            <button
              type="button"
              class="inline-icon good"
              :disabled="installingPWA"
              @click="handlePWAInstall"
            >
              <HugeiconsIcon :icon="Download01Icon" width="1em" height="1em" :strokeWidth="iconStrokeWidth" />
              <span>{{ installingPWA ? 'Installing…' : 'Install app' }}</span>
            </button>
          </div>
        </div>
      </StatusCard>

      <StatusCard v-else karma="warning">
        <div class="status-block">
          <HugeiconsIcon
            :icon="QuestionIcon"
            width="1.5em"
            height="1.5em"
            :strokeWidth="iconStrokeWidth"
            class="status-block-icon"
          />
          <div>
            <h4 class="status-heading">Installation not available</h4>
            <p class="subtle">PWA installation is not currently available. Possible reasons:</p>
            <ul v-if="pwaStatus.reasons?.length" class="reasons-list subtle">
              <li v-for="(reason, index) in pwaStatus.reasons" :key="index">{{ reason }}</li>
            </ul>
            <div class="help-panel">
              <p><strong>To enable installation</strong></p>
              <ul class="reasons-list subtle">
                <li>Use a modern browser (Chrome, Edge, Safari, or Firefox)</li>
                <li>Serve the app over HTTPS (or localhost for development)</li>
                <li>Confirm your browser supports PWA installation</li>
                <li>On iOS, use Safari</li>
              </ul>
            </div>
          </div>
        </div>
      </StatusCard>

      <div class="diagnostics-toolbar">
        <h3 class="subsection-title diagnostics-title">Detailed diagnostics</h3>
        <button type="button" class="neutral" @click="toggleDiagnostics">
          {{ showDiagnostics ? 'Hide diagnostics' : 'Show diagnostics' }}
        </button>
      </div>

      <div v-if="showDiagnostics" class="diagnostics-panel">
        <div v-if="diagnostics.length === 0" class="muted diagnostics-loading">Loading diagnostics…</div>
        <div v-else class="diagnostics-list">
          <StatusCard
            v-for="(diagnostic, index) in diagnostics"
            :key="index"
            :karma="diagnosticKarma(diagnostic.status)"
            compact
          >
            <h4 class="status-heading">{{ diagnostic.name }}</h4>
            <p class="subtle">{{ diagnostic.message }}</p>
            <p v-if="diagnostic.details" class="diagnostic-details subtle">{{ diagnostic.details }}</p>
          </StatusCard>
        </div>
        <button type="button" class="neutral full-width" @click="loadDiagnostics">
          <HugeiconsIcon :icon="RefreshIcon" width="1em" height="1em" :strokeWidth="iconStrokeWidth" />
          <span>Refresh diagnostics</span>
        </button>
      </div>
    </div>
  </Section>
</template>

<style scoped>
.pwa-section {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.subsection-title {
  margin: 1.25rem 0 0.35rem;
  color: var(--text-color, inherit);
}

.subsection-title:first-child {
  margin-top: 0;
}

.status-heading {
  margin: 0 0 0.35rem;
  color: var(--text-color, inherit);
  font-size: 1.05rem;
}

.status-lead {
  margin: 0;
  color: var(--text-color, inherit);
}

.status-block {
  display: flex;
  gap: 0.75rem;
  align-items: flex-start;
}

.status-block-icon {
  flex-shrink: 0;
  color: var(--text-color, inherit);
}

.meta-list {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 0.35rem 1rem;
  margin: 0;
}

.meta-list dt {
  margin: 0;
  color: var(--muted-text-color, inherit);
  font-weight: 500;
}

.meta-list dd {
  margin: 0;
  color: var(--text-color, inherit);
}

.mono {
  word-break: break-all;
  background: var(--standout-bg-color, transparent);
  border: 1px solid var(--border-color, currentColor);
  padding: 0.15rem 0.35rem;
  border-radius: 3px;
}

.reasons-list {
  margin: 0.5rem 0 0;
  padding-left: 1.25rem;
}

.help-panel {
  margin-top: 0.75rem;
  padding: 0.75rem 1rem;
  border-radius: 4px;
  background: var(--standout-bg-color, transparent);
  border: 1px solid var(--border-color, currentColor);
}

.help-panel p {
  margin: 0 0 0.35rem;
  color: var(--text-color, inherit);
}

.diagnostics-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  margin-top: 0.5rem;
}

.diagnostics-title {
  margin: 0;
}

.diagnostics-panel {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.diagnostics-list {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.diagnostics-loading {
  padding: 1rem 0;
}

.diagnostic-details {
  margin: 0.35rem 0 0;
  font-size: 0.9em;
}

.full-width {
  width: 100%;
  justify-content: center;
}
</style>
