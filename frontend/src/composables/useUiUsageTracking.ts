// Opt-in UI usage tracking for the signed-in user. Once installed (App.vue), route changes and
// clicks on sidebar and header controls are buffered while tracking is on, kept in localStorage
// between page loads, and sent to the server (RecordUIUsage) every FLUSH_INTERVAL_MS and when the
// page is hidden. Nothing is recorded unless the user has turned on "Track and log UI usage".
import { ref } from 'vue'
import type { Router } from 'vue-router'
import type { createApiClient } from '../stores/api'
import {
  appendCapped,
  describeClick,
  newSessionId,
  parsePending,
  pendingStorageKey,
  type UiUsageEvent,
} from '../utils/uiUsage'

const FLUSH_INTERVAL_MS = 30_000

type ApiClient = ReturnType<typeof createApiClient>

const enabled = ref(false)
const sessionId = newSessionId()
let username = ''
let appVersion = ''
let client: ApiClient | null = null
let installed = false
let sending = false

function readPending(): UiUsageEvent[] {
  if (!username) return []
  try {
    return parsePending(localStorage.getItem(pendingStorageKey(username)))
  } catch {
    return []
  }
}

function writePending(events: UiUsageEvent[]) {
  if (!username) return
  try {
    if (events.length) {
      localStorage.setItem(pendingStorageKey(username), JSON.stringify(events))
    } else {
      localStorage.removeItem(pendingStorageKey(username))
    }
  } catch {
    // Storage unavailable (private mode, quota): events are dropped rather than kept.
  }
}

function record(event: Omit<UiUsageEvent, 'timestampMs' | 'sessionId' | 'appVersion'>) {
  if (!enabled.value || !username) return
  writePending(appendCapped(readPending(), [{ ...event, timestampMs: Date.now(), sessionId, appVersion }]))
}

/** Sends pending events. Events stay pending until the server has accepted them. */
async function flush() {
  if (!enabled.value || !client || sending) return
  const pending = readPending()
  if (pending.length === 0) return
  sending = true
  try {
    const res = await client.recordUIUsage({ events: pending.map(e => ({ ...e, timestampMs: BigInt(e.timestampMs) })) })
    // Keep anything recorded while the request was in flight.
    writePending(readPending().slice(pending.length))
    if (!res.trackingEnabled) {
      setTrackingEnabled(false)
    }
  } catch {
    // Offline or failed: try again on the next flush.
  } finally {
    sending = false
  }
}

function onClick(e: MouseEvent) {
  const click = describeClick(e.target, window.location.origin)
  if (click) {
    record({ ...click, fromPath: currentPath() })
  }
}

let routerRef: Router | null = null

function currentPath(): string {
  return routerRef?.currentRoute.value.fullPath ?? window.location.pathname
}

/** Turns tracking on or off in this tab; turning it off drops anything not yet sent. */
function setTrackingEnabled(on: boolean) {
  enabled.value = on
  if (!on) writePending([])
}

/**
 * Starts listening (once per page load). Tracking stays off until configure() is given a user who
 * has opted in.
 */
function install(router: Router, apiClient: ApiClient) {
  if (installed) return
  installed = true
  client = apiClient
  routerRef = router
  router.afterEach((to, from, failure) => {
    if (failure) return
    record({ kind: 'route', fromPath: from.matched.length ? from.fullPath : '', toPath: to.fullPath, label: String(to.name ?? '') })
  })
  document.addEventListener('click', onClick, true)
  window.setInterval(() => void flush(), FLUSH_INTERVAL_MS)
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'hidden') void flush()
  })
}

/** Loads the signed-in user's setting; call after sign-in (and with an empty username after sign-out). */
async function configure(user: string, version: string) {
  username = user
  appVersion = version
  if (!user || !client) {
    enabled.value = false
    return
  }
  try {
    const res = await client.getUIUsageSettings({})
    enabled.value = res.trackingEnabled
    if (enabled.value) {
      record({ kind: 'route', fromPath: '', toPath: currentPath(), label: 'page load' })
      void flush()
    }
  } catch {
    enabled.value = false
  }
}

export function useUiUsageTracking() {
  return { enabled, install, configure, setTrackingEnabled, flush }
}
