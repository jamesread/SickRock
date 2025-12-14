<template>
  <div v-if="visible" class="modal-overlay" @click="close" @keydown.escape="close" tabindex="0">
    <div class="modal-content keyboard-shortcuts-modal" @click.stop>
      <div class="modal-header">
        <h2>Keyboard Shortcuts</h2>
        <button @click="close" class="button neutral" title="Close">
          ✕
        </button>
      </div>
      <div class="modal-body">
        <div class="shortcuts-section">
          <h3>Global Shortcuts</h3>
          <div class="shortcuts-list">
            <div v-for="shortcut in globalShortcuts" :key="shortcut.key" class="shortcut-item">
              <div class="shortcut-keys">
                <kbd v-for="(key, idx) in formatShortcutKeys(shortcut)" :key="idx">{{ key }}</kbd>
              </div>
              <div class="shortcut-description">{{ shortcut.description }}</div>
            </div>
          </div>
        </div>

        <div class="shortcuts-section">
          <h3>Table Navigation</h3>
          <div class="shortcuts-list">
            <div v-for="shortcut in tableShortcuts" :key="shortcut.key" class="shortcut-item">
              <div class="shortcut-keys">
                <kbd v-for="(key, idx) in formatShortcutKeys(shortcut)" :key="idx">{{ key }}</kbd>
              </div>
              <div class="shortcut-description">{{ shortcut.description }}</div>
            </div>
          </div>
        </div>

        <div class="shortcuts-section">
          <h3>Navigation</h3>
          <div class="shortcuts-list">
            <div v-for="shortcut in navigationShortcuts" :key="shortcut.key" class="shortcut-item">
              <div class="shortcut-keys">
                <kbd v-for="(key, idx) in formatShortcutKeys(shortcut)" :key="idx">{{ key }}</kbd>
              </div>
              <div class="shortcut-description">{{ shortcut.description }}</div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, watch, onMounted, onUnmounted, nextTick } from 'vue'

const props = defineProps<{
  visible: boolean
}>()

const emit = defineEmits<{
  'update:visible': [value: boolean]
}>()

const isMac = computed(() => {
  return navigator.platform.toUpperCase().indexOf('MAC') >= 0
})

function close() {
  emit('update:visible', false)
}

function handleEscape(e: KeyboardEvent) {
  if (e.key === 'Escape' && props.visible) {
    close()
  }
}

// Watch for visibility changes and add/remove escape listener
watch(() => props.visible, (isVisible) => {
  if (isVisible) {
    window.addEventListener('keydown', handleEscape)
    // Focus the overlay so it can receive keyboard events
    nextTick(() => {
      const overlay = document.querySelector('.modal-overlay') as HTMLElement
      if (overlay) {
        overlay.focus()
      }
    })
  } else {
    window.removeEventListener('keydown', handleEscape)
  }
})

onMounted(() => {
  if (props.visible) {
    window.addEventListener('keydown', handleEscape)
  }
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleEscape)
})

function formatShortcutKeys(shortcut: { key: string; ctrl?: boolean; meta?: boolean; shift?: boolean; alt?: boolean }): string[] {
  const keys: string[] = []

  if (shortcut.ctrl || shortcut.meta) {
    keys.push(isMac.value ? '⌘' : 'Ctrl')
  }
  if (shortcut.shift) {
    keys.push('Shift')
  }
  if (shortcut.alt) {
    keys.push('Alt')
  }

  // Format the key
  let key = shortcut.key
  if (key === ' ') key = 'Space'
  if (key === '/') key = '/'
  if (key === '?') key = '?'
  if (key.length === 1 && key >= 'a' && key <= 'z') {
    key = key.toUpperCase()
  }

  keys.push(key)

  return keys
}

const globalShortcuts = [
  { key: 'k', ctrl: true, description: 'Focus QuickSearch' },
  { key: '/', ctrl: true, description: 'Focus QuickSearch' },
  { key: 'i', ctrl: true, description: 'Insert new row (open quick add)' },
  { key: 'f', ctrl: true, description: 'Focus table filter/search' },
  { key: 's', ctrl: true, description: 'Save current edit' },
  { key: 'Escape', description: 'Close modals/dialogs, cancel editing' },
  { key: '/', ctrl: true, shift: true, description: 'Show keyboard shortcuts help' },
]

const tableShortcuts = [
  { key: 'ArrowUp', description: 'Navigate to previous cell' },
  { key: 'ArrowDown', description: 'Navigate to next cell' },
  { key: 'ArrowLeft', description: 'Navigate to previous column' },
  { key: 'ArrowRight', description: 'Navigate to next column' },
  { key: 'Tab', description: 'Next cell' },
  { key: 'Tab', shift: true, description: 'Previous cell' },
  { key: 'Enter', description: 'Edit cell' },
  { key: 'Escape', description: 'Cancel editing' },
  { key: 'Delete', description: 'Delete selected rows (with confirmation)' },
  { key: 'Backspace', description: 'Delete selected rows (with confirmation)' },
]

const navigationShortcuts = [
  { key: 'g', description: 'Then press:' },
  { key: 'h', description: 'Go to Home' },
  { key: 't', description: 'Go to Tables list' },
  { key: 'c', description: 'Go to Control Panel' },
  { key: '?', description: 'Show keyboard shortcuts help' },
  { key: 'b', description: 'Toggle bookmarks toolbar' },
]
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 10000;
  padding: 1rem;
  box-sizing: border-box;
}

.keyboard-shortcuts-modal {
  background: white;
  border-radius: 8px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.3);
  max-width: 600px;
  max-height: 80vh;
  overflow-y: auto;
  width: 90%;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem;
  border-bottom: 1px solid #ddd;
}

.modal-header h2 {
  margin: 0;
  font-size: 1.5rem;
}

.modal-body {
  padding: 1rem;
}

.shortcuts-section {
  margin-bottom: 2rem;
}

.shortcuts-section:last-child {
  margin-bottom: 0;
}

.shortcuts-section h3 {
  margin: 0 0 1rem 0;
  font-size: 1.1rem;
  font-weight: 600;
  color: #333;
}

.shortcuts-list {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.shortcut-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0.5rem;
  border-radius: 4px;
  background: #f8f9fa;
}

.shortcut-keys {
  display: flex;
  gap: 0.25rem;
  align-items: center;
}

.shortcut-keys kbd {
  display: inline-block;
  padding: 0.25rem 0.5rem;
  background: white;
  border: 1px solid #ddd;
  border-radius: 3px;
  font-family: monospace;
  font-size: 0.875rem;
  font-weight: 600;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.1);
  min-width: 1.5rem;
  text-align: center;
}

.shortcut-description {
  color: #666;
  font-size: 0.9rem;
  margin-left: 1rem;
  text-align: right;
}

@media (max-width: 768px) {
  .shortcut-item {
    flex-direction: column;
    align-items: flex-start;
    gap: 0.5rem;
  }

  .shortcut-description {
    margin-left: 0;
    text-align: left;
  }
}
</style>
