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

/**
 * Sets the iOS home-screen name (apple-mobile-web-app-title). Other browsers take the
 * name from /manifest.json, which the server builds with the same title.
 */
export function syncAppTitleMeta(title: string, doc: Document = document): void {
  const name = title.trim() || 'SickRock'
  let meta = doc.querySelector<HTMLMetaElement>('meta[name="apple-mobile-web-app-title"]')
  if (!meta) {
    meta = doc.createElement('meta')
    meta.name = 'apple-mobile-web-app-title'
    doc.head.appendChild(meta)
  }
  meta.content = name
}
