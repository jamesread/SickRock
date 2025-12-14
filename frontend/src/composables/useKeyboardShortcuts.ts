import { onMounted, onUnmounted, Ref } from 'vue'

export interface KeyboardShortcut {
  key: string
  ctrl?: boolean
  meta?: boolean
  shift?: boolean
  alt?: boolean
  handler: (e: KeyboardEvent) => void
  description?: string
  preventDefault?: boolean
}

export function useKeyboardShortcuts(shortcuts: Ref<KeyboardShortcut[]>) {
  const handleKeyDown = (e: KeyboardEvent) => {
    // Don't intercept if user is typing in an input field, textarea, or contenteditable
    // (unless it's a global shortcut like Ctrl+K)
    const target = e.target as HTMLElement
    const isTyping = target && (
      target.tagName === 'INPUT' ||
      target.tagName === 'TEXTAREA' ||
      target.isContentEditable ||
      (target.tagName === 'SELECT')
    )

    // Allow global shortcuts (Ctrl/Cmd + key) even when typing
    const isGlobalShortcut = e.ctrlKey || e.metaKey

    for (const shortcut of shortcuts.value) {
      // Skip non-global shortcuts if user is typing
      if (isTyping && !isGlobalShortcut && !shortcut.ctrl && !shortcut.meta) {
        continue
      }
      // Handle "?" key - can be either "?" or "/" with shift
      let keyMatches = false
      if (shortcut.key === '?') {
        // "?" can be reported as either "?" or "/" with shift
        // Check both the actual key and if it's "/" with shift pressed
        keyMatches = e.key === '?' || (e.key === '/' && e.shiftKey)
      } else if (shortcut.key === '/' && shortcut.shift) {
        // Handle "/" with shift (which produces "?")
        keyMatches = (e.key === '/' && e.shiftKey) || e.key === '?'
      } else {
        keyMatches = shortcut.key.toLowerCase() === e.key.toLowerCase()
      }

      const ctrlMatches = shortcut.ctrl ? (e.ctrlKey || e.metaKey) : !(e.ctrlKey || e.metaKey)
      const metaMatches = shortcut.meta !== undefined
        ? (shortcut.meta ? (e.ctrlKey || e.metaKey) : !(e.ctrlKey || e.metaKey))
        : true

      // Special handling for "?" - it requires shift, so we need to allow shift
      let shiftMatches = false
      if (shortcut.key === '?') {
        // "?" requires shift (it's Shift+/), so shift must be pressed
        shiftMatches = e.shiftKey
      } else {
        shiftMatches = shortcut.shift ? e.shiftKey : !e.shiftKey
      }

      const altMatches = shortcut.alt ? e.altKey : !e.altKey

      // For Ctrl/Cmd shortcuts, accept either Ctrl (Windows/Linux) or Cmd (Mac)
      const modifierMatches = shortcut.ctrl || shortcut.meta
        ? (e.ctrlKey || e.metaKey)
        : ctrlMatches && metaMatches

      if (keyMatches && modifierMatches && shiftMatches && altMatches) {
        if (shortcut.preventDefault !== false) {
          e.preventDefault()
        }
        shortcut.handler(e)
        break
      }
    }
  }

  onMounted(() => {
    window.addEventListener('keydown', handleKeyDown)
  })

  onUnmounted(() => {
    window.removeEventListener('keydown', handleKeyDown)
  })

  return {
    handleKeyDown
  }
}

// Helper to format shortcut display
export function formatShortcut(shortcut: KeyboardShortcut, isMac: boolean = false): string {
  const parts: string[] = []

  if (shortcut.ctrl || shortcut.meta) {
    parts.push(isMac ? '⌘' : 'Ctrl')
  }
  if (shortcut.shift) {
    parts.push('Shift')
  }
  if (shortcut.alt) {
    parts.push('Alt')
  }

  // Format the key
  let key = shortcut.key
  if (key === ' ') key = 'Space'
  if (key === '/') key = '/'
  if (key === '?') key = '?'
  if (key.length === 1 && key >= 'a' && key <= 'z') {
    key = key.toUpperCase()
  }

  parts.push(key)

  return parts.join(' + ')
}
