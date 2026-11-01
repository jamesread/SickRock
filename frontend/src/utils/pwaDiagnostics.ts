/** The manifest as fetched for the PWA diagnostics page. */
export interface ManifestCheck {
  data: Record<string, unknown> | null
  error: string
}

/**
 * Fetches and parses the web app manifest, describing what went wrong in words a person can
 * act on (for example when a server answers with its HTML page instead of the manifest).
 */
export async function fetchManifest(href: string, fetchFn: typeof fetch = fetch): Promise<ManifestCheck> {
  let response: Response
  try {
    response = await fetchFn(href, { cache: 'no-store' })
  } catch (e) {
    return { data: null, error: `Could not fetch ${href}: ${e instanceof Error ? e.message : String(e)}` }
  }
  if (!response.ok) {
    return { data: null, error: `HTTP ${response.status}${response.statusText ? ` ${response.statusText}` : ''} for ${href}` }
  }

  const type = response.headers.get('Content-Type') || ''
  const body = await response.text()
  if (type.includes('text/html') || /^\s*</.test(body)) {
    return {
      data: null,
      error: `${href} returned an HTML page instead of the manifest. The SickRock server builds it; under the Vite dev server it is proxied there, so check the backend is running.`,
    }
  }

  let data: unknown
  try {
    data = JSON.parse(body)
  } catch (e) {
    return { data: null, error: `${href} is not valid JSON: ${e instanceof Error ? e.message : String(e)}` }
  }
  if (!data || typeof data !== 'object' || Array.isArray(data)) {
    return { data: null, error: `${href} is not a JSON object` }
  }
  if (!type.includes('application/manifest+json') && !type.includes('application/json')) {
    return { data: data as Record<string, unknown>, error: `${href} is served as "${type || 'no type'}"; expected application/manifest+json` }
  }
  return { data: data as Record<string, unknown>, error: '' }
}
