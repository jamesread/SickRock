<template>
  <section class="login-section">
    <h2>
      <img :src="logo" alt="SickRock" class="logo icon" width="32" style="display: inline-block; vertical-align: sub;"/>
      SickRock
    </h2>

    <Login
      ref="loginFormRef"
      :oauth-providers="[]"
      :show-default-tabs="false"
      :custom-tabs="[
        { id: 'local', label: 'Username & Password' },
        { id: 'device-code', label: 'Device Code' }
      ]"
      @local-login="handleLocalLogin"
      @tab-change="onTabChange"
    >
      <template #tab-device-code>
        <DeviceCodeLogin />
      </template>
    </Login>
  </section>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import Login from 'picocrank/vue/components/Login.vue'
import DeviceCodeLogin from './DeviceCodeLogin.vue'
import logo from '../resources/images/logo.png'

const router = useRouter()
const authStore = useAuthStore()
const loginFormRef = ref<InstanceType<typeof Login> | null>(null)

// Redirect to home if already authenticated (runs whenever authentication state changes)
watch(() => authStore.isAuthenticated, (isAuthenticated) => {
  if (isAuthenticated) {
    router.push('/')
  }
}, { immediate: true })

const handleLocalLogin = async (credentials: { username: string; password: string }) => {
  if (!loginFormRef.value) return

  // Clear any previous error
  loginFormRef.value.setLocalLoginError('')

  try {
    const success = await authStore.login(credentials.username, credentials.password)
    if (success) {
      const redirectParam = (router.currentRoute.value.query.redirect as string) || '/'
      router.replace(redirectParam)
    } else {
      // Set error message from auth store
      const errorMessage = authStore.error || 'Login failed. Please check your credentials.'
      loginFormRef.value.setLocalLoginError(errorMessage)
    }
  } catch (error) {
    console.error('Login error:', error)
    const errorMessage = error instanceof Error ? error.message : 'An unexpected error occurred during login.'
    loginFormRef.value.setLocalLoginError(errorMessage)
  }
}

const onTabChange = (tab: any, tabId: string) => {
  // Clear errors when switching tabs
  if (loginFormRef.value) {
    loginFormRef.value.setLocalLoginError('')
  }
}
</script>

<style scoped>
.login-section {
  max-width: 500px;
  margin: 3em auto;
  padding: 0 1rem;
}

h2 {
  text-align: center;
  margin-top: 1.5em;
  padding-bottom: 20px;
  font-size: 1.5em;
}

.logo {
  margin-right: 0.5rem;
}
</style>
