<template>
  <section>

      <h2>
        <img :src="logo" alt="SickRock" class="logo icon" width = "32" style = "display: inline-block; vertical-align: sub;"/>
        SickRock
      </h2>

      <div class="login-tabs">
        <button
          @click="loginMode = 'password'"
          :class="{ active: loginMode === 'password' }"
          class="tab-button"
        >
          Username & Password
        </button>
        <button
          @click="loginMode = 'device'"
          :class="{ active: loginMode === 'device' }"
          class="tab-button"
        >
          Device Code
        </button>
      </div>

      <form @submit.prevent="handleLogin" class="login-form">
        <!-- Username/Password Mode -->
        <div v-if="loginMode === 'password'" class = "username-password-mode">
          <input
            id="username"
            v-model="username"
            type="text"
            required
            :disabled="isLoading"
            placeholder="Enter username"
          />

          <input
            id="password"
            v-model="password"
            type="password"
            required
            :disabled="isLoading"
            placeholder="Enter password"
          />

          <button
            type="submit"
            :disabled="isLoading || !username || !password"
            class="login-button"
          >
            <span v-if="isLoading">Signing in...</span>
            <span v-else>Login</span>
        </button>

        </div>

        <!-- Device Code Mode -->
        <div v-if="loginMode === 'device'">
          <div v-if="!deviceCode" class="device-code-section">
            <p>Generating your 4-digit device code...</p>
            <div class="loading-spinner"></div>
          </div>

          <div v-if="deviceCode" class="device-code-display">
            <p>Your device code is:</p>
            <div class="code-display">{{ deviceCode }}</div>
            <p class="code-instructions">
              Enter this code in another logged-in session to authenticate this device.
            </p>
            <p class="code-expires">
              Code expires at: {{ formatTime(deviceCodeExpiresAt) }}
            </p>
            <div class="button-group">
              <button
                type="button"
                @click="checkDeviceCodeStatus"
                :disabled="isLoading"
                class="good"
              >
                Check Status
              </button>
              <button
                type="button"
                @click="generateDeviceCode"
                :disabled="isLoading"
                class="good"
              >
                Generate New Code
              </button>
            </div>
          </div>
        </div>

        <div v-if="error" class="error-message">
          {{ error }}
        </div>

      </form>
    </section>
</template>

<script setup lang="ts">
import { ref, computed, watch, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { createApiClient } from '../stores/api'
import logo from '../resources/images/logo.png'
import { formatUnixTimestamp } from '../utils/dateFormatting'

const router = useRouter()
const authStore = useAuthStore()
const apiClient = createApiClient()

const username = ref('')
const password = ref('')
const loginMode = ref<'password' | 'device'>('password')
const deviceCode = ref('')
const deviceCodeExpiresAt = ref(0)
const deviceCodePollingInterval = ref<number | null>(null)
const isLoading = computed(() => authStore.isLoading)
const error = computed(() => authStore.error)

// Redirect to home if already authenticated (runs whenever authentication state changes)
watch(() => authStore.isAuthenticated, (isAuthenticated) => {
  if (isAuthenticated) {
    router.push('/')
  }
}, { immediate: true })

// Auto-generate device code when switching to device code tab
watch(loginMode, (newMode) => {
  if (newMode === 'device' && !deviceCode.value) {
    generateDeviceCode()
  } else if (newMode === 'password') {
    // Clear device code and stop polling when switching back to password mode
    stopDeviceCodePolling()
    deviceCode.value = ''
    deviceCodeExpiresAt.value = 0
  }
})

const handleLogin = async () => {
  const success = await authStore.login(username.value, password.value)
  if (success) {
    const redirectParam = (router.currentRoute.value.query.redirect as string) || '/'
    router.replace(redirectParam)
  }
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
    }
  } catch (err) {
    console.error('Failed to check device code status:', err)
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

const stopDeviceCodePolling = () => {
  if (deviceCodePollingInterval.value !== null) {
    clearInterval(deviceCodePollingInterval.value)
    deviceCodePollingInterval.value = null
  }
}

const formatTime = (timestamp: number) => {
  return formatUnixTimestamp(timestamp)
}

// Clean up polling when component is unmounted
onUnmounted(() => {
  stopDeviceCodePolling()
})
</script>

<style scoped>
.username-password-mode {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

h2 {
  text-align: center;
  padding-bottom: 20px;
  font-size: 1.5em;
}

section {
  max-width: 400px;
  margin: 3em auto;
}

.login-tabs {
  display: flex;
  margin-bottom: 20px;
  border-bottom: 1px solid #ddd;
}

.tab-button {
  flex: 1;
  padding: 10px;
  border: none;
  background: none;
  cursor: pointer;
  border-bottom: 2px solid transparent;
  transition: all 0.3s ease;
}

.tab-button.active {
  border-bottom-color: #007bff;
  color: #007bff;
  font-weight: bold;
}

.tab-button:hover {
  background-color: #f8f9fa;
}

.device-code-section {
  text-align: center;
  padding: 20px 0;
}

.loading-spinner {
  width: 40px;
  height: 40px;
  border: 4px solid #f3f3f3;
  border-top: 4px solid #007bff;
  border-radius: 50%;
  animation: spin 1s linear infinite;
  margin: 20px auto;
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

.device-code-display {
  text-align: center;
  padding: 20px 0;
}

.code-display {
  font-size: 2em;
  font-weight: bold;
  color: #007bff;
  background-color: #f8f9fa;
  padding: 15px;
  margin: 15px 0;
  border-radius: 8px;
  letter-spacing: 2px;
  font-family: monospace;
}

.code-instructions {
  color: #666;
  margin: 15px 0;
  font-size: 0.9em;
}

.code-expires {
  color: #888;
  font-size: 0.8em;
  margin: 10px 0;
}

.button-group {
  display: flex;
  gap: 10px;
  justify-content: center;
  margin: 20px 0;
}

button:disabled {
  background: #ccc;
  cursor: not-allowed;
  opacity: 0.6;
}

.login-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
}

.login-card {
  background: white;
  border-radius: 12px;
  box-shadow: 0 20px 40px rgba(0, 0, 0, 0.1);
  padding: 40px;
  width: 100%;
  max-width: 400px;
}

.login-form {
  display: grid;
  grid-template-columns: 1fr;
  gap: 20px;
}

.error-message {
  background-color: #fee;
  color: #c33;
  padding: 12px 16px;
  border-radius: 8px;
  border: 1px solid #fcc;
  font-size: 0.9rem;
}

</style>
