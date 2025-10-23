/**
 * Date formatting utilities for SickRock frontend
 * Provides locale-aware date formatting functions
 */

/**
 * Format a Unix timestamp (seconds since epoch) as a locale-aware date string
 * @param timestamp Unix timestamp in seconds
 * @param options Intl.DateTimeFormatOptions for customization
 * @returns Formatted date string using user's locale
 */
export function formatUnixTimestamp(
  timestamp: number | bigint | string,
  options: Intl.DateTimeFormatOptions = {}
): string {
  if (!timestamp) return ''
  
  const numTimestamp = typeof timestamp === 'bigint' ? Number(timestamp) : Number(timestamp)
  
  if (!Number.isFinite(numTimestamp) || numTimestamp <= 0) {
    return ''
  }
  
  const date = new Date(numTimestamp * 1000)
  
  // Default options for a readable date format
  const defaultOptions: Intl.DateTimeFormatOptions = {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: true,
    ...options
  }
  
  return date.toLocaleString(undefined, defaultOptions)
}

/**
 * Format a Unix timestamp as a date only (no time)
 * @param timestamp Unix timestamp in seconds
 * @param options Intl.DateTimeFormatOptions for customization
 * @returns Formatted date string using user's locale
 */
export function formatUnixDate(
  timestamp: number | bigint | string,
  options: Intl.DateTimeFormatOptions = {}
): string {
  const dateOptions: Intl.DateTimeFormatOptions = {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    ...options
  }
  
  return formatUnixTimestamp(timestamp, dateOptions)
}

/**
 * Format a Unix timestamp as a time only (no date)
 * @param timestamp Unix timestamp in seconds
 * @param options Intl.DateTimeFormatOptions for customization
 * @returns Formatted time string using user's locale
 */
export function formatUnixTime(
  timestamp: number | bigint | string,
  options: Intl.DateTimeFormatOptions = {}
): string {
  const timeOptions: Intl.DateTimeFormatOptions = {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: true,
    ...options
  }
  
  return formatUnixTimestamp(timestamp, timeOptions)
}

/**
 * Format a MySQL datetime string as a locale-aware date string
 * @param dateStr MySQL datetime string (YYYY-MM-DD HH:MM:SS)
 * @param options Intl.DateTimeFormatOptions for customization
 * @returns Formatted date string using user's locale
 */
export function formatMySQLDateTime(
  dateStr: string,
  options: Intl.DateTimeFormatOptions = {}
): string {
  if (!dateStr) return ''
  
  // Parse MySQL datetime format: YYYY-MM-DD HH:MM:SS
  const datetimeMatch = dateStr.match(/^(\d{4})-(\d{2})-(\d{2})(?: (\d{2}):(\d{2}):(\d{2}))?$/)
  
  if (!datetimeMatch) {
    // Fallback to standard Date parsing
    const date = new Date(dateStr)
    if (isNaN(date.getTime())) return ''
    return date.toLocaleString(undefined, options)
  }
  
  const [, year, month, day, hour = '0', minute = '0', second = '0'] = datetimeMatch
  
  // Create date in local timezone
  const date = new Date(
    parseInt(year, 10),
    parseInt(month, 10) - 1, // Month is 0-indexed
    parseInt(day, 10),
    parseInt(hour, 10),
    parseInt(minute, 10),
    parseInt(second, 10)
  )
  
  if (isNaN(date.getTime())) return ''
  
  // Default options for a readable date format
  const defaultOptions: Intl.DateTimeFormatOptions = {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: true,
    ...options
  }
  
  return date.toLocaleString(undefined, defaultOptions)
}

/**
 * Get a relative time string (e.g., "2 hours ago", "3 days ago")
 * @param timestamp Unix timestamp in seconds
 * @returns Relative time string
 */
export function formatRelativeTime(timestamp: number | bigint | string): string {
  if (!timestamp) return ''
  
  const numTimestamp = typeof timestamp === 'bigint' ? Number(timestamp) : Number(timestamp)
  
  if (!Number.isFinite(numTimestamp) || numTimestamp <= 0) {
    return ''
  }
  
  const date = new Date(numTimestamp * 1000)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffMins = Math.floor(diffMs / (1000 * 60))
  const diffHours = Math.floor(diffMs / (1000 * 60 * 60))
  const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24))
  
  if (diffMins < 1) return 'Just now'
  if (diffMins < 60) return `${diffMins}m ago`
  if (diffHours < 24) return `${diffHours}h ago`
  if (diffDays < 7) return `${diffDays}d ago`
  
  // For older dates, show the actual date
  return formatUnixDate(timestamp)
}
