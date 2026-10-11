import { ref } from 'vue'

/** Configurable application title (from table_settings.appTitle); shared by header and document title. */
export const appTitle = ref('SickRock')

export function setAppTitle(title: string) {
  const trimmed = title.trim()
  if (trimmed) {
    appTitle.value = trimmed
  }
}

/** Browser tab title: page heading plus configured app name. */
export function formatDocumentTitle(pageTitle: string, baseTitle = appTitle.value): string {
  const page = pageTitle.trim()
  const base = baseTitle.trim() || 'SickRock'
  return page ? `${page} - ${base}` : base
}
