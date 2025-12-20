<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { usePWAInstall } from '../composables/usePWAInstall'
import { HugeiconsIcon } from '@hugeicons/vue'
import { Download01Icon } from '@hugeicons/core-free-icons'
import * as Hugeicons from '@hugeicons/core-free-icons'

const { isInstallable, isInstalled, promptInstall } = usePWAInstall()

const showPrompt = ref(false)
const dismissedPrompt = ref(false)
const installing = ref(false)

const DISMISSED_KEY = 'sickrock-pwa-install-dismissed'
const PROMPT_DELAY = 3000 // Show prompt after 3 seconds

const shouldShowPrompt = computed(() => {
  return (
    isInstallable.value &&
    !isInstalled.value &&
    !dismissedPrompt.value &&
    showPrompt.value &&
    !installing.value
  )
})

function showPromptAfterDelay() {
  setTimeout(() => {
    if (isInstallable.value && !isInstalled.value && !dismissedPrompt.value) {
      showPrompt.value = true
    }
  }, PROMPT_DELAY)
}

onMounted(() => {
  // Check if user previously dismissed the prompt
  try {
    const dismissed = localStorage.getItem(DISMISSED_KEY)
    if (dismissed === 'true') {
      dismissedPrompt.value = true
      return
    }
  } catch (e) {
    // Ignore localStorage errors
  }

  // Show prompt after a delay (only if installable)
  if (isInstallable.value) {
    showPromptAfterDelay()
  }
})

// Watch for installable state changes
watch(isInstallable, (newVal) => {
  if (newVal && !dismissedPrompt.value && !isInstalled.value) {
    // Show prompt after delay if it becomes installable
    showPromptAfterDelay()
  }
})

async function handleInstall() {
  installing.value = true
  try {
    const accepted = await promptInstall()
    if (accepted) {
      showPrompt.value = false
    }
  } catch (error) {
    console.error('Failed to install PWA:', error)
  } finally {
    installing.value = false
  }
}

function handleDismiss() {
  showPrompt.value = false
  dismissedPrompt.value = true
  try {
    localStorage.setItem(DISMISSED_KEY, 'true')
  } catch (e) {
    // Ignore localStorage errors
  }
}
</script>

<template>
  <div v-if="shouldShowPrompt" class="pwa-install-prompt">
    <div class="pwa-install-content">
      <div class="pwa-install-icon">
        <HugeiconsIcon :icon="Hugeicons.Download01Icon" />
      </div>
      <div class="pwa-install-text">
        <h3>Install SickRock</h3>
        <p>Install SickRock as an app for a better experience and offline access.</p>
      </div>
      <div class="pwa-install-actions">
        <button
          @click="handleInstall"
          :disabled="installing"
          class="pwa-install-button"
        >
          {{ installing ? 'Installing...' : 'Install' }}
        </button>
        <button
          @click="handleDismiss"
          class="pwa-dismiss-button"
          :title="'Dismiss'"
        >
          ✕
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.pwa-install-prompt {
  position: fixed;
  bottom: 20px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 10000;
  max-width: 500px;
  width: calc(100% - 40px);
  animation: slideUp 0.3s ease-out;
}

@keyframes slideUp {
  from {
    transform: translateX(-50%) translateY(100%);
    opacity: 0;
  }
  to {
    transform: translateX(-50%) translateY(0);
    opacity: 1;
  }
}

.pwa-install-content {
  background: white;
  border-radius: 8px;
  padding: 16px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  display: flex;
  align-items: center;
  gap: 16px;
}

.pwa-install-icon {
  flex-shrink: 0;
  width: 48px;
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #444444;
  color: white;
  border-radius: 8px;
}

.pwa-install-icon svg {
  width: 24px;
  height: 24px;
}

.pwa-install-text {
  flex: 1;
  min-width: 0;
}

.pwa-install-text h3 {
  margin: 0 0 4px 0;
  font-size: 16px;
  font-weight: 600;
  color: #1a1a1a;
}

.pwa-install-text p {
  margin: 0;
  font-size: 14px;
  color: #666;
  line-height: 1.4;
}

.pwa-install-actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}

.pwa-install-button {
  padding: 8px 16px;
  background: #444444;
  color: white;
  border: none;
  border-radius: 4px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.2s;
}

.pwa-install-button:hover:not(:disabled) {
  background: #333333;
}

.pwa-install-button:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.pwa-dismiss-button {
  padding: 8px;
  background: transparent;
  color: #666;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background 0.2s;
}

.pwa-dismiss-button:hover {
  background: #f0f0f0;
}

.pwa-dismiss-button svg {
  width: 20px;
  height: 20px;
}

/* Dark mode support */
@media (prefers-color-scheme: dark) {
  .pwa-install-content {
    background: #2a2a2a;
  }

  .pwa-install-text h3 {
    color: #ffffff;
  }

  .pwa-install-text p {
    color: #cccccc;
  }

  .pwa-dismiss-button {
    color: #cccccc;
  }

  .pwa-dismiss-button:hover {
    background: #3a3a3a;
  }
}

/* Mobile responsive */
@media (max-width: 640px) {
  .pwa-install-prompt {
    bottom: 10px;
    width: calc(100% - 20px);
  }

  .pwa-install-content {
    flex-direction: column;
    text-align: center;
    padding: 12px;
  }

  .pwa-install-actions {
    width: 100%;
    justify-content: center;
  }

  .pwa-install-button {
    flex: 1;
  }
}
</style>
