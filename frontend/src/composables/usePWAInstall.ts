import { ref, onMounted, onUnmounted } from 'vue'

interface BeforeInstallPromptEvent extends Event {
  prompt: () => Promise<void>
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>
}

const installPrompt = ref<BeforeInstallPromptEvent | null>(null)
const isInstallable = ref(false)
const isInstalled = ref(false)

/**
 * Composable for handling PWA install prompt
 */
export function usePWAInstall() {
  const handleBeforeInstallPrompt = (e: Event) => {
    // Prevent the default browser install prompt
    e.preventDefault()

    // Store the event for later use
    installPrompt.value = e as BeforeInstallPromptEvent
    isInstallable.value = true

    console.log('[PWA] Install prompt available')
  }

  const handleAppInstalled = () => {
    isInstalled.value = true
    isInstallable.value = false
    installPrompt.value = null
    console.log('[PWA] App installed')
  }

  const promptInstall = async (): Promise<boolean> => {
    if (!installPrompt.value) {
      console.warn('[PWA] No install prompt available')
      return false
    }

    try {
      // Show the install prompt
      await installPrompt.value.prompt()

      // Wait for user's choice
      const choiceResult = await installPrompt.value.userChoice

      if (choiceResult.outcome === 'accepted') {
        console.log('[PWA] User accepted install prompt')
        isInstallable.value = false
        installPrompt.value = null
        return true
      } else {
        console.log('[PWA] User dismissed install prompt')
        return false
      }
    } catch (error) {
      console.error('[PWA] Error showing install prompt:', error)
      return false
    }
  }

  const checkIfInstalled = () => {
    // Check if app is running in standalone mode (installed)
    if (window.matchMedia('(display-mode: standalone)').matches) {
      isInstalled.value = true
      isInstallable.value = false
      return true
    }

    // Check if running as TWA (Trusted Web Activity)
    if ((window.navigator as any).standalone === true) {
      isInstalled.value = true
      isInstallable.value = false
      return true
    }

    return false
  }

  onMounted(() => {
    // Check if already installed
    checkIfInstalled()

    // Listen for the beforeinstallprompt event
    window.addEventListener('beforeinstallprompt', handleBeforeInstallPrompt)

    // Listen for app installed event
    window.addEventListener('appinstalled', handleAppInstalled)

    // Check periodically if installed (for cases where event doesn't fire)
    const checkInterval = setInterval(() => {
      if (!isInstalled.value) {
        checkIfInstalled()
      } else {
        clearInterval(checkInterval)
      }
    }, 1000)

    // Clean up interval after 10 seconds
    setTimeout(() => clearInterval(checkInterval), 10000)
  })

  onUnmounted(() => {
    window.removeEventListener('beforeinstallprompt', handleBeforeInstallPrompt)
    window.removeEventListener('appinstalled', handleAppInstalled)
  })

  return {
    isInstallable,
    isInstalled,
    promptInstall,
    checkIfInstalled
  }
}
