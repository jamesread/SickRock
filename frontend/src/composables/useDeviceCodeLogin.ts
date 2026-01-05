// Device code login functionality - kept for future use
// This code was extracted from LoginForm.vue when porting to picocrank Login component

import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { createApiClient } from '../stores/api'
import { formatUnixTimestamp } from '../utils/dateFormatting'

export function useDeviceCodeLogin() {
  const router = useRouter()
  const authStore = useAuthStore()
  const apiClient = createApiClient()

  const deviceCode = ref('')
  const deviceCodeExpiresAt = ref(0)
  const deviceCodePollingInterval = ref<number | null>(null)

  const stopDeviceCodePolling = () => {
    if (deviceCodePollingInterval.value !== null) {
      clearInterval(deviceCodePollingInterval.value)
      deviceCodePollingInterval.value = null
    }
  }

  const checkDeviceCodeStatus = async () => {
    if (!deviceCode.value) return

    try {
      const response = await apiClient.checkDeviceCode({ code: deviceCode.value })

      if (response.claimed && response.token) {
        // Device code was claimed and we have the session information
        localStorage.setItem('session-token', response.token)
        const userData = {
          username: response.username,
          token: response.token,
          expiresAt: Number(response.expiresAt)
        }
        authStore.user = userData

        // Stop polling since login was successful
        stopDeviceCodePolling()

        const redirectParam = (router.currentRoute.value.query.redirect as string) || '/'
        router.replace(redirectParam)
        return true
      }
      return false
    } catch (err) {
      console.error('Failed to check device code status:', err)
      throw err
    }
  }

  const startDeviceCodePolling = () => {
    // Clear any existing polling
    stopDeviceCodePolling()

    deviceCodePollingInterval.value = setInterval(() => {
      if (deviceCode.value) {
        checkDeviceCodeStatus()
      } else {
        stopDeviceCodePolling()
      }
    }, 2000) // Check every 2 seconds

    // Stop polling after 10 minutes
    setTimeout(() => {
      stopDeviceCodePolling()
      if (deviceCode.value) {
        deviceCode.value = ''
      }
    }, 10 * 60 * 1000)
  }

  const generateDeviceCode = async () => {
    try {
      // Stop any existing polling before generating new code
      stopDeviceCodePolling()

      const response = await apiClient.generateDeviceCode({})
      deviceCode.value = response.code
      deviceCodeExpiresAt.value = Number(response.expiresAt)

      // Start polling for status
      startDeviceCodePolling()
    } catch (err) {
      console.error('Failed to generate device code:', err)
      throw err
    }
  }

  const formatTime = (timestamp: number) => {
    return formatUnixTimestamp(timestamp)
  }

  return {
    deviceCode,
    deviceCodeExpiresAt,
    generateDeviceCode,
    checkDeviceCodeStatus,
    startDeviceCodePolling,
    stopDeviceCodePolling,
    formatTime
  }
}
