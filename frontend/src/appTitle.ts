import { ref } from 'vue'

/** Configurable application title (from table_settings.appTitle); shared by header and document title. */
export const appTitle = ref('SickRock')

export function setAppTitle(title: string) {
  const trimmed = title.trim()
  if (trimmed) {
    appTitle.value = trimmed
  }
}
