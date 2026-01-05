import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { createApiClient } from './api'
import type { InitResponse } from '../gen/sickrock_pb'

const SESSION_TOKEN_KEY = 'session-token'
const LEGACY_TOKEN_KEY = 'auth_token'

export interface User {
  username: string
  token: string
  expiresAt: number
}

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const isLoading = ref(false)
  const error = ref<string | null>(null)
  const initResponse = ref<InitResponse | null>(null)

  const isAuthenticated = computed(() => {
    if (!user.value) return false
    if (user.value.expiresAt < Date.now() / 1000) {
      // Token expired
      user.value = null
      localStorage.removeItem(SESSION_TOKEN_KEY)
      localStorage.removeItem(LEGACY_TOKEN_KEY)
      return false
    }
    return true
  })

  const login = async (username: string, password: string) => {
    isLoading.value = true
    error.value = null

    try {
      const client = createApiClient()
      const response = await client.login({
        username,
        password
      })

      if (response.success) {
        const userData: User = {
          username,
          token: response.token,
          expiresAt: Number(response.expiresAt)
        }
        user.value = userData
        localStorage.setItem(SESSION_TOKEN_KEY, response.token)
        localStorage.removeItem(LEGACY_TOKEN_KEY)
        return true
      } else {
        error.value = response.message || 'Login failed'
        return false
      }
    } catch (err) {
      error.value = 'Network error during login'
      console.error('Login error:', err)
      return false
    } finally {
      isLoading.value = false
    }
  }

  const logout = async () => {
    try {
      if (user.value) {
        const client = createApiClient()
        await client.logout({})
      }
    } catch (err) {
      console.error('Logout error:', err)
    } finally {
      user.value = null
      initResponse.value = null // Clear init response on logout
      localStorage.removeItem(SESSION_TOKEN_KEY)
      localStorage.removeItem(LEGACY_TOKEN_KEY)
    }
  }

  const validateToken = async () => {
    let token = localStorage.getItem(SESSION_TOKEN_KEY)
    if (!token) {
      token = localStorage.getItem(LEGACY_TOKEN_KEY)
      if (token) {
        localStorage.setItem(SESSION_TOKEN_KEY, token)
        localStorage.removeItem(LEGACY_TOKEN_KEY)
      } else {
        return false
      }
    }

    try {
      const client = createApiClient()
      const response = await client.validateToken({ token })

      if (response.valid) {
        user.value = {
          username: response.username,
          token,
          expiresAt: Number(response.expiresAt)
        }
        return true
      } else {
        localStorage.removeItem(SESSION_TOKEN_KEY)
        localStorage.removeItem(LEGACY_TOKEN_KEY)
        return false
      }
    } catch (err) {
      console.error('Token validation error:', err)
      localStorage.removeItem(SESSION_TOKEN_KEY)
      localStorage.removeItem(LEGACY_TOKEN_KEY)
      return false
    }
  }

  const getAuthHeaders = () => {
    if (!user.value) return {}
    return {
      'Authorization': `Bearer ${user.value.token}`,
      'Session-Token': user.value.token
    }
  }

  const setInitResponse = (response: InitResponse) => {
    initResponse.value = response
  }

  return {
    user,
    isLoading,
    error,
    isAuthenticated,
    initResponse,
    login,
    logout,
    validateToken,
    getAuthHeaders,
    setInitResponse
  }
})
