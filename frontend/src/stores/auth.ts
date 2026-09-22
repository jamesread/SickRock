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
  rbacPermissions: string[]
  rbacIsSuperuser: boolean
}

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const isLoading = ref(false)
  const error = ref<string | null>(null)
  const initResponse = ref<InitResponse | null>(null)

  const isAuthenticated = computed(() => {
    if (!user.value) return false
    if (user.value.expiresAt > 0 && user.value.expiresAt < Date.now() / 1000) {
      user.value = null
      localStorage.removeItem(SESSION_TOKEN_KEY)
      localStorage.removeItem(LEGACY_TOKEN_KEY)
      return false
    }
    return true
  })

  const hasPermission = (permission: string) => {
    if (!user.value) return false
    if (user.value.rbacIsSuperuser) return true
    return user.value.rbacPermissions.includes(permission)
  }

  const setUserFromInit = (response: InitResponse, token?: string) => {
    if (!response.currentUsername) {
      user.value = null
      return
    }
    const storedToken = token ?? localStorage.getItem(SESSION_TOKEN_KEY) ?? ''
    user.value = {
      username: response.currentUsername,
      token: storedToken,
      expiresAt: 0,
      rbacPermissions: [...(response.rbacPermissions ?? [])],
      rbacIsSuperuser: response.rbacIsSuperuser ?? false,
    }
  }

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
          expiresAt: Number(response.expiresAt),
          rbacPermissions: [],
          rbacIsSuperuser: false,
        }
        user.value = userData
        localStorage.setItem(SESSION_TOKEN_KEY, response.token)
        localStorage.removeItem(LEGACY_TOKEN_KEY)

        const validated = await validateToken()
        return validated
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
      const client = createApiClient()
      await client.logout({})
    } catch (err) {
      console.error('Logout error:', err)
    } finally {
      user.value = null
      initResponse.value = null
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
      }
    }

    try {
      const client = createApiClient()
      const response = await client.validateToken({ token: token ?? '' })

      if (response.valid) {
        user.value = {
          username: response.username,
          token: token ?? '',
          expiresAt: Number(response.expiresAt),
          rbacPermissions: [...(response.rbacPermissions ?? [])],
          rbacIsSuperuser: response.rbacIsSuperuser ?? false,
        }
        if (token) {
          localStorage.setItem(SESSION_TOKEN_KEY, token)
        }
        return true
      }

      localStorage.removeItem(SESSION_TOKEN_KEY)
      localStorage.removeItem(LEGACY_TOKEN_KEY)
      user.value = null
      return false
    } catch (err) {
      console.error('Token validation error:', err)
      localStorage.removeItem(SESSION_TOKEN_KEY)
      localStorage.removeItem(LEGACY_TOKEN_KEY)
      user.value = null
      return false
    }
  }

  const getAuthHeaders = () => {
    const headers: Record<string, string> = {}
    if (user.value?.token) {
      headers['Authorization'] = `Bearer ${user.value.token}`
      headers['Session-Token'] = user.value.token
    }
    return headers
  }

  const setInitResponse = (response: InitResponse) => {
    initResponse.value = response
    if (response.currentUsername) {
      setUserFromInit(response)
    }
  }

  return {
    user,
    isLoading,
    error,
    isAuthenticated,
    initResponse,
    hasPermission,
    setUserFromInit,
    login,
    logout,
    validateToken,
    getAuthHeaders,
    setInitResponse
  }
})
