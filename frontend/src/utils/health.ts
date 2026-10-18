/** Window event fired after health issues change (run, acknowledge) so the sidebar badge refreshes. */
export const HEALTH_CHANGED_EVENT = 'sickrock:health-changed'

export function notifyHealthChanged() {
  window.dispatchEvent(new Event(HEALTH_CHANGED_EVENT))
}

export type HealthSeverity = 'error' | 'warning' | 'info'

/** Sort order and labels for issue severities. */
export const HEALTH_SEVERITIES: Array<{ id: HealthSeverity; one: string; many: string }> = [
  { id: 'error', one: 'error', many: 'errors' },
  { id: 'warning', one: 'warning', many: 'warnings' },
  { id: 'info', one: 'info', many: 'info' },
]
