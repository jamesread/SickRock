<template>
  <section class="login-section">
    <h2>
      <img :src="logo" alt="SickRock" class="logo icon" width="32" style="display: inline-block; vertical-align: sub;"/>
      SickRock
    </h2>

    <Login
      ref="loginFormRef"
      :oauth-providers="oauthProviders"
      :show-default-tabs="false"
      :custom-tabs="loginTabs"
      @local-login="handleLocalLogin"
      @oauth-login="handleOAuthLogin"
      @tab-change="onTabChange"
    >
      <template #tab-device-code>
        <DeviceCodeLogin />
      </template>
    </Login>
  </section>
</template>

<script setup lang="ts">
import { ref, watch, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import Login from 'picocrank/vue/components/Login.vue'
import DeviceCodeLogin from './DeviceCodeLogin.vue'
import logo from '../resources/images/logo.png'
import { createApiClient } from '../stores/api'

const router = useRouter()
const authStore = useAuthStore()
const loginFormRef = ref<InstanceType<typeof Login> | null>(null)
const client = createApiClient()

const oauthProviders = computed(() => {
  const list = authStore.initResponse?.oauthProviders ?? []
  return list.map(p => ({
    id: p.id,
    name: p.label || p.id,
    authUrl: p.authUrl,
    class: 'neutral',
  }))
})

const loginTabs = computed(() => {
  const tabs: Array<{ id: string; label: string }> = [
    { id: 'local', label: 'Username & Password' },
  ]
  if (oauthProviders.value.length > 0) {
    tabs.push({ id: 'oauth', label: 'OAuth2' })
  }
  tabs.push({ id: 'device-code', label: 'Device Code' })
  return tabs
})

onMounted(async () => {
  if (!authStore.initResponse) {
    try {
      const response = await client.init({})
      authStore.setInitResponse(response)
    } catch (error) {
      console.warn('Failed to load Init for OAuth providers:', error)
    }
  }
})

// Redirect to home if already authenticated (runs whenever authentication state changes)
watch(() => authStore.isAuthenticated, (isAuthenticated) => {
  if (isAuthenticated) {
    router.push('/')
  }
}, { immediate: true })

const handleLocalLogin = async (credentials: { username: string; password: string }) => {
  if (!loginFormRef.value) return

  loginFormRef.value.setLocalLoginError('')

  try {
    const success = await authStore.login(credentials.username, credentials.password)
    if (success) {
      const redirectParam = (router.currentRoute.value.query.redirect as string) || '/'
      router.replace(redirectParam)
    } else {
      const errorMessage = authStore.error || 'Login failed. Please check your credentials.'
      loginFormRef.value.setLocalLoginError(errorMessage)
    }
  } catch (error) {
    console.error('Login error:', error)
    const errorMessage = error instanceof Error ? error.message : 'An unexpected error occurred during login.'
    loginFormRef.value.setLocalLoginError(errorMessage)
  }
}

const handleOAuthLogin = (provider: { authUrl?: string; auth_url?: string }) => {
  const url = provider.authUrl || provider.auth_url
  if (!url) {
    return
  }
  const redirect = (router.currentRoute.value.query.redirect as string) || '/'
  try {
    const target = new URL(url, window.location.origin)
    if (redirect && redirect !== '/') {
      target.searchParams.set('redirect', redirect)
    }
    window.location.href = target.toString()
  } catch {
    window.location.href = url
  }
}

const onTabChange = () => {
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
