import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { createApiClient } from './api'

export interface User {
  username: string
  token: string
  expiresAt: number
}

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const isLoading = ref(false)
  const error = ref<string | null>(null)

  const isAuthenticated = computed(() => {
    if (!user.value) return false
    if (user.value.expiresAt < Date.now() / 1000) {
      // Token expired
      user.value = null
      localStorage.removeItem('auth_token')
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
          expiresAt: response.expiresAt
        }
        user.value = userData
        localStorage.setItem('auth_token', response.token)
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
      localStorage.removeItem('auth_token')
    }
  }

  const validateToken = async () => {
    const token = localStorage.getItem('auth_token')
    if (!token) return false

    try {
      const client = createApiClient()
      const response = await client.validateToken({ token })
      
      if (response.valid) {
        user.value = {
          username: response.username,
          token,
          expiresAt: response.expiresAt
        }
        return true
      } else {
        localStorage.removeItem('auth_token')
        return false
      }
    } catch (err) {
      console.error('Token validation error:', err)
      localStorage.removeItem('auth_token')
      return false
    }
  }

  const getAuthHeaders = () => {
    if (!user.value) return {}
    return {
      'Authorization': `Bearer ${user.value.token}`
    }
  }

  return {
    user,
    isLoading,
    error,
    isAuthenticated,
    login,
    logout,
    validateToken,
    getAuthHeaders
  }
})
