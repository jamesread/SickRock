<script setup lang="ts">
import { computed, ref, onMounted, nextTick } from 'vue'
import { useAuthStore } from '../stores/auth'
import { useRouter } from 'vue-router'
import { useRbac } from '../composables/useRbac'
import Section from 'picocrank/vue/components/Section.vue'
import Navigation from 'picocrank/vue/components/Navigation.vue'
import NavigationGrid from 'picocrank/vue/components/NavigationGrid.vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { UserIcon, BookmarkIcon, SettingsIcon, KeyIcon, NotificationIcon, Download01Icon, LogoutIcon, ShieldKeyIcon } from '@hugeicons/core-free-icons'

const authStore = useAuthStore()
const router = useRouter()
const { hasPermission } = useRbac()
const user = computed(() => authStore.user)
const localNavigation = ref(null)

async function handleLogout() {
  await authStore.logout()
  router.push('/login')
}

// Filter out any null/undefined links so picocrank's isActive(link) never receives undefined (avoids "reading 'path' of undefined")
function onlyValidLinks(link: unknown) {
  return link != null && typeof link === 'object'
}

onMounted(() => {
  nextTick(() => {
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

    if (hasPermission('apikeys.use')) {
      localNavigation.value.addNavigationLink({
        id: 'user-api-keys',
        name: 'user-api-keys',
        title: 'API Keys',
        path: '/user-api-keys',
        icon: KeyIcon,
        type: 'route',
        description: 'Create and manage your API keys for programmatic access',
      })
    }

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

    // My Permissions
    localNavigation.value.addNavigationLink({
      id: 'my-permissions',
      name: 'my-permissions',
      title: 'My Permissions',
      path: '/my-permissions',
      icon: ShieldKeyIcon,
      type: 'route',
      description: 'Review your group membership and effective permissions',
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

    if (hasPermission('devicecode.claim')) {
      localNavigation.value.addNavigationLink({
        id: 'device-code-claimer',
        name: 'device-code-claimer',
        title: 'Device Code Claimer',
        path: '/device-code-claimer',
        icon: KeyIcon,
        type: 'route',
        description: 'Complete device code authentication',
      })
    }
  }
  })
})
</script>

<template>
  <Section
    title="User Control Panel"
    subtitle="Manage your account settings and preferences"
    :icon="UserIcon"
  >
    <div class="control-panel-container">
      <div class="user-welcome">
        <h2>Welcome, {{ user?.username }}</h2>
        <p class="subtle welcome-message">Manage your account settings and preferences</p>
      </div>

      <Navigation ref="localNavigation">
        <NavigationGrid :filter="onlyValidLinks" />
      </Navigation>

      <div class="logout-section">
        <button type="button" class="inline-icon bad" @click="handleLogout">
          <HugeiconsIcon :icon="LogoutIcon" width="1em" height="1em" />
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
  color: var(--text-color, inherit);
  font-size: 1.75rem;
  font-weight: 600;
}

.welcome-message {
  margin: 0;
}

.logout-section {
  margin-top: 2rem;
  padding-top: 2rem;
  border-top: 1px solid var(--border-color, currentColor);
  display: flex;
  justify-content: center;
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
