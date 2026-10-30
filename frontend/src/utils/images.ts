import { currentSessionToken } from '../stores/api'

/** Largest image the server accepts (internal/images.MaxBytes). */
export const MAX_IMAGE_BYTES = 15 * 1024 * 1024

/** Image types the server accepts (internal/images), by preference for a pasted image. */
export const ACCEPTED_IMAGE_TYPES = ['image/png', 'image/jpeg', 'image/webp', 'image/gif']

/** The accepted image type to read from clipboard content offering types, or '' when there is none. */
export function pickClipboardImageType(types: readonly string[]): string {
  return ACCEPTED_IMAGE_TYPES.find(type => types.includes(type)) ?? ''
}

/** Filename for an image pasted at when, e.g. "pasted-2026-10-30T07-52-18-000Z.png". */
export function pastedImageFilename(mimeType: string, when = new Date()): string {
  const ext = { 'image/jpeg': 'jpg', 'image/png': 'png', 'image/webp': 'webp', 'image/gif': 'gif' }[mimeType] ?? 'img'
  return `pasted-${when.toISOString().replace(/[:.]/g, '-')}.${ext}`
}

/** Longest side, in pixels, of a photo taken with the camera. */
export const MAX_CAPTURE_DIMENSION = 2560

/** Object URLs kept for images already fetched; the oldest are released past this many. */
const MAX_CACHED_IMAGES = 300

const cache = new Map<string, Promise<string>>()

/** Path an image is served from (behind the same auth as the API). */
export function imagePath(id: string | number): string {
  return `/api/images/${encodeURIComponent(String(id))}`
}

/**
 * Fetches an image with the session token (an <img src> cannot send it) and returns an object URL
 * for it. Results are shared, so each image is downloaded once per page load.
 */
export function loadImageObjectUrl(id: string | number): Promise<string> {
  const key = String(id)
  const cached = cache.get(key)
  if (cached) {
    // Re-insert so the most recently used images are released last.
    cache.delete(key)
    cache.set(key, cached)
    return cached
  }
  const pending = fetchImage(key)
  cache.set(key, pending)
  pending.catch(() => cache.delete(key))
  evictOldest()
  return pending
}

async function fetchImage(id: string): Promise<string> {
  const token = currentSessionToken()
  const headers: Record<string, string> = token ? { 'Session-Token': token } : {}
  const res = await fetch(imagePath(id), { headers, credentials: 'include' })
  if (!res.ok) {
    throw new Error(res.status === 404 ? 'Image not found' : `Image failed to load (${res.status})`)
  }
  return URL.createObjectURL(await res.blob())
}

function evictOldest() {
  while (cache.size > MAX_CACHED_IMAGES) {
    const [oldestKey, oldest] = cache.entries().next().value as [string, Promise<string>]
    cache.delete(oldestKey)
    oldest.then(url => URL.revokeObjectURL(url), () => {})
  }
}

/** Size to draw a width x height frame at so its longest side is at most maxDimension. */
export function scaleToFit(width: number, height: number, maxDimension: number): { width: number; height: number } {
  const longest = Math.max(width, height)
  if (longest <= maxDimension || longest === 0) {
    return { width, height }
  }
  const scale = maxDimension / longest
  return { width: Math.round(width * scale), height: Math.round(height * scale) }
}

/** Human-readable size in bytes, e.g. "2.4 MB". */
export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}
