<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { useAuthStore } from '../stores/auth'
import { useRouter } from 'vue-router'
import Section from 'picocrank/vue/components/Section.vue'
import Navigation from 'picocrank/vue/components/Navigation.vue'
import NavigationGrid from 'picocrank/vue/components/NavigationGrid.vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { UserIcon, BookmarkIcon, SettingsIcon, KeyIcon, NotificationIcon, Download01Icon, LogoutIcon } from '@hugeicons/core-free-icons'

const authStore = useAuthStore()
const router = useRouter()
const user = computed(() => authStore.user)
const localNavigation = ref(null)

async function handleLogout() {
  await authStore.logout()
  router.push('/login')
}

onMounted(() => {
  if (localNavigation.value) {
    // User Preferences
    localNavigation.value.addNavigationLink({
      id: 'user-preferences',
      name: 'user-preferences',
      title: 'User Preferences',
      path: '/user-preferences',
      icon: SettingsIcon,
      type: 'route',
      description: 'Manage your account settings, theme, language, and preferences'
    })

    // Bookmarks
    localNavigation.value.addNavigationLink({
      id: 'user-bookmarks',
      name: 'user-bookmarks',
      title: 'Bookmarks',
      path: '/user-bookmarks',
      icon: BookmarkIcon,
      type: 'route',
      description: 'View and manage your saved bookmarks'
    })

    // API Keys
    localNavigation.value.addNavigationLink({
      id: 'user-api-keys',
      name: 'user-api-keys',
      title: 'API Keys',
      path: '/user-api-keys',
      icon: KeyIcon,
      type: 'route',
      description: 'Create and manage your API keys for programmatic access'
    })

    // Notifications
    localNavigation.value.addNavigationLink({
      id: 'user-notifications',
      name: 'user-notifications',
      title: 'Notifications',
      path: '/user-notifications',
      icon: NotificationIcon,
      type: 'route',
      description: 'Configure notification channels and event subscriptions'
    })

    // PWA & Service Worker
    localNavigation.value.addNavigationLink({
      id: 'pwa-installation',
      name: 'pwa-installation',
      title: 'PWA & Service Worker',
      path: '/admin/pwa-installation',
      icon: Download01Icon,
      type: 'route',
      description: 'Manage Progressive Web App installation and service worker'
    })

    // Device Code Claimer
    localNavigation.value.addNavigationLink({
      id: 'device-code-claimer',
      name: 'device-code-claimer',
      title: 'Device Code Claimer',
      path: '/device-code-claimer',
      icon: KeyIcon,
      type: 'route',
      description: 'Complete device code authentication'
    })

    // Logout
    localNavigation.value.addCallback('Logout', async () => { await handleLogout() }, { icon: LogoutIcon })
  }
})
</script>

<template>
  <Section title="User Control Panel" subtitle="Manage your account settings and preferences">
    <div class="control-panel-container">
      <div class="user-welcome">
        <h2>Welcome, {{ user?.username }}</h2>
        <p class="welcome-message">Manage your account settings and preferences</p>
      </div>

      <Navigation ref="localNavigation">
        <NavigationGrid />
      </Navigation>

      <div class="logout-section">
        <button @click="handleLogout" class="logout-button">
          <HugeiconsIcon :icon="LogoutIcon" />
          <span>Logout</span>
        </button>
      </div>
    </div>
  </Section>
</template>

<style scoped>
.control-panel-container {
  max-width: 1200px;
  margin: 0 auto;
  padding: 2rem 1rem;
}

.user-welcome {
  margin-bottom: 2rem;
  text-align: center;
}

.user-welcome h2 {
  margin: 0 0 0.5rem 0;
  color: #212529;
  font-size: 1.75rem;
  font-weight: 600;
}

.welcome-message {
  margin: 0;
  color: #6c757d;
  font-size: 1rem;
}

.logout-section {
  margin-top: 2rem;
  padding-top: 2rem;
  border-top: 1px solid rgba(148, 163, 184, 0.3);
  display: flex;
  justify-content: center;
}

.logout-button {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.75rem 1.5rem;
  background: #dc2626;
  color: white;
  border: none;
  border-radius: 6px;
  font-size: 1rem;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
}

.logout-button:hover {
  background: #b91c1c;
  transform: translateY(-1px);
  box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
}

.logout-button:active {
  transform: translateY(0);
}

.logout-button svg {
  width: 20px;
  height: 20px;
}

/* Responsive design */
@media (max-width: 768px) {
  .control-panel-container {
    padding: 1rem;
  }

  .user-welcome h2 {
    font-size: 1.5rem;
  }
}
</style>
